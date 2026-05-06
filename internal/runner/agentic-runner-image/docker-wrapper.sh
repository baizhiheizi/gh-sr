#!/bin/bash
# gh-sr docker CLI shim (installed as /opt/gh-sr/docker-shim/docker in the container runner image).
#
# This is NOT the mechanism that isolates multiple agentic runners on one host — use
# runner_mode: container for that (each outer runner container gets its own inner dockerd,
# network namespace, MCP port 80, and /tmp/gh-aw). This script is a gh-aw compatibility
# layer for jobs running inside those containers: MCP gateway self-inspect bypass, gateway
# container cleanup, and AWF agent gateway routing (see below).
#
# Makes gh-aw-mcpg's self-inspect validation block a no-op by forcing a
# non-hex container hostname.
#
# Background
# ----------
# gh-aw's launcher runs the MCP Gateway as:
#
#     docker run -i --rm --network host ... ghcr.io/github/gh-aw-mcpg:<tag>
#
# Inside the container, /app/run_containerized.sh detects CONTAINER_ID from
# /proc/self/cgroup or falls back to `hostname`, then runs:
#
#     if [ -n "$CONTAINER_ID" ]; then
#         validate_port_mapping "$CONTAINER_ID"       # needs port in NetworkSettings.Ports
#         validate_stdin_interactive "$CONTAINER_ID"  # needs Config.OpenStdin == true
#         ...
#     fi
#
# Under `--network host` the Ports map is empty, so validate_port_mapping
# fails. validate_stdin_interactive is also flaky in DinD. Both kill the
# gateway with exit 1.
#
# The upstream script already has a clean escape hatch: if CONTAINER_ID
# cannot be determined, the whole block is skipped. On cgroup v2 hosts
# /proc/self/cgroup yields no hex string, so detection falls back to the
# container hostname. We override --hostname to a non-hex value so the
# hostname fallback also fails, CONTAINER_ID stays empty, and the gateway
# starts through its intended "could not determine container ID" code path.
#
# For `docker run` (not `create`), we also inject a stable name and cidfile
# and do not `exec` the real docker binary: gh-aw's stop_mcp_gateway.sh tracks
# the PID of this wrapper process. If upstream falls back to signalling that
# PID after POST /close, the wrapper removes the gateway container so port 80 is
# released for later jobs.
#
# This wrapper intercepts `docker run|create ... ghcr.io/github/gh-aw-mcpg:* ...`.
# It injects `--hostname gh-aw-mcpg` only when missing (create keeps exec
# passthrough; run uses the supervisor path above regardless of whether hostname
# was already supplied).
#
# For `docker run ... ghcr.io/github/gh-aw-mcpg:* ...`, it leaves gh-aw's
# piped gateway JSON schema-valid, then may rewrite generated Claude MCP config
# URLs from `http://host.docker.internal:<port>/mcp/` to the same port on a
# concrete IPv4 target when one can be determined (see `resolve_awf_host_route_target`).
# That avoids intermittent HTTP client/proxy routing to `host.docker.internal` in
# nested-Docker AWF setups. When a rewrite runs, diagnostics go to stderr prefixed
# with `[gh-sr:mcp-claude-urls]` (visible in Actions job logs). If only Docker
# `host-gateway` is available, URL rewrite is skipped (URLs stay schema-valid).
#
# It also intercepts `docker run|create ... ghcr.io/github/gh-aw-firewall/agent:* ...`
# and injects `--add-host=host.docker.internal:<target>` when the caller did not
# already add an explicit `host.docker.internal` host entry. Target resolution:
# `GH_SR_AWF_BRIDGE_GATEWAY_IP` (explicit) → first non-loopback IPv4 from
# `host.docker.internal` resolution in this namespace → Docker `host-gateway`.
#
# All other `docker` invocations are passed through untouched.
#
# Tests may set GH_SR_DOCKER_WRAPPER_REAL to a program that records argv (e.g.
# /bin/echo) instead of invoking the real docker daemon.
#
# Optional: set GH_SR_MCP_REWRITE_TARGET_IP before starting the gateway to pin Claude MCP
# URL rewrites without re-resolving (tests and advanced setups). Optional:
# GH_SR_MCP_REWRITE_PORT pins the rewrite to a single port; when unset the
# watcher rewrites `host.docker.internal:<any-port>/mcp/` URLs and preserves
# the matched port (gh-aw uses 80 by default but 8080 in many compiled
# workflows via `MCP_GATEWAY_PORT`).

# gh-aw / start_mcp_gateway: expect `docker run -i --rm --network host ...`; stdin is the
# gateway JSON. stop_mcp_gateway.sh POSTs /close then signals the docker client PID.
real="${GH_SR_DOCKER_WRAPPER_REAL:-/usr/bin/docker}"

# Resolve a concrete IPv4 for AWF host-access and Claude MCP URL rewrites, or emit
# literal `HOST_GATEWAY` when only Docker host-gateway mapping is appropriate.
# Order: explicit GH_SR_AWF_BRIDGE_GATEWAY_IP → non-loopback host.docker.internal → host-gateway.
resolve_awf_host_route_target() {
    if [[ -n "${GH_SR_AWF_BRIDGE_GATEWAY_IP:-}" ]]; then
        printf '%s\n' "$GH_SR_AWF_BRIDGE_GATEWAY_IP"
        return 0
    fi
    local ip=""
    ip=$(getent hosts host.docker.internal 2>/dev/null | awk '{print $1; exit}')
    if [[ -n "$ip" && "$ip" != "127.0.0.1" && "$ip" != "::1" ]]; then
        printf '%s\n' "$ip"
        return 0
    fi
    printf 'HOST_GATEWAY\n'
}

docker_option_value() {
    local option="$1"
    shift
    local prev=false
    local a
    for a in "$@"; do
        if [[ "$prev" == true ]]; then
            printf '%s\n' "$a"
            return 0
        fi
        if [[ "$a" == "$option" ]]; then
            prev=true
            continue
        fi
        case "$a" in
            "$option"=*)
                printf '%s\n' "${a#*=}"
                return 0
                ;;
        esac
    done
    return 1
}

is_mcpg_invocation() {
    local sub="${1:-}"
    case "$sub" in
        run | create) ;;
        *) return 1 ;;
    esac

    local arg
    for arg in "$@"; do
        case "$arg" in
            ghcr.io/github/gh-aw-mcpg:*) return 0 ;;
        esac
    done

    return 1
}

has_hostname_arg() {
    local prev_hostname=false
    local arg
    for arg in "$@"; do
        if [[ "$prev_hostname" == true ]]; then
            return 0
        fi
        case "$arg" in
            --hostname | -h)
                prev_hostname=true
                ;;
            --hostname=* | -h=*)
                return 0
                ;;
        esac
    done
    return 1
}

has_detach_arg() {
    local arg
    for arg in "$@"; do
        case "$arg" in
            ghcr.io/github/gh-aw-mcpg:*)
                return 1
                ;;
            --detach | --detach=true | -d)
                return 0
                ;;
            --detach=false)
                ;;
            --*)
                ;;
        esac
    done
    return 1
}

rewrite_claude_mcp_gateway_urls() {
    local config_path="${1:-/tmp/gh-aw/mcp-config/mcp-servers.json}"
    local target="${GH_SR_MCP_REWRITE_TARGET_IP:-}"
    # Port pattern for matching `host.docker.internal:<port>/mcp/` URLs in the
    # Claude MCP config. When GH_SR_MCP_REWRITE_PORT is set, only that exact
    # port is rewritten (backward-compat). Otherwise we match any numeric port:
    # gh-aw's converter writes whatever port the gateway listens on (configured
    # via `sandbox.mcp.port` / `MCP_GATEWAY_PORT`, default 80 but commonly
    # 8080), so a hardcoded port leaves URLs un-rewritten in those workflows
    # and the agent falls back to `host.docker.internal:<port>` which the AWF
    # Squid sidecar proxies (and rejects) instead of bypassing.
    local port_pattern="${GH_SR_MCP_REWRITE_PORT:-[0-9]+}"
    local previous_sig=""
    local current_sig=""
    local i
    local saw_file=false

    if [[ -z "$target" || "$target" == "HOST_GATEWAY" ]]; then
        echo "[gh-sr:mcp-claude-urls] watcher_skip_no_concrete_target path=${config_path} target=${target:-empty}" >&2
        return 0
    fi

    local ip_esc
    ip_esc=$(printf '%s' "$target" | sed 's/\./\\./g')

    on_rewrite_watcher_signal() {
        echo "[gh-sr:mcp-claude-urls] watcher_stop signal path=${config_path}" >&2
        trap - TERM INT
        exit 0
    }
    trap 'on_rewrite_watcher_signal' TERM INT

    echo "[gh-sr:mcp-claude-urls] watcher_start path=${config_path} max_iterations=120 port_pattern=${port_pattern} target=${target}" >&2

    for i in $(seq 1 120); do
        if [[ -f "$config_path" ]]; then
            if [[ "$saw_file" == false ]]; then
                echo "[gh-sr:mcp-claude-urls] config_appeared size_bytes=$(stat -c '%s' "$config_path" 2>/dev/null || echo '?')" >&2
                saw_file=true
            fi
            current_sig=$(stat -c '%s:%Y' "$config_path" 2>/dev/null || true)
            if [[ -n "$current_sig" && "$current_sig" == "$previous_sig" ]]; then
                if grep -qE "http://host\\.docker\\.internal:${port_pattern}/mcp/" "$config_path" 2>/dev/null; then
                    local hb ha br
                    hb=$(grep -cE "http://host\\.docker\\.internal:${port_pattern}/mcp/" "$config_path" 2>/dev/null || true)
                    hb=${hb:-0}
                    echo "[gh-sr:mcp-claude-urls] stable_file iteration=${i} sig=${current_sig} host_mcp_url_hits=${hb} applying_rewrite" >&2
                    # Capture the matched port and reuse it on the right-hand
                    # side so a single watcher rewrites all configured ports.
                    sed -i -E "s#http://host\\.docker\\.internal:(${port_pattern})/mcp/#http://${ip_esc}:\\1/mcp/#g" "$config_path" 2>/dev/null || true
                    ha=$(grep -cE "http://host\\.docker\\.internal:${port_pattern}/mcp/" "$config_path" 2>/dev/null || true)
                    ha=${ha:-0}
                    br=$(grep -cE "http://${ip_esc}:${port_pattern}/mcp/" "$config_path" 2>/dev/null || true)
                    br=${br:-0}
                    echo "[gh-sr:mcp-claude-urls] rewrite_applied iteration=${i} host_mcp_url_hits_after=${ha} bridge_mcp_url_hits=${br}" >&2
                fi
            fi
            previous_sig="$current_sig"
        else
            previous_sig=""
        fi
        sleep 1
    done

    trap - TERM INT

    if [[ -f "$config_path" ]] && grep -qE "http://host\\.docker\\.internal:${port_pattern}/mcp/" "$config_path" 2>/dev/null; then
        local fh fb
        fh=$(grep -cE "http://host\\.docker\\.internal:${port_pattern}/mcp/" "$config_path" 2>/dev/null || true)
        fh=${fh:-0}
        fb=$(grep -cE "http://${ip_esc}:${port_pattern}/mcp/" "$config_path" 2>/dev/null || true)
        fb=${fb:-0}
        echo "[gh-sr:mcp-claude-urls] watcher_exit WARNING still_host_mcp_urls path=${config_path} host_mcp_url_hits=${fh} bridge_mcp_url_hits=${fb}" >&2
    else
        echo "[gh-sr:mcp-claude-urls] watcher_exit path=${config_path}" >&2
    fi
    return 0
}

needs_awf_agent_bridge_host() {
    local sub="${1:-}"
    case "$sub" in
        run | create) ;;
        *) return 1 ;;
    esac

    local has_agent=false
    local has_hdki_host=false
    local prev_add_host=false
    local arg

    for arg in "$@"; do
        if [[ "$prev_add_host" == true ]]; then
            case "$arg" in
                host.docker.internal:* | host.docker.internal)
                    has_hdki_host=true
                    ;;
            esac
            prev_add_host=false
        fi
        case "$arg" in
            --add-host)
                prev_add_host=true
                ;;
            --add-host=host.docker.internal:* | --add-host=host.docker.internal)
                has_hdki_host=true
                ;;
            ghcr.io/github/gh-aw-firewall/agent:*)
                has_agent=true
                ;;
        esac
    done

    [[ "$has_agent" == true && "$has_hdki_host" == false ]]
}

if is_mcpg_invocation "$@"; then
    sub="$1"
    shift
    if [[ "$sub" == "run" ]]; then
        mcpg_tmpdir=""
        mcpg_own_tmpdir=false
        mcpg_cidfile="$(docker_option_value --cidfile "$@" || true)"
        mcpg_name="$(docker_option_value --name "$@" || true)"
        extra=()
        if ! has_hostname_arg "$@"; then
            extra+=(--hostname gh-aw-mcpg)
        fi
        if has_detach_arg "$@"; then
            exec "$real" "$sub" "${extra[@]}" "$@"
        fi
        if [[ -z "$mcpg_name" || -z "$mcpg_cidfile" ]]; then
            mcpg_tmpdir=$(mktemp -d /tmp/gh-sr-mcpg.XXXXXX) || exit 1
            mcpg_own_tmpdir=true
            suffix="${mcpg_tmpdir##*.}"
        fi
        if [[ -z "$mcpg_name" ]]; then
            mcpg_name="gh-aw-mcpg-ghsr-$suffix"
            extra+=(--name "$mcpg_name")
        fi
        if [[ -z "$mcpg_cidfile" ]]; then
            mcpg_cidfile="$mcpg_tmpdir/container.cid"
            extra+=(--cidfile "$mcpg_cidfile")
        fi

        mcpg_docker_child_pid=""
        mcpg_rewriter_pid=""

        cleanup_mcpg_container() {
            local cid=""
            if [[ -n "$mcpg_cidfile" && -f "$mcpg_cidfile" ]]; then
                cid=$(tr -d ' \t\n\r' <"$mcpg_cidfile" 2>/dev/null || true)
                rm -f "$mcpg_cidfile" 2>/dev/null || true
            fi
            if [[ -n "$cid" ]]; then
                "$real" rm -f "$cid" 2>/dev/null || true
            fi
            if [[ -n "$mcpg_name" ]]; then
                "$real" rm -f "$mcpg_name" 2>/dev/null || true
            fi
            if [[ "$mcpg_own_tmpdir" == true && -n "$mcpg_tmpdir" ]]; then
                rm -rf "$mcpg_tmpdir" 2>/dev/null || true
            fi
            if [[ -n "$mcpg_rewriter_pid" ]] && kill -0 "$mcpg_rewriter_pid" 2>/dev/null; then
                kill -TERM "$mcpg_rewriter_pid" 2>/dev/null || true
                wait "$mcpg_rewriter_pid" 2>/dev/null || true
            fi
            if [[ -n "$mcpg_docker_child_pid" ]] && kill -0 "$mcpg_docker_child_pid" 2>/dev/null; then
                kill -TERM "$mcpg_docker_child_pid" 2>/dev/null || true
                wait "$mcpg_docker_child_pid" 2>/dev/null || true
            fi
        }

        on_mcpg_signal() {
            cleanup_mcpg_container
            trap - EXIT
            exit 143
        }
        trap cleanup_mcpg_container EXIT
        trap on_mcpg_signal INT TERM HUP

        set +e
        if [[ -z "${GH_SR_MCP_REWRITE_TARGET_IP:-}" ]]; then
            GH_SR_MCP_REWRITE_TARGET_IP="$(resolve_awf_host_route_target)"
        fi
        export GH_SR_MCP_REWRITE_TARGET_IP
        rewrite_claude_mcp_gateway_urls "${GH_SR_MCP_CONFIG_REWRITE_PATH:-/tmp/gh-aw/mcp-config/mcp-servers.json}" &
        mcpg_rewriter_pid=$!
        # Background jobs in non-interactive bash get /dev/null stdin unless
        # attached explicitly; gh-aw pipes MCP JSON into this docker run -i.
        "$real" "$sub" "${extra[@]}" "$@" <&0 &
        mcpg_docker_child_pid=$!
        wait "$mcpg_docker_child_pid"
        code=$?
        set -e
        trap - INT TERM HUP
        trap - EXIT
        cleanup_mcpg_container
        exit "$code"
    else
        extra=()
        if ! has_hostname_arg "$@"; then
            extra+=(--hostname gh-aw-mcpg)
        fi
        exec "$real" "$sub" "${extra[@]}" "$@"
    fi
fi

if needs_awf_agent_bridge_host "$@"; then
    sub="$1"
    shift
    awf_host_target="$(resolve_awf_host_route_target)"
    if [[ "$awf_host_target" == "HOST_GATEWAY" ]]; then
        exec "$real" "$sub" --add-host=host.docker.internal:host-gateway "$@"
    fi
    exec "$real" "$sub" --add-host=host.docker.internal:"$awf_host_target" "$@"
fi

exec "$real" "$@"

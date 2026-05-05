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
# piped gateway JSON schema-valid, then rewrites the generated Claude MCP
# config URLs from host.docker.internal to the AWF bridge gateway IP. Claude's
# HTTP MCP client can intermittently route host.docker.internal through the
# proxy despite NO_PROXY, while 172.30.0.1 is already exempted by AWF as a
# host-access gateway. When a rewrite runs, it prints grep-safe diagnostics to
# stderr prefixed with `[gh-sr:mcp-claude-urls]` (visible in Actions job logs).
#
# It also intercepts `docker run|create ... ghcr.io/github/gh-aw-firewall/agent:* ...`
# and injects `--add-host=host.docker.internal:<AWF bridge gateway>` when the caller did
# not already add an explicit `host.docker.internal` host entry. AWF agent containers
# sit on custom Docker networks; inner DNS for `host.docker.internal` can flake.
# Docker's `host-gateway` often maps to an inner-bridge IP (e.g. 172.18.0.1) that is
# not where AWF exposes host-published service ports — the AWF bridge gateway
# (default 172.30.0.1, same as MCP URL rewrite above) is the stable route for port 80,
# 5432, 6379, etc. Override with GH_SR_AWF_BRIDGE_GATEWAY_IP if your AWF layout differs.
#
# All other `docker` invocations are passed through untouched.
#
# Tests may set GH_SR_DOCKER_WRAPPER_REAL to a program that records argv (e.g.
# /bin/echo) instead of invoking the real docker daemon.
#
# See: https://github.com/github/gh-aw/issues/25511

real="${GH_SR_DOCKER_WRAPPER_REAL:-/usr/bin/docker}"

# AWF host-access / bridge gateway inside the inner Docker network (see gh-aw firewall
# allow-host-service-ports and MCP rewrite to 172.30.0.1 in this shim).
AWF_HOST_DOCKER_INTERNAL_IP="${GH_SR_AWF_BRIDGE_GATEWAY_IP:-172.30.0.1}"

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
    local previous_sig=""
    local current_sig=""
    local i
    local saw_file=false

    on_rewrite_watcher_signal() {
        echo "[gh-sr:mcp-claude-urls] watcher_stop signal path=${config_path}" >&2
        trap - TERM INT
        exit 0
    }
    trap 'on_rewrite_watcher_signal' TERM INT

    echo "[gh-sr:mcp-claude-urls] watcher_start path=${config_path} max_iterations=120" >&2

    for i in $(seq 1 120); do
        if [[ -f "$config_path" ]]; then
            if [[ "$saw_file" == false ]]; then
                echo "[gh-sr:mcp-claude-urls] config_appeared size_bytes=$(stat -c '%s' "$config_path" 2>/dev/null || echo '?')" >&2
                saw_file=true
            fi
            current_sig=$(stat -c '%s:%Y' "$config_path" 2>/dev/null || true)
            if [[ -n "$current_sig" && "$current_sig" == "$previous_sig" ]]; then
                if grep -qE 'http://host\.docker\.internal:80/mcp/' "$config_path" 2>/dev/null; then
                    local hb ha br
                    hb=$(grep -cE 'http://host\.docker\.internal:80/mcp/' "$config_path" 2>/dev/null || true)
                    hb=${hb:-0}
                    echo "[gh-sr:mcp-claude-urls] stable_file iteration=${i} sig=${current_sig} host_mcp_url_hits=${hb} applying_rewrite" >&2
                    sed -i 's#http://host\.docker\.internal:80/mcp/#http://172.30.0.1:80/mcp/#g' "$config_path" 2>/dev/null || true
                    ha=$(grep -cE 'http://host\.docker\.internal:80/mcp/' "$config_path" 2>/dev/null || true)
                    ha=${ha:-0}
                    br=$(grep -cE 'http://172\.30\.0\.1:80/mcp/' "$config_path" 2>/dev/null || true)
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

    if [[ -f "$config_path" ]] && grep -qE 'http://host\.docker\.internal:80/mcp/' "$config_path" 2>/dev/null; then
        local fh fb
        fh=$(grep -cE 'http://host\.docker\.internal:80/mcp/' "$config_path" 2>/dev/null || true)
        fh=${fh:-0}
        fb=$(grep -cE 'http://172\.30\.0\.1:80/mcp/' "$config_path" 2>/dev/null || true)
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
    exec "$real" "$sub" --add-host=host.docker.internal:"$AWF_HOST_DOCKER_INTERNAL_IP" "$@"
fi

exec "$real" "$@"

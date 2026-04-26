#!/bin/bash
# gh-sr docker CLI shim (installed as /opt/gh-sr/docker-shim/docker in the container runner image).
#
# This is NOT the mechanism that isolates multiple agentic runners on one host — use
# runner_mode: container for that (each outer runner container gets its own inner dockerd,
# network namespace, MCP port 80, and /tmp/gh-aw). This script is a gh-aw compatibility
# layer for jobs running inside those containers: MCP gateway self-inspect bypass, gateway
# container cleanup, and AWF agent host.docker.internal routing (see below).
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
# It also intercepts `docker run|create ... ghcr.io/github/gh-aw-firewall/agent:* ...`
# and injects `--add-host=host.docker.internal:host-gateway` when the caller did
# not already add an explicit `host.docker.internal` host entry. AWF agent
# containers often sit on a custom Docker network where inner DNS resolution of
# `host.docker.internal` can flake; Docker's host-gateway mapping is the stable
# route to the MCP gateway on the inner host network (port 80).
#
# All other `docker` invocations are passed through untouched.
#
# Tests may set GH_SR_DOCKER_WRAPPER_REAL to a program that records argv (e.g.
# /bin/echo) instead of invoking the real docker daemon.
#
# See: https://github.com/github/gh-aw/issues/25511

real="${GH_SR_DOCKER_WRAPPER_REAL:-/usr/bin/docker}"

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

needs_awf_agent_host_gateway() {
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
        "$real" "$sub" "${extra[@]}" "$@" &
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

if needs_awf_agent_host_gateway "$@"; then
    sub="$1"
    shift
    exec "$real" "$sub" --add-host=host.docker.internal:host-gateway "$@"
fi

exec "$real" "$@"

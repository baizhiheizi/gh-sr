#!/bin/bash
# gh-sr docker wrapper
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
# This wrapper intercepts `docker run|create ... ghcr.io/github/gh-aw-mcpg:* ...`
# and injects `--hostname gh-aw-mcpg` right after the subcommand.
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

needs_hostname_injection() {
    local sub="${1:-}"
    case "$sub" in
        run|create) ;;
        *) return 1 ;;
    esac

    local has_hostname=false
    local has_mcpg=false
    local arg
    for arg in "$@"; do
        case "$arg" in
            --hostname|--hostname=*|-h) has_hostname=true ;;
            ghcr.io/github/gh-aw-mcpg:*) has_mcpg=true ;;
        esac
    done

    [[ "$has_mcpg" == true && "$has_hostname" == false ]]
}

needs_awf_agent_host_gateway() {
    local sub="${1:-}"
    case "$sub" in
        run|create) ;;
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

if needs_hostname_injection "$@"; then
    sub="$1"
    shift
    exec "$real" "$sub" --hostname gh-aw-mcpg "$@"
fi

if needs_awf_agent_host_gateway "$@"; then
    sub="$1"
    shift
    exec "$real" "$sub" --add-host=host.docker.internal:host-gateway "$@"
fi

exec "$real" "$@"

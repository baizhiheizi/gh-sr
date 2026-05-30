#!/bin/bash
# gh-sr Docker CLI shim (installed as /opt/gh-sr/docker-shim/docker in the container runner image).
#
# This is NOT what isolates concurrent agentic runners on one host — runner_mode: container does
# that (each outer gh-sr-<instance> container has its own inner dockerd, network namespace, MCP
# gateway port, and /tmp/gh-aw). Pristine-per-job state is guaranteed by the runner job hooks
# (/opt/gh-sr/hooks/job-started.sh and /opt/gh-sr/hooks/job-completed.sh), NOT by this shim.
#
# The shim does two deterministic things. When gh-aw launches the MCP Gateway:
#
#     docker run -i --rm --network host ... ghcr.io/github/gh-aw-mcpg:<tag>
#
#   1. it injects `--hostname gh-aw-mcpg` (a non-hex hostname) when the caller did not set one;
#   2. for `docker run`, it injects `--name gh-aw-mcpg-ghsr-<unique>` when the caller did not set a
#      name, so the gateway container is always reapable by the gh-aw-mcpg- prefix.
#
# Why (1): inside the gateway, /app/run_containerized.sh derives CONTAINER_ID from /proc/self/cgroup
# or `hostname`, then runs validate_port_mapping / validate_stdin_interactive. Under `--network host`
# the container's Ports map is empty, so validate_port_mapping fails and kills the gateway (exit 1).
# A non-hex hostname makes CONTAINER_ID detection fall through to the upstream "could not determine
# container ID" path, which skips that validation, and the gateway starts normally.
#
# Why (2): gh-aw runs the gateway with `docker run --rm`, which normally removes it on exit. But if a
# job is killed (or `stop_mcp_gateway.sh` cannot stop it), an unnamed gateway would get a random
# Docker name that the per-job reset hooks and `gh sr doctor` (which reap by the `gh-aw-mcpg-` name
# prefix) could not find — a stale gateway holding the MCP port would then break the next job. The
# unique suffix avoids `name already in use` collisions with such a leftover. This is just a name, not
# a supervisor: after injecting the flags the shim `exec`s the real docker, so stdin (the piped
# gateway JSON), signals, and the exit code pass through unchanged. No cidfile, background watcher,
# or config rewriting is involved — per-job cleanup is the job hooks' responsibility.
#
# host.docker.internal resolution is handled deterministically by the image-baked Docker daemon DNS
# (pinned default-bridge gateway 172.17.0.1 + dnsmasq; see daemon.json / dnsmasq-gh-sr.conf), so the
# shim no longer injects --add-host or rewrites generated MCP config URLs.
#
# All other docker invocations pass through untouched.
#
# Tests may set GH_SR_DOCKER_WRAPPER_REAL to a recorder (e.g. /bin/echo) instead of the real docker.

real="${GH_SR_DOCKER_WRAPPER_REAL:-/usr/bin/docker}"

# is_mcpg_invocation: true when this is `docker run|create ... ghcr.io/github/gh-aw-mcpg:* ...`.
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

# has_hostname_arg: true when --hostname / -h is already present (so we do not duplicate it).
has_hostname_arg() {
    local prev=false
    local arg
    for arg in "$@"; do
        if [[ "$prev" == true ]]; then
            return 0
        fi
        case "$arg" in
            --hostname | -h)
                prev=true
                ;;
            --hostname=* | -h=*)
                return 0
                ;;
        esac
    done
    return 1
}

# has_name_arg: true when --name is already present (so we do not override the caller's name).
has_name_arg() {
    local prev=false
    local arg
    for arg in "$@"; do
        if [[ "$prev" == true ]]; then
            return 0
        fi
        case "$arg" in
            --name)
                prev=true
                ;;
            --name=*)
                return 0
                ;;
        esac
    done
    return 1
}

if is_mcpg_invocation "$@"; then
    sub="$1"
    shift
    extra=()
    if ! has_hostname_arg "$@"; then
        extra+=(--hostname gh-aw-mcpg)
    fi
    # Name `docker run` gateways so leftover containers are always reapable by the
    # gh-aw-mcpg- prefix. The unique suffix avoids collisions with a stale gateway from
    # a crashed job. `create` is left untouched (gh-aw launches the gateway via run).
    if [[ "$sub" == "run" ]] && ! has_name_arg "$@"; then
        extra+=(--name "gh-aw-mcpg-ghsr-$$-${RANDOM}")
    fi
    exec "$real" "$sub" "${extra[@]}" "$@"
fi

exec "$real" "$@"

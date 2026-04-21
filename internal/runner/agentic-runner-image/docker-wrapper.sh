#!/bin/bash
# gh-sr docker wrapper
#
# Transparently patches gh-aw-mcpg images so they work with `--network host`.
#
# Upstream's /app/run_containerized.sh hard-requires MCP_GATEWAY_PORT to
# appear in the container's NetworkSettings.Ports. The image only `EXPOSE`s
# port 8000, but gh-aw sets MCP_GATEWAY_PORT=80 on self-hosted runners, so
# the port check fails and the gateway exits with code 1 on startup.
#
# See: https://github.com/github/gh-aw/issues/25511
#
# This wrapper detects any `docker run|create ... ghcr.io/github/gh-aw-mcpg:* ...`
# invocation, pulls the image if needed, and rebuilds it with the port check
# neutralised. The rebuild is a single sed layer, tagged identically, and
# labelled `gh-sr.patched=true` so subsequent invocations are no-ops.
#
# All other `docker` commands are passed through untouched.

real=/usr/bin/docker
lockfile=/tmp/gh-sr-docker-patch.lock

patch_mcpg_image() {
    local image="$1"

    if "$real" image inspect "$image" \
        --format '{{index .Config.Labels "gh-sr.patched"}}' 2>/dev/null \
        | grep -qx true; then
        return 0
    fi

    if ! "$real" image inspect "$image" >/dev/null 2>&1; then
        "$real" pull "$image" >/dev/null 2>&1 || return 0
        if "$real" image inspect "$image" \
            --format '{{index .Config.Labels "gh-sr.patched"}}' 2>/dev/null \
            | grep -qx true; then
            return 0
        fi
    fi

    local tmp
    tmp=$(mktemp -d)
    cat > "$tmp/Dockerfile" <<EOF
FROM $image
LABEL gh-sr.patched=true
# Disable validate_port_mapping call in the containerized entrypoint so the
# gateway starts correctly when run with --network host (no -p flags).
# The upstream script hard-exits when MCP_GATEWAY_PORT is not in
# NetworkSettings.Ports, which is empty for host-networked containers.
RUN sed -i 's|^\([[:space:]]*\)validate_port_mapping[[:space:]]|\1# gh-sr: skipped (--network host) validate_port_mapping |' /app/run_containerized.sh
EOF

    "$real" build -q -t "$image" "$tmp" >/dev/null 2>&1 || true
    rm -rf "$tmp"
}

maybe_patch_args() {
    local sub="${1:-}"
    case "$sub" in
        run|create) ;;
        *) return 0 ;;
    esac

    local arg
    for arg in "$@"; do
        if [[ "$arg" == ghcr.io/github/gh-aw-mcpg:* ]]; then
            (
                flock -x 200
                patch_mcpg_image "$arg"
            ) 200>"$lockfile" 2>/dev/null || true
            return 0
        fi
    done
}

maybe_patch_args "$@" || true
exec "$real" "$@"

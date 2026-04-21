#!/bin/bash
# gh-sr docker wrapper
#
# Transparently patches gh-aw-mcpg images so they work with `--network host`.
#
# Upstream's /app/run_containerized.sh runs a self-inspect validation block
# that hard-exits the gateway when launched inside a DinD runner:
#
#   1. validate_port_mapping fails because MCP_GATEWAY_PORT (80) is never
#      present in NetworkSettings.Ports — the image only EXPOSEs 8000, and
#      host-networked containers have an empty Ports map anyway.
#   2. validate_stdin_interactive fails because `docker inspect .Config.OpenStdin`
#      from inside the container doesn't reliably reflect the `-i` flag used
#      at run time.
#
# See: https://github.com/github/gh-aw/issues/25511
#
# This wrapper detects any `docker run|create ... ghcr.io/github/gh-aw-mcpg:* ...`
# invocation, pulls the image if needed, and rebuilds it with those self-inspect
# checks commented out. The rebuild is a single sed layer, tagged identically,
# and labelled `gh-sr.patched=<version>` so subsequent invocations are no-ops
# unless the patch logic itself changes (bump PATCH_VERSION below).
#
# All other `docker` commands are passed through untouched.

real=/usr/bin/docker
lockfile=/tmp/gh-sr-docker-patch.lock

# Bump this when changing the sed transformation below so stale cached
# images (from previous gh-sr versions) are detected and re-patched.
PATCH_VERSION=v2

patch_mcpg_image() {
    local image="$1"

    if "$real" image inspect "$image" \
        --format '{{index .Config.Labels "gh-sr.patched"}}' 2>/dev/null \
        | grep -qx "$PATCH_VERSION"; then
        return 0
    fi

    if ! "$real" image inspect "$image" >/dev/null 2>&1; then
        "$real" pull "$image" >/dev/null 2>&1 || return 0
        if "$real" image inspect "$image" \
            --format '{{index .Config.Labels "gh-sr.patched"}}' 2>/dev/null \
            | grep -qx "$PATCH_VERSION"; then
            return 0
        fi
    fi

    local tmp
    tmp=$(mktemp -d)
    cat > "$tmp/Dockerfile" <<'EOF'
FROM __BASE_IMAGE__
LABEL gh-sr.patched=__PATCH_VERSION__
# Neutralise the self-inspect validation block in the containerized
# entrypoint. All of these checks do `docker inspect <self>` and compare
# against settings that don't apply when the gateway is launched with
# `--network host` inside a DinD runner:
#
#   - validate_port_mapping:       requires MCP_GATEWAY_PORT in NetworkSettings.Ports
#   - validate_stdin_interactive:  requires Config.OpenStdin == true
#   - validate_container_config:   warns only (docker socket mount)
#   - validate_log_directory_mount: warns only (log dir mount)
#
# gh-aw always passes `-i`, mounts the docker socket, and uses --network host,
# so skipping these checks is safe for our environment.
# See: https://github.com/github/gh-aw/issues/25511
RUN sed -i -E 's|^([[:space:]]+)(validate_[a-z_]+ "\$CONTAINER_ID".*)|\1# gh-sr: skipped \2|' /app/run_containerized.sh
EOF
    sed -i \
        -e "s|__BASE_IMAGE__|$image|" \
        -e "s|__PATCH_VERSION__|$PATCH_VERSION|" \
        "$tmp/Dockerfile"

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

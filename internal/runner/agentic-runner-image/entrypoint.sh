#!/bin/bash
# entrypoint.sh — runs inside each gh-sr agentic runner container
#
# Startup order:
#   1. Start inner dockerd (DinD)
#   2. Configure dnsmasq so host.docker.internal resolves inside agent containers
#   3. Register the actions runner against GitHub (idempotent)
#   4. exec run.sh (the actions runner loop)
#
# Environment variables injected by `docker run`:
#   GH_SR_RUNNER_NAME   — unique runner name (e.g. "myrepo-agentic-1")
#   GH_SR_RUNNER_TOKEN  — registration token from GitHub API
#   GH_SR_RUNNER_URL    — https://github.com/<owner>/<repo> or https://github.com/<org>
#   GH_SR_RUNNER_LABELS — comma-separated extra labels (e.g. "self-hosted,Linux,X64,agentic")
#   GH_SR_RUNNER_GROUP  — runner group (optional, default: "Default")
#   GH_SR_RUNNER_EPHEMERAL — "true" to register as ephemeral

set -euo pipefail

RUNNER_DIR="/home/runner/actions-runner"
RUNNER_STATE_DIR="/runner-state"
RUNNER_WORK_DIR="${RUNNER_STATE_DIR}/_work"
RUNNER_TEMP_DIR="${RUNNER_STATE_DIR}/_temp"

# ── 1. Inner dockerd (DinD) ────────────────────────────────────────────────────
# Store Docker data in the bind-mounted state dir so layers survive container
# restarts (avoids re-pulling gh-aw images on every job).
DOCKER_DATA_ROOT="${RUNNER_STATE_DIR}/docker-data"
mkdir -p "${DOCKER_DATA_ROOT}"

# Enable cgroup v2 nesting so the inner dockerd can create child cgroups for
# its containers (otherwise awf's compose stack fails with
# `cannot enter cgroupv2 "/sys/fs/cgroup/docker" with domain controllers --
# it is in threaded mode`).
#
# Mirrors upstream docker:dind — see moby/hack/dind:
# https://github.com/moby/moby/blob/v26.0.1/hack/dind
#
# Steps:
#   1. Evacuate all processes from the root cgroup into /sys/fs/cgroup/init.
#      Writing to cgroup.subtree_control fails with EBUSY if the cgroup has
#      member processes; the "no internal processes" rule is a v2 invariant.
#   2. Enable every controller for descendants by mirroring
#      cgroup.controllers into cgroup.subtree_control with "+" prefixes.
if [ -f /sys/fs/cgroup/cgroup.controllers ]; then
    echo "[entrypoint] enabling cgroup v2 nesting..."
    mkdir -p /sys/fs/cgroup/init
    xargs -rn1 < /sys/fs/cgroup/cgroup.procs > /sys/fs/cgroup/init/cgroup.procs 2>/dev/null || true
    sed -e 's/ / +/g' -e 's/^/+/' < /sys/fs/cgroup/cgroup.controllers \
        > /sys/fs/cgroup/cgroup.subtree_control 2>/dev/null || \
        echo "[entrypoint] WARNING: failed to enable cgroup controllers (continuing anyway)"
fi

echo "[entrypoint] starting dockerd..."
dockerd \
    --data-root="${DOCKER_DATA_ROOT}" \
    --host=unix:///var/run/docker.sock \
    --log-level=warn \
    &>/runner-state/dockerd.log &

DOCKERD_PID=$!

# Wait until the socket is available (up to 30s).
for i in $(seq 1 30); do
    if docker info &>/dev/null 2>&1; then
        echo "[entrypoint] dockerd is up"
        break
    fi
    if [ "$i" -eq 30 ]; then
        echo "[entrypoint] ERROR: dockerd did not start within 30 seconds" >&2
        exit 1
    fi
    sleep 1
done

# ── 2. host.docker.internal via dnsmasq ───────────────────────────────────────
# gh-aw agent containers reach the MCP Gateway via http://host.docker.internal:80.
# On Linux, Docker does not populate this entry automatically; we use dnsmasq.

DOCKER0_IP=$(ip -4 addr show docker0 2>/dev/null | grep -oP '(?<=inet )\d+\.\d+\.\d+\.\d+' || true)
if [ -n "${DOCKER0_IP}" ]; then
    echo "[entrypoint] configuring host.docker.internal → ${DOCKER0_IP}"
    cat > /etc/dnsmasq.d/host-docker-internal.conf <<EOF
address=/host.docker.internal/${DOCKER0_IP}
EOF
    # Reload or start dnsmasq.
    if pgrep -x dnsmasq &>/dev/null; then
        kill -HUP "$(pgrep -x dnsmasq)" 2>/dev/null || true
    else
        dnsmasq --no-daemon --conf-dir=/etc/dnsmasq.d &
    fi

    # Pass our dnsmasq to inner Docker containers via daemon.json. Use the
    # docker0 bridge address; 127.0.0.1 inside child containers is their own
    # loopback and will bypass the runner container's dnsmasq.
    mkdir -p /etc/docker
    cat > /etc/docker/daemon.json <<EOF
{
  "dns": ["${DOCKER0_IP}", "8.8.8.8"],
  "dns-search": []
}
EOF
    # Restart dockerd to pick up daemon.json changes.
    kill "${DOCKERD_PID}" 2>/dev/null && wait "${DOCKERD_PID}" 2>/dev/null || true
    dockerd \
        --data-root="${DOCKER_DATA_ROOT}" \
        --host=unix:///var/run/docker.sock \
        --log-level=warn \
        &>/runner-state/dockerd.log &
    DOCKERD_PID=$!
    for i in $(seq 1 20); do
        docker info &>/dev/null 2>&1 && break
        sleep 1
    done
else
    echo "[entrypoint] WARNING: docker0 interface not found; host.docker.internal may not resolve"
fi

# ── 3. Register the actions runner ────────────────────────────────────────────
cd "${RUNNER_DIR}"

RUNNER_NAME="${GH_SR_RUNNER_NAME:-gh-sr-runner}"
RUNNER_URL="${GH_SR_RUNNER_URL:?GH_SR_RUNNER_URL is required}"
RUNNER_TOKEN="${GH_SR_RUNNER_TOKEN:?GH_SR_RUNNER_TOKEN is required}"
RUNNER_LABELS="${GH_SR_RUNNER_LABELS:-self-hosted,Linux,X64,agentic}"
RUNNER_GROUP="${GH_SR_RUNNER_GROUP:-Default}"

mkdir -p "${RUNNER_WORK_DIR}" "${RUNNER_TEMP_DIR}"
# Work and temp dirs are created as root; give the runner user ownership so it
# can create job workspaces inside them.
chown runner:runner "${RUNNER_WORK_DIR}" "${RUNNER_TEMP_DIR}"

CONFIG_ARGS=(
    --url "${RUNNER_URL}"
    --token "${RUNNER_TOKEN}"
    --name "${RUNNER_NAME}"
    --labels "${RUNNER_LABELS}"
    --work "${RUNNER_WORK_DIR}"
    --runnergroup "${RUNNER_GROUP}"
    --unattended
    --replace
)

if [ "${GH_SR_RUNNER_EPHEMERAL:-false}" = "true" ]; then
    CONFIG_ARGS+=(--ephemeral)
fi

# RUNNER_TEMP must not be /tmp (gh-aw explicitly requires a non-/tmp path).
export RUNNER_TEMP="${RUNNER_TEMP_DIR}"

# RUNNER_TOOL_CACHE must be /opt/hostedtoolcache so tools installed by
# actions/setup-node, actions/setup-python, etc. land where gh-aw's agent
# container looks for them. Its "Execute" step hard-codes:
#
#     PATH="$(find /opt/hostedtoolcache -maxdepth 4 -type d -name bin …):$PATH"
#
# If we leave RUNNER_TOOL_CACHE unset the runner defaults it to
# $RUNNER_WORK/_tool (here /runner-state/_work/_tool) and Node ends up
# invisible to the agent, so `claude` fails with "command not found".
# The /opt/hostedtoolcache directory is created and chowned to runner in
# the Dockerfile, so it is writable by the runner user.
export RUNNER_TOOL_CACHE="/opt/hostedtoolcache"

# su - resets the environment to a login shell's defaults, so RUNNER_TEMP
# and RUNNER_TOOL_CACHE are re-exported on the runner-user side of every
# invocation below.
RUNNER_ENV="RUNNER_TEMP='${RUNNER_TEMP_DIR}' RUNNER_TOOL_CACHE='${RUNNER_TOOL_CACHE}'"

# Only configure if not already done. GitHub registration tokens are one-time-use;
# running config.sh on every restart would consume the token and fail on the second start.
# (Same behaviour as native runners — configure once, restart many times.)
if [ ! -f "${RUNNER_DIR}/.runner" ]; then
    echo "[entrypoint] configuring runner..."
    su - runner -c "cd '${RUNNER_DIR}' && ${RUNNER_ENV} ./config.sh ${CONFIG_ARGS[*]@Q}" 2>&1
else
    echo "[entrypoint] runner already configured, skipping config.sh"
fi

# ── 4. Pre-pull gh-aw images (best-effort, background) ────────────────────────
su - runner -c "
    docker pull ghcr.io/github/gh-aw-firewall/agent:latest &>/dev/null &
    docker pull ghcr.io/github/gh-aw-mcpg:latest &>/dev/null &
" 2>/dev/null || true

# ── 5. Graceful shutdown handler ──────────────────────────────────────────────
_shutdown() {
    echo "[entrypoint] received SIGTERM — stopping runner..."
    # Ask the runner to finish the current job then stop.
    if [ -f "${RUNNER_DIR}/.runner" ]; then
        su - runner -c "cd '${RUNNER_DIR}' && ./config.sh remove --token '${RUNNER_TOKEN}'" 2>/dev/null || true
    fi
    # Shut down the inner dockerd.
    kill "${DOCKERD_PID}" 2>/dev/null && wait "${DOCKERD_PID}" 2>/dev/null || true
    exit 0
}
trap _shutdown SIGTERM SIGINT

# ── 6. Run ────────────────────────────────────────────────────────────────────
echo "[entrypoint] starting actions runner as user 'runner'..."
exec su - runner -c "cd '${RUNNER_DIR}' && ${RUNNER_ENV} ./run.sh"

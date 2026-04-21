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

    # Pass our dnsmasq to inner Docker containers via daemon.json.
    DNSMASQ_LISTEN="127.0.0.1"
    mkdir -p /etc/docker
    cat > /etc/docker/daemon.json <<EOF
{
  "dns": ["${DNSMASQ_LISTEN}", "8.8.8.8"],
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

# Always re-configure so label/token changes take effect on container restart.
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

su - runner -c "cd '${RUNNER_DIR}' && RUNNER_TEMP='${RUNNER_TEMP_DIR}' ./config.sh ${CONFIG_ARGS[*]@Q}" 2>&1

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
exec su - runner -c "cd '${RUNNER_DIR}' && RUNNER_TEMP='${RUNNER_TEMP_DIR}' ./run.sh"

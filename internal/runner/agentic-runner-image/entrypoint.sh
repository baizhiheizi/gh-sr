#!/bin/bash
# entrypoint.sh — runs inside each gh-sr container runner.
#
# Startup (deterministic, single dockerd start):
#   1. Enable cgroup v2 nesting (DinD requirement).
#   2. Start the inner dockerd ONCE. host.docker.internal DNS is baked into the
#      image (/etc/docker/daemon.json pins the default-bridge gateway to 10.200.0.1
#      and points container DNS at the bundled dnsmasq) — no runtime daemon.json
#      rewrite or dockerd restart. 10.200.0.1 is used (not 172.17.0.1) because the
#      outer runner container sits on the host's default Docker bridge
#      (172.17.0.0/16); a 172.17.x inner bridge would collide with the container's
#      own gateway/eth0 subnet and black-hole all outbound traffic.
#   3. Start dnsmasq from its baked config so host.docker.internal resolves inside
#      inner containers and external DNS is forwarded upstream.
#   4. Install the AWF service-routing bypass (one-shot; lets AWF agents reach
#      workflow `services:` published ports).
#   5. Register the actions runner against GitHub (idempotent; one-time token).
#   6. Wire the per-job reset hooks into the runner .env.
#   7. exec run.sh (the actions runner loop).
#
# Per-job environment hygiene (clean /tmp/gh-aw, remove leftover containers, prune
# networks, flush AWF iptables) is handled by /opt/gh-sr/hooks/job-started.sh and
# /opt/gh-sr/hooks/job-completed.sh, so every job runs from a known-clean state on
# this long-lived runner. The inner Docker image-layer cache under
# /runner-state/docker-data is preserved across jobs (never pruned).
#
# Environment variables injected by `docker run`:
#   GH_SR_RUNNER_NAME   — unique runner name (e.g. "myrepo-agentic-1")
#   GH_SR_RUNNER_TOKEN  — registration token from GitHub API
#   GH_SR_RUNNER_URL    — https://github.com/<owner>/<repo> or https://github.com/<org>
#   GH_SR_RUNNER_LABELS — comma-separated extra labels (e.g. "self-hosted,Linux,X64,agentic")
#   GH_SR_RUNNER_GROUP  — runner group (optional, default: "Default")
#   GH_SR_RUNNER_EPHEMERAL — "true" to register as ephemeral
#   GH_SR_AWF_SUBNET    — AWF bridge subnet for the service-routing bypass (default 172.30.0.0/24)

set -euo pipefail

RUNNER_DIR="/home/runner/actions-runner"
RUNNER_STATE_DIR="/runner-state"
RUNNER_WORK_DIR="${RUNNER_STATE_DIR}/_work"
RUNNER_TEMP_DIR="${RUNNER_STATE_DIR}/_temp"

# Persistent inner-Docker image-layer cache. This is the ONLY state preserved across
# jobs; per-job runtime state (/tmp/gh-aw, leftover containers, networks, iptables) is
# reset by the job hooks. Keeping the cache here avoids re-pulling gh-aw's images.
DOCKER_DATA_ROOT="${RUNNER_STATE_DIR}/docker-data"
mkdir -p "${DOCKER_DATA_ROOT}"

# ── 1. cgroup v2 nesting ───────────────────────────────────────────────────────
# Enable cgroup v2 nesting so the inner dockerd can create child cgroups for its
# containers (otherwise awf's compose stack fails with
# `cannot enter cgroupv2 "/sys/fs/cgroup/docker" with domain controllers --
# it is in threaded mode`). Mirrors upstream docker:dind (moby/hack/dind).
if [ -f /sys/fs/cgroup/cgroup.controllers ]; then
    echo "[entrypoint] enabling cgroup v2 nesting..."
    mkdir -p /sys/fs/cgroup/init
    xargs -rn1 < /sys/fs/cgroup/cgroup.procs > /sys/fs/cgroup/init/cgroup.procs 2>/dev/null || true
    sed -e 's/ / +/g' -e 's/^/+/' < /sys/fs/cgroup/cgroup.controllers \
        > /sys/fs/cgroup/cgroup.subtree_control 2>/dev/null || \
        echo "[entrypoint] WARNING: failed to enable cgroup controllers (continuing anyway)"
fi

# ── 2. Inner dockerd (single start; DNS baked into /etc/docker/daemon.json) ─────
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

# ── 3. host.docker.internal via dnsmasq (baked config) ──────────────────────────
# gh-aw agent containers reach the MCP gateway via http://host.docker.internal:<port>.
# dnsmasq (config baked at build time) listens on the pinned bridge gateway 10.200.0.1
# and answers host.docker.internal there; daemon.json already points inner containers'
# DNS at 10.200.0.1. dockerd is NOT restarted.
echo "[entrypoint] starting dnsmasq..."
if pgrep -x dnsmasq &>/dev/null; then
    kill -HUP "$(pgrep -x dnsmasq)" 2>/dev/null || true
else
    dnsmasq --conf-dir=/etc/dnsmasq.d 2>/dev/null || \
        echo "[entrypoint] WARNING: dnsmasq failed to start; host.docker.internal may not resolve"
fi

# ── 4. AWF service-routing bypass (one-shot) ────────────────────────────────────
# Bypass inner dockerd's PREROUTING DNAT for traffic from awf-net to a local IP so
# AWF agents can reach workflow `services:` published ports (postgres/redis/etc.).
# The job-started hook re-asserts this per job; installing it once here keeps an idle
# runner consistent for `gh sr doctor`.
AWF_SUBNET="${GH_SR_AWF_SUBNET:-172.30.0.0/24}"
echo "[entrypoint] installing AWF service-routing bypass for ${AWF_SUBNET}..."
while iptables -t nat -D PREROUTING -s "${AWF_SUBNET}" -m addrtype --dst-type LOCAL -j RETURN 2>/dev/null; do :; done
iptables -t nat -I PREROUTING -s "${AWF_SUBNET}" -m addrtype --dst-type LOCAL -j RETURN \
    || echo "[entrypoint] WARNING: failed to install AWF service-routing bypass (iptables NAT PREROUTING)"

# ── 5. Register the actions runner ──────────────────────────────────────────────
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
# container looks for them ("Execute" step hard-codes /opt/hostedtoolcache).
export RUNNER_TOOL_CACHE="/opt/hostedtoolcache"

# ── 6. Wire per-job reset hooks into the runner .env ────────────────────────────
# The Actions runner reads .env at startup. We write it deterministically (the file
# lives in the image rootfs and persists across container restarts, so overwrite
# rather than append to stay idempotent).
cat > "${RUNNER_DIR}/.env" <<EOF
RUNNER_TEMP=${RUNNER_TEMP_DIR}
RUNNER_TOOL_CACHE=/opt/hostedtoolcache
ACTIONS_RUNNER_HOOK_JOB_STARTED=/opt/gh-sr/hooks/job-started.sh
ACTIONS_RUNNER_HOOK_JOB_COMPLETED=/opt/gh-sr/hooks/job-completed.sh
EOF
chown runner:runner "${RUNNER_DIR}/.env"

# su - resets the environment to a login shell's defaults, so RUNNER_TEMP
# and RUNNER_TOOL_CACHE are re-exported on the runner-user side of every
# invocation below.
# Prepend gh-sr docker shim so gh-aw / job steps use /opt/gh-sr/docker-shim/docker; root
# entrypoint above still uses /usr/bin/docker (shim is not on root's default PATH).
RUNNER_ENV="PATH=/opt/gh-sr/docker-shim:\$PATH RUNNER_TEMP='${RUNNER_TEMP_DIR}' RUNNER_TOOL_CACHE='${RUNNER_TOOL_CACHE}'"

# Only configure if not already done. GitHub registration tokens are one-time-use;
# running config.sh on every restart would consume the token and fail on the second start.
# (Same behaviour as native runners — configure once, restart many times.)
if [ ! -f "${RUNNER_DIR}/.runner" ]; then
    echo "[entrypoint] configuring runner..."
    su - runner -c "cd '${RUNNER_DIR}' && ${RUNNER_ENV} ./config.sh ${CONFIG_ARGS[*]@Q}" 2>&1
else
    echo "[entrypoint] runner already configured, skipping config.sh"
fi

# ── 7. Pre-pull gh-aw images (best-effort, background) ──────────────────────────
su - runner -c "
    docker pull ghcr.io/github/gh-aw-firewall/agent:latest &>/dev/null &
    docker pull ghcr.io/github/gh-aw-mcpg:latest &>/dev/null &
" 2>/dev/null || true

# ── 8. Graceful shutdown handler ────────────────────────────────────────────────
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

# ── 9. Run ──────────────────────────────────────────────────────────────────────
echo "[entrypoint] starting actions runner as user 'runner'..."
exec su - runner -c "cd '${RUNNER_DIR}' && ${RUNNER_ENV} ./run.sh"

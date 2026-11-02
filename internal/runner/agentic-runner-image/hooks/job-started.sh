#!/bin/bash
# gh-sr job-started hook — runs after a job is assigned, before its steps run.
#
# Wired via ACTIONS_RUNNER_HOOK_JOB_STARTED (see entrypoint.sh). Runs as a
# synchronous "Set up runner" step. It guarantees a pristine inner environment
# for the job:
#
#   1. Aggressively reset leftover inner state. ACTIONS_RUNNER_HOOK_JOB_STARTED runs
#      "when a job has been assigned to a runner, but before the job starts running"
#      (GitHub docs) — i.e. BEFORE the job lifecycle's prepare_job step that creates the
#      job/service (`services:`) containers and network. So removing ALL inner containers
#      here cannot touch the current job's containers; it only reaps leftovers from a
#      previous job whose completed-hook did not run (e.g. the runner was killed mid-job).
#   2. Repair /runner-state/_work ownership. Jobs that run steps as root (gh-aw agent
#      sandboxes mount the workspace into a root container; job containers default to
#      root) can leave uid-0 files in the shared workspace. The runner user cannot
#      unlink entries inside a root-owned directory, so the next checkout's
#      "Deleting the contents of ..." then fails with EACCES and EVERY subsequent job
#      on this long-lived runner fails at Checkout until an operator intervenes.
#      Sweep foreign-owned entries in the job-free hook window (same safety argument
#      as step 1). Both the scan AND the removal run under passwordless sudo: a bare
#      rm runs as the runner user, and a half-completed removal is worse than none —
#      it leaves a corrupt workspace (e.g. a .git stripped down to root-owned
#      commit-graph leftovers) that makes checkout fail with "not a git repository"
#      (issue #589). When sudo is unavailable the sweep stays best-effort; when sudo
#      works but entries still survive the removal, fail the job loudly in "Set up
#      runner" — checkout would fail anyway, and the hook message names the fix.
#   3. Verify the inner dockerd is responsive. This is the only hard failure: if
#      dockerd is down the job cannot run anyway, and failing here surfaces a clear
#      message in "Set up runner" instead of a confusing failure minutes later.
#
# The hook runs as the runner user and talks to the inner dockerd through the
# /var/run/docker.sock socket (group `docker`, which the runner is in). Never
# removes images / volumes — the /runner-state/docker-data image cache is
# preserved so the job does not re-pull gh-aw's images.

set +e

log() { echo "[gh-sr:job-started] $*"; }

# 1. Aggressive reset — safe because no job is running on this runner yet.
log "resetting inner environment to a pristine state (image cache preserved)..."
docker ps -aq 2>/dev/null | xargs -r docker rm -f >/dev/null 2>&1
docker network prune -f >/dev/null 2>&1
rm -rf /tmp/gh-aw >/dev/null 2>&1

# 2. Repair _work ownership so a root-privileged job cannot poison the next one.
RUNNER_WORK_DIR="/runner-state/_work"
_gh_sr_uid=$(id -u)
_sweep_foreign() {
    sudo -n find "${RUNNER_WORK_DIR}" -xdev -mindepth 1 \( ! -uid "${_gh_sr_uid}" \) -print0 2>/dev/null
}
if sudo -n true 2>/dev/null; then
    _orphans=$(_sweep_foreign | tr '\0' '\n' | grep -c . || true)
    if [ "${_orphans}" -gt 0 ]; then
        log "removing ${_orphans} ${RUNNER_WORK_DIR} entry/entries not owned by uid ${_gh_sr_uid} (left by root-privileged jobs)..."
        # Elevate the removal too: piped into a bare rm it would run as the runner
        # user and stop at the first root-owned directory (issue #589 — a stripped
        # .git left every later checkout failing with "not a git repository").
        _sweep_foreign | sudo -n xargs -r -0 rm -rf --
        _left=$(_sweep_foreign | tr '\0' '\n' | grep -c . || true)
        if [ "${_left}" -gt 0 ]; then
            log "ERROR: ${_left} foreign-owned ${RUNNER_WORK_DIR} entries survived the removal (immutable bits or a read-only mount?); checkout would fail — clear ${RUNNER_WORK_DIR} as root, then re-run the job"
            exit 1
        fi
        log "workspace ownership repaired"
    fi
else
    log "WARNING: passwordless sudo unavailable; skipping ${RUNNER_WORK_DIR} ownership repair"
fi

# 3. Arm the AWF service bridge (opt-in: runners.yml `awf_service_bridge: true`,
#    agentic profile only — propagated via the runner .env). This hook runs
#    before prepare_job creates the job's service containers/network and long
#    before the agent step creates awf-net, so the bridge joins from a detached
#    waiter (it polls for both networks and exits once joined, on timeout, or
#    when job-completed.sh kills it). See hooks/awf-service-bridge.sh.
if [ "${GH_SR_AWF_SERVICE_BRIDGE:-0}" = "1" ]; then
    log "awf_service_bridge enabled — arming service bridge waiter"
    : > /tmp/gh-sr-awf-service-bridge.log 2>/dev/null
    nohup /opt/gh-sr/hooks/awf-service-bridge.sh >/dev/null 2>&1 &
    disown
fi

# 4. Ensure the inner dockerd is responsive (entrypoint starts it before run.sh;
#    this guards against a daemon that died between jobs).
for i in $(seq 1 30); do
    if docker info >/dev/null 2>&1; then
        log "inner dockerd healthy; environment is clean"
        exit 0
    fi
    sleep 1
done

log "ERROR: inner dockerd is not responding; cannot run job"
exit 1

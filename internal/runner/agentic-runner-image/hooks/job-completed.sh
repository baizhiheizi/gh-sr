#!/bin/bash
# gh-sr job-completed hook — runs after every job's steps finish.
#
# Wired via ACTIONS_RUNNER_HOOK_JOB_COMPLETED (see entrypoint.sh). The Actions
# runner always runs this hook (even on job failure) as a synchronous "Complete
# runner" step. Its job is to tear down the gh-aw / AWF runtime state the job
# created so the NEXT job on this long-lived runner container starts pristine.
#
# This replaces the scattered "leftover from a crashed job" cleanup that used to
# live in entrypoint.sh and the docker-wrapper supervisor.
#
# IMPORTANT:
#   * Always exit 0. A non-zero exit from this hook fails an otherwise-successful
#     job (there is no continue-on-error for runner hooks).
#   * Never remove images / volumes. The inner Docker image-layer cache under
#     /runner-state/docker-data is the persistent cache that lets the next job
#     start without re-pulling gh-aw's (large) images. We only remove containers,
#     unused networks, AWF egress rules, and the /tmp/gh-aw runtime tree.
#   * Scope container removal to gh-aw / AWF names so we do not race the runner's
#     own teardown of workflow `services:` containers. `docker network prune`
#     only touches networks with no attached containers, so it is safe here too.

set +e
export PATH="/opt/gh-sr/docker-shim:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"

log() { echo "[gh-sr:job-completed] $*"; }

log "tearing down gh-aw/AWF runtime state (image cache preserved)..."

# 1. Remove gh-aw / AWF containers by name (MCP gateway, firewall agent/squid/api-proxy).
for filter in 'name=gh-aw-mcpg' 'name=awf-' 'name=gh-aw'; do
    docker ps -aq --filter "$filter" 2>/dev/null | xargs -r docker rm -f >/dev/null 2>&1
done

# 1b. Defense-in-depth: also remove gh-aw / AWF containers by ancestor image, in case one
#     was started without a gh-sr-recognisable name (the name filters above would miss it).
#     This is scoped to gh-aw images only, so it never touches workflow `services:` containers.
for img in ghcr.io/github/gh-aw-mcpg \
           ghcr.io/github/gh-aw-firewall/agent \
           ghcr.io/github/gh-aw-firewall/squid \
           ghcr.io/github/gh-aw-firewall/api-proxy; do
    docker ps -aq --filter "ancestor=$img" 2>/dev/null | xargs -r docker rm -f >/dev/null 2>&1
done

# 2. Prune unused networks (awf-net, github_network_* once their containers are gone).
docker network prune -f >/dev/null 2>&1

# 3. Flush AWF egress rules from the inner netfilter state. DOCKER-USER is recreated
#    by dockerd; flushing (not deleting) it clears stale per-job AWF rules.
if command -v iptables >/dev/null 2>&1; then
    sudo -n iptables -F DOCKER-USER >/dev/null 2>&1
fi

# 4. Remove the gh-aw runtime tree so the next job's setup starts from a clean slate.
sudo -n rm -rf /tmp/gh-aw >/dev/null 2>&1 || rm -rf /tmp/gh-aw >/dev/null 2>&1

log "teardown complete"
exit 0

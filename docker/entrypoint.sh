#!/bin/bash
set -e

# Required env vars:
#   REPO_URL      - e.g. https://github.com/owner/repo
#   ACCESS_TOKEN  - Fine-grained PAT with Administration write on the repo
#   RUNNER_NAME   - Unique name for this runner instance
#   LABELS        - Comma-separated labels, e.g. self-hosted,linux,ci
# Optional:
#   RUNNER_WORKDIR - Work directory name (default: _work)

WORKDIR="${RUNNER_WORKDIR:-_work}"

# Derive API base URL from repo URL
# https://github.com/owner/repo  →  https://api.github.com/repos/owner/repo
REPO_PATH="${REPO_URL#https://github.com/}"
API_BASE="https://api.github.com/repos/${REPO_PATH}"

echo "Registering runner '${RUNNER_NAME}' for ${REPO_URL}..."

# Get a registration token using the PAT
REG_TOKEN=$(curl -s -X POST \
  -H "Authorization: Bearer ${ACCESS_TOKEN}" \
  -H "Accept: application/vnd.github+json" \
  "${API_BASE}/actions/runners/registration-token" \
  | jq -r '.token')

if [[ "$REG_TOKEN" == "null" || -z "$REG_TOKEN" ]]; then
  echo "ERROR: Could not get registration token."
  echo "  Check ACCESS_TOKEN is set and has 'Administration' write permission on ${REPO_URL}"
  exit 1
fi

# Configure the runner
./config.sh \
  --url "$REPO_URL" \
  --token "$REG_TOKEN" \
  --name "$RUNNER_NAME" \
  --labels "$LABELS" \
  --work "$WORKDIR" \
  --unattended \
  --replace

# Gracefully deregister on container shutdown (SIGTERM from docker stop)
cleanup() {
  echo "Deregistering runner '${RUNNER_NAME}'..."
  REMOVE_TOKEN=$(curl -s -X POST \
    -H "Authorization: Bearer ${ACCESS_TOKEN}" \
    -H "Accept: application/vnd.github+json" \
    "${API_BASE}/actions/runners/remove-token" \
    | jq -r '.token')
  ./config.sh remove --token "$REMOVE_TOKEN" || true
  exit 0
}
trap cleanup SIGTERM SIGINT

echo "Runner '${RUNNER_NAME}' registered. Starting..."
./run.sh &
wait $!

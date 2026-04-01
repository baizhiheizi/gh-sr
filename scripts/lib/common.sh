#!/usr/bin/env bash
# Shared helpers for gh-runners scripts

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
CONFIG="$ROOT/config/runners.yml"
COMPOSE_FILE="$ROOT/docker-compose.yml"

# Load .env
load_env() {
  if [[ -f "$ROOT/.env" ]]; then
    set -a
    source "$ROOT/.env"
    set +a
  else
    echo "Error: .env file not found. Copy .env.example to .env and set your GITHUB_PAT." >&2
    exit 1
  fi
}

# Check required tools
check_deps() {
  local missing=()
  for cmd in yq jq curl docker; do
    if ! command -v "$cmd" &>/dev/null; then
      missing+=("$cmd")
    fi
  done
  if [[ ${#missing[@]} -gt 0 ]]; then
    echo "Error: Missing required tools: ${missing[*]}" >&2
    exit 1
  fi
}

# Get a registration token for a repo
# Usage: get_reg_token owner/repo
get_reg_token() {
  local repo="$1"
  local pat_var
  pat_var="$(yq '.github.pat_env_var' "$CONFIG")"
  local pat="${!pat_var}"

  if [[ -z "$pat" ]]; then
    echo "Error: $pat_var is not set in .env" >&2
    return 1
  fi

  local token
  token="$(curl -s -X POST \
    -H "Authorization: Bearer $pat" \
    -H "Accept: application/vnd.github+json" \
    "https://api.github.com/repos/${repo}/actions/runners/registration-token" \
    | jq -r '.token')"

  if [[ "$token" == "null" || -z "$token" ]]; then
    echo "Error: Failed to get registration token for $repo. Check your PAT permissions." >&2
    return 1
  fi

  echo "$token"
}

# List offline runners for a repo and delete them
# Usage: cleanup_offline_runners owner/repo
cleanup_offline_runners() {
  local repo="$1"
  local pat_var
  pat_var="$(yq '.github.pat_env_var' "$CONFIG")"
  local pat="${!pat_var}"

  local runners
  runners="$(curl -s \
    -H "Authorization: Bearer $pat" \
    -H "Accept: application/vnd.github+json" \
    "https://api.github.com/repos/${repo}/actions/runners")"

  local offline_ids
  offline_ids="$(echo "$runners" | jq -r '.runners[] | select(.status == "offline") | .id')"

  if [[ -z "$offline_ids" ]]; then
    echo "  No offline runners for $repo"
    return
  fi

  for id in $offline_ids; do
    local name
    name="$(echo "$runners" | jq -r ".runners[] | select(.id == $id) | .name")"
    echo "  Removing offline runner: $name (id=$id)"
    curl -s -X DELETE \
      -H "Authorization: Bearer $pat" \
      -H "Accept: application/vnd.github+json" \
      "https://api.github.com/repos/${repo}/actions/runners/${id}"
  done
}


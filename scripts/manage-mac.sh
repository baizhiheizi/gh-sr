#!/usr/bin/env bash
# Manage native macOS GitHub Actions runners.
#
# Designed to run ON the Mac (invoked via SSH by the runner CLI).
# Handles download, configuration, start, stop, and removal of runner processes.
#
# Usage:
#   manage-mac.sh --action install --runners-dir <path> --config '<json>' --pat <token>
#   manage-mac.sh --action start   --runners-dir <path>
#   manage-mac.sh --action stop    --runners-dir <path>
#   manage-mac.sh --action remove  --runners-dir <path> --config '<json>' --pat <token>
set -euo pipefail

# --- Argument parsing ---

action=""
runners_dir=""
config_json=""
pat=""

while [[ $# -gt 0 ]]; do
  case "$1" in
    --action)      action="$2";      shift 2 ;;
    --runners-dir) runners_dir="$2"; shift 2 ;;
    --config)      config_json="$2"; shift 2 ;;
    --pat)         pat="$2";         shift 2 ;;
    *) echo "Unknown argument: $1" >&2; exit 1 ;;
  esac
done

if [[ -z "$action" ]]; then
  echo "Error: --action is required (install|start|stop|remove)" >&2
  exit 1
fi

if [[ -z "$runners_dir" ]]; then
  echo "Error: --runners-dir is required" >&2
  exit 1
fi

# --- Dependency check ---

if ! command -v jq &>/dev/null; then
  echo "Error: jq is required but not found. Install it with: brew install jq" >&2
  exit 1
fi

if ! command -v curl &>/dev/null; then
  echo "Error: curl is required but not found." >&2
  exit 1
fi

# --- Helpers ---

detect_arch() {
  local machine
  machine="$(uname -m)"
  case "$machine" in
    x86_64) echo "x64" ;;
    arm64)  echo "arm64" ;;
    *)
      echo "Error: unsupported architecture: $machine" >&2
      exit 1
      ;;
  esac
}

get_latest_version() {
  local version
  version="$(curl -s \
    -H "Accept: application/vnd.github+json" \
    "https://api.github.com/repos/actions/runner/releases/latest" \
    | jq -r '.tag_name | ltrimstr("v")')"

  if [[ -z "$version" || "$version" == "null" ]]; then
    echo "Error: Failed to fetch latest runner version from GitHub API." >&2
    exit 1
  fi
  echo "$version"
}

get_reg_token() {
  local repo="$1"
  local token
  token="$(curl -s -X POST \
    -H "Authorization: Bearer $pat" \
    -H "Accept: application/vnd.github+json" \
    "https://api.github.com/repos/${repo}/actions/runners/registration-token" \
    | jq -r '.token')"

  if [[ -z "$token" || "$token" == "null" ]]; then
    echo "Error: Failed to get registration token for $repo. Check PAT permissions." >&2
    return 1
  fi
  echo "$token"
}

get_remove_token() {
  local repo="$1"
  local token
  token="$(curl -s -X POST \
    -H "Authorization: Bearer $pat" \
    -H "Accept: application/vnd.github+json" \
    "https://api.github.com/repos/${repo}/actions/runners/remove-token" \
    | jq -r '.token')"

  if [[ -z "$token" || "$token" == "null" ]]; then
    echo "Error: Failed to get removal token for $repo. Check PAT permissions." >&2
    return 1
  fi
  echo "$token"
}

# --- Actions ---

install_runners() {
  if [[ -z "$config_json" ]]; then
    echo "Error: --config is required for install" >&2
    exit 1
  fi
  if [[ -z "$pat" ]]; then
    echo "Error: --pat is required for install" >&2
    exit 1
  fi

  local runner_count
  runner_count="$(echo "$config_json" | jq 'length')"
  if [[ "$runner_count" -eq 0 ]]; then
    echo "No Mac runners defined in config."
    return
  fi

  local arch
  arch="$(detect_arch)"
  echo "Detected architecture: $arch"

  echo "Fetching latest runner version..."
  local version
  version="$(get_latest_version)"
  echo "Using runner v${version}"

  local tarball="/tmp/actions-runner-osx-${arch}-${version}.tar.gz"
  local tarball_url="https://github.com/actions/runner/releases/download/v${version}/actions-runner-osx-${arch}-${version}.tar.gz"

  if [[ ! -f "$tarball" ]]; then
    echo "Downloading runner tarball..."
    curl -L -o "$tarball" "$tarball_url"
  else
    echo "Runner tarball already cached at $tarball"
  fi

  mkdir -p "$runners_dir"

  local i
  for i in $(seq 0 $((runner_count - 1))); do
    local runner_name repo count labels
    runner_name="$(echo "$config_json" | jq -r ".[$i].name")"
    repo="$(echo "$config_json" | jq -r ".[$i].repo")"
    count="$(echo "$config_json" | jq -r ".[$i].count // 1")"
    labels="$(echo "$config_json" | jq -r ".[$i].labels | join(\",\")")"

    local n
    for n in $(seq 1 "$count"); do
      local name="${runner_name}-${n}"
      local dir="${runners_dir}/${name}"

      if [[ -f "${dir}/.runner" ]]; then
        echo "  $name: already configured, skipping."
        continue
      fi

      echo "  Installing $name..."
      mkdir -p "$dir"
      tar xz -C "$dir" -f "$tarball"

      echo "  Getting registration token for $repo..."
      local reg_token
      reg_token="$(get_reg_token "$repo")"

      "${dir}/config.sh" --unattended \
        --url "https://github.com/${repo}" \
        --token "$reg_token" \
        --name "$name" \
        --labels "$labels" \
        --work "_work" \
        --replace

      echo "  $name: configured."
    done
  done

  echo "Done."
}

start_runners() {
  if [[ ! -d "$runners_dir" ]] || [[ -z "$(ls -A "$runners_dir" 2>/dev/null)" ]]; then
    echo "No runners installed. Run 'mac-install' first."
    return
  fi

  for dir in "$runners_dir"/*/; do
    [[ -d "$dir" ]] || continue
    local name pid_file
    name="$(basename "$dir")"
    pid_file="${dir}.runner_pid"

    if [[ -f "$pid_file" ]]; then
      local pid
      pid="$(cat "$pid_file")"
      if kill -0 "$pid" 2>/dev/null; then
        echo "  $name: already running (PID $pid)"
        continue
      else
        rm -f "$pid_file"
      fi
    fi

    echo "  Starting $name..."
    # nohup + output redirect ensures the process survives SSH session close
    nohup "${dir}run.sh" > "${dir}runner.log" 2>&1 &
    echo $! > "$pid_file"
    echo "  $name: started (PID $!)"
  done
}

stop_runners() {
  if [[ ! -d "$runners_dir" ]] || [[ -z "$(ls -A "$runners_dir" 2>/dev/null)" ]]; then
    echo "No runners installed."
    return
  fi

  for dir in "$runners_dir"/*/; do
    [[ -d "$dir" ]] || continue
    local name pid_file
    name="$(basename "$dir")"
    pid_file="${dir}.runner_pid"

    if [[ ! -f "$pid_file" ]]; then
      echo "  $name: not running."
      continue
    fi

    local pid
    pid="$(cat "$pid_file")"

    if ! kill -0 "$pid" 2>/dev/null; then
      echo "  $name: not running (stale PID file)."
      rm -f "$pid_file"
      continue
    fi

    echo "  Stopping $name (PID $pid)..."
    kill "$pid"

    # Wait up to 10s for graceful exit, then SIGKILL
    local i
    for i in $(seq 1 10); do
      if ! kill -0 "$pid" 2>/dev/null; then
        break
      fi
      sleep 1
    done
    if kill -0 "$pid" 2>/dev/null; then
      kill -9 "$pid" 2>/dev/null || true
    fi

    rm -f "$pid_file"
    echo "  $name: stopped."
  done
}

remove_runners() {
  if [[ -z "$config_json" ]]; then
    echo "Error: --config is required for remove" >&2
    exit 1
  fi
  if [[ -z "$pat" ]]; then
    echo "Error: --pat is required for remove" >&2
    exit 1
  fi

  if [[ ! -d "$runners_dir" ]] || [[ -z "$(ls -A "$runners_dir" 2>/dev/null)" ]]; then
    echo "No runners installed."
    return
  fi

  # Stop all runners first
  stop_runners

  for dir in "$runners_dir"/*/; do
    [[ -d "$dir" ]] || continue
    local name base_name
    name="$(basename "$dir")"
    # Strip trailing -N to find the runner group name
    base_name="$(echo "$name" | sed 's/-[0-9]*$//')"

    local config_sh="${dir}config.sh"
    if [[ -f "$config_sh" ]]; then
      local repo
      repo="$(echo "$config_json" | jq -r ".[] | select(.name == \"$base_name\") | .repo" 2>/dev/null || true)"
      if [[ -n "$repo" && "$repo" != "null" ]]; then
        echo "  Deregistering $name from $repo..."
        local remove_token
        remove_token="$(get_remove_token "$repo")" || true
        if [[ -n "$remove_token" && "$remove_token" != "null" ]]; then
          "${config_sh}" remove --token "$remove_token" 2>/dev/null || true
        fi
      fi
    fi

    echo "  Removing $name directory..."
    rm -rf "$dir"
  done

  echo "Done."
}

# --- Dispatch ---

case "$action" in
  install) install_runners ;;
  start)   start_runners ;;
  stop)    stop_runners ;;
  remove)  remove_runners ;;
  *)
    echo "Error: unknown action '$action'. Must be one of: install, start, stop, remove" >&2
    exit 1
    ;;
esac

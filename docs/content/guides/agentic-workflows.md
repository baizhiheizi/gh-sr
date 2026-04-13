---
title: "Agentic Workflows"
weight: 20
---

# Preparing a Self-Hosted Runner for GitHub Agentic Workflows (gh-aw)

[GitHub Agentic Workflows](https://github.github.com/gh-aw/) (`gh-aw`) are markdown-based workflow files compiled to GitHub Actions via `gh aw compile`. They run a live AI model inside a sandboxed Docker container that decides what steps to take, what tools to call, and how to respond — rather than executing a fixed YAML script.

## 1. Introduction

Two key runtime components underpin every agentic workflow run:

**AWF (Agent Workflow Firewall)** — manages the agent sandbox container and enforces network egress policy. It writes `iptables` rules directly to the `DOCKER-USER` chain on the host kernel to control what domains the agent container can reach.

**MCP Gateway** (`ghcr.io/github/gh-aw-mcpg`) — runs on the host network (`--network host`) and hosts the MCP servers that give the agent its tools (GitHub API access, safe output handling, etc.). The agent container connects to it at `http://host.docker.internal:80`.

GitHub-hosted `ubuntu-latest` runners have everything pre-installed and pre-configured. Self-hosted runners need explicit preparation because:

- `host.docker.internal` does not exist on Linux by default
- The runner user needs passwordless `sudo` for `iptables`
- `RUNNER_TEMP` must not resolve to `/tmp` (gh-aw writes its runtime tree to `/tmp/gh-aw`)
- Language runtimes, Docker, and the `gh-aw` extension must be installed manually

## 2. System Requirements

| Requirement | Details |
|---|---|
| **OS** | Linux only — Ubuntu/Debian strongly recommended. macOS and Windows are **not supported**. |
| **Architecture** | `amd64` or `arm64` |
| **`sudo` access** | **Mandatory.** AWF writes `iptables` rules to the `DOCKER-USER` chain. ARC configurations with `allowPrivilegeEscalation: false` are explicitly not supported. |
| **`RUNNER_TEMP`** | Must **not** be `/tmp`. gh-aw writes its runtime tree to `/tmp/gh-aw` and the setup script will error if `RUNNER_TEMP` resolves to `/tmp`. |

## 3. Required Software Installation

### 3a. Base system tools

```bash
sudo apt-get update && sudo apt-get install -y \
  curl git jq make ca-certificates unzip tar
```

```bash
git --version && curl --version | head -1 && jq --version && make --version | head -1
```

### 3b. GitHub CLI (`gh`)

```bash
(type -p wget >/dev/null || sudo apt-get install wget -y) \
  && sudo mkdir -p -m 755 /etc/apt/keyrings \
  && out=$(mktemp) && wget -nv -O$out https://cli.github.com/packages/githubcli-archive-keyring.gpg \
  && cat $out | sudo tee /etc/apt/keyrings/githubcli-archive-keyring.gpg > /dev/null \
  && sudo chmod go+r /etc/apt/keyrings/githubcli-archive-keyring.gpg \
  && echo "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/githubcli-archive-keyring.gpg] https://cli.github.com/packages stable main" \
     | sudo tee /etc/apt/sources.list.d/github-cli.list > /dev/null \
  && sudo apt-get update && sudo apt-get install gh -y
```

Verify:

```bash
gh --version         # e.g. gh version 2.x.x
gh auth status       # must show "Logged in to github.com"
```

### 3c. `gh-aw` extension

```bash
curl -fsSL https://raw.githubusercontent.com/github/gh-aw/refs/heads/main/install-gh-aw.sh | bash
```

Verify:

```bash
gh aw version        # e.g. gh-aw version v1.x.x
```

### 3d. Go

The repo requires Go matching `go.mod` (e.g. `1.25.8`). Use `actions/setup-go` in your workflow (recommended) or install manually:

```bash
GO_VERSION=1.25.8
curl -fsSL "https://go.dev/dl/go${GO_VERSION}.linux-amd64.tar.gz" -o /tmp/go.tar.gz
sudo rm -rf /usr/local/go && sudo tar -C /usr/local -xzf /tmp/go.tar.gz
echo 'export PATH=$PATH:/usr/local/go/bin' >> ~/.bashrc && source ~/.bashrc
```

Verify:

```bash
go version           # must match go.mod
```

### 3e. Node.js (minimum v20, v24 recommended)

Use `actions/setup-node` in your workflow or install via nvm:

```bash
curl -o- https://raw.githubusercontent.com/nvm-sh/nvm/v0.39.7/install.sh | bash
source ~/.bashrc && nvm install 24 && nvm alias default 24
```

Verify:

```bash
node --version       # v24.x.x
```

### 3f. `uv` (Python)

```bash
curl -LsSf https://astral.sh/uv/install.sh | sh
source ~/.bashrc
```

Verify:

```bash
uv --version
```

### 3g. Docker

```bash
sudo apt-get install -y docker.io
sudo systemctl enable --now docker
sudo usermod -aG docker $USER
newgrp docker
```

Verify:

```bash
docker run hello-world
```

### Software summary

| Tool | Minimum version | Install method | Verify |
|---|---|---|---|
| `gh` (GitHub CLI) | latest | official apt repo | `gh --version` + `gh auth status` |
| `gh-aw` extension | latest | `curl … install-gh-aw.sh \| bash` | `gh aw version` |
| Go | matching `go.mod` | tarball or `actions/setup-go` | `go version` |
| Node.js | v20 (v24 recommended) | nvm or `actions/setup-node` | `node --version` |
| `uv` | latest | astral installer or `astral-sh/setup-uv` | `uv --version` |
| Docker | latest stable | `docker.io` apt package | `docker run hello-world` |
| `make`, `git`, `jq`, `curl` | any | apt | `--version` checks |

## 4. Critical Docker Configuration

### 4a. Runner user in the `docker` group

The MCP gateway (`gh-aw-mcpg`) runs with `-v /var/run/docker.sock:/var/run/docker.sock` to spawn MCP server containers. The runner user must be in the `docker` group:

```bash
sudo usermod -aG docker $USER
# Log out and back in, or run:
newgrp docker
```

Verify:

```bash
docker ps   # must not say "permission denied"
```

### 4b. `host.docker.internal` DNS — the most common failure point

On GitHub-hosted runners (Docker Desktop on macOS/Windows), `host.docker.internal` works automatically. On Linux self-hosted runners **it does not exist by default**.

The MCP gateway is reached by the agent container via `http://host.docker.internal:80`. The `gh aw compile` step automatically rewrites all `localhost`/`127.0.0.1` MCP URLs to `host.docker.internal`, so this name must resolve correctly.

**Why `/etc/hosts: 127.0.0.1 host.docker.internal` fails**

Adding `127.0.0.1` breaks the connection because the agent container has its own `127.0.0.1` (its own loopback). It would connect to its own port 80, not the host's MCP gateway. The correct IP is the **Docker bridge gateway** — the host's address on the container network.

**Fix: add the bridge gateway IP to `/etc/hosts`**

```bash
echo "$(docker inspect bridge --format='{{(index .IPAM.Config 0).Gateway}}')  host.docker.internal" \
  | sudo tee -a /etc/hosts
```

Verify on the host:

```bash
getent hosts host.docker.internal
# Expected: 172.17.0.1  host.docker.internal  (not 127.0.0.1)
```

Verify from inside a container:

```bash
docker run --rm --add-host=host.docker.internal:host-gateway alpine \
  sh -c "getent hosts host.docker.internal"
# Expected: 172.17.0.1  host.docker.internal
```

If you use `gh sr setup` with `profile: agentic`, this DNS configuration is handled automatically — see [§9 What `profile: agentic` automates](#9-what-profile-agentic-automates) for details.

### 4c. Docker socket access for the MCP Gateway

The MCP Gateway (`ghcr.io/github/gh-aw-mcpg`) runs on the host network and spawns MCP server containers via the Docker socket. This is **Docker-outside-of-Docker** (DooD) — the gateway container accesses the **host's** Docker daemon, not an inner Docker daemon.

```
MCP Gateway container (host network)
  → /var/run/docker.sock
  → spawns sibling MCP server containers on the host's network
```

Verify the socket is accessible from inside a container:

```bash
docker run --rm -v /var/run/docker.sock:/var/run/docker.sock docker:cli docker ps
# Expected: a table of running containers (or an empty table), no permission error
```

### 4d. Note on the Codex engine

The Codex engine (`convert_gateway_config_codex.sh`) bypasses DNS entirely and hardcodes `172.30.0.1` (the AWF bridge gateway IP) to avoid Rust DNS resolution issues. No additional DNS configuration is needed for the Codex engine specifically.

## 5. `sudo` + `iptables` Setup

AWF applies host-level `iptables` rules. The runner user needs passwordless `sudo` for `iptables`:

```bash
echo "$(whoami) ALL=(ALL) NOPASSWD: /usr/sbin/iptables, /usr/sbin/ip6tables" \
  | sudo tee /etc/sudoers.d/runner-iptables
```

Verify (must not prompt for a password):

```bash
sudo -n iptables -L DOCKER-USER -n
```

## 6. `RUNNER_TEMP` Configuration

gh-aw writes its runtime tree to `/tmp/gh-aw`. If `RUNNER_TEMP` is `/tmp`, the setup script will error. Set a different path:

```bash
# Add to ~/actions-runner/.env
echo "RUNNER_TEMP=/home/runner/_temp" >> ~/actions-runner/.env
mkdir -p /home/runner/_temp
```

Verify:

```bash
grep RUNNER_TEMP ~/actions-runner/.env
# Expected: RUNNER_TEMP=/home/runner/_temp
```

## 7. Network Configuration

### 7a. Outbound HTTPS domains the runner must reach

| Category | Domains |
|---|---|
| GitHub core | `github.com`, `api.github.com`, `*.actions.githubusercontent.com` |
| Go modules | `proxy.golang.org`, `sum.golang.org` |
| npm | `registry.npmjs.org` |
| Containers | `ghcr.io` |
| AI endpoints (engine-dependent) | `*.githubcopilot.com`, `api.anthropic.com`, `api.openai.com`, `generativelanguage.googleapis.com` |

### 7b. Workflow `network.allowed` configuration

`host.docker.internal` is automatically included in the `defaults` bundle — do not add it manually. Always include `- defaults`:

```yaml
network:
  firewall: true
  allowed:
    - defaults
    - "api.example.com"
```

### 7c. Copilot engine: use MCP toolsets instead of direct API access

The Copilot engine cannot use `api.github.com` directly. Use the GitHub MCP server via toolsets:

```yaml
engine: copilot
tools:
  github:
    toolsets: [default]
```

## 8. Workflow Configuration for Self-Hosted Runners

```yaml
---
on: issues
runs-on: [self-hosted, linux, x64]
runs-on-slim: self-hosted   # controls framework jobs (activation, safe-outputs, etc.)
safe-outputs:
  create-issue: {}
  threat-detection:
    runs-on: ubuntu-latest  # optionally run threat detection on GitHub-hosted
---
```

## 9. What `profile: agentic` automates

gh-sr handles the following during `gh sr setup` for runners with `profile: agentic` on Linux:

| Step | What gh-sr does | Why |
|------|----------------|-----|
| **Docker DNS (dnsmasq)** | Installs dnsmasq, writes `/etc/dnsmasq.d/gh-sr-docker.conf`, merges DNS into `/etc/docker/daemon.json`, restarts services | Agent containers must resolve `host.docker.internal` to reach the MCP Gateway; external DNS must work for model API calls |
| **dnsmasq config** | Listens on docker0 bridge IP, resolves `host.docker.internal` statically, forwards all other queries upstream with `server=127.0.0.53` and `server=8.8.8.8` | Without upstream `server=` directives dnsmasq refuses all non-static queries, breaking model API connectivity |
| **`/opt/hostedtoolcache`** | Bind-mounts npm global prefix to `/opt/hostedtoolcache`, persists in `/etc/fstab` | gh-aw agent containers look for engine binaries (`claude`, `codex`, etc.) in `/opt/hostedtoolcache/*/bin` |
| **`gh-aw` CLI** | Installs from upstream script (`curl \| bash`) | Provides CLI tooling for managing AWF containers |
| **`agentic` label** | Appends `agentic` to runner labels | Routes agentic workflow jobs to this runner |

## 10. End-to-End Verification Script

Save as `/tmp/verify-aw-runner.sh` and run on the runner host:

```bash
#!/bin/bash
set -euo pipefail
PASS=0; FAIL=0

check() {
  local label="$1"; shift
  if "$@" > /dev/null 2>&1; then
    echo "✓ $label"; ((PASS++)) || true
  else
    echo "✗ $label"; ((FAIL++)) || true
  fi
}

echo "=== gh-aw Self-Hosted Runner Verification ==="
echo ""

# 1. RUNNER_TEMP is not /tmp
echo "--- Prerequisites ---"
if [ "${RUNNER_TEMP:-}" = "/tmp" ] || [ "${RUNNER_TEMP:-}" = "" ]; then
  echo "✗ RUNNER_TEMP (value: '${RUNNER_TEMP:-<unset>}') — must not be /tmp"
  ((FAIL++)) || true
else
  echo "✓ RUNNER_TEMP=${RUNNER_TEMP}"
  ((PASS++)) || true
fi

# 2. Docker daemon accessible
check "Docker daemon accessible" docker info

# 3. Docker socket accessible
check "Docker socket accessible" test -S /var/run/docker.sock

# 4. host.docker.internal resolves on host
echo ""
echo "--- DNS ---"
check "host.docker.internal resolves on host" getent hosts host.docker.internal

HDKI_IP=$(getent hosts host.docker.internal 2>/dev/null | awk '{print $1}')
if [ "$HDKI_IP" = "127.0.0.1" ] || [ "$HDKI_IP" = "::1" ]; then
  echo "  ⚠ WARNING: resolves to loopback ($HDKI_IP) — containers will connect to themselves, not the host"
fi

# 5. host.docker.internal resolves from inside a container
check "host.docker.internal resolves inside container" \
  docker run --rm --add-host=host.docker.internal:host-gateway alpine \
    sh -c "getent hosts host.docker.internal | grep -v '^127\\.'"

# 6. Docker-in-Docker works
echo ""
echo "--- Docker-in-Docker ---"
check "DinD: can spawn containers via socket" \
  docker run --rm -v /var/run/docker.sock:/var/run/docker.sock docker:cli docker ps

# 7. sudo iptables works without password
echo ""
echo "--- sudo / iptables ---"
check "sudo iptables without password prompt" sudo -n iptables -L DOCKER-USER -n

# 8. Simulated mcpg reachability
echo ""
echo "--- MCP Gateway reachability simulation ---"
# Start a temporary HTTP listener on a non-privileged port on the host.
# (Port 80 requires root; we use 8080 here to test host.docker.internal routing.)
if command -v python3 > /dev/null 2>&1; then
  python3 -m http.server 8080 --bind 0.0.0.0 > /tmp/http-test.log 2>&1 &
  HTTP_PID=$!
  trap 'kill $HTTP_PID 2>/dev/null || true' EXIT
  sleep 1
  if docker run --rm --add-host=host.docker.internal:host-gateway alpine \
       sh -c "wget -qO- http://host.docker.internal:8080/" > /dev/null 2>&1; then
    echo "✓ Container can reach host port 8080 via host.docker.internal"
    ((PASS++)) || true
  else
    echo "✗ Container cannot reach host port 8080 via host.docker.internal"
    ((FAIL++)) || true
  fi
  kill $HTTP_PID 2>/dev/null || true
  trap - EXIT
else
  echo "  (skipped: python3 not available for HTTP listener test)"
fi

# 9. Outbound HTTPS from inside containers
echo ""
echo "--- Outbound HTTPS from containers ---"
for url in \
  "https://github.com" \
  "https://api.github.com" \
  "https://registry.npmjs.org" \
  "https://ghcr.io"; do
  if docker run --rm alpine sh -c "apk add --no-cache curl > /dev/null 2>&1 && curl -sf '$url' > /dev/null 2>&1"; then
    echo "✓ $url"
    ((PASS++)) || true
  else
    echo "✗ $url"
    ((FAIL++)) || true
  fi
done

echo ""
echo "=== Results: $PASS passed, $FAIL failed ==="
[ "$FAIL" -eq 0 ] && echo "✅ Runner is ready for agentic workflows." || echo "❌ Fix the failures above before running agentic workflows."
```

Run it:

```bash
bash /tmp/verify-aw-runner.sh
```

### Automatic diagnostics

`gh sr doctor` performs the most critical checks automatically:

```bash
gh sr doctor --host your-runner
```

Checks performed:
- Docker CLI and daemon are available
- `docker compose` plugin is available
- `iptables` is available and `DOCKER-USER` chain exists
- Passwordless `sudo` is available for `iptables` rules
- `host.docker.internal` resolves to a non-loopback IP inside containers
- External DNS (`github.com`) resolves inside containers

## 11. Quick Reference Troubleshooting

| Symptom | Root cause | Fix |
|---|---|---|
| MCP tool calls silently fail | `host.docker.internal` not in `/etc/hosts` | Add it pointing to Docker bridge gateway IP (see [§4b](#4b-hostdockerinternal-dns--the-most-common-failure-point)) |
| AWF fails to start | `sudo iptables` requires a password | Add passwordless sudoers rule for iptables (see [§5](#5-sudo--iptables-setup)) |
| Setup errors with "RUNNER_TEMP resolves to /tmp" | `RUNNER_TEMP=/tmp` | Set `RUNNER_TEMP=/home/runner/_temp` in `.env` (see [§6](#6-runner_temp-configuration)) |
| mcpg can't spawn MCP server containers | Runner user not in `docker` group | `sudo usermod -aG docker runner` |
| Copilot engine can't access GitHub API | Direct `api.github.com` access attempted | Use GitHub MCP toolsets instead (see [§7c](#7c-copilot-engine-use-mcp-toolsets-instead-of-direct-api-access)) |
| `localhost` MCP URLs broken with firewall | Docker networking isolation | Already handled by compiler rewriting to `host.docker.internal` — no action needed |
| 503 from model API inside container | dnsmasq has no upstream `server=` directives | `gh sr setup` fixes this; manually add `server=127.0.0.53` + `server=8.8.8.8` to dnsmasq config |
| `ERR_API: MCP server(s) failed to launch` | `host.docker.internal` returns NXDOMAIN | Run `gh sr setup` (`profile: agentic`) or manually configure dnsmasq |

## Files created on the host

When you run `gh sr setup` for an agentic runner, gh-sr may create or modify these files:

| File | Purpose |
|------|---------|
| `/etc/dnsmasq.d/gh-sr-docker.conf` | dnsmasq config for `host.docker.internal` + upstream DNS forwarding |
| `/etc/docker/daemon.json` | Docker daemon DNS settings (merged with existing config) |
| `/opt/hostedtoolcache` | Bind mount to your npm global prefix |
| `/etc/fstab` | Persistent entry for the `/opt/hostedtoolcache` bind mount |
| `~/.local/share/gh/extensions/gh-aw/` | The `gh-aw` CLI installation directory |

```bash
# Inspect dnsmasq config
cat /etc/dnsmasq.d/gh-sr-docker.conf

# Inspect Docker DNS settings
cat /etc/docker/daemon.json
```

---

## title: "Agentic Workflows"
weight: 20

# Preparing a Self-Hosted Runner for GitHub Agentic Workflows (gh-aw)

[GitHub Agentic Workflows](https://github.github.com/gh-aw/) (`gh-aw`) are markdown-based workflow files compiled to GitHub Actions via `gh aw compile`. They run a live AI model inside a sandboxed Docker container that decides what steps to take, what tools to call, and how to respond — rather than executing a fixed YAML script.

## 1. Introduction

Two key runtime components underpin every agentic workflow run:

**AWF (Agent Workflow Firewall)** — manages the agent sandbox container and enforces network egress policy. It writes `iptables` rules directly to the `DOCKER-USER` chain on the host kernel to control what domains the agent container can reach.

**MCP Gateway** (`ghcr.io/github/gh-aw-mcpg`) — runs on the host network (`--network host`) and hosts the MCP servers that give the agent its tools (GitHub API access, safe output handling, etc.). Native runners expose it to the agent container at `http://host.docker.internal:80`; container-mode runners keep the gateway config schema-valid, then rewrite Claude's generated MCP URLs to the AWF bridge gateway IP to avoid proxying local MCP traffic.

GitHub-hosted `ubuntu-latest` runners have everything pre-installed and pre-configured. Self-hosted runners need explicit preparation because:

- `host.docker.internal` does not exist on Linux by default
- The runner user needs passwordless `sudo` for `iptables`
- `RUNNER_TEMP` must not resolve to `/tmp` (gh-aw writes its runtime tree to `/tmp/gh-aw`)
- Language runtimes, Docker, and the `gh-aw` extension must be installed manually

## 2. System Requirements


| Requirement       | Details                                                                                                                                                        |
| ----------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| **OS**            | Linux only — Ubuntu/Debian strongly recommended. macOS and Windows are **not supported**.                                                                      |
| **Architecture**  | `amd64` or `arm64`                                                                                                                                             |
| `**sudo` access** | **Mandatory.** AWF writes `iptables` rules to the `DOCKER-USER` chain. ARC configurations with `allowPrivilegeEscalation: false` are explicitly not supported. |
| `**RUNNER_TEMP`** | Must **not** be `/tmp`. gh-aw writes its runtime tree to `/tmp/gh-aw` and the setup script will error if `RUNNER_TEMP` resolves to `/tmp`.                     |


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


| Tool                        | Minimum version       | Install method                           | Verify                            |
| --------------------------- | --------------------- | ---------------------------------------- | --------------------------------- |
| `gh` (GitHub CLI)           | latest                | official apt repo                        | `gh --version` + `gh auth status` |
| `gh-aw` extension           | latest                | `curl … install-gh-aw.sh | bash`         | `gh aw version`                   |
| Go                          | matching `go.mod`     | tarball or `actions/setup-go`            | `go version`                      |
| Node.js                     | v20 (v24 recommended) | nvm or `actions/setup-node`              | `node --version`                  |
| `uv`                        | latest                | astral installer or `astral-sh/setup-uv` | `uv --version`                    |
| Docker                      | latest stable         | `docker.io` apt package                  | `docker run hello-world`          |
| `make`, `git`, `jq`, `curl` | any                   | apt                                      | `--version` checks                |


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


| Category                        | Domains                                                                                           |
| ------------------------------- | ------------------------------------------------------------------------------------------------- |
| GitHub core                     | `github.com`, `api.github.com`, `*.actions.githubusercontent.com`                                 |
| Go modules                      | `proxy.golang.org`, `sum.golang.org`                                                              |
| npm                             | `registry.npmjs.org`                                                                              |
| Containers                      | `ghcr.io`                                                                                         |
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

### 8a. Concurrent jobs, MCP gateway ports, and gh-sr *(native mode only)*

> **This section applies to `runner_mode: native` (the default).** If you use `runner_mode: container`, you do not need to configure ports or labels — see [§8b](#8b-runner-modes-native-vs-container-recommended-for-concurrency).

The gh-aw MCP gateway listens on the host network (`docker --network host`). The TCP port comes from workflow frontmatter `sandbox.mcp.port` (default **80** after `gh aw compile`). If several agentic jobs run on the **same physical host**, they must use **different** ports, or only one such job may run at a time.

For **multiple concurrent agentic runner instances on one host**, prefer `[runner_mode: container](#8b-runner-modes-native-vs-container-recommended-for-concurrency)` instead of juggling ports and labels in native mode: each runner container gets its own network namespace and MCP gateway on port 80 without conflicts (see §8b).

**Configure per-instance ports in gh-sr** (`runners.yml`): on a runner with `profile: agentic`, set either:

- `agentic_mcp_port_base: 9080` — instances get ports `9080`, `9081`, … up to `count-1`, or
- `agentic_mcp_ports: [9080, 9081]` — explicit list whose length must equal `count`.

gh-sr then registers each instance with an extra label `**gh-sr-mcp-<port>*`* (for example `gh-sr-mcp-9080`) so GitHub can route jobs to a runner whose MCP port matches the workflow.

Match the workflow frontmatter to that port and label:

```yaml
features:
  mcp-gateway: true
sandbox:
  mcp:
    port: 9080
runs-on:
  - self-hosted
  - Linux
  - X64
  - agentic
  - gh-sr-mcp-9080
```

**Operational checks:** `gh sr doctor` validates config, GitHub API access, and host prerequisites (including agentic/container tooling). It does **not** scan workflow markdown for MCP ports anymore.

For same-host concurrency without hand-maintaining distinct `sandbox.mcp.port` values per workflow, use `**runner_mode: container`** (see §8b). If you stay on native mode with `count > 1`, align ports, `gh-sr-mcp-*` labels, and `runs-on` yourself and re-run `gh aw compile` after editing frontmatter; see [gh-aw Sandbox configuration](https://github.github.com/gh-aw/reference/sandbox/) for `sandbox.mcp` and `features.mcp-gateway`.

**Limitation:** two concurrent jobs that use the **same compiled workflow** (same `sandbox.mcp.port`) on one host still conflict; fixing that requires different workflow sources, separate machines, limiting concurrency, or changes in gh-aw.

### 8b. Runner modes: native vs container (recommended for concurrency)

`runner_mode: container` is available for **any** Linux runner definition: with `profile: agentic` you get gh-aw host setup (§9) plus the same container image; **without** `profile: agentic` you still get a self-contained actions runner in DinD — useful when you want per-instance Docker and workspace isolation without agentic workflows. Both cases use the **same** built image (`gh-sr/agentic-runner:<actions-runner-version>` — the name is historical; the image is the generic container runner). If you set global `container_runner_image.extra_apt_packages` in `runners.yml`, the tag becomes `gh-sr/agentic-runner:<version>-x<8-hex>` so Docker does not reuse an image built without those packages.

For **agentic** workflows specifically, gh-aw hardcodes `/tmp/gh-aw` in compiled `.lock.yml` files (~80 references per workflow: prompt files, agent stdio logs, step summaries, MCP payloads/logs, firewall logs, and the `-v /tmp/gh-aw:/tmp/gh-aw:rw` Docker bind mount). When multiple agentic jobs run simultaneously on the **same host** in native mode, they overwrite each other's files at this path.

`runner_mode: native` (the default) requires you to manage this conflict with the per-instance MCP port labeling described in [§8a](#8a-concurrent-jobs-mcp-gateway-ports-and-gh-sr). `RUNNER_TEMP` is already isolated per runner instance by gh-sr.

`runner_mode: container` (opt-in) resolves all `/tmp/gh-aw` conflicts for agentic jobs by running each runner instance in its own **privileged Docker container with an inner dockerd** (Docker-in-Docker). Every container has:


| Resource               | Native mode                                    | Container mode                                                          |
| ---------------------- | ---------------------------------------------- | ----------------------------------------------------------------------- |
| `/tmp/gh-aw`           | **shared** — jobs overwrite each other         | **isolated** per container filesystem                                   |
| `iptables` / AWF rules | **shared** host netfilter state                | isolated per container network namespace                                |
| MCP gateway port 80    | **shared** — conflicts on multi-instance hosts | isolated (`--network host` inside container = that container's network) |
| `RUNNER_TEMP`          | isolated per runner instance                   | isolated per container                                                  |
| Docker image cache     | shared host Docker                             | per-container (stored in bind-mounted state dir)                        |


In container mode, `agentic_mcp_ports` / `agentic_mcp_port_base` and the `gh-sr-mcp-<port>` label trick are **not needed**. Each container runs its own MCP gateway on port 80 without conflict.

**Example `runners.yml`:**

```yaml
runners:
  - name: my-agentic
    repo: owner/repo
    host: my-linux-host
    count: 3               # 3 concurrent agentic jobs
    profile: agentic
    runner_mode: container # enables DinD isolation
    # No agentic_mcp_ports needed — port 80 is isolated per container
```

gh-sr will build the runner container image locally on first `gh sr setup` (Ubuntu 24.04 + Docker CE + gh-aw + dnsmasq + actions runner). Core apt dependencies live in the repo manifest `internal/runner/agentic-runner-image/apt-packages-core.txt` (baked into the gh-sr binary). Optional tools: add a global `container_runner_image.extra_apt_packages` list in `runners.yml` (Debian package names only). Subsequent setups skip the build if the image for that tag already exists; after changing the extra list or upgrading gh-sr when the core manifest changed, run `gh sr rebuild` for container-mode runners if needed.

In `gh sr status` (and the dashboard), the **IMAGE** column shows the Docker image reference; its tag is the **GitHub Actions runner** package version (not the gh-sr CLI version). The **BUILD** column compares a short **layout revision** baked into the image at `docker build` time with the revision your current `gh`/`gh sr` binary expects: **ok (…)** means the running container’s image matches this gh-sr build; **stale (…)** means you should run `gh sr rebuild` (or rebuild after upgrading gh-sr); **?** means the image predates revision labels (rebuild once to stamp them).

**When to use which mode:**

- `**runner_mode: native`** — default; suitable when you run only one agentic job at a time per host, or already manage port assignments via [§8a](#8a-concurrent-jobs-mcp-gateway-ports-and-gh-sr). Zero extra disk space beyond the runner binaries.
- `**runner_mode: container**` — use for concurrent **agentic** jobs on one host without MCP port labels, **or** for ordinary CI when you want each runner instance in its own DinD sandbox (no `profile: agentic` required). The image includes Docker CE, dnsmasq, and the actions runner; gh-aw is pre-installed inside the image but is only needed for agentic jobs.

#### Inner Docker CLI shim (container mode only)

Same-host **concurrency** comes from `runner_mode: container` itself (each outer `gh-sr-<instance>` container has its own inner `dockerd`, network namespace, MCP gateway on port 80, and `/tmp/gh-aw`). That isolation is **not** implemented by the Docker shim.

The container image installs `internal/runner/agentic-runner-image/docker-wrapper.sh` as `**/opt/gh-sr/docker-shim/docker`** and puts that directory first on the **runner** user's `PATH` (`/etc/profile.d/gh-sr-docker-shim.sh`, `entrypoint.sh` `RUNNER_ENV`, and `Defaults:runner secure_path=…` so `sudo -E awf` still sees the shim). Root and the entrypoint continue to use the real `**/usr/bin/docker`** for `docker info` / `dockerd` startup. The shim only adjusts selected `docker run`/`create` invocations for `gh-aw-mcpg` and `gh-aw-firewall/agent` images — see the script header in-repo for the full rationale.

## 9. What `profile: agentic` automates

gh-sr handles the following during `gh sr setup` for runners with `profile: agentic` on Linux:


| Step                       | What gh-sr does                                                                                                                                             | Why                                                                                                                                       |
| -------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------- |
| **Docker DNS (dnsmasq)**   | Installs dnsmasq, writes `/etc/dnsmasq.d/gh-sr-docker.conf`, merges DNS into `/etc/docker/daemon.json`, restarts services                                   | Agent containers must resolve `host.docker.internal` to reach the MCP Gateway; external DNS must work for model API calls                 |
| **dnsmasq config**         | Listens on docker0 bridge IP, resolves `host.docker.internal` statically, forwards all other queries upstream with `server=127.0.0.53` and `server=8.8.8.8` | Without upstream `server=` directives dnsmasq refuses all non-static queries, breaking model API connectivity                             |
| `**/opt/hostedtoolcache`** | Bind-mounts npm global prefix to `/opt/hostedtoolcache`, persists in `/etc/fstab`                                                                           | gh-aw agent containers look for engine binaries (`claude`, `codex`, etc.) in `/opt/hostedtoolcache/*/bin`                                 |
| `**gh-aw` CLI**            | Installs from upstream script (`curl | bash`)                                                                                                               | Provides CLI tooling for managing AWF containers                                                                                          |
| `**agentic` label**        | Appends `agentic` to runner labels                                                                                                                          | Routes agentic workflow jobs to this runner                                                                                               |
| `**gh-sr-mcp-`* labels**   | When `agentic_mcp_port_base` or `agentic_mcp_ports` is set, each instance gets `gh-sr-mcp-<port>`                                                           | Lets `runs-on` target a runner whose MCP listener matches `sandbox.mcp.port` (see [§8a](#8a-concurrent-jobs-mcp-gateway-ports-and-gh-sr)) |


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

# 5b. Same from a host-network container (gh-aw-mcpg uses --network host)
echo ""
echo "--- host.docker.internal (host-network, MCP gateway style) ---"
HNET_IP=$(docker run --rm --network host alpine getent hosts host.docker.internal 2>/dev/null | awk '{print $1; exit}')
if [ -z "$HNET_IP" ] || [ "$HNET_IP" = "127.0.0.1" ] || [ "$HNET_IP" = "::1" ]; then
  echo "✗ host.docker.internal from --network host container (got: '${HNET_IP:-<empty>}')"
  ((FAIL++)) || true
else
  echo "✓ host.docker.internal from --network host: $HNET_IP"
  ((PASS++)) || true
fi

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
- `host.docker.internal` resolves to a non-loopback IP inside default-bridge containers
- The same name resolves from a `--network host` container (MCP gateway style); **warns** if not
- Inner `docker run` with the same `--add-host=host.docker.internal:<target>` logic as the shim (explicit `GH_SR_AWF_BRIDGE_GATEWAY_IP`, else resolved `host.docker.internal`, else `host-gateway`) can reach an HTTP probe on the runner (mirrors AWF agent reachability to host-published ports and the MCP gateway)
- External DNS (`github.com`) resolves inside containers

## 11. Quick Reference Troubleshooting


| Symptom                                                                                 | Root cause                                                                                                                                                                                                                                                                                    | Fix                                                                                                                                                                                                                                                                                                                                                                                                                          |
| --------------------------------------------------------------------------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| MCP tool calls silently fail                                                            | `host.docker.internal` not in `/etc/hosts`                                                                                                                                                                                                                                                    | Add it pointing to Docker bridge gateway IP (see [§4b](#4b-hostdockerinternal-dns--the-most-common-failure-point))                                                                                                                                                                                                                                                                                                           |
| AWF fails to start                                                                      | `sudo iptables` requires a password                                                                                                                                                                                                                                                           | Add passwordless sudoers rule for iptables (see [§5](#5-sudo--iptables-setup))                                                                                                                                                                                                                                                                                                                                               |
| Setup errors with "RUNNER_TEMP resolves to /tmp"                                        | `RUNNER_TEMP=/tmp`                                                                                                                                                                                                                                                                            | Set `RUNNER_TEMP=/home/runner/_temp` in `.env` (see [§6](#6-runner_temp-configuration))                                                                                                                                                                                                                                                                                                                                      |
| mcpg can't spawn MCP server containers                                                  | Runner user not in `docker` group                                                                                                                                                                                                                                                             | `sudo usermod -aG docker runner`                                                                                                                                                                                                                                                                                                                                                                                             |
| Copilot engine can't access GitHub API                                                  | Direct `api.github.com` access attempted                                                                                                                                                                                                                                                      | Use GitHub MCP toolsets instead (see [§7c](#7c-copilot-engine-use-mcp-toolsets-instead-of-direct-api-access))                                                                                                                                                                                                                                                                                                                |
| `localhost` MCP URLs broken with firewall                                               | Docker networking isolation                                                                                                                                                                                                                                                                   | Already handled by compiler rewriting to `host.docker.internal` — no action needed                                                                                                                                                                                                                                                                                                                                           |
| 503 from model API inside container                                                     | dnsmasq has no upstream `server=` directives                                                                                                                                                                                                                                                  | `gh sr setup` fixes this; manually add `server=127.0.0.53` + `server=8.8.8.8` to dnsmasq config                                                                                                                                                                                                                                                                                                                              |
| `ERR_API: MCP server(s) failed to launch` (job step **Parse agent logs**)               | Often `host.docker.internal` / DNS from **host-network** containers, AWF mount permissions, or stale inner Docker state                                                                                                                                                                       | Run `gh sr doctor` (native: host checks; container mode: inner Docker reachability checks), then follow [§11a](#11a-diagnosing-parse-agent-logs--mcp_servers-failed-inside-awf)                                                                                                                                                                                                                                              |
| Claude init shows `"mcp_servers":[{"name":"github","status":"failed"},…]`               | MCP clients could not start inside the AWF sandbox (network, token, or mounts)                                                                                                                                                                                                                | Same as row above; collect **Execute Claude Code CLI** + **Print firewall logs** output                                                                                                                                                                                                                                                                                                                                      |
| `[entrypoint][WARN] Failed to transfer …/safeoutputs ownership to chroot user`          | AWF could not `chown` the bind-mounted `gh-aw/safeoutputs` tree for the sandbox user                                                                                                                                                                                                          | This warning can appear on **successful** runs too; treat it as noise unless MCP actually fails. If Claude init lists MCP servers as failed, focus on gateway reachability from the **agent** container (see §11a). Otherwise: ensure passwordless `sudo` for AWF entrypoint expectations; avoid root-only files under `${RUNNER_TEMP}/gh-aw`; upgrade gh-aw/AWF if a release notes a fix                                    |
| **Stop MCP Gateway** exits `1` after `Gateway shutdown initiated` / `serversTerminated` | `gh-aw`’s `stop_mcp_gateway.sh` waits only ~10s for the **same PID** as **Start MCP Gateway** to disappear; the MCP gateway can still be tearing down child MCP containers for longer. On `runner_mode: container`, that PID is gh-sr’s inner `docker` wrapper, not only the gateway process. | Usually **harmless** if the step has `continue-on-error: true` (compiled workflows do). A **stale** gateway on inner `--network host` port **80** can still break the **next** job on the same runner instance. **Rebuild** the agentic runner image (`gh sr rebuild <runner>`) so the wrapper assigns `gh-aw-mcpg-ghsr-`* + `--cidfile` and can run `docker rm -f` when upstream falls back to signalling the recorded PID. |


### 11a. Diagnosing **Parse agent logs** / `mcp_servers` failed inside AWF

GitHub may show the **agent** job as failed on **Parse agent logs for step summary** even when **Execute Claude Code CLI** is green. That step scans `agent-stdio.log` for fatal markers such as:

```text
ERR_API: MCP server(s) failed to launch: github, safeoutputs
```

Upstream, Claude Code’s JSON init line can list both servers as `"status":"failed"`, and tool calls such as `create_pull_request` or `Skill(safeoutputs)` then return “no such tool”.

**Correlate evidence in this order:**

1. **Host / DNS (MCP gateway)** — Safe Outputs is reached at `http://host.docker.internal:$GH_AW_SAFE_OUTPUTS_PORT`; the gateway uses `--network host`. On the runner host run:
  ```bash
   getent hosts host.docker.internal
   docker run --rm --network host alpine sh -c "getent hosts host.docker.internal"
  ```
   Both must resolve to the **docker bridge gateway**, not `127.0.0.1`. `gh sr doctor` now warns when the second check fails while default-bridge DNS still works.
2. **Distinguish gateway health vs agent reachability** — If **Start MCP Gateway** / gateway health checks are green but Claude’s init JSON shows `"mcp_servers":[{"name":"github","status":"failed"},…]`, both tools often fail together because they share the same gateway URL. In native mode this is usually `http://host.docker.internal:80/...`; in container-mode runner images gh-sr may rewrite generated `mcp-servers.json` URLs from `http://host.docker.internal:<port>/mcp/...` to the same port on a **concrete IPv4** resolved at runtime (override `GH_SR_AWF_BRIDGE_GATEWAY_IP`, else `getent hosts host.docker.internal`, else URL rewrite is skipped when only `host-gateway` applies) so Claude does not have to honor `NO_PROXY` for local MCP traffic. The break is usually the **AWF agent container → gateway** hop, not the gateway processes themselves.
3. **AWF entrypoint** — In the same job log, search for `Failed to transfer` and `safeoutputs`. That line can appear on successful runs; only treat it as primary evidence when it lines up with real MCP/tool failures **and** mount permission errors.
4. **Firewall / MCP gateway** — Use the workflow’s **Print firewall logs** (and MCP gateway debug if enabled) to confirm the gateway spawned child MCP containers and that egress allowlists include `host.docker.internal`.
5. **Workflow bundle** — If the host matches [§10](#10-end-to-end-verification-script) but jobs still fail, recompile workflows with a current `gh aw` (`gh aw compile` in the repo) so lockfiles pick up upstream gh-aw / AWF fixes.

**Observed correlation (example):** In one failing **agent** job, **Start MCP Gateway** set `MCP_GATEWAY_DOMAIN=host.docker.internal` (with `gh-aw-mcpg` on `--network host`) while **Execute Claude Code CLI** logged `[entrypoint][WARN] Failed to transfer …/gh-aw/safeoutputs ownership to chroot user` immediately before Claude’s init JSON listed `"github"` and `"safeoutputs"` MCP servers with `"status":"failed"`, leading to `ERR_API: MCP server(s) failed to launch: github, safeoutputs` in **Parse agent logs**. That sequence points to **in-sandbox MCP startup** (DNS + mounts), not to a missing Actions checkout.

**Container-mode note:** `runner_mode: container` isolates cross-runner `/tmp/gh-aw`, inner Docker, iptables, and MCP port 80 state. Container-mode images may rewrite generated Claude MCP URLs from `host.docker.internal` to a **runtime-resolved IPv4** on the same port (or skip rewrite when no concrete target exists), avoiding intermittent Claude HTTP MCP proxy-bypass failures without violating `gh-aw-mcpg`'s gateway config schema. Optional **`GH_SR_MCP_REWRITE_TARGET_IP`** pins the rewrite target without re-resolving. The Docker shim logs rewrite passes to the job log, prefixed with **`[gh-sr:mcp-claude-urls]`** (grep the **Start MCP Gateway** step); **`watcher_skip_no_concrete_target`** means rewrite was skipped. When the gateway `docker run` exits, the background watcher may log **`watcher_stop`** instead of **`watcher_exit`**; both are normal. If random MCP init failures persist in container mode, check for stale inner Docker containers/rules from a previous crash, broken AWF bridge reachability from **nested** agent containers (not only from the runner rootfs), or persistent mount permissions under `/runner-state`.

Current gh-sr images install a **path-scoped** Docker CLI shim at `**/opt/gh-sr/docker-shim/docker`** (not a global `/usr/local/bin/docker` replacement) so only the actions runner / `sudo awf` job path uses it:

- **AWF agent images** (`ghcr.io/github/gh-aw-firewall/agent:`*): inject `--add-host=host.docker.internal:<target>` on `docker run`/`create` when missing. Target order: **`GH_SR_AWF_BRIDGE_GATEWAY_IP`** (explicit) → first IPv4 from **`getent hosts host.docker.internal`** (non-loopback) → Docker **`host-gateway`**. This matches gh-aw’s documented use of `host.docker.internal` for service ports while avoiding brittle hardcoded gateway IPs.
- **MCP gateway** (`ghcr.io/github/gh-aw-mcpg:`*): inject `--hostname gh-aw-mcpg` when missing so upstream skips flaky self-inspect under inner `--network host` / DinD.
- `**docker run` for mcpg only**: stable `--name gh-aw-mcpg-ghsr-`*, `--cidfile`, and signal/exit cleanup so stale inner host-network listeners on port **80** are removed when upstream escalates from `POST /close` to signalling the recorded gateway PID.

## Files created on the host

When you run `gh sr setup` for an agentic runner, gh-sr may create or modify these files:


| File                                  | Purpose                                                             |
| ------------------------------------- | ------------------------------------------------------------------- |
| `/etc/dnsmasq.d/gh-sr-docker.conf`    | dnsmasq config for `host.docker.internal` + upstream DNS forwarding |
| `/etc/docker/daemon.json`             | Docker daemon DNS settings (merged with existing config)            |
| `/opt/hostedtoolcache`                | Bind mount to your npm global prefix                                |
| `/etc/fstab`                          | Persistent entry for the `/opt/hostedtoolcache` bind mount          |
| `~/.local/share/gh/extensions/gh-aw/` | The `gh-aw` CLI installation directory                              |


```bash
# Inspect dnsmasq config
cat /etc/dnsmasq.d/gh-sr-docker.conf

# Inspect Docker DNS settings
cat /etc/docker/daemon.json
```

## 12. Container mode operations (`runner_mode: container`)

> **Relevant for any `runner_mode: container` runner** (agentic or standard CI; see [§8b](#8b-runner-modes-native-vs-container-recommended-for-concurrency)).**

### Setup and lifecycle

```bash
# Set up all container-mode runners (builds the image on first run)
gh sr setup

# Start / stop all runner containers
gh sr up
gh sr down

# View status
gh sr status

# Stream recent logs for a specific instance
gh sr logs my-agentic
```

Each runner instance runs as a Docker container named `gh-sr-<instance>` with `--restart unless-stopped`, so it auto-starts when Docker starts on the host and auto-restarts after a job completes.

### Rebuild the runner image

The image (`gh-sr/agentic-runner:<version>`) is built once per runner version. To force a rebuild after local changes or a new runner version, use `gh sr rebuild <runner-name>` (native-mode runners in the selection are skipped; only `runner_mode: container` runners are rebuilt) or remove the image and re-run `gh sr setup`. Rebuild removes **only** the resolved image tag for that build (not every `gh-sr/agentic-runner` image on the host), so other container-mode runners on the same machine keep their Docker image metadata and `gh sr status` **BUILD** column accurate.

```bash
gh sr rebuild <runner-name>   # preferred: tears down containers, rebuilds, restarts

# Or: remove the image on the host, then re-run setup
docker rmi gh-sr/agentic-runner:<version>
gh sr setup
```

The version tag matches the GitHub Actions runner version resolved at setup time.

### Health checks (`gh sr doctor`)

On each Linux host in scope, `gh sr doctor` validates **native** runners by checking the host directory under `$HOME/.gh-sr/runners/<instance>/` for `run.sh` and `.runner`. It does **not** use those paths for `runner_mode: container` instances.

For **container** runners on Linux it additionally checks:

- Host Docker CLI/daemon and that a short `--privileged` test container runs (required for DinD).
- For each configured instance: a Docker object named `gh-sr-<instance>` exists and is **running** (warns if created/exited).
- **Inner DinD**: `docker exec gh-sr-<instance> docker info` succeeds.
- **Registration**: `.runner` exists at `/home/runner/actions-runner/.runner` inside the outer container.

For **native** `profile: agentic`, it runs host-level AWF orphan / `DOCKER-USER` hygiene (same as before). For **container** `profile: agentic`, it runs the same style of checks against the **inner** Docker daemon (`docker exec gh-sr-<instance> docker ps …`), because AWF containers live under inner dockerd, not on the host. It also verifies that inner containers can resolve `host.docker.internal`, reach a temporary HTTP listener in the runner namespace from a default-bridge inner container, **and** reach the same listener using the same adaptive `--add-host=host.docker.internal:<target>` path as the shim (override → `getent` → `host-gateway`). Orphan checks still flag any `gh-aw-mcpg-*` inner containers (including `gh-aw-mcpg-ghsr-*` names from the MCP gateway wrapper) left over from crashed jobs.

### Attach to a running runner container

```bash
# Exec into a running runner container (e.g. to inspect /tmp/gh-aw state)
docker exec -it gh-sr-<instance> bash

# Inside the container — check inner dockerd status
docker info

# Check agent containers spawned by a running job
docker ps

# Tail the entrypoint log
tail -f /runner-state/dockerd.log
```

### Cleaning up stale AWF artefacts

If a job crashes mid-run, orphan containers and iptables rules may remain *inside the runner container*. To clean up:

```bash
# Inside the runner container
docker exec gh-sr-<instance> bash -c "
  docker ps -a --filter 'name=awf-' --format '{{.ID}}' | xargs -r docker rm -f
  docker ps -a --filter 'name=gh-aw' --format '{{.ID}}' | xargs -r docker rm -f
  docker ps -a --filter 'name=gh-aw-mcpg-' --format '{{.ID}}' | xargs -r docker rm -f
"

# Flush stale AWF iptables rules inside the container
docker exec gh-sr-<instance> iptables -F DOCKER-USER
```

`gh sr doctor` reports orphan containers and stale rules as warnings.

### Security: `--privileged` and Sysbox

By default, container-mode runners use `--privileged` because the inner dockerd needs full Linux capabilities. This is appropriate for trusted infrastructure but increases the attack surface.

**Alternative: [Sysbox](https://github.com/nestybox/sysbox)** is an OCI runtime that enables Docker-in-Docker without `--privileged`, using user namespaces for isolation. If Sysbox is installed on the host, you can run the runner container with `--runtime sysbox-runc` instead of `--privileged`. Sysbox is not auto-configured by gh-sr; refer to Sysbox documentation for installation and then update the `docker create` command in your setup accordingly.

### State persistence

Each runner container bind-mounts `$HOME/.gh-sr/runners/<instance>` as `/runner-state` inside the container. This directory stores:


| Path                         | Contents                                                                 |
| ---------------------------- | ------------------------------------------------------------------------ |
| `/runner-state/docker-data/` | Inner Docker layer cache (preserves pulled gh-aw images across restarts) |
| `/runner-state/_work/`       | Runner job workspace                                                     |
| `/runner-state/_temp/`       | `RUNNER_TEMP` — isolated per container                                   |
| `/runner-state/dockerd.log`  | Inner dockerd log                                                        |



---
title: "Linux on Windows"
weight: 40
---

# Linux runners on a Windows host

You can run Linux container runners on a Windows machine without a separate SSH endpoint into WSL2. Set `mode: docker` on a runner that targets an `os: windows` host and **ghr** will manage the Docker container over the same SSH connection (Docker CLI invoked through the same encoded PowerShell path as native Windows runners).

**Requirements on the Windows host:**

- OpenSSH Server enabled (see [Host setup — Windows](../host-setup.md#windows-openssh-and-docker))
- Docker Desktop installed and running with the default Linux containers mode

```yaml
hosts:
  win-pc:
    addr: user@192.168.1.51
    os: windows
    arch: amd64

runners:
  # Native Windows runner
  - name: myapp-win
    repo: owner/repo
    host: win-pc
    labels: [self-hosted, Windows, X64]

  # Linux runner via Docker Desktop on the same Windows host
  - name: myapp-linux
    repo: owner/repo
    host: win-pc
    mode: docker
    labels: [self-hosted, Linux, X64]

  # Same Linux runner, tuned for GitHub Agentic Workflows (MCP gateway + awf firewall)
  - name: myapp-linux-gh-aw
    repo: owner/repo
    host: win-pc
    mode: docker
    docker_network_mode: host
    docker_cap_add: [NET_ADMIN]
    labels: [self-hosted, Linux, X64]
```

Both runners share a single SSH connection to Windows. The native runner starts `run.cmd` via PowerShell; the Docker runner calls `docker run` the same way, which talks to Docker Desktop's Linux engine.

Docker-mode Linux containers include a bind mount of `/var/run/docker.sock` so jobs can run `docker` against the same engine (GitHub Actions service containers, image pulls, [Agentic Workflows](https://github.github.com/gh-aw/guides/self-hosted-runners/) tooling, and similar). Use **Linux containers** in Docker Desktop (the default); Windows containers mode cannot run the `actions-runner` Linux image.

**Docker socket permissions:** Inside Docker Desktop's Linux engine, `/var/run/docker.sock` is often group-restricted (like on Linux). The official `actions-runner` image runs as user `runner` (uid 1001), so **ghr** probes the socket's owning GID by running a short disposable `docker run` (using the same runner image) and, when it gets a non-zero numeric GID, passes **`--group-add`** on `docker run` for your runner container (the same fix as on bare Linux hosts, without needing Unix `stat` on the Windows side). If the probe fails, ghr still bind-mounts the socket (mount-only fallback). No `docker_socket` override is needed or supported on Windows hosts.

**GitHub Agentic Workflows:** Use **`docker_network_mode: host`** and **`docker_cap_add: [NET_ADMIN]`** so jobs can reach the MCP gateway on `localhost` and run the Agent Workflow Firewall (`awf`), which needs `iptables` inside the runner container. See [Host setup — GitHub Agentic Workflows](../host-setup.md#github-agentic-workflows-gh-aw).

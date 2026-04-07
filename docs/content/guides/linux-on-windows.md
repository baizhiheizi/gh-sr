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
```

Both runners share a single SSH connection to Windows. The native runner starts `run.cmd` via PowerShell; the Docker runner calls `docker run` the same way, which talks to Docker Desktop's Linux engine.

Docker-mode Linux containers include a bind mount of `/var/run/docker.sock` so jobs can run `docker` against the same engine (GitHub Actions service containers, image pulls, [Agentic Workflows](https://github.github.com/gh-aw/guides/self-hosted-runners/) tooling, and similar). Use **Linux containers** in Docker Desktop (the default); Windows containers mode cannot run the `actions-runner` Linux image.

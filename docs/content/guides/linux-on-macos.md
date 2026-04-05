---
title: "Linux on macOS"
weight: 50
---

# Linux runners on a macOS host

Set `mode: docker` on a runner that targets an `os: darwin` host to run the same Linux container image as on Linux. **ghr** does **not** run the Linux Docker install script on macOS; install a Docker runtime yourself.

**Requirements on the Mac:**

- Docker Desktop, OrbStack, or Colima installed, with `docker` working in the environment where **ghr** runs commands (for example the same user over SSH).

```yaml
hosts:
  mac-mini:
    addr: user@192.168.1.50
    os: darwin
    arch: arm64

runners:
  - name: myapp-mac-native
    repo: owner/repo
    host: mac-mini
    labels: [self-hosted, macOS, ARM64]

  - name: myapp-linux-on-mac
    repo: owner/repo
    host: mac-mini
    mode: docker
    labels: [self-hosted, Linux, ARM64]
```

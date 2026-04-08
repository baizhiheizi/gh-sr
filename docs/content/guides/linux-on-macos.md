---
title: "Linux on macOS"
weight: 50
---

# Linux runners on a macOS host

Set `mode: docker` on a runner that targets an `os: darwin` host to run the same Linux container image as on Linux. **gh wm** does **not** run the Linux Docker install script on macOS; install a Docker runtime yourself.

**Requirements on the Mac:**

- Docker Desktop, OrbStack, or Colima installed, with `docker` working in the environment where **gh wm** runs commands (for example the same user over SSH).

**Docker socket permissions:** macOS runtimes (Docker Desktop, OrbStack, Colima) expose a socket that is accessible to all processes — there is no host `docker` group GID issue. gh wm skips `--group-add` on macOS hosts. gh wm bind-mounts the Docker socket into the container at `/var/run/docker.sock` so jobs can reach the Docker daemon. If `docker_socket` is unset, gh wm picks `/var/run/docker.sock` when present, otherwise the `unix://` path from your default Docker context (typical for Colima), otherwise `~/.colima/default/docker.sock` on macOS. **Colima + virtiofs:** when the resolved path is under `~/.colima/…`, gh wm uses `/var/run/docker.sock` as the bind-mount source for the runner container (VM path), not the macOS host socket file, to avoid `docker run` failing with `operation not supported` ([Colima #997](https://github.com/abiosoft/colima/issues/997)); you can also use `colima start --mount-type sshfs` if needed. Set `docker_socket` only when the default Docker context does not match the engine you want (e.g. a named Colima profile):

```yaml
hosts:
  mac-mini:
    addr: user@192.168.1.50
    os: darwin
    arch: arm64
    docker_socket: /Users/me/.colima/myprofile/docker.sock  # optional; omit when default context points at your runtime
```

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

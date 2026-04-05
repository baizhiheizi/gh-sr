---
title: "Local Host"
weight: 30
---

# Local host runners

You can set up runners on the same machine where **ghr** runs, without SSH. Set `addr: local` on a host and **ghr** will execute commands directly via `os/exec` instead of dialing an SSH connection.

When `addr` is `local`, the `os` and `arch` fields are **auto-detected** from the Go runtime (`runtime.GOOS` / `runtime.GOARCH`) and can be omitted. You can still set them explicitly to override.

```yaml
hosts:
  my-laptop:
    addr: local

runners:
  - name: my-local-runner
    repo: owner/repo
    host: my-laptop
    count: 1
    labels: [self-hosted, Linux, X64]
    mode: native
```

Both `native` and `docker` modes work with local hosts. Docker mode requires Docker to be installed and accessible to the current user.

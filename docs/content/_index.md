---
title: "Self-hosted runner manager for GitHub"
weight: 1
---

# Self-hosted runner manager for GitHub

**Self-hosted runner manager for GitHub** (`gh sr`) is a [GitHub CLI](https://cli.github.com/) extension to manage self-hosted [GitHub Actions](https://docs.github.com/en/actions) runners across multiple machines from a single control host—typically your laptop—using SSH or local execution.

## Highlights

- **Unified commands** — `up`, `down`, `status`, `setup`, and more work for Linux, macOS, and Windows runners.
- **SSH and local** — Manage remote hosts over SSH, or use `addr: local` to run runners on the same machine as the CLI.
- **Declarative config** — One YAML file lists hosts and runners.
- **Multi-host** — Operate any number of runner machines (desktops, Mac minis, VPS).
- **Docker and native** — Sensible defaults per OS; override with `mode: docker` or `mode: native` where supported.
- **Interactive dashboard** — Run `gh sr` on a TTY for a full TUI (same as `gh sr dashboard`).

## Architecture (overview)

```text
Your laptop (control plane)
  └── gh sr CLI
        ├── local  → This machine (native/Docker runners)
        ├── SSH → Mac mini (native and/or Docker runners)
        ├── SSH → Windows PC (native + Docker runners)
        └── SSH → VPS (Docker runners)
```

The CLI reads your YAML config, connects to each host over SSH (or runs locally for `addr: local`), and runs the right commands to install, start, and stop runner processes. Only the machine where you run **gh sr** needs the GitHub CLI and this extension (or the `gh-sr` binary on `PATH`).

For control plane vs execution plane, lifecycle commands, and how status and logs work, see [Architecture](architecture.md).

## Quick start

```bash
gh extension install an-lee/gh-sr
gh sr init
# Edit ~/.gh-sr/runners.yml; run gh auth login on this machine
gh sr doctor
gh sr setup
gh sr up
gh sr status
```

See [Installation](installation.md) for prerequisites, building from source, and the Makefile. For authentication and YAML fields, see [Authentication](authentication.md) and [Configuration](configuration.md).

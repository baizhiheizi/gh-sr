---
title: "Commands"
weight: 50
---

# Commands

Running **`gh wm` with no subcommand** opens the interactive dashboard on a **TTY** (same as `gh wm dashboard`). If stdout is not a terminal (for example in a pipe or CI), gh wm prints a short hint to stderr and exits successfully; use `gh wm status` or `gh wm --help` in those environments.

```bash
gh wm                      # Open dashboard (TTY only; see above)
gh wm init [--force]       # Create ~/.gh-wm with template runners.yml and env file
gh wm doctor [--strict]    # Check config, GitHub API, and host prerequisites
gh wm setup [names...]     # Install runner binary and configure on hosts
gh wm up [names...]        # Start runners
gh wm down [names...]      # Stop runners
gh wm restart [names...]   # Stop then start
gh wm status               # Show status table
gh wm hosts                # Show host resource usage (CPU, memory, disk, load, uptime)
gh wm logs <name>          # Show recent logs from a runner
gh wm cleanup              # Remove offline/ghost runners from GitHub
gh wm update [names...]    # Update runner binary (remove + setup + start)
gh wm service install [--system] [names...]   # Native: OS autostart (systemd / launchd / task)
gh wm service uninstall [names...]            # Remove gh wm-installed autostart
gh wm service status [names...]               # Autostart + unit state (docker: policy note)
gh wm config path          # Print resolved config and ~/.gh-wm/env paths
gh wm config show          # Print resolved configuration (PAT redacted)
gh wm config edit          # Edit resolved runners.yml in $VISUAL / $EDITOR
gh wm config edit-env      # Edit ~/.gh-wm/env in $VISUAL / $EDITOR
gh wm config validate      # Validate config (exit 0 if OK)
gh wm dashboard            # Same as bare gh wm: launch interactive TUI dashboard
```

The dashboard includes live status, per-runner actions (setup, up, down, restart, update, logs), global actions (doctor, cleanup, show/validate/edit config and env), and host/repo filters. Press `h` to open the host metrics panel (CPU, memory, disk, load, uptime) or `?` for the full key map.

**Backward compatibility:** older releases treated `gh wm config` as “print configuration”. That is now `gh wm config show`; plain `gh wm config` lists subcommands. Older scripts that expected bare `gh wm` to print usage should use `gh wm --help` instead.

## Filters

All commands accept `--host` and `--repo` flags:

```bash
gh wm doctor --host mac-mini
gh wm status --host mac-mini
gh wm logs myapp-1 --host win-pc   # disambiguate when the same instance name exists on multiple hosts
gh wm up --repo an-lee/enjoy
gh wm down enjoy-win-1
```

## Quick start sequence

```bash
# 0. Verify config, GitHub access, and hosts (optional but recommended)
gh wm doctor

# 1. Set up runners on all hosts
gh wm setup

# 2. Start everything
gh wm up

# 3. Check status
gh wm status

# 4. Launch dashboard for live monitoring (or run `gh wm` alone on a TTY)
gh wm
# gh wm dashboard   # equivalent
```

For what each lifecycle command does on the host, see [Architecture — Lifecycle commands](architecture.md#lifecycle-commands-what-they-do).

## Host metrics

**`gh wm hosts`** collects and displays real-time resource usage from every configured host over SSH:

```
HOST         CPU%   MEM              DISK              LOAD             UPTIME
──────────────────────────────────────────────────────────────────────────────
mac-mini      12%   3.2 / 16.0 GiB   45 / 500 GiB     0.42 0.38 0.31  5 days
linux-vps     55%   1.8 /  4.0 GiB    8 /  80 GiB     1.20 0.95 0.80  12 days
win-pc         8%   6.1 / 32.0 GiB   80 / 500 GiB     -               3 days
```

The same panel is available interactively in `gh wm dashboard` by pressing `h`.

Use `--host` to limit output to a single host:

```bash
gh wm hosts --host mac-mini
```

> **Note:** Load averages are not available on Windows and show as `-`.


---
title: "Commands"
weight: 50
---

# Commands

Running **`gh sr` with no subcommand** opens the interactive dashboard on a **TTY** (same as `gh sr dashboard`). If stdout is not a terminal (for example in a pipe or CI), gh sr prints a short hint to stderr and exits successfully; use `gh sr status` or `gh sr --help` in those environments.

```bash
gh sr                      # Open dashboard (TTY only; see above)
gh sr init [--force]       # Create ~/.gh-sr with template runners.yml and env file
gh sr doctor [--strict]    # Check config, GitHub API, and host prerequisites
gh sr setup [names...]     # Install runner binary and configure on hosts
gh sr up [names...]        # Start runners
gh sr down [names...]      # Stop runners
gh sr restart [names...]   # Stop then start
gh sr status               # Show status table
gh sr hosts                # Show host resource usage (CPU, memory, disk, load, uptime)
gh sr logs <name>          # Show recent logs from a runner
gh sr cleanup              # Remove offline/ghost runners from GitHub
gh sr update [names...]    # Update runner binary (remove + setup + start)
gh sr service install [--system] [names...]   # Native: OS autostart (systemd / launchd / task)
gh sr service uninstall [names...]            # Remove gh sr-installed autostart
gh sr service status [names...]               # Autostart + unit state (docker: policy note)
gh sr config path          # Print resolved config and ~/.gh-sr/env paths
gh sr config show          # Print resolved configuration (token source summarized)
gh sr config edit          # Edit resolved runners.yml in $VISUAL / $EDITOR
gh sr config edit-env      # Edit ~/.gh-sr/env in $VISUAL / $EDITOR
gh sr config validate      # Validate config (exit 0 if OK)
gh sr dashboard            # Same as bare gh sr: launch interactive TUI dashboard
```

The dashboard includes live status, per-runner actions (setup, up, down, restart, update, logs), global actions (doctor, cleanup, show/validate/edit config and env), and host/repo filters. Press `h` to open the host metrics panel (CPU, memory, disk, load, uptime) or `?` for the full key map.

**Backward compatibility:** older releases treated `gh sr config` as “print configuration”. That is now `gh sr config show`; plain `gh sr config` lists subcommands. Older scripts that expected bare `gh sr` to print usage should use `gh sr --help` instead.

## Filters

All commands accept `--host` and `--repo` flags:

```bash
gh sr doctor --host mac-mini
gh sr status --host mac-mini
gh sr logs myapp-1 --host win-pc   # disambiguate when the same instance name exists on multiple hosts
gh sr up --repo an-lee/enjoy
gh sr down enjoy-win-1
```

## Quick start sequence

```bash
# 0. Verify config, GitHub access, and hosts (optional but recommended)
gh sr doctor

# 1. Set up runners on all hosts
gh sr setup

# 2. Start everything
gh sr up

# 3. Check status
gh sr status

# 4. Launch dashboard for live monitoring (or run `gh sr` alone on a TTY)
gh sr
# gh sr dashboard   # equivalent
```

For what each lifecycle command does on the host, see [Architecture — Lifecycle commands](architecture.md#lifecycle-commands-what-they-do).

## Host metrics

**`gh sr hosts`** collects and displays real-time resource usage from every configured host over SSH. All hosts are queried **concurrently**, so the command completes in roughly the time it takes to collect metrics from the slowest single host regardless of how many hosts you have configured:

```
HOST         CPU%   MEM              DISK              LOAD             UPTIME
──────────────────────────────────────────────────────────────────────────────
mac-mini      12%   3.2 / 16.0 GiB   45 / 500 GiB     0.42 0.38 0.31  5 days
linux-vps     55%   1.8 /  4.0 GiB    8 /  80 GiB     1.20 0.95 0.80  12 days
win-pc         8%   6.1 / 32.0 GiB   80 / 500 GiB     -               3 days
```

The same panel is available interactively in `gh sr dashboard` by pressing `h`.

Use `--host` to limit output to a single host:

```bash
gh sr hosts --host mac-mini
```

> **Note:** Load averages are not available on Windows and show as `-`.


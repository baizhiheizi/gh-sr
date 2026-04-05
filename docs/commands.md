# Commands

Running **`ghr` with no subcommand** opens the interactive dashboard on a **TTY** (same as `ghr dashboard`). If stdout is not a terminal (for example in a pipe or CI), ghr prints a short hint to stderr and exits successfully; use `ghr status` or `ghr --help` in those environments.

```bash
ghr                      # Open dashboard (TTY only; see above)
ghr init [--force]       # Create ~/.ghr with template runners.yml and env file
ghr doctor [--strict]    # Check config, GitHub API, and host prerequisites
ghr setup [names...]     # Install runner binary and configure on hosts
ghr up [names...]        # Start runners
ghr down [names...]      # Stop runners
ghr restart [names...]   # Stop then start
ghr status               # Show status table
ghr logs <name>          # Show recent logs from a runner
ghr cleanup              # Remove offline/ghost runners from GitHub
ghr update [names...]    # Update runner binary (remove + setup + start)
ghr service install [--system] [names...]   # Native: OS autostart (systemd / launchd / task)
ghr service uninstall [names...]            # Remove ghr-installed autostart
ghr service status [names...]               # Autostart + unit state (docker: policy note)
ghr config path          # Print resolved config and ~/.ghr/env paths
ghr config show          # Print resolved configuration (PAT redacted)
ghr config edit          # Edit resolved runners.yml in $VISUAL / $EDITOR
ghr config edit-env      # Edit ~/.ghr/env in $VISUAL / $EDITOR
ghr config validate      # Validate config (exit 0 if OK)
ghr dashboard            # Same as bare ghr: launch interactive TUI dashboard
```

The dashboard includes live status, per-runner actions (setup, up, down, restart, update, logs), global actions (doctor, cleanup, show/validate/edit config and env), and host/repo filters. Press `?` inside the TUI for the full key map.

**Backward compatibility:** older releases treated `ghr config` as “print configuration”. That is now `ghr config show`; plain `ghr config` lists subcommands. Older scripts that expected bare `ghr` to print usage should use `ghr --help` instead.

## Filters

All commands accept `--host` and `--repo` flags:

```bash
ghr doctor --host mac-mini
ghr status --host mac-mini
ghr logs myapp-1 --host win-pc   # disambiguate when the same instance name exists on multiple hosts
ghr up --repo an-lee/enjoy
ghr down enjoy-win-1
```

## Quick start sequence

```bash
# 0. Verify config, GitHub access, and hosts (optional but recommended)
ghr doctor

# 1. Set up runners on all hosts
ghr setup

# 2. Start everything
ghr up

# 3. Check status
ghr status

# 4. Launch dashboard for live monitoring (or run `ghr` alone on a TTY)
ghr
# ghr dashboard   # equivalent
```

For what each lifecycle command does on the host, see [Architecture — Lifecycle commands](architecture.md#lifecycle-commands-what-they-do).

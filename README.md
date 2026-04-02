# ghr

A CLI tool to manage self-hosted GitHub Actions runners across multiple hosts — all from your laptop over SSH.

- **Unified commands** — `up`, `down`, `status`, `setup` work for Linux, macOS, and Windows runners
- **SSH-only transport** — no agents or scripts to install on remote hosts
- **Declarative config** — one YAML file defines your hosts and runners
- **Multi-host** — manage runners on any number of machines (local PCs, Mac Minis, VPS)
- **Docker & native** — Linux runners can use Docker containers; macOS and Windows run natively
- **Interactive dashboard** — TUI with live status updates

---

## Architecture

```
Your Laptop (control plane)
  └── ghr CLI
        ├── SSH → Mac Mini (native runners)
        ├── SSH → Windows PC (native runners)
        └── SSH → VPS (Docker runners)
```

The CLI reads a YAML config (see [Config file location](#config-file-location)), connects to each host over SSH, and executes the appropriate commands to manage runner processes. Your laptop is the only machine that needs the `ghr` binary.

---

## Install

```bash
go install github.com/an-lee/ghr/cmd/ghr@latest
```

`go install` does not create config files. After installing, run:

```bash
ghr init
```

This creates `~/.ghr/runners.yml` from a template and `~/.ghr/env` for secrets (optional). Then edit those files (or use `ghr config edit` / `ghr config edit-env`), run `ghr config validate`, and you are ready to use `ghr status` and other commands.

Or build from source:

```bash
git clone https://github.com/an-lee/ghr.git
cd ghr
go build -o ghr ./cmd/ghr/
```

When developing in this repository, `./config/runners.yml` is used automatically if that file exists (see precedence below).

---

## Prerequisites

**On your laptop:**
- Go 1.22+ (for building)
- SSH key-based access to all runner hosts

**On runner hosts:**
- **Linux** — Docker installed (for `mode: docker`) or just a shell (for `mode: native`)
- **macOS** — `curl` available (pre-installed)
- **Windows** — OpenSSH Server enabled:
  ```powershell
  Add-WindowsCapability -Online -Name OpenSSH.Server~~~~0.0.1.0
  Start-Service sshd
  Set-Service -Name sshd -StartupType Automatic
  ```

---

## Config file location

When you do **not** pass `--config` / `-c`, the config file is chosen in this order:

1. **`GHR_CONFIG`** — path to a YAML file (absolute or relative to the current working directory).
2. **`./config/runners.yml`** — if this file exists in the current working directory, it is used (handy for this repo or a project-local checkout).
3. **`~/.ghr/runners.yml`** — used after `ghr init` when no project-local file exists.

If you pass `-c /path/to/runners.yml`, that path is always used (and `GHR_CONFIG` is ignored).

Run `ghr config path` to see which config file and `~/.ghr/env` path apply in your environment.

## Secrets (`~/.ghr/env`)

Before the YAML file is loaded, `ghr` applies environment variables from **`~/.ghr/env`** if that file exists (dotenv-style: `KEY=value`, optional `export `, `#` comments). This keeps secrets out of your shell history and out of the YAML file. Pair it with `github.pat: env:GITHUB_PAT` in `runners.yml`. Create the directory and file with `ghr init`, or run `ghr config edit-env`.

Keep `~/.ghr` permissions tight (`chmod 700 ~/.ghr`, `chmod 600 ~/.ghr/env` if you create files by hand).

## Configuration

### 1. Create a GitHub PAT

Go to **GitHub → Settings → Developer settings → Personal access tokens → Fine-grained tokens**.

- **Repository access**: select repos you want runners for
- **Permissions → Repository → Administration**: Read and write

### 2. Set up environment

Either put the token in `~/.ghr/env`:

```bash
# ~/.ghr/env
GITHUB_PAT=github_pat_...
```

Or export it in your shell:

```bash
export GITHUB_PAT=github_pat_...
```

### 3. Edit config

Edit `~/.ghr/runners.yml` (after `ghr init`) or `./config/runners.yml` for a project-local file. You can open the resolved file in `$VISUAL` or `$EDITOR` with `ghr config edit`.

```yaml
github:
  pat: env:GITHUB_PAT

hosts:
  mac-mini:
    addr: user@192.168.1.50
    os: darwin
    arch: arm64

  win-pc:
    addr: user@192.168.1.51
    os: windows
    arch: amd64

  vps-1:
    addr: root@203.0.113.10
    os: linux
    arch: amd64

runners:
  - name: enjoy-mac
    repo: an-lee/enjoy
    host: mac-mini
    count: 1
    labels: [self-hosted, macOS, ARM64]

  - name: enjoy-win
    repo: an-lee/enjoy
    host: win-pc
    count: 1
    labels: [self-hosted, Windows, X64]

  - name: hangar-ci
    repo: an-lee/hangar
    host: vps-1
    count: 2
    labels: [self-hosted, Linux, X64]
    mode: docker
```

### Config reference

| Field | Description |
|---|---|
| `github.pat` | GitHub PAT. Use `env:VAR_NAME` to read from environment. |
| `hosts.<name>.addr` | SSH target (`user@host` or `user@ip`) |
| `hosts.<name>.os` | `linux`, `darwin`, or `windows` |
| `hosts.<name>.arch` | `amd64` or `arm64` |
| `runners[].name` | Base name (instances become `name-1`, `name-2`, ...) |
| `runners[].repo` | GitHub `owner/repo` |
| `runners[].host` | References a key under `hosts` |
| `runners[].count` | Number of parallel instances (default: 1) |
| `runners[].labels` | Labels for workflow `runs-on` matching |
| `runners[].mode` | `docker` or `native` (default: `docker` for Linux, `native` for others) |

---

## Commands

```bash
ghr init [--force]       # Create ~/.ghr with template runners.yml and env file
ghr setup [names...]     # Install runner binary and configure on hosts
ghr up [names...]        # Start runners
ghr down [names...]      # Stop runners
ghr restart [names...]   # Stop then start
ghr status               # Show status table
ghr logs <name>          # Show recent logs from a runner
ghr cleanup              # Remove offline/ghost runners from GitHub
ghr update [names...]    # Update runner binary (remove + setup + start)
ghr config path          # Print resolved config and ~/.ghr/env paths
ghr config show          # Print resolved configuration (PAT redacted)
ghr config edit          # Edit resolved runners.yml in $VISUAL / $EDITOR
ghr config edit-env      # Edit ~/.ghr/env in $VISUAL / $EDITOR
ghr config validate      # Validate config (exit 0 if OK)
ghr dashboard            # Launch interactive TUI dashboard
```

**Backward compatibility:** older releases treated `ghr config` as “print configuration”. That is now `ghr config show`; plain `ghr config` lists subcommands.

### Filters

All commands accept `--host` and `--repo` flags:

```bash
ghr status --host mac-mini
ghr up --repo an-lee/enjoy
ghr down enjoy-win-1
```

---

## Quick start

```bash
# 1. Set up runners on all hosts
ghr setup

# 2. Start everything
ghr up

# 3. Check status
ghr status

# 4. Launch dashboard for live monitoring
ghr dashboard
```

---

## Using runners in workflows

Reference runners by label in your workflow files:

```yaml
jobs:
  build-linux:
    runs-on: [self-hosted, Linux, X64]

  build-mac:
    runs-on: [self-hosted, macOS, ARM64]

  build-win:
    runs-on: [self-hosted, Windows, X64]
```

---

## Common tasks

### Add a new host

1. Add an entry under `hosts` in `config/runners.yml`
2. Ensure SSH key-based access works: `ssh user@host true`
3. Add runner entries referencing the new host
4. Run `ghr setup && ghr up`

### Scale up

Change `count` in `runners.yml`, then:

```bash
ghr setup   # configures new instances
ghr up      # starts them
```

### Update runner version

```bash
ghr update
```

This removes existing runners, downloads the latest runner binary, reconfigures, and starts them.

### Clean up ghost runners

```bash
ghr cleanup
```

---

## File structure

```
ghr/
  cmd/
    ghr/
      main.go               # CLI entry point
  internal/
    config/
      config.go             # YAML config parsing and validation
      envfile.go            # ~/.ghr/env dotenv loader
      paths.go              # Config path resolution, ~/.ghr helpers
      load.go               # LoadFromPath with missing-file hints
      template.go           # Embedded template for ghr init
      runners.yml.template  # Default runners.yml content
    editor/
      editor.go             # $VISUAL / $EDITOR / platform default
    host/
      host.go               # Host abstraction
      connection.go         # SSH connection management
    runner/
      runner.go             # Runner lifecycle orchestration
      native.go             # Native runner management (mac/win/linux)
      docker.go             # Docker runner management
      github.go             # GitHub API client
    tui/
      dashboard.go          # Interactive TUI dashboard
      status.go             # Status table rendering
      styles.go             # Lipgloss styles
  config/
    runners.yml             # Example / dev runner configuration (optional)
  go.mod
  go.sum
```

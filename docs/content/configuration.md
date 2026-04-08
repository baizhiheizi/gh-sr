---
title: "Configuration"
weight: 20
---

# Configuration

## Config file location

When you do **not** pass `--config` / `-c`, the config file is chosen in this order:

1. **`GHR_CONFIG`** — path to a YAML file (absolute or relative to the current working directory).
2. **`~/.ghr/runners.yml`** — default after `ghr init`.

There is no automatic discovery of `./config/runners.yml` in the current directory; use `GHR_CONFIG` or `-c` if your file lives elsewhere.

If you pass `-c /path/to/runners.yml`, that path is always used (and `GHR_CONFIG` is ignored).

Run `ghr config path` to see which config file and `~/.ghr/env` path apply in your environment.

## Authentication

**ghr** resolves a GitHub token in this order:

1. `github.pat` in `runners.yml` (supports `env:VAR_NAME` to read from environment)
2. `GITHUB_PAT` or `GITHUB_TOKEN` environment variable
3. `gh` CLI — if you have `gh auth login` done, ghr picks up the token automatically

The simplest setup is to install the [GitHub CLI](https://cli.github.com/) and run `gh auth login`. No env files or PAT fields needed.

For PAT creation and scopes, see [Authentication](github-pat.md).

## Secrets (`~/.ghr/env`)

Before the YAML file is loaded, **ghr** applies environment variables from **`~/.ghr/env`** if that file exists (dotenv-style: `KEY=value`, optional `export `, `#` comments). This keeps secrets out of your shell history and out of the YAML file. Pair it with `github.pat: env:GITHUB_PAT` in `runners.yml`. Create the directory and file with `ghr init`, or run `ghr config edit-env`.

If you use `gh auth login`, the env file is optional — you can skip `GITHUB_PAT` entirely.

Keep `~/.ghr` permissions tight (`chmod 700 ~/.ghr`, `chmod 600 ~/.ghr/env` if you create files by hand).

## Example `runners.yml`

Edit `~/.ghr/runners.yml` (after `ghr init`), or set `GHR_CONFIG` / `-c` to another YAML file. You can open the resolved file in `$VISUAL` or `$EDITOR` with `ghr config edit`.

```yaml
github:
  # Optional if you use `gh auth login`. Otherwise set a PAT:
  # pat: env:GITHUB_PAT

hosts:
  my-laptop:
    addr: local              # run on the local machine (no SSH)
    # os and arch are auto-detected; override if needed

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
  - name: enjoy-local
    repo: an-lee/enjoy
    host: my-laptop
    count: 1
    labels: [self-hosted, Linux, X64]
    mode: native

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

  - name: enjoy-linux-on-win
    repo: an-lee/enjoy
    host: win-pc
    mode: docker   # Linux container via Docker Desktop
    count: 1
    labels: [self-hosted, Linux, X64]

  - name: hangar-ci
    repo: an-lee/hangar
    host: vps-1
    count: 2
    labels: [self-hosted, Linux, X64]
    mode: docker

  # GitHub Agentic Workflows — profile: agentic sets docker mode,
  # host networking, NET_ADMIN, and a gh-aw label automatically.
  - name: hangar-aw
    repo: an-lee/hangar
    host: vps-1
    profile: agentic
    count: 2
```

## Config reference

| Field | Description |
|---|---|
| `github.pat` | GitHub PAT (optional if `gh auth login` is used). Use `env:VAR_NAME` to read from environment. |
| `hosts.<name>.addr` | SSH target (`user@host` or `user@ip`), or `local` to run on the machine where ghr is running. Remote commands run as that user; on Linux, privilege expectations for `setup` / `update` follow [Linux SSH user and privileges](host-setup.md#linux-ssh-user-and-privileges). |
| `hosts.<name>.os` | `linux`, `darwin`, or `windows`. Auto-detected when `addr` is `local`. |
| `hosts.<name>.arch` | `amd64` or `arm64`. Auto-detected when `addr` is `local`. |
| `hosts.<name>.windows_ps` | Optional; **Windows hosts only.** Which executable runs remote PowerShell payloads: `powershell` (default, `powershell.exe`) or `pwsh` (`pwsh.exe`). ghr uses `-EncodedCommand` so the user’s SSH default shell (cmd.exe or pwsh) does not break nested quoting. |
| `runners[].name` | Base name (instances become `name-1`, `name-2`, ...) |
| `runners[].repo` | GitHub `owner/repo`. Required unless `org` is set. |
| `runners[].org` | GitHub organization name. Use instead of `repo` for org-level runners. |
| `runners[].group` | Runner group name (org-level runners only). Passed as `--runnergroup` during registration. |
| `runners[].host` | References a key under `hosts` |
| `runners[].count` | Number of parallel instances (default: 1) |
| `runners[].labels` | Labels for workflow `runs-on` matching |
| `runners[].mode` | `docker` or `native` (default: `docker` for Linux hosts, `native` for macOS and Windows). Set `docker` for Linux container runners on Windows (Docker Desktop) or macOS (Docker Desktop, OrbStack, or Colima). |
| `runners[].profile` | Optional. `agentic` auto-configures for [GitHub Agentic Workflows](https://github.github.com/gh-aw/): sets `mode: docker`, `docker_network_mode: host`, `docker_cap_add: [NET_ADMIN]`, and adds a `gh-aw` label. See [Host setup — gh-aw](host-setup.md#github-agentic-workflows-gh-aw). |
| `runners[].ephemeral` | Optional boolean. When `true`, the runner handles one job and deregisters. Docker-mode uses `--restart no`; native passes `--ephemeral` to `config.sh`. |
| `runners[].docker_network_mode` | Optional. `bridge` (default) or `host`. Only for **`mode: docker`** runners. `host` runs the actions-runner container with Docker **`--network host`** so jobs share the engine host network (needed for [GitHub Agentic Workflows](https://github.github.com/gh-aw/guides/self-hosted-runners/) MCP gateway health checks). On **Linux** hosts, the container joins the host's network namespace directly. On **macOS** and **Windows** (Docker Desktop), it joins the Linux VM's network namespace — sufficient for gh-aw since the MCP gateway runs in the same VM. Weaker isolation; port **80** must be free on the host (Linux) or inside the VM (macOS/Windows). See [Host setup — GitHub Agentic Workflows](host-setup.md#github-agentic-workflows-gh-aw). |
| `runners[].docker_cap_add` | Optional list of Linux capability names for **`docker run --cap-add`**. Only for **`mode: docker`** runners. Use **`[NET_ADMIN]`** when workflows run the [Agent Workflow Firewall](https://github.com/github/gh-aw-firewall) (`awf`) so jobs can configure iptables (e.g. `DOCKER-USER`); combine with **`docker_network_mode: host`** for gh-aw. Defaults to none (stronger isolation). See [Host setup — GitHub Agentic Workflows](host-setup.md#github-agentic-workflows-gh-aw). |

**Unique runner names per repository:** GitHub registers each self-hosted runner by its **instance** name (`name-1`, `name-2`, …). That name must be **unique within a given `owner/repo`**. If two machines use the same base `name` and `count: 1`, both try to register as `name-1` and only one registration remains active. Prefer distinct base names (for example `myapp-win` vs `myapp-linux`) so every machine has its own GitHub runner record and `ghr status` matches the right row.

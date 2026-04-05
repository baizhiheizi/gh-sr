# Configuration

## Config file location

When you do **not** pass `--config` / `-c`, the config file is chosen in this order:

1. **`GHR_CONFIG`** — path to a YAML file (absolute or relative to the current working directory).
2. **`~/.ghr/runners.yml`** — default after `ghr init`.

There is no automatic discovery of `./config/runners.yml` in the current directory; use `GHR_CONFIG` or `-c` if your file lives elsewhere.

If you pass `-c /path/to/runners.yml`, that path is always used (and `GHR_CONFIG` is ignored).

Run `ghr config path` to see which config file and `~/.ghr/env` path apply in your environment.

## Secrets (`~/.ghr/env`)

Before the YAML file is loaded, **ghr** applies environment variables from **`~/.ghr/env`** if that file exists (dotenv-style: `KEY=value`, optional `export `, `#` comments). This keeps secrets out of your shell history and out of the YAML file. Pair it with `github.pat: env:GITHUB_PAT` in `runners.yml`. Create the directory and file with `ghr init`, or run `ghr config edit-env`.

Keep `~/.ghr` permissions tight (`chmod 700 ~/.ghr`, `chmod 600 ~/.ghr/env` if you create files by hand).

For PAT creation and scopes, see [GitHub PAT](github-pat.md).

## Example `runners.yml`

Edit `~/.ghr/runners.yml` (after `ghr init`), or set `GHR_CONFIG` / `-c` to another YAML file. You can open the resolved file in `$VISUAL` or `$EDITOR` with `ghr config edit`.

```yaml
github:
  pat: env:GITHUB_PAT

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
```

## Config reference

| Field | Description |
|---|---|
| `github.pat` | GitHub PAT. Use `env:VAR_NAME` to read from environment. |
| `hosts.<name>.addr` | SSH target (`user@host` or `user@ip`), or `local` to run on the machine where ghr is running. Remote commands run as that user; on Linux, privilege expectations for `setup` / `update` follow [Linux SSH user and privileges](host-setup.md#linux-ssh-user-and-privileges). |
| `hosts.<name>.os` | `linux`, `darwin`, or `windows`. Auto-detected when `addr` is `local`. |
| `hosts.<name>.arch` | `amd64` or `arm64`. Auto-detected when `addr` is `local`. |
| `hosts.<name>.windows_ps` | Optional; **Windows hosts only.** Which executable runs remote PowerShell payloads: `powershell` (default, `powershell.exe`) or `pwsh` (`pwsh.exe`). ghr uses `-EncodedCommand` so the user’s SSH default shell (cmd.exe or pwsh) does not break nested quoting. |
| `runners[].name` | Base name (instances become `name-1`, `name-2`, ...) |
| `runners[].repo` | GitHub `owner/repo` |
| `runners[].host` | References a key under `hosts` |
| `runners[].count` | Number of parallel instances (default: 1) |
| `runners[].labels` | Labels for workflow `runs-on` matching |
| `runners[].mode` | `docker` or `native` (default: `docker` for Linux hosts, `native` for macOS and Windows). Set `docker` for Linux container runners on Windows (Docker Desktop) or macOS (Docker Desktop, OrbStack, or Colima). |

**Unique runner names per repository:** GitHub registers each self-hosted runner by its **instance** name (`name-1`, `name-2`, …). That name must be **unique within a given `owner/repo`**. If two machines use the same base `name` and `count: 1`, both try to register as `name-1` and only one registration remains active. Prefer distinct base names (for example `myapp-win` vs `myapp-linux`) so every machine has its own GitHub runner record and `ghr status` matches the right row.

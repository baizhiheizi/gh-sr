# ghr

A CLI tool to manage self-hosted GitHub Actions runners across multiple hosts — all from your laptop over SSH.

- **Unified commands** — `up`, `down`, `status`, `setup` work for Linux, macOS, and Windows runners
- **SSH & local** — manage remote hosts over SSH, or run runners on the local machine with `addr: local`
- **Declarative config** — one YAML file defines your hosts and runners
- **Multi-host** — manage runners on any number of machines (local PCs, Mac Minis, VPS)
- **Docker & native** — Linux defaults to Docker; macOS and Windows default to native, but you can use `mode: docker` on any host that has Docker (Linux, macOS with Docker Desktop / OrbStack / Colima, or Windows with Docker Desktop)
- **Interactive dashboard** — full-featured TUI (default when you run `ghr` on a terminal); same as `ghr dashboard`

---

## Architecture

```
Your Laptop (control plane)
  └── ghr CLI
        ├── local  → This machine (native/Docker runners)
        ├── SSH → Mac Mini (native and/or Docker runners)
        ├── SSH → Windows PC (native + Docker runners)
        └── SSH → VPS (Docker runners)
```

The CLI reads a YAML config (see [Config file location](#config-file-location)), connects to each host over SSH (or runs commands locally for `addr: local` hosts), and executes the appropriate commands to manage runner processes. Your laptop is the only machine that needs the `ghr` binary. For a deeper walkthrough, see [How it works](#how-it-works).

---

## How it works

This section describes where runners actually run, how `ghr` reaches each host, and how `status`, `dashboard`, and `logs` collect information.

### Control plane vs execution plane

`ghr` is a **control plane** only: it runs on your machine and issues commands. The **GitHub Actions runner** (native process or Docker container) always runs on the **target host** from your config—either the same machine as `ghr` when `addr: local`, or a remote machine over SSH.

```mermaid
flowchart LR
  subgraph controlPlane [Control_plane]
    ghr[ghr_CLI]
  end
  subgraph targets [Runner_hosts]
    h1[Remote_host_SSH]
    h2[Local_addr_local]
  end
  ghr -->|SSH_session| h1
  ghr -->|os_exec| h2
  h1 --> n1[Native_runner_or_Docker]
  h2 --> n2[Native_runner_or_Docker]
```

### Where runners run

**Mode** (`native` vs `docker`) is resolved per runner. If you omit `mode`, Linux hosts default to **docker**; Windows and macOS default to **native**. You can override with `runners[].mode`.

| Host OS | Default mode | Where the workload runs |
|--------|--------------|-------------------------|
| Linux | `docker` | Docker on that host: container name `gh-runner-<instance>`, image `ghcr.io/actions/actions-runner:latest`. ghr starts it with `config.sh --unattended` (once per container) then `run.sh`, using `ACTIONS_RUNNER_INPUT_*` env vars. On non-Windows hosts the container typically mounts the Docker socket for workflow `docker` steps. |
| Linux | `native` (if set) | Files under `~/.ghr/runners/<instance>`; process via `run.sh` and a PID file. |
| Windows | `native` | `%USERPROFILE%\.ghr\runners\<instance>`; process via `run.cmd` and a PID file. |
| Windows | `docker` (if set) | Same Docker image on Docker Desktop over the same SSH connection as native Windows runners. |
| macOS | `native` | `~/.ghr/runners/<instance>`. |
| macOS | `docker` (if set) | Same Linux runner image as on Linux; requires Docker Desktop, OrbStack, or Colima with a working `docker` CLI. |

**Instances:** `count` creates separate runners named `<name>-1`, `<name>-2`, … on the same host, each with its own directory (native) or container (docker).

```mermaid
flowchart TB
  subgraph linuxHost [Linux_host]
    ld[Default_docker]
    ln[Explicit_native]
    ld --> c1[Container_gh_runner_name_1]
    ln --> d1[Dir_HOME_ghr_runners_name_1]
  end
  subgraph winHost [Windows_host]
    wn[Default_native]
    wd[Explicit_docker]
    wn --> wd1[Dir_USERPROFILE_ghr_runners_name_1]
    wd --> wc1[Docker_Desktop_container]
  end
  subgraph macHost [macOS_host]
    mn[Default_native]
    md[Explicit_docker]
    mn --> md1[Dir_HOME_ghr_runners_name_1]
    md --> mc1[Docker_Linux_container]
  end
```

### How commands reach a host

- **`addr: local`** — `ghr` runs commands on the machine where the CLI runs (no SSH).
- **Remote SSH** — `ghr` opens one SSH client per host for the duration of that subcommand; each remote action uses a new session on that connection.
- **Windows over SSH** — remote automation uses PowerShell (`powershell.exe` or `pwsh.exe` per `windows_ps`) with an encoded command so quoting works regardless of the user’s default SSH shell.
- **Linux / macOS over SSH** — commands run as shell commands on the remote user’s environment.

```mermaid
sequenceDiagram
  participant User
  participant Ghr as ghr_CLI
  participant Host as Target_host
  User->>Ghr: ghr_up_runner_name
  Ghr->>Ghr: Load_config_resolve_mode
  Ghr->>Host: Connect_SSH_or_local
  Ghr->>Host: Start_native_or_docker_per_instance
  Host-->>Ghr: OK_or_error
```

### Lifecycle commands (what they do)

| Command | Behavior |
|---------|----------|
| `ghr setup` | Install/configure runner software or Docker image pull on the host. For **docker** mode on a given host, only the **first** matching runner row runs host-level Docker setup; additional docker-mode runners on that host skip duplicate setup. |
| `ghr up` / `ghr down` | Start or stop each instance (native process or container). |
| `ghr restart` | `down` then `up`; stop errors are ignored before start. |
| `ghr update` | Remove local runner registration (native) or container (docker), then setup and start again—use when upgrading the runner stack. |
| `ghr cleanup` | Deletes **offline** runners from the GitHub API only; it does not remove local install dirs or Docker containers. |
| `ghr service install` / `uninstall` / `status` | **Native** runners only: install OS autostart (systemd on Linux, LaunchAgent on macOS, scheduled task at logon on Windows). **Docker** rows are skipped; containers already use Docker’s `--restart unless-stopped`. After install, `ghr up` and `ghr down` start/stop the same supervisor instead of a bare `nohup` / Win32 process. |
| `ghr doctor` | Read-only checks: local paths, config, GitHub API, SSH targets, Docker vs native prerequisites. Use `--strict` to treat **WARN** as failure. |

```mermaid
flowchart TD
  setup[ghr_setup] --> mode{Mode}
  mode -->|docker| dock[Ensure_Docker_and_pull_image]
  mode -->|native| nat[Download_configure_actions_runner]
  up[ghr_up] --> start[Start_instances]
  down[ghr_down] --> stop[Stop_instances]
  update[ghr_update] --> remove[Remove_then_setup_then_up]
  cleanup[ghr_cleanup] --> api[GitHub_API_delete_offline_only]
```

### Status, dashboard, and logs

**`ghr status`** and **`ghr dashboard`** both:

1. Connect to each host (or mark instances **unreachable** if connection fails).
2. Ask the host whether each instance is **running** or **stopped** (native: PID file + process check; docker: `docker inspect`).
3. Call the **GitHub API** to match runner **names** and the runner’s **OS** (from the API’s `os` field vs native vs docker mode) so two config rows with the same instance name on different platforms do not show each other’s GitHub state.

**`ghr logs <name>`** looks up the runner block by **base name** or a full **instance** name (`name-1`, `name-2`, …), connects to the host, then tails logs for that instance. Use **`--host`** when the same instance name exists on more than one machine.

- **Native** — last lines of `runner.log` under that instance’s directory (`~/.ghr/runners/<instance>` on Linux/macOS, or `%USERPROFILE%\.ghr\runners\<instance>` on Windows). On Windows, if that file is missing or empty, ghr tails the newest `*.log` under `_diag` in the same directory.
- **Docker** — last lines from `docker logs` for container `gh-runner-<instance>`.

If `count` is greater than 1, pass the specific instance (for example `myapp-1`) so logs match the right directory or container.

```mermaid
flowchart LR
  subgraph localCheck [On_each_host]
    L[Process_or_container_state]
  end
  subgraph remoteApi [GitHub]
    A[Runner_list_API]
  end
  statusCmd[ghr_status_or_dashboard] --> localCheck
  statusCmd --> A
  localCheck --> table[CLI_or_TUI_table]
  A --> table
  logsCmd[ghr_logs] --> localCheck
  logsCmd --> tail[Log_tail_native_or_docker]
```

### Autostart and reboot

**Docker mode:** `docker run` uses **`--restart unless-stopped`**. If the container was **running** when the host shut down, it is typically started again when the Docker daemon comes up after reboot. **`ghr down`** runs `docker stop`; a **stopped** container is **not** brought back on boot until you run **`ghr up`** (or start the container another way).

**Native mode:** A normal **`ghr up`** starts the listener as a background process that **does not** survive reboot. Install autostart once per instance:

```bash
ghr service install              # all runners (native only; docker rows are skipped)
ghr service install myrunner     # one runner block
ghr service install --system     # Linux only: unit in /etc/systemd/system (needs passwordless sudo or root SSH)
ghr service status
ghr service uninstall
```

**Linux (user units, default):** Units live under `~/.config/systemd/user/`. On many **headless** servers the user manager does not run at boot until someone logs in. Enable **lingering** once (as root): `loginctl enable-linger <ssh-user>` so `systemctl --user` services start at boot.

**Linux (`--system`):** Writes `/etc/systemd/system/ghr-runner-<instance>.service` with `User=` / `Group=` set to the SSH user. Requires the same non-interactive **`sudo`** behavior as `ghr setup` on Linux.

**macOS:** LaunchAgents run in the **logged-in** user session. A Mac mini without an interactive login may need autologin or another approach for purely headless use.

**Windows:** The scheduled task uses an **at logon** trigger (`RunLevel Limited`). Servers that never log on interactively may need a different trigger or service wrapper.

---

## Install

```bash
go install github.com/an-lee/ghr/cmd/ghr@latest
```

`go install` does not create config files. After installing, run:

```bash
ghr init
```

This creates `~/.ghr/runners.yml` from a template and `~/.ghr/env` for secrets (optional). Then edit those files (or use `ghr config edit` / `ghr config edit-env`), run `ghr config validate` and `ghr doctor`, and you are ready to use `ghr status` and other commands.

Or build from source:

```bash
git clone https://github.com/an-lee/ghr.git
cd ghr
go build -o ghr ./cmd/ghr/
```

On Windows without GNU Make, use `go build` as above; Go writes `ghr.exe` when you pass `-o ghr`.

**Makefile (optional):** Requires [GNU Make](https://www.gnu.org/software/make/) and the same Go version as in `go.mod` / CI (see [Prerequisites](#prerequisites)). Works on Linux and macOS, and on Windows in a Unix-like environment (for example WSL2 or MSYS2) where `make`, `rm`, and a POSIX shell are available. The `install` target uses the `install` utility from coreutils and is intended for Unix-like systems only, not plain cmd.exe or PowerShell.

| Target | Description |
|--------|-------------|
| `make` / `make build` | Build `./cmd/ghr` into `ghr` (or `ghr.exe` on Windows when `OS` is `Windows_NT`). |
| `make test` | Run `go test ./... -race -count=1` (same as CI). |
| `make vet` | Run `go vet ./...`. |
| `make check` | Run `vet` then `test`. |
| `make clean` | Remove the built binary in the repo root. |
| `make install` | Install the binary to `$(PREFIX)/bin` (default `PREFIX=/usr/local`). Set `DESTDIR` for staged installs (packaging). |

To use the checked-in example at `config/runners.yml` while hacking on this repo, point the CLI at it explicitly, for example `export GHR_CONFIG="$PWD/config/runners.yml"` or `ghr -c config/runners.yml status`.

---

## Prerequisites

**On your laptop:**
- Go 1.22+ (for building)
- SSH key-based access to remote runner hosts (not needed for `addr: local` hosts)

**On runner hosts:**
- **Linux** — Docker installed (for `mode: docker`) or just a shell (for `mode: native`)
- **macOS** — `curl` available (pre-installed). For `mode: docker`, install Docker Desktop, OrbStack, or Colima and ensure `docker` works in the same environment as your SSH session (ghr does not auto-install Docker on macOS).

#### Linux SSH user and privileges

`ghr setup` and `ghr update` run remote commands as the SSH user in `hosts.*.addr`. On **Linux**, when a step needs **root** (package installs, GitHub’s `installdependencies.sh` for native runners, or Docker’s install script when Docker is missing), `ghr` uses **`sudo -n`** only — non-interactive `sudo` that never prompts for a password.

**Why `sudo -n`:** Remote automation uses SSH sessions **without a PTY** (no interactive terminal), the same way `ssh` runs a remote command by default. Interactive `sudo` would try to read a password from a TTY and fails with errors such as *“a terminal is required to read the password”*. So `ghr` requires **SSH as root** or **passwordless sudo** (`sudo -n true` must succeed on the host) for those install steps.

**If privilege checks fail**, remote stderr may include:

```text
ghr: remote Linux commands need root SSH or passwordless sudo (non-interactive); SSH has no TTY for sudo passwords. Use NOPASSWD, connect as root, or install software manually. Run: ghr doctor
```

Run **`ghr doctor`** on your laptop to check Linux hosts: it prints **“linux: non-root user has passwordless sudo”** when `sudo -n` works, or **“linux: passwordless sudo not available…”** when it does not. Use **`ghr doctor --strict`** if you want that warning to fail the command.

##### Setting up the SSH user on Linux

- **Dedicated user + SSH keys** — Create a user on the runner host, authorize your public key in `~/.ssh/authorized_keys`, and set `hosts.*.addr` to `user@host` (or hostname).
- **`root@host` in `hosts.*.addr`** — Avoids sudo entirely for `ghr`’s install paths. Only use if your security policy allows SSH as root; prefer key-based auth, disable password login, and restrict network access.
- **Passwordless sudo (`NOPASSWD`)** — On the host, use `visudo` or a drop-in under `/etc/sudoers.d/`. A broad rule such as `runner ALL=(ALL) NOPASSWD: ALL` is common for a **dedicated CI user**. **Command-scoped NOPASSWD** is hard to maintain for Docker’s `get.docker.com` script and `installdependencies.sh` because they run many different commands; if you cannot grant broad NOPASSWD, use **`root@host`** or **pre-install** software so `ghr` does not need those scripts.
- **Pre-install to limit elevation** — Install **Docker** before `ghr setup` and add the SSH user to the **`docker`** group so `docker` works without `sudo`. For **native** mode, ensure **`curl`** and **`tar`** are on `PATH` and install distro packages the runner needs in advance; that reduces how often `installdependencies.sh` must run with sudo.

**Verify from your laptop** (same `user@host` as in `hosts.*.addr`):

```bash
ssh -o BatchMode=yes user@host true
ssh -o BatchMode=yes user@host 'sudo -n true'
```

The first command checks non-interactive SSH (see also [All remote hosts](#all-remote-hosts)). The second must exit **0** if you rely on a non-root user with sudo for automated installs; if it fails, configure `NOPASSWD` for that user or use `root@host`.

- **Docker mode on Linux** — If Docker is not already installed, expect a privilege path (root or working passwordless `sudo`). If Docker is installed and your user can run `docker` without `sudo` (for example via the `docker` group), routine `ghr` operations often do not need elevation. On **macOS**, ghr never auto-installs Docker; install Docker Desktop, OrbStack, or Colima first.
- **Native mode** — You can avoid `sudo` if `curl` and `tar` are present and OS packages the runner needs are already installed; otherwise `ghr` may print warnings and the runner might be incomplete.

`ghr` does not deeply verify sudoers rules; failures show up as remote command errors or warnings.

- **Windows** — OpenSSH Server enabled; Docker Desktop if you want Linux container runners (`mode: docker`):
  ```powershell
  Add-WindowsCapability -Online -Name OpenSSH.Server~~~~0.0.1.0
  Start-Service sshd
  Set-Service -Name sshd -StartupType Automatic
  ```
  **SSH default shell:** OpenSSH may use cmd.exe or PowerShell 7 (`pwsh`) as the remote shell depending on your setup. ghr runs Windows automation via `powershell.exe` or `pwsh.exe` with `-EncodedCommand`, so it works with either default. Use `windows_ps: pwsh` on the host if you rely on PowerShell 7 only and do not have Windows PowerShell 5.1.

## Host setup (manual steps)

ghr automates runner installation and lifecycle over SSH, but some host preparation is still manual. After you edit config and secrets, run **`ghr doctor`** from your laptop to verify config paths, GitHub API access, SSH connectivity, and per-host tools (Docker vs native). By default the command exits with a non-zero status only when a check is **FAIL**; use **`ghr doctor --strict`** if you also want **WARN** lines to fail (for example in CI).

### All remote hosts

- Confirm non-interactive SSH works: `ssh -o BatchMode=yes user@host true` (use the same user and host as in `hosts.*.addr`).
- **Host keys:** when `~/.ssh/known_hosts` exists, ghr verifies server keys the same way as the Go `knownhosts` package. Connect once with plain `ssh` if you need to accept a new host key before `ghr doctor` or `ghr setup`.
- Remote commands run as the SSH user from your config; that user must have permission to install or run runners as documented for each OS below.

Run **`ghr doctor`** (optionally with `--host` / `--repo`) to confirm connectivity from your control machine.

### Linux

For the full explanation of non-interactive SSH, the `ghr: remote Linux commands need root…` error, `NOPASSWD`, and verification commands, see **[Linux SSH user and privileges](#linux-ssh-user-and-privileges)** under Prerequisites.

- **Docker mode (default on Linux):** If Docker is not installed, `ghr setup` can run Docker’s install script, which requires **root** or **passwordless sudo** (`sudo -n`) over SSH (see the section linked above). To avoid that path, install Docker yourself and ensure the SSH user can run `docker` (for example membership in the `docker` group).
- **Native mode:** Ensure `curl` and `tar` are on `PATH`. `ghr setup` may still invoke GitHub’s `installdependencies.sh` with **`sudo -n`** when the SSH user is not root and passwordless sudo is available.

Run **`ghr doctor`** after the host is prepared.

### macOS

- **Native:** `curl` is usually sufficient; ghr downloads the Actions runner over HTTPS.
- **Docker mode:** Install [Docker Desktop](https://www.docker.com/products/docker-desktop/), [OrbStack](https://orbstack.dev/), or [Colima](https://github.com/abiosoft/colima). The `docker` CLI must work in the **same** environment as your SSH session (user, `PATH`, and Docker socket). Start the Docker engine (app or `colima start`, etc.) before relying on docker mode.

Run **`ghr doctor`** to confirm `docker info` from the SSH session.

### Windows

- Install and enable **OpenSSH Server** (see the PowerShell snippet under [Prerequisites](#prerequisites)).
- **Docker mode (Linux containers on the same Windows host):** Install [Docker Desktop](https://www.docker.com/products/docker-desktop/), use the default **Linux containers** mode, and keep Docker running. ghr does not install or start Docker Desktop for you. Because Docker's Windows CLI may auto-detect `wincred` in non-interactive SSH sessions and fail with `A specified logon session does not exist`, ghr runs Windows Docker commands with an isolated `DOCKER_CONFIG` that disables credential helpers instead of relying on `%USERPROFILE%/.docker/config.json`.

- **Native mode:** After `ghr setup`, run **`ghr up`** to start the listener. Registration logs under `_diag` show **configure** finishing with exit code 0; that is normal and does not mean the runner is listening. For the **run** phase, check **`%USERPROFILE%\.ghr\runners\<instance>\runner.log`**. Over SSH, **`Start-Process` listeners are killed when the session disconnects**; ghr starts the listener with **`Win32_Process.Create` (CIM)** so it keeps running after `ghr` exits. If the runner shows **stopped** with `runner registration has been deleted from the server`, GitHub auto-pruned the stale registration. `ghr up` detects this automatically, re-registers, and retries. You can also fix it manually with **`ghr update <runner>`** (which removes, re-configures, and starts in one step). If `ghr status` shows **stopped** immediately after `ghr up` for other reasons, check `runner.log` for error details. As a fallback you can run **`run.cmd`** from an interactive session (RDP), use **Task Scheduler** at logon, or install the runner as a **Windows service** via GitHub’s `config.cmd` options.

Run **`ghr doctor`** to confirm SSH and Docker readiness.

---

## Config file location

When you do **not** pass `--config` / `-c`, the config file is chosen in this order:

1. **`GHR_CONFIG`** — path to a YAML file (absolute or relative to the current working directory).
2. **`~/.ghr/runners.yml`** — default after `ghr init`.

There is no automatic discovery of `./config/runners.yml` in the current directory; use `GHR_CONFIG` or `-c` if your file lives elsewhere.

If you pass `-c /path/to/runners.yml`, that path is always used (and `GHR_CONFIG` is ignored).

Run `ghr config path` to see which config file and `~/.ghr/env` path apply in your environment.

## Secrets (`~/.ghr/env`)

Before the YAML file is loaded, `ghr` applies environment variables from **`~/.ghr/env`** if that file exists (dotenv-style: `KEY=value`, optional `export `, `#` comments). This keeps secrets out of your shell history and out of the YAML file. Pair it with `github.pat: env:GITHUB_PAT` in `runners.yml`. Create the directory and file with `ghr init`, or run `ghr config edit-env`.

Keep `~/.ghr` permissions tight (`chmod 700 ~/.ghr`, `chmod 600 ~/.ghr/env` if you create files by hand).

## Configuration

### 1. Create a GitHub PAT

The GitHub user that creates the token must have **admin** access to every repository listed under `runners[].repo` in your config; the REST API requires that for self-hosted runner management.

`ghr` uses the PAT against the GitHub API for:

| Operation | REST (summary) | Used for |
| --- | --- | --- |
| Start / register runners | `POST /repos/{owner}/{repo}/actions/runners/registration-token` | Native and Docker runner startup |
| Stop / remove runners (native) | `POST .../actions/runners/remove-token` | Native runner removal |
| Dashboard / status | `GET .../actions/runners` | Match runner names; online / offline / busy |
| `ghr cleanup` | `DELETE .../actions/runners/{runner_id}` | Remove offline runners from GitHub |

Fetching the latest runner package version uses the public `actions/runner` releases API and does not require extra token permissions beyond a valid request.

#### Fine-grained personal access token (recommended)

1. Go to **GitHub → Settings → Developer settings → Personal access tokens → Fine-grained tokens**.
2. Under **Repository access**, include every `owner/repo` you configure in `runners`.
3. Under **Permissions → Repository permissions**, set **Administration** to **Read and write**. That level covers listing runners, creating registration and removal tokens, and deleting runners, as defined in GitHub’s [repository permissions for “Administration”](https://docs.github.com/en/rest/authentication/permissions-required-for-fine-grained-personal-access-tokens#repository-permissions-for-administration). See also [REST API endpoints for self-hosted runners](https://docs.github.com/en/rest/actions/self-hosted-runners).

#### Personal access token (classic)

If you use a **classic** token instead, GitHub documents the **`repo`** scope for repository-level runner endpoints (for example [create a registration token for a repository](https://docs.github.com/en/rest/actions/self-hosted-runners#create-a-registration-token-for-a-repository)).

If you see **403** responses or an empty registration token, confirm **Administration** read/write on a fine-grained token (and that the token includes the target repositories), or the **`repo`** scope on a classic token.

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

### Config reference

| Field | Description |
|---|---|
| `github.pat` | GitHub PAT. Use `env:VAR_NAME` to read from environment. |
| `hosts.<name>.addr` | SSH target (`user@host` or `user@ip`), or `local` to run on the machine where ghr is running. Remote commands run as that user; on Linux, privilege expectations for `setup` / `update` follow the [Linux SSH user and privileges](#linux-ssh-user-and-privileges) section. |
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

---

## Commands

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

### Filters

All commands accept `--host` and `--repo` flags:

```bash
ghr doctor --host mac-mini
ghr status --host mac-mini
ghr logs myapp-1 --host win-pc   # disambiguate when the same instance name exists on multiple hosts
ghr up --repo an-lee/enjoy
ghr down enjoy-win-1
```

---

## Quick start

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

1. Add an entry under `hosts` in your resolved config file (`~/.ghr/runners.yml` by default)
2. Ensure SSH key-based access works: `ssh user@host true`
3. Add runner entries referencing the new host
4. Run `ghr setup && ghr up`

### Scale up

Change `count` in your runners YAML, then:

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

## Local host runners

You can set up runners on the same machine where ghr runs, without SSH. Set `addr: local` on a host and ghr will execute commands directly via `os/exec` instead of dialing an SSH connection.

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

---

## Linux runners on a Windows host

You can run Linux container runners on a Windows machine without a separate SSH endpoint into WSL2. Set `mode: docker` on a runner that targets a `os: windows` host and ghr will manage the Docker container over the same SSH connection (Docker CLI invoked through the same encoded PowerShell path as native Windows runners).

**Requirements on the Windows host:**
- OpenSSH Server enabled (see [Prerequisites](#prerequisites))
- Docker Desktop installed and running with the default Linux containers mode

```yaml
hosts:
  win-pc:
    addr: user@192.168.1.51
    os: windows
    arch: amd64

runners:
  # Native Windows runner
  - name: myapp-win
    repo: owner/repo
    host: win-pc
    labels: [self-hosted, Windows, X64]

  # Linux runner via Docker Desktop on the same Windows host
  - name: myapp-linux
    repo: owner/repo
    host: win-pc
    mode: docker
    labels: [self-hosted, Linux, X64]
```

Both runners share a single SSH connection to Windows. The native runner starts `run.cmd` via PowerShell; the Docker runner calls `docker run` the same way, which talks to Docker Desktop's Linux engine.

---

## Linux runners on a macOS host

Set `mode: docker` on a runner that targets an `os: darwin` host to run the same Linux container image as on Linux. ghr does **not** run the Linux Docker install script on macOS; install a Docker runtime yourself.

**Requirements on the Mac:**
- Docker Desktop, OrbStack, or Colima installed, with `docker` working in the environment where `ghr` runs commands (for example the same user over SSH).

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
      connection.go         # SSH connection management (Executor interface)
      local.go              # Local command execution (addr: local)
    doctor/
      doctor.go             # ghr doctor diagnostics
    ops/
      ops.go                # Shared setup/up/down/restart/update/status/logs/cleanup (CLI + TUI)
      service.go            # ghr service install/uninstall/status
    autostart/
      autostart.go          # systemd / launchd / Windows task install and Detect/Start/Stop
      active.go             # Supervisor active check for ghr status
      generate.go           # Unit and plist text generation
      sanitize.go           # Safe names for unit files and tasks
    runner/
      runner.go             # Runner lifecycle orchestration
      native.go             # Native runner management (mac/win/linux)
      docker.go             # Docker runner management
      github.go             # GitHub API client
    tui/
      dashboard.go          # Interactive TUI dashboard (model + update)
      dashboard_view.go     # TUI views and layout
      status.go             # Status table rendering
      styles.go             # Lipgloss styles
  config/
    runners.yml             # Example YAML (not auto-loaded; use GHR_CONFIG or -c)
  go.mod
  go.sum
```

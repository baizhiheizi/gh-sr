---
title: "Architecture"
weight: 60
---

# Architecture

This page explains where runners run, how **gh sr** reaches each host, and how `status`, `dashboard`, and `logs` collect information.

## Control plane vs execution plane

**gh sr** is a **control plane** only: it runs on your machine and issues commands. The **GitHub Actions runner** (native process or Docker container) always runs on the **target host** from your config—either the same machine as **gh sr** when `addr: local`, or a remote machine over SSH.

```mermaid
flowchart LR
  subgraph controlPlane [Control_plane]
    gh sr[gh sr_CLI]
  end
  subgraph targets [Runner_hosts]
    h1[Remote_host_SSH]
    h2[Local_addr_local]
  end
  gh sr -->|SSH_session| h1
  gh sr -->|os_exec| h2
  h1 --> n1[Native_runner_or_Docker]
  h2 --> n2[Native_runner_or_Docker]
```

## Where runners run

**Mode** (`native` vs `docker`) is resolved per runner. If you omit `mode`, Linux hosts default to **docker**; Windows and macOS default to **native**. You can override with `runners[].mode`.

| Host OS | Default mode | Where the workload runs |
|--------|--------------|-------------------------|
| Linux | `docker` | Docker on that host: container name `gh-runner-<instance>`, image `ghcr.io/actions/actions-runner:latest`. gh sr starts it with `config.sh --unattended` (once per container) then `run.sh`, using `ACTIONS_RUNNER_INPUT_*` env vars. On non-Windows hosts the container typically mounts the Docker socket for workflow `docker` steps. |
| Linux | `native` (if set) | Files under `~/.gh-sr/runners/<instance>`; process via `run.sh` and a PID file. |
| Windows | `native` | `%USERPROFILE%\.gh-sr\runners\<instance>`; process via `run.cmd` and a PID file. |
| Windows | `docker` (if set) | Same Docker image on Docker Desktop over the same SSH connection as native Windows runners. |
| macOS | `native` | `~/.gh-sr/runners/<instance>`. |
| macOS | `docker` (if set) | Same Linux runner image as on Linux; requires Docker Desktop, OrbStack, or Colima with a working `docker` CLI. |

**Instances:** `count` creates separate runners named `<name>-1`, `<name>-2`, … on the same host, each with its own directory (native) or container (docker).

```mermaid
flowchart TB
  subgraph linuxHost [Linux_host]
    ld[Default_docker]
    ln[Explicit_native]
    ld --> c1[Container_gh_runner_name_1]
    ln --> d1[Dir_HOME_gh sr_runners_name_1]
  end
  subgraph winHost [Windows_host]
    wn[Default_native]
    wd[Explicit_docker]
    wn --> wd1[Dir_USERPROFILE_gh sr_runners_name_1]
    wd --> wc1[Docker_Desktop_container]
  end
  subgraph macHost [macOS_host]
    mn[Default_native]
    md[Explicit_docker]
    mn --> md1[Dir_HOME_gh sr_runners_name_1]
    md --> mc1[Docker_Linux_container]
  end
```

## How commands reach a host

- **`addr: local`** — **gh sr** runs commands on the machine where the CLI runs (no SSH).
- **Remote SSH** — **gh sr** opens one SSH client per host for the duration of that subcommand; each remote action uses a new session on that connection.
- **Windows over SSH** — Remote automation uses PowerShell (`powershell.exe` or `pwsh.exe` per `windows_ps`) with an encoded command so quoting works regardless of the user’s default SSH shell.
- **Linux / macOS over SSH** — Commands run as shell commands on the remote user’s environment.

```mermaid
sequenceDiagram
  participant User
  participant GhWm as gh sr_CLI
  participant Host as Target_host
  User->>GhWm: gh sr_up_runner_name
  GhWm->>GhWm: Load_config_resolve_mode
  GhWm->>Host: Connect_SSH_or_local
  GhWm->>Host: Start_native_or_docker_per_instance
  Host-->>GhWm: OK_or_error
```

## Lifecycle commands (what they do)

| Command | Behavior |
|---------|----------|
| `gh sr setup` | Install/configure runner software or Docker image pull on the host. For **docker** mode on a given host, only the **first** matching runner row runs host-level Docker setup; additional docker-mode runners on that host skip duplicate setup. |
| `gh sr up` / `gh sr down` | Start or stop each instance (native process or container). |
| `gh sr restart` | `down` then `up`; stop errors are ignored before start. |
| `gh sr update` | Remove local runner registration (native) or container (docker), then setup and start again—use when upgrading the runner stack. |
| `gh sr cleanup` | Deletes **offline** runners from the GitHub API only; it does not remove local install dirs or Docker containers. |
| `gh sr service install` / `uninstall` / `status` | **Native** runners only: install OS autostart (systemd on Linux, LaunchAgent on macOS, scheduled task at logon on Windows). **Docker** rows are skipped; containers already use Docker’s `--restart unless-stopped`. After install, `gh sr up` and `gh sr down` start/stop the same supervisor instead of a bare `nohup` / Win32 process. |
| `gh sr doctor` | Read-only checks: local paths, config, GitHub API, SSH targets, Docker vs native prerequisites. Use `--strict` to treat **WARN** as failure. |

```mermaid
flowchart TD
  setup[gh sr_setup] --> mode{Mode}
  mode -->|docker| dock[Ensure_Docker_and_pull_image]
  mode -->|native| nat[Download_configure_actions_runner]
  up[gh sr_up] --> start[Start_instances]
  down[gh sr_down] --> stop[Stop_instances]
  update[gh sr_update] --> remove[Remove_then_setup_then_up]
  cleanup[gh sr_cleanup] --> api[GitHub_API_delete_offline_only]
```

## Status, dashboard, and logs

**`gh sr status`** and **`gh sr dashboard`** both:

1. Connect to each host **concurrently** (or mark instances **unreachable** if connection fails) — all SSH connections are opened in parallel so the command completes in roughly the time it takes to reach the slowest host.
2. Ask the host whether each instance is **running** or **stopped** (native: PID file + process check; docker: `docker inspect`).
3. Call the **GitHub API** for each repo **concurrently** to match runner **names** and the runner’s **OS** (from the API’s `os` field vs native vs docker mode) so two config rows with the same instance name on different platforms do not show each other’s GitHub state.

**`gh sr logs <name>`** looks up the runner block by **base name** or a full **instance** name (`name-1`, `name-2`, …), connects to the host, then tails logs for that instance. Use **`--host`** when the same instance name exists on more than one machine.

- **Native** — Last lines of `runner.log` under that instance’s directory (`~/.gh-sr/runners/<instance>` on Linux/macOS, or `%USERPROFILE%\.gh-sr\runners\<instance>` on Windows). On Windows, if that file is missing or empty, gh sr tails the newest `*.log` under `_diag` in the same directory.
- **Docker** — Last lines from `docker logs` for container `gh-runner-<instance>`.

If `count` is greater than 1, pass the specific instance (for example `myapp-1`) so logs match the right directory or container.

```mermaid
flowchart LR
  subgraph localCheck [On_each_host]
    L[Process_or_container_state]
  end
  subgraph remoteApi [GitHub]
    A[Runner_list_API]
  end
  statusCmd[gh sr_status_or_dashboard] --> localCheck
  statusCmd --> A
  localCheck --> table[CLI_or_TUI_table]
  A --> table
  logsCmd[gh sr_logs] --> localCheck
  logsCmd --> tail[Log_tail_native_or_docker]
```

## Autostart and reboot

**Docker mode:** `docker run` uses **`--restart unless-stopped`**. If the container was **running** when the host shut down, it is typically started again when the Docker daemon comes up after reboot. **`gh sr down`** runs `docker stop`; a **stopped** container is **not** brought back on boot until you run **`gh sr up`** (or start the container another way).

**Native mode:** A normal **`gh sr up`** starts the listener as a background process that **does not** survive reboot. Install autostart once per instance:

```bash
gh sr service install              # all runners (native only; docker rows are skipped)
gh sr service install myrunner     # one runner block
gh sr service install --system     # Linux only: unit in /etc/systemd/system (needs passwordless sudo or root SSH)
gh sr service status
gh sr service uninstall
```

**Linux (user units, default):** Units live under `~/.config/systemd/user/`. On many **headless** servers the user manager does not run at boot until someone logs in. Enable **lingering** once (as root): `loginctl enable-linger <ssh-user>` so `systemctl --user` services start at boot.

**Linux (`--system`):** Writes `/etc/systemd/system/ghsr-runner-<instance>.service` with `User=` / `Group=` set to the SSH user. Requires the same non-interactive **`sudo`** behavior as `gh sr setup` on Linux.

**macOS:** LaunchAgents run in the **logged-in** user session. A Mac mini without an interactive login may need autologin or another approach for purely headless use.

**Windows:** The scheduled task uses an **at logon** trigger (`RunLevel Limited`). Servers that never log on interactively may need a different trigger or service wrapper.

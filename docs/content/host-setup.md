---
title: "Host Setup"
weight: 40
---

# Host setup (manual steps)

**ghr** automates runner installation and lifecycle over SSH, but some host preparation is still manual. After you edit config and secrets, run **`ghr doctor`** from your laptop to verify config paths, GitHub API access, SSH connectivity, and per-host tools (Docker vs native). By default the command exits with a non-zero status only when a check is **FAIL**; use **`ghr doctor --strict`** if you also want **WARN** lines to fail (for example in CI).

## Linux SSH user and privileges

`ghr setup` and `ghr update` run remote commands as the SSH user in `hosts.*.addr`. On **Linux**, when a step needs **root** (package installs, GitHub’s `installdependencies.sh` for native runners, or Docker’s install script when Docker is missing), **ghr** uses **`sudo -n`** only — non-interactive `sudo` that never prompts for a password.

**Why `sudo -n`:** Remote automation uses SSH sessions **without a PTY** (no interactive terminal), the same way `ssh` runs a remote command by default. Interactive `sudo` would try to read a password from a TTY and fails with errors such as *“a terminal is required to read the password”*. So **ghr** requires **SSH as root** or **passwordless sudo** (`sudo -n true` must succeed on the host) for those install steps.

**If privilege checks fail**, remote stderr may include:

```text
ghr: remote Linux commands need root SSH or passwordless sudo (non-interactive); SSH has no TTY for sudo passwords. Use NOPASSWD, connect as root, or install software manually. Run: ghr doctor
```

Run **`ghr doctor`** on your laptop to check Linux hosts: it prints **“linux: non-root user has passwordless sudo”** when `sudo -n` works, or **“linux: passwordless sudo not available…”** when it does not. Use **`ghr doctor --strict`** if you want that warning to fail the command.

### Setting up the SSH user on Linux

- **Dedicated user + SSH keys** — Create a user on the runner host, authorize your public key in `~/.ssh/authorized_keys`, and set `hosts.*.addr` to `user@host` (or hostname).
- **`root@host` in `hosts.*.addr`** — Avoids sudo entirely for **ghr**’s install paths. Only use if your security policy allows SSH as root; prefer key-based auth, disable password login, and restrict network access.
- **Passwordless sudo (`NOPASSWD`)** — On the host, use `visudo` or a drop-in under `/etc/sudoers.d/`. A broad rule such as `runner ALL=(ALL) NOPASSWD: ALL` is common for a **dedicated CI user**. **Command-scoped NOPASSWD** is hard to maintain for Docker’s `get.docker.com` script and `installdependencies.sh` because they run many different commands; if you cannot grant broad NOPASSWD, use **`root@host`** or **pre-install** software so **ghr** does not need those scripts.
- **Pre-install to limit elevation** — Install **Docker** before `ghr setup` and add the SSH user to the **`docker`** group so `docker` works without `sudo`. For **native** mode, ensure **`curl`** and **`tar`** are on `PATH` and install distro packages the runner needs in advance; that reduces how often `installdependencies.sh` must run with sudo.

**Verify from your laptop** (same `user@host` as in `hosts.*.addr`):

```bash
ssh -o BatchMode=yes user@host true
ssh -o BatchMode=yes user@host 'sudo -n true'
```

The first command checks non-interactive SSH (see also [All remote hosts](#all-remote-hosts)). The second must exit **0** if you rely on a non-root user with sudo for automated installs; if it fails, configure `NOPASSWD` for that user or use `root@host`.

- **Docker mode on Linux** — If Docker is not already installed, expect a privilege path (root or working passwordless `sudo`). If Docker is installed and your user can run `docker` without `sudo` (for example via the `docker` group), routine **ghr** operations often do not need elevation. On **macOS**, ghr never auto-installs Docker; install Docker Desktop, OrbStack, or Colima first.
- **Native mode** — You can avoid `sudo` if `curl` and `tar` are present and OS packages the runner needs are already installed; otherwise **ghr** may print warnings and the runner might be incomplete.

**ghr** does not deeply verify sudoers rules; failures show up as remote command errors or warnings.

### Docker socket permissions (Linux/macOS)

When `ghr` starts a docker-mode runner container on Linux or macOS, it bind-mounts the Docker socket into the container at `/var/run/docker.sock` so jobs can reach the Docker daemon. **Resolving the host socket:** if `docker_socket` is not set, ghr uses the first path that exists: `/var/run/docker.sock`; otherwise the `unix://` endpoint from the host’s current default Docker context (`docker context inspect`, which matches Colima and Linux rootless when that context is active); on **macOS** only, `~/.colima/default/docker.sock` as a last resort. Set `docker_socket` when your engine uses a different path (for example a named Colima profile that is not the default context).

**Colima on macOS (virtiofs):** If that resolved path is under `~/.colima/…`, ghr still uses it to find the API over SSH, but for `docker run -v` it bind-mounts **`/var/run/docker.sock`** (the path inside the Colima VM) instead of the macOS host file under `~/.colima/…`. Bind-mounting the host Colima client socket fails on common setups with virtiofs (`operation not supported`; see [Colima #997](https://github.com/abiosoft/colima/issues/997)). If you need a manual workaround instead, you can start Colima with `colima start --mount-type sshfs`.

On **Linux**, the **`runner` user inside the container** (uid 1001) must also have permission to use the socket — ghr handles this automatically by:

1. Querying the socket's owning GID on the host with `stat -c '%g'` on the resolved host socket path.
2. Passing `--group-add <GID>` to `docker run` so the container's runner user becomes a member of that group.
3. Verifying the socket with `test -S` on the resolved path before starting the container.

On **macOS** (Docker Desktop, OrbStack, Colima), the socket is accessible to all processes inside the VM — there is no docker group GID mismatch. ghr skips the `stat` query and `--group-add` on macOS hosts; only the bind-mount is applied.

**If agentic-workflow tooling inside a job gets `permission denied` on the Docker socket**, the container was started without this `--group-add` flag (for example, by an older version of ghr or a manually issued `docker run`). Recreate the runner:

```bash
ghr down <runner-name>
ghr up <runner-name>
```

**Rootless Docker** (Linux) and **Colima** often use a non-default socket; ghr usually finds it via the default Docker context without extra config. If the active context does not point at the socket you want (for example a **named Colima profile** that is not selected in `docker context`), set `docker_socket` explicitly (supported on Linux and macOS; not applicable on Windows):

```yaml
hosts:
  my-linux:
    addr: user@192.168.1.10
    os: linux
    arch: amd64
    docker_socket: /run/user/1000/docker.sock
```

ghr will bind-mount the custom path into the container at `/var/run/docker.sock` (preserving the default `DOCKER_HOST` path that job scripts expect) and still adds `--group-add` for the socket's GID.

**Verify from inside a running runner container:**

```bash
docker exec gh-runner-<instance> test -S /var/run/docker.sock && echo ok || echo missing
```

`ghr doctor` performs this check automatically for all running docker-mode containers on Linux and macOS hosts and reports a WARN if the socket is not accessible inside.

## GitHub Agentic Workflows (gh-aw)

Repositories using [GitHub Agentic Workflows](https://github.github.com/gh-aw/guides/self-hosted-runners/) start an MCP gateway in Docker with host networking. Health checks use `http://localhost:80` from the job environment, which only matches that gateway when the job runs on the **same network namespace** as the Docker engine (for example GitHub-hosted runners or a **native** self-hosted runner).

| Host | Runner mode | Typical approach for gh-aw |
|------|-------------|----------------------------|
| **Linux** | **Native** | Works; job runs on the host. |
| **Linux** | **Docker** (default bridge) | Health check often **fails** (job `localhost` is inside the runner container). Set **`docker_network_mode: host`** on that runner so the container uses the host network (weaker isolation; port **80** must be free on the host). Then **`ghr down`** / **`ghr up`** to recreate the container. |
| **macOS** | **Docker** (Desktop, OrbStack, Colima) | Set **`docker_network_mode: host`**. On Docker Desktop / OrbStack / Colima the runner container joins the Linux VM's network namespace — the same namespace the MCP gateway uses — so `localhost:80` health checks and `host.docker.internal` resolution work. Port **80** must be free inside the VM. Then **`ghr down`** / **`ghr up`** to recreate the container. |
| **Windows** | **Docker** (Linux containers) | Set **`docker_network_mode: host`**. Docker Desktop for Windows runs Linux containers inside a WSL2 VM. With host networking, the runner container and the MCP gateway share the VM's network namespace, so health checks and MCP connections succeed. Port **80** must be free inside the VM. Then **`ghr down`** / **`ghr up`** to recreate the container. |

> **Note:** On macOS and Windows, `--network host` means the Docker Desktop Linux VM's network namespace, not the macOS/Windows host itself. This is sufficient for gh-aw because all containers (runner, MCP gateway, MCP servers) share that same VM namespace.

Workflows that run the [Agent Workflow Firewall](https://github.com/github/gh-aw-firewall) (`awf`) also need **`docker_cap_add: [NET_ADMIN]`** on **`mode: docker`** runners (see [Configuration](configuration.md)). `awf` configures host-level iptables (including the `DOCKER-USER` chain); the actions-runner container must have the **`NET_ADMIN`** capability or `sudo awf` can fail during firewall setup even when networking is correct. After editing config, run **`ghr down`** / **`ghr up`** so the container is recreated with the new flags.

**Verification (optional):** Keep Docker’s default iptables integration (do not set **`"iptables": false"`** in the engine `daemon.json`). On Docker Desktop, you can open a shell in the Linux engine (for example the **`docker-desktop`** WSL distro) and run **`sudo iptables -L DOCKER-USER -n`** — when the daemon is healthy, that chain is normally present. Then run a workflow that uses `awf` to confirm the job completes past firewall setup.

**`ghr doctor`** prints a **WARN** when any docker-mode runner on a host still uses the default bridge network, as a reminder for gh-aw.

## Windows (OpenSSH and Docker)

- **Windows** — OpenSSH Server enabled; Docker Desktop if you want Linux container runners (`mode: docker`):

  ```powershell
  Add-WindowsCapability -Online -Name OpenSSH.Server~~~~0.0.1.0
  Start-Service sshd
  Set-Service -Name sshd -StartupType Automatic
  ```

  **SSH default shell:** OpenSSH may use cmd.exe or PowerShell 7 (`pwsh`) as the remote shell depending on your setup. ghr runs Windows automation via `powershell.exe` or `pwsh.exe` with `-EncodedCommand`, so it works with either default. Use `windows_ps: pwsh` on the host if you rely on PowerShell 7 only and do not have Windows PowerShell 5.1.

## All remote hosts

- Confirm non-interactive SSH works: `ssh -o BatchMode=yes user@host true` (use the same user and host as in `hosts.*.addr`).
- **Host keys:** when `~/.ssh/known_hosts` exists, ghr verifies server keys the same way as the Go `knownhosts` package. Connect once with plain `ssh` if you need to accept a new host key before `ghr doctor` or `ghr setup`.
- Remote commands run as the SSH user from your config; that user must have permission to install or run runners as documented for each OS below.

Run **`ghr doctor`** (optionally with `--host` / `--repo`) to confirm connectivity from your control machine.

## Linux

For the full explanation of non-interactive SSH, the `ghr: remote Linux commands need root…` error, `NOPASSWD`, and verification commands, see [Linux SSH user and privileges](#linux-ssh-user-and-privileges) above.

- **Docker mode (default on Linux):** If Docker is not installed, `ghr setup` can run Docker’s install script, which requires **root** or **passwordless sudo** (`sudo -n`) over SSH (see the section linked above). To avoid that path, install Docker yourself and ensure the SSH user can run `docker` (for example membership in the `docker` group). ghr automatically passes `--group-add` for the socket GID and verifies the socket before starting containers; see [Docker socket permissions](#docker-socket-permissions-linuxmacos) above for path resolution and the optional `docker_socket` override.
- **Native mode:** Ensure `curl` and `tar` are on `PATH`. `ghr setup` may still invoke GitHub’s `installdependencies.sh` with **`sudo -n`** when the SSH user is not root and passwordless sudo is available.

Run **`ghr doctor`** after the host is prepared.

## macOS

- **Native:** `curl` is usually sufficient; ghr downloads the Actions runner over HTTPS.
- **Docker mode:** Install [Docker Desktop](https://www.docker.com/products/docker-desktop/), [OrbStack](https://orbstack.dev/), or [Colima](https://github.com/abiosoft/colima). The `docker` CLI must work in the **same** environment as your SSH session (user, `PATH`, and Docker context). Start the Docker engine (app or `colima start`, etc.) before relying on docker mode. **Docker socket permissions:** macOS Docker Desktop, OrbStack, and Colima use a Unix socket that is accessible to all processes; ghr does not need `--group-add` on macOS and skips the GID detection step. ghr bind-mounts the resolved host socket at `/var/run/docker.sock` inside the container (matching the Linux default). Override with `docker_socket` only when needed — see [Docker socket permissions](#docker-socket-permissions-linuxmacos) below.

Run **`ghr doctor`** to confirm `docker info` from the SSH session.

## Windows (runner behavior)

- Install and enable **OpenSSH Server** (see the PowerShell snippet under [Windows (OpenSSH and Docker)](#windows-openssh-and-docker)).
- **Docker mode (Linux containers on the same Windows host):** Install [Docker Desktop](https://www.docker.com/products/docker-desktop/), use the default **Linux containers** mode, and keep Docker running. ghr does not install or start Docker Desktop for you. Because Docker's Windows CLI may auto-detect `wincred` in non-interactive SSH sessions and fail with `A specified logon session does not exist`, ghr runs Windows Docker commands with an isolated `DOCKER_CONFIG` that disables credential helpers instead of relying on `%USERPROFILE%/.docker/config.json`.

- **Native mode:** After `ghr setup`, run **`ghr up`** to start the listener. Registration logs under `_diag` show **configure** finishing with exit code 0; that is normal and does not mean the runner is listening. For the **run** phase, check **`%USERPROFILE%\.ghr\runners\<instance>\runner.log`**. Over SSH, **`Start-Process` listeners are killed when the session disconnects**; ghr starts the listener with **`Win32_Process.Create` (CIM)** so it keeps running after `ghr` exits. If the runner shows **stopped** with `runner registration has been deleted from the server`, GitHub auto-pruned the stale registration. `ghr up` detects this automatically, re-registers, and retries. You can also fix it manually with **`ghr update <runner>`** (which removes, re-configures, and starts in one step). If `ghr status` shows **stopped** immediately after `ghr up` for other reasons, check `runner.log` for error details. As a fallback you can run **`run.cmd`** from an interactive session (RDP), use **Task Scheduler** at logon, or install the runner as a **Windows service** via GitHub’s `config.cmd` options.

Run **`ghr doctor`** to confirm SSH and Docker readiness.

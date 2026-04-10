---
title: "Host Setup"
weight: 40
---

# Host setup (manual steps)

**gh sr** automates runner installation and lifecycle over SSH, but some host preparation is still manual. After you edit config, run **`gh sr doctor`** from your laptop to verify config paths, GitHub API access, SSH connectivity, and per-host tools (Docker vs native). By default the command exits with a non-zero status only when a check is **FAIL**; use **`gh sr doctor --strict`** if you also want **WARN** lines to fail (for example in CI).

## Linux SSH user and privileges

`gh sr setup` and `gh sr update` run remote commands as the SSH user in `hosts.*.addr`. On **Linux**, when a step needs **root** (package installs, GitHub’s `installdependencies.sh` for native runners, or Docker’s install script when Docker is missing), **gh sr** uses **`sudo -n`** only — non-interactive `sudo` that never prompts for a password.

**Why `sudo -n`:** Remote automation uses SSH sessions **without a PTY** (no interactive terminal), the same way `ssh` runs a remote command by default. Interactive `sudo` would try to read a password from a TTY and fails with errors such as *“a terminal is required to read the password”*. So **gh sr** requires **SSH as root** or **passwordless sudo** (`sudo -n true` must succeed on the host) for those install steps.

**If privilege checks fail**, remote stderr may include:

```text
gh sr: remote Linux commands need root SSH or passwordless sudo (non-interactive); SSH has no TTY for sudo passwords. Use NOPASSWD, connect as root, or install software manually. Run: gh sr doctor
```

Run **`gh sr doctor`** on your laptop to check Linux hosts: it prints **“linux: non-root user has passwordless sudo”** when `sudo -n` works, or **“linux: passwordless sudo not available…”** when it does not. Use **`gh sr doctor --strict`** if you want that warning to fail the command.

### Setting up the SSH user on Linux

- **Dedicated user + SSH keys** — Create a user on the runner host, authorize your public key in `~/.ssh/authorized_keys`, and set `hosts.*.addr` to `user@host` (or hostname).
- **`root@host` in `hosts.*.addr`** — Avoids sudo entirely for **gh sr**’s install paths. Only use if your security policy allows SSH as root; prefer key-based auth, disable password login, and restrict network access.
- **Passwordless sudo (`NOPASSWD`)** — On the host, use `visudo` or a drop-in under `/etc/sudoers.d/`. A broad rule such as `runner ALL=(ALL) NOPASSWD: ALL` is common for a **dedicated CI user**. **Command-scoped NOPASSWD** is hard to maintain for Docker’s `get.docker.com` script and `installdependencies.sh` because they run many different commands; if you cannot grant broad NOPASSWD, use **`root@host`** or **pre-install** software so **gh sr** does not need those scripts.
- **Pre-install to limit elevation** — Install **Docker** before `gh sr setup` and add the SSH user to the **`docker`** group so `docker` works without `sudo`. For **native** mode, ensure **`curl`** and **`tar`** are on `PATH` and install distro packages the runner needs in advance; that reduces how often `installdependencies.sh` must run with sudo.

**Verify from your laptop** (same `user@host` as in `hosts.*.addr`):

```bash
ssh -o BatchMode=yes user@host true
ssh -o BatchMode=yes user@host 'sudo -n true'
```

The first command checks non-interactive SSH (see also [All remote hosts](#all-remote-hosts)). The second must exit **0** if you rely on a non-root user with sudo for automated installs; if it fails, configure `NOPASSWD` for that user or use `root@host`.

- **Docker mode on Linux** — If Docker is not already installed, expect a privilege path (root or working passwordless `sudo`). If Docker is installed and your user can run `docker` without `sudo` (for example via the `docker` group), routine **gh sr** operations often do not need elevation. On **macOS**, gh sr never auto-installs Docker; install Docker Desktop, OrbStack, or Colima first.
- **Native mode** — You can avoid `sudo` if `curl` and `tar` are present and OS packages the runner needs are already installed; otherwise **gh sr** may print warnings and the runner might be incomplete.

**gh sr** does not deeply verify sudoers rules; failures show up as remote command errors or warnings.

### Docker socket permissions (Linux/macOS)

When `gh sr` starts a docker-mode runner container on Linux or macOS, it bind-mounts the Docker socket into the container at `/var/run/docker.sock` so jobs can reach the Docker daemon. **Resolving the host socket:** if `docker_socket` is not set, gh sr uses the first path that exists: `/var/run/docker.sock`; otherwise the `unix://` endpoint from the host’s current default Docker context (`docker context inspect`, which matches Colima and Linux rootless when that context is active); on **macOS** only, `~/.colima/default/docker.sock` as a last resort. Set `docker_socket` when your engine uses a different path (for example a named Colima profile that is not the default context).

**Colima on macOS (virtiofs):** If that resolved path is under `~/.colima/…`, gh sr still uses it to find the API over SSH, but for `docker run -v` it bind-mounts **`/var/run/docker.sock`** (the path inside the Colima VM) instead of the macOS host file under `~/.colima/…`. Bind-mounting the host Colima client socket fails on common setups with virtiofs (`operation not supported`; see [Colima #997](https://github.com/abiosoft/colima/issues/997)). If you need a manual workaround instead, you can start Colima with `colima start --mount-type sshfs`.

On **Linux**, the **`runner` user inside the container** (uid 1001) must also have permission to use the socket — gh sr handles this automatically by:

1. Querying the socket's owning GID on the host with `stat -c '%g'` on the resolved host socket path.
2. Passing `--group-add <GID>` to `docker run` so the container's runner user becomes a member of that group.
3. Verifying the socket with `test -S` on the resolved path before starting the container.

On **macOS** (Docker Desktop, OrbStack, Colima), the socket is accessible to all processes inside the VM — there is no docker group GID mismatch. gh sr skips the `stat` query and `--group-add` on macOS hosts; only the bind-mount is applied.

**If agentic-workflow tooling inside a job gets `permission denied` on the Docker socket**, the container was started without this `--group-add` flag (for example, by an older version of gh sr or a manually issued `docker run`). Recreate the runner:

```bash
gh sr down <runner-name>
gh sr up <runner-name>
```

**Rootless Docker** (Linux) and **Colima** often use a non-default socket; gh sr usually finds it via the default Docker context without extra config. If the active context does not point at the socket you want (for example a **named Colima profile** that is not selected in `docker context`), set `docker_socket` explicitly (supported on Linux and macOS; not applicable on Windows):

```yaml
hosts:
  my-linux:
    addr: user@192.168.1.10
    os: linux
    arch: amd64
    docker_socket: /run/user/1000/docker.sock
```

gh sr will bind-mount the custom path into the container at `/var/run/docker.sock` (preserving the default `DOCKER_HOST` path that job scripts expect) and still adds `--group-add` for the socket's GID.

**Verify from inside a running runner container:**

```bash
docker exec gh-runner-<instance> test -S /var/run/docker.sock && echo ok || echo missing
```

`gh sr doctor` performs this check automatically for all running docker-mode containers on Linux and macOS hosts and reports a WARN if the socket is not accessible inside.

## GitHub Agentic Workflows (gh-aw)

The easiest way to set up a runner for [GitHub Agentic Workflows](https://github.github.com/gh-aw/guides/self-hosted-runners/) is `profile: agentic`:

```yaml
runners:
  - name: aw-runner
    repo: owner/repo
    host: vps-1
    profile: agentic
    count: 2
```

This automatically configures docker mode, host networking, `NET_ADMIN` capability, an `agentic` label, and installs `iptables` in the runner container for the Agent Workflow Firewall. You can also add it from the CLI:

```bash
gh sr add runner aw-runner --repo owner/repo --host vps-1 --profile agentic
```

For **organization-level runners** with runner groups, use `org` and `group` instead of `repo`:

```yaml
runners:
  - name: org-runner
    org: my-org
    group: my-runner-group
    host: vps-1
    profile: agentic
```

For **ephemeral runners** (clean-slate per job, good for security isolation):

```yaml
runners:
  - name: aw-ephemeral
    repo: owner/repo
    host: vps-1
    profile: agentic
    ephemeral: true
```

### How it works

gh-aw starts an MCP gateway in Docker with host networking. Health checks use `http://localhost:80` from the job environment, which only matches that gateway when the job runs on the **same network namespace** as the Docker engine.

| Host | Runner mode | Typical approach for gh-aw |
|------|-------------|----------------------------|
| **Linux** | **Docker** with `profile: agentic` | Recommended. Profile sets host network + NET_ADMIN automatically. |
| **Linux** | **Native** | Works; job runs on the host. |
| **Linux** | **Docker** (default bridge) | Health check often **fails**. Use `profile: agentic` or set `docker_network_mode: host` manually. |
| **macOS** | **Docker** (Desktop, OrbStack, Colima) | Use `profile: agentic`. Port **80** must be free inside the VM. |
| **Windows** | **Docker** (Linux containers) | Use `profile: agentic`. Port **80** must be free inside the VM. |

> **Note:** On macOS and Windows, `--network host` means the Docker Desktop Linux VM's network namespace, not the macOS/Windows host itself. This is sufficient for gh-aw because all containers (runner, MCP gateway, MCP servers) share that same VM namespace.

Workflows that run the [Agent Workflow Firewall](https://github.com/github/gh-aw-firewall) (`awf`) need the **`NET_ADMIN`** capability. The `profile: agentic` shortcut handles this. If configuring manually, add **`docker_cap_add: [NET_ADMIN]`** on **`mode: docker`** runners.

**Verification (optional):** Keep Docker’s default iptables integration (do not set **`"iptables": false"`** in the engine `daemon.json`). On Docker Desktop, you can open a shell in the Linux engine (for example the **`docker-desktop`** WSL distro) and run **`sudo iptables -L DOCKER-USER -n`** — when the daemon is healthy, that chain is normally present. Then run a workflow that uses `awf` to confirm the job completes past firewall setup.

**`gh sr doctor`** checks hosts used by **`profile: agentic`** runners or by runners whose **`labels`** include **`agentic`** (or the legacy **`gh-aw`**) for port 80 availability, iptables presence, and sudo access, in addition to the standard bridge-network warning.

### Native Linux runners and `sudo` (gh-aw) {#native-linux-runners-and-sudo-gh-aw}

[GitHub Agentic Workflows self-hosted guidance](https://github.github.com/gh-aw/guides/self-hosted-runners/) requires a **`sudo`-capable environment** (non-sudo-only setups are not supported). With **`mode: native`**, the [Agent Workflow Firewall](https://github.com/github/gh-aw-firewall) and related tooling may invoke **`sudo` during jobs** (for example iptables rules). That must work **without a password prompt**, because Actions job steps are not interactive.

**Ensure this for the same OS account that runs the runner process** (the user that owns `~/.gh-sr/runners/<instance>` and executes `run.sh` — typically the SSH user from `hosts.*.addr`, or the **`User=`** in a systemd unit from **`gh sr service install --system`**):

1. **Passwordless sudo** — On the host, configure **`sudo -n`** to succeed for that user, for example a drop-in under `/etc/sudoers.d/` with **`NOPASSWD`** (a dedicated CI user with `runner ALL=(ALL) NOPASSWD: ALL` is common; tighten with command-scoped rules only if you can maintain them for whatever `sudo` lines gh-aw runs).
2. **Verify as that user** — `sudo -n true` must exit **0** (see also [Linux SSH user and privileges](#linux-ssh-user-and-privileges)). Over SSH:  
   `ssh -o BatchMode=yes user@host 'sudo -n true'`
3. **`gh sr doctor`** — With **`profile: agentic`** or **`labels`** including **`agentic`** (or legacy **`gh-aw`**), doctor reports whether passwordless sudo is available for AWF firewall setup on Linux.

Docker must still be installed on the machine for gh-aw’s containers (MCP gateway, AWF, etc.); native mode only means the **Actions runner** is not inside a container.

## Windows (OpenSSH and Docker)

- **Windows** — OpenSSH Server enabled; Docker Desktop if you want Linux container runners (`mode: docker`):

  ```powershell
  Add-WindowsCapability -Online -Name OpenSSH.Server~~~~0.0.1.0
  Start-Service sshd
  Set-Service -Name sshd -StartupType Automatic
  ```

  **SSH default shell:** OpenSSH may use cmd.exe or PowerShell 7 (`pwsh`) as the remote shell depending on your setup. gh sr runs Windows automation via `powershell.exe` or `pwsh.exe` with `-EncodedCommand`, so it works with either default. Use `windows_ps: pwsh` on the host if you rely on PowerShell 7 only and do not have Windows PowerShell 5.1.

## All remote hosts

- Confirm non-interactive SSH works: `ssh -o BatchMode=yes user@host true` (use the same user and host as in `hosts.*.addr`).
- **Host keys:** when `~/.ssh/known_hosts` exists, gh sr verifies server keys the same way as the Go `knownhosts` package. Connect once with plain `ssh` if you need to accept a new host key before `gh sr doctor` or `gh sr setup`.
- Remote commands run as the SSH user from your config; that user must have permission to install or run runners as documented for each OS below.

Run **`gh sr doctor`** (optionally with `--host` / `--repo`) to confirm connectivity from your control machine.

## Linux

For the full explanation of non-interactive SSH, the `gh sr: remote Linux commands need root…` error, `NOPASSWD`, and verification commands, see [Linux SSH user and privileges](#linux-ssh-user-and-privileges) above.

- **Docker mode (default on Linux):** If Docker is not installed, `gh sr setup` can run Docker’s install script, which requires **root** or **passwordless sudo** (`sudo -n`) over SSH (see the section linked above). To avoid that path, install Docker yourself and ensure the SSH user can run `docker` (for example membership in the `docker` group). gh sr automatically passes `--group-add` for the socket GID and verifies the socket before starting containers; see [Docker socket permissions](#docker-socket-permissions-linuxmacos) above for path resolution and the optional `docker_socket` override.
- **Native mode:** Ensure `curl` and `tar` are on `PATH`. `gh sr setup` may still invoke GitHub’s `installdependencies.sh` with **`sudo -n`** when the SSH user is not root and passwordless sudo is available.

Run **`gh sr doctor`** after the host is prepared.

## macOS

- **Native:** `curl` is usually sufficient; gh sr downloads the Actions runner over HTTPS.
- **Docker mode:** Install [Docker Desktop](https://www.docker.com/products/docker-desktop/), [OrbStack](https://orbstack.dev/), or [Colima](https://github.com/abiosoft/colima). The `docker` CLI must work in the **same** environment as your SSH session (user, `PATH`, and Docker context). Start the Docker engine (app or `colima start`, etc.) before relying on docker mode. **Docker socket permissions:** macOS Docker Desktop, OrbStack, and Colima use a Unix socket that is accessible to all processes; gh sr does not need `--group-add` on macOS and skips the GID detection step. gh sr bind-mounts the resolved host socket at `/var/run/docker.sock` inside the container (matching the Linux default). Override with `docker_socket` only when needed — see [Docker socket permissions](#docker-socket-permissions-linuxmacos) below.

Run **`gh sr doctor`** to confirm `docker info` from the SSH session.

## Windows (runner behavior)

- Install and enable **OpenSSH Server** (see the PowerShell snippet under [Windows (OpenSSH and Docker)](#windows-openssh-and-docker)).
- **Docker mode (Linux containers on the same Windows host):** Install [Docker Desktop](https://www.docker.com/products/docker-desktop/), use the default **Linux containers** mode, and keep Docker running. gh sr does not install or start Docker Desktop for you. Because Docker's Windows CLI may auto-detect `wincred` in non-interactive SSH sessions and fail with `A specified logon session does not exist`, gh sr runs Windows Docker commands with an isolated `DOCKER_CONFIG` that disables credential helpers instead of relying on `%USERPROFILE%/.docker/config.json`.

- **Native mode:** After `gh sr setup`, run **`gh sr up`** to start the listener. Registration logs under `_diag` show **configure** finishing with exit code 0; that is normal and does not mean the runner is listening. For the **run** phase, check **`%USERPROFILE%\.gh-sr\runners\<instance>\runner.log`**. Over SSH, **`Start-Process` listeners are killed when the session disconnects**; gh sr starts the listener with **`Win32_Process.Create` (CIM)** so it keeps running after `gh sr` exits. If the runner shows **stopped** with `runner registration has been deleted from the server`, GitHub auto-pruned the stale registration. `gh sr up` detects this automatically, re-registers, and retries. You can also fix it manually with **`gh sr update <runner>`** (which removes, re-configures, and starts in one step). If `gh sr status` shows **stopped** immediately after `gh sr up` for other reasons, check `runner.log` for error details. As a fallback you can run **`run.cmd`** from an interactive session (RDP), use **Task Scheduler** at logon, or install the runner as a **Windows service** via GitHub’s `config.cmd` options.

Run **`gh sr doctor`** to confirm SSH and Docker readiness.

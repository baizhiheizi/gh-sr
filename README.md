# gh-runners

Manage self-hosted GitHub Actions runners across multiple repos, running on your local Windows 11 machine.

- **Linux runners** — Docker containers in WSL2, built from the [official `actions/runner` releases](https://github.com/actions/runner/releases)
- **Windows runners** — Native processes on Windows, managed via PowerShell from WSL2
- **Mac runners** — Native processes on a remote Mac, managed via SSH
- **Single config file** — declare all repos, runner counts, and labels in `config/runners.yml`
- **Single CLI** — `./scripts/runner` handles everything

---

## How it works

### The big picture

```
config/runners.yml
       │
       ▼
scripts/generate-compose.sh   ← reads your config, writes docker-compose.yml
       │
       ▼
docker-compose.yml            ← one service per Linux runner instance
       │
       ▼
docker compose up             ← starts containers; each one self-registers with GitHub
```

Each Linux runner is an identical Docker container. They differ only in their environment variables (repo URL, unique name, labels). All containers share the same built image.

### The Docker image (`docker/Dockerfile`)

Built from `ubuntu:22.04`. At build time it downloads the official GitHub Actions runner tarball from `https://github.com/actions/runner/releases` and runs the bundled `installdependencies.sh`. The runner version is controlled by the `RUNNER_VERSION` build arg (defaults to `latest`, resolved at build time against the GitHub API).

The image is tagged `gh-runner:local`. All runner containers on your machine share this one image — no redundant downloads.

### Container lifecycle (`docker/entrypoint.sh`)

When a container starts:

1. **Gets a registration token** — calls `POST /repos/{owner}/{repo}/actions/runners/registration-token` using your PAT. This token is short-lived (1 hour) and is only used once.
2. **Registers the runner** — runs `./config.sh --url ... --token ... --name ... --labels ...`. The `--replace` flag means if a runner with the same name already exists (e.g. after a crash), it gets replaced automatically.
3. **Starts listening for jobs** — runs `./run.sh` and waits.
4. **Deregisters on shutdown** — traps `SIGTERM` (sent by `docker stop`). Gets a removal token and calls `./config.sh remove` before exiting. This keeps your GitHub runner list clean.

### Config file (`config/runners.yml`)

This is the only file you edit day-to-day. `docker-compose.yml` is always generated from it and is gitignored.

```yaml
github:
  pat_env_var: GITHUB_PAT      # name of the env var in .env that holds your token
  mac_host: user@mac-hostname  # SSH target for Mac runners (omit if unused)

runners:
  - name: hangar-ci            # base name; instances become hangar-ci-1, hangar-ci-2, ...
    repo: an-lee/hangar        # owner/repo
    os: linux
    count: 2                   # number of parallel runner instances
    labels: [self-hosted, linux, ci]

  - name: enjoy-win
    repo: an-lee/enjoy
    os: windows
    count: 1
    labels: [self-hosted, windows, x64]

  - name: enjoy-mac
    repo: an-lee/enjoy
    os: mac
    count: 1
    labels: [self-hosted, mac, arm64]
```

`os: linux` → Docker container via WSL2  
`os: windows` → native Windows process  
`os: mac` → native macOS process on the remote `mac_host` via SSH

### Secrets (`.env`)

Never committed. Contains:

```
GITHUB_PAT=github_pat_...    # fine-grained PAT
RUNNER_VERSION=latest         # or e.g. 2.333.1 to pin a version
```

The `GITHUB_PAT` is passed into each container as `ACCESS_TOKEN`. It's used at container startup to obtain short-lived registration/removal tokens — the PAT itself never touches the runner process.

### Windows runners (`scripts/manage-windows.ps1`)

Windows container jobs are [not supported by GitHub Actions](https://github.com/actions/runner/issues/904), so Windows runners run as native processes. The PowerShell script is called from WSL2 via `powershell.exe` interop and handles:

- Downloading the `actions-runner-win-x64` tarball from GitHub releases
- Extracting it to `windows/runners/<name>-<N>/`
- Calling `config.cmd` to register with GitHub
- Starting runners with `run.cmd` (tracked by PID file)
- Stopping and deregistering on removal

### Mac runners (`scripts/manage-mac.sh`)

Mac runners run as native processes on a remote Mac. The `manage-mac.sh` script runs on the Mac itself and is invoked via SSH by the `runner` CLI. It handles:

- Auto-detecting architecture (Intel x64 or Apple Silicon arm64) to download the correct tarball
- Downloading `actions-runner-osx-{arch}` from GitHub releases to `/tmp/` (cached per version)
- Extracting to `~/gh-runners/runners/<name>-<N>/` on the Mac
- Calling `config.sh` to register with GitHub
- Starting runners with `run.sh` in background via `nohup` (tracked by PID file)
- Stopping and deregistering on removal

SSH key-based authentication must be pre-configured from WSL2 to the Mac. Set `github.mac_host` to the SSH target (e.g., `user@192.168.1.50` or a hostname in your `~/.ssh/config`).

---

## Prerequisites

**WSL2 (for Linux runners):**
- Docker Desktop with WSL2 backend, or Docker Engine in WSL2
- `yq` v4+ (Go version) — installer: `~/.local/bin/yq` is set up automatically if you ran the setup
- `jq`, `curl` — standard packages

**Windows (for Windows runners):**
- PowerShell 5.1+ (built into Windows 11)
- `yq` available in PowerShell path (download from [mikefarah/yq releases](https://github.com/mikefarah/yq/releases))

**Mac (for Mac runners):**
- macOS 12 (Monterey) or later
- `jq` — install with `brew install jq`
- SSH key-based access configured from WSL2 to the Mac (no passphrase prompts)

---

## Setup

### 1. Create your PAT

Go to **GitHub → Settings → Developer settings → Personal access tokens → Fine-grained tokens**.

- **Resource owner**: your account (or org)
- **Repository access**: select the specific repos you want runners for
- **Permissions → Repository permissions → Administration**: Read and write

### 2. Configure your environment

```bash
cp .env.example .env
# Edit .env and paste your PAT
```

### 3. Define your runners

Edit `config/runners.yml`. Add one entry per runner group:

```yaml
github:
  pat_env_var: GITHUB_PAT

runners:
  - name: my-repo-ci
    repo: myorg/my-repo
    os: linux
    count: 2
    labels: [self-hosted, linux]
```

### 4. Start Linux runners

```bash
./scripts/runner up
```

This:
1. Generates `docker-compose.yml` from your config
2. Builds the `gh-runner:local` Docker image (downloads runner from GitHub releases)
3. Starts all Linux runner containers
4. Each container registers itself with GitHub

Check GitHub: **repo → Settings → Actions → Runners** — your runners should appear as "Idle".

### 5. Start Windows runners (optional)

```bash
./scripts/runner win-install   # downloads runner, configures it with GitHub
./scripts/runner win-up        # starts the runner processes
```

### 6. Start Mac runners (optional)

Set `github.mac_host` in `config/runners.yml` to the SSH target of your Mac, then:

```bash
./scripts/runner mac-install   # copies manage-mac.sh to Mac, downloads runner, configures it
./scripts/runner mac-up        # starts the runner processes on the Mac
```

---

## Daily commands

```bash
./scripts/runner status         # show Docker container status + GitHub API status for all runners
./scripts/runner up             # start (or restart) all Linux runners
./scripts/runner down           # stop all Linux runners (deregisters from GitHub)
./scripts/runner logs           # tail all container logs
./scripts/runner logs hangar-ci-1   # tail a specific runner's logs
./scripts/runner restart        # restart all containers
./scripts/runner restart hangar-ci-1  # restart one container
```

---

## Common tasks

### Add a new repo

1. Add an entry to `config/runners.yml`
2. Make sure your PAT has Administration write on the new repo
3. Run `./scripts/runner up`

New containers are added; existing ones are left untouched.

### Scale up runners

Change `count: 1` to `count: 3` in `runners.yml`, then:

```bash
./scripts/runner up
```

Compose creates the new containers. Existing containers are unaffected.

### Scale down runners

Reduce `count` in `runners.yml`, then:

```bash
./scripts/runner down && ./scripts/runner up
```

Stopped containers deregister themselves from GitHub before exiting.

### Update runner version

To get the latest runner release:

```bash
./scripts/runner update
```

This rebuilds the image with `--no-cache` (picks up the newest release from GitHub) and recreates all containers.

To pin a specific version, set `RUNNER_VERSION=2.333.1` in `.env` before running `update`.

### Rotate your PAT

1. Create a new PAT on GitHub
2. Update `GITHUB_PAT` in `.env`
3. Run:

```bash
./scripts/runner rotate-token
```

For Linux: stops and restarts all containers (they re-register on startup with the new token).  
For Windows: follow the printed instructions to remove and reinstall.

### Clean up ghost runners

If containers were killed without a clean shutdown, they may leave "offline" runners in GitHub:

```bash
./scripts/runner cleanup
```

This calls the GitHub API to delete any offline runner entries for all repos in your config.

### Remove all Windows runners

```bash
./scripts/runner win-down     # stop processes
./scripts/runner win-remove   # deregister from GitHub and delete local files
```

### Remove all Mac runners

```bash
./scripts/runner mac-down     # stop processes on Mac
./scripts/runner mac-remove   # deregister from GitHub and delete remote files
```

---

## Using runners in workflows

Reference runners by label in your workflow files:

```yaml
jobs:
  build:
    runs-on: [self-hosted, linux, ci]   # matches runners with all three labels
```

Or use just `self-hosted` to use any available self-hosted runner:

```yaml
    runs-on: self-hosted
```

---

## File reference

```
gh-runners/
├── config/
│   └── runners.yml              # Edit this — source of truth for all runners
├── docker/
│   ├── Dockerfile               # Builds runner image from official releases
│   └── entrypoint.sh            # Registration, run, graceful deregistration
├── scripts/
│   ├── runner                   # Main CLI — run this for everything
│   ├── generate-compose.sh      # runners.yml → docker-compose.yml (called by runner)
│   ├── manage-windows.ps1       # Windows runner lifecycle (called by runner)
│   ├── manage-mac.sh            # Mac runner lifecycle (copied to Mac and invoked via SSH)
│   └── lib/
│       └── common.sh            # Shared bash helpers
├── windows/
│   └── runners/                 # Windows runner installs (gitignored)
├── docker-compose.yml           # Auto-generated — do not edit (gitignored)
├── .env                         # Your secrets (gitignored)
└── .env.example                 # Template — copy to .env and fill in
```

---

## Troubleshooting

**Runner appears in GitHub but stays "Offline"**  
The container started but the runner process crashed. Check logs:
```bash
./scripts/runner logs <runner-name>
```
Common causes: wrong repo URL, PAT lacks Administration permission on the repo.

**"Could not get registration token" in container logs**  
Your `GITHUB_PAT` is expired, revoked, or lacks the right permissions. Create a new PAT and run `./scripts/runner rotate-token`.

**Container keeps restarting**  
The `restart: unless-stopped` policy means a failing container retries. Check logs to see the error. Once fixed, `docker-compose.yml` will be regenerated on the next `up`.

**Ghost/offline runners accumulating in GitHub**  
Run `./scripts/runner cleanup`. This is normal after hard crashes or `kill -9`. Clean shutdowns via `docker stop` (or `./scripts/runner down`) deregister automatically.

**`yq: command not found`**  
Install Mike Farah's yq v4+ (not the Python wrapper):
```bash
wget -O ~/.local/bin/yq https://github.com/mikefarah/yq/releases/latest/download/yq_linux_amd64
chmod +x ~/.local/bin/yq
```

**"Cannot SSH to Mac host" error**  
Configure SSH key-based auth from WSL2 to the Mac:
```bash
ssh-keygen -t ed25519    # if you don't have a key yet
ssh-copy-id user@mac-hostname
ssh user@mac-hostname    # verify it works without a password prompt
```

**Mac runner process dies after mac-up returns**  
This shouldn't happen — `manage-mac.sh` uses `nohup` to detach the process. If it occurs, SSH into the Mac and check `~/gh-runners/runners/<name>/runner.log` for errors.

**Mac runner appears in GitHub but stays "Offline"**  
SSH into the Mac and check whether the process is running:
```bash
ssh user@mac-hostname "ps aux | grep run.sh"
cat ~/gh-runners/runners/<runner-name>/runner.log
```

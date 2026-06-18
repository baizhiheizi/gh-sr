## Why

Container-mode and agentic runners require Docker on Linux hosts, but `gh sr setup` currently jumps straight to `docker build` without checking whether Docker is installed. Documentation already states that setup *may* invoke Docker's install script when Docker is missing, yet that behavior is not implemented — users hit opaque `permission denied` or `command not found` errors mid-setup. Auto-installing Docker during setup (with a clear re-run step for docker group membership) closes the doc/code gap and matches the precedent set by native mode's curl/tar bootstrap.

## What Changes

- Add `EnsureHostDocker` (or equivalent) called at the start of container setup on Linux hosts
- Detect Docker CLI via `docker --version` and daemon access via `docker info`
- When Docker CLI is missing, install via `https://get.docker.com` using non-interactive `sudo -n` (same privilege model as native `installdependencies.sh`)
- After fresh install, add the SSH user to the `docker` group and exit setup early with an explicit message to re-run `gh sr setup` (Option B UX — new SSH session picks up group membership)
- When Docker is installed but the daemon is stopped, attempt `systemctl enable --now docker` without re-running the install script
- Bootstrap `curl` if missing (same apt/yum/apk pattern as native setup) before fetching get.docker.com
- Update doctor remediation hints to mention auto-install during setup where appropriate
- No change to macOS or Windows (manual Docker install remains required / container mode unsupported)

## Capabilities

### New Capabilities

- `container-host-docker`: Host Docker detection, auto-install via get.docker.com, docker group handling, and re-run guidance during container-mode setup on Linux

### Modified Capabilities

<!-- No existing openspec specs cover host Docker setup behavior -->

## Impact

- **Packages**: `internal/runner` (new docker helper, hook in `setupContainer`), possibly `internal/agentic` (align doctor remediation text)
- **Docs**: `docs/content/host-setup.md` — clarify two-run setup flow when Docker is auto-installed
- **Tests**: unit tests for detection/install shell scripts and EnsureHostDocker outcomes (mock executor)
- **User-facing CLI**: `gh sr setup` and `gh sr up` (via `EnsureSetup`) may now install Docker on Linux; first run after install exits with actionable re-run message instead of failing mid image build
- **Privileges**: unchanged — still requires root SSH or passwordless `sudo -n` for install steps

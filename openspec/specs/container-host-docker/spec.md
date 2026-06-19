# container-host-docker Specification

## Purpose
TBD - created by archiving change ensure-docker-on-setup. Update Purpose after archive.
## Requirements
### Requirement: Container setup detects Docker before use

When setting up a container-mode runner on a Linux host, gh sr MUST verify Docker CLI availability and daemon accessibility before running any `docker build`, `docker create`, or other Docker commands.

#### Scenario: Docker already available

- **WHEN** `gh sr setup` runs for a container-mode runner on Linux and `docker --version` succeeds and `docker info` succeeds for the SSH user
- **THEN** setup MUST proceed to image build and container creation without attempting installation

#### Scenario: Non-Linux container mode

- **WHEN** `gh sr setup` runs for a container-mode runner on a non-Linux host
- **THEN** setup MUST fail with the existing unsupported-OS error and MUST NOT attempt Docker installation

### Requirement: Missing Docker CLI triggers get.docker.com install

When the Docker CLI is not found on a Linux host during container-mode setup, gh sr MUST install Docker Engine using the official convenience script at `https://get.docker.com`, executed with non-interactive elevation (`sudo -n` or root).

#### Scenario: Docker CLI absent

- **WHEN** `docker --version` fails or does not report a Docker version during container-mode setup on Linux
- **THEN** gh sr MUST run `curl -fsSL https://get.docker.com | sh` with root or `sudo -n`
- **AND** MUST enable and start the Docker service via `systemctl enable --now docker` when systemd is available

#### Scenario: curl missing before install

- **WHEN** Docker CLI is absent and `curl` is not on PATH during container-mode setup on Linux
- **THEN** gh sr MUST attempt to install `curl` using the same distro package manager pattern as native runner setup (apt-get, yum, or apk) before fetching get.docker.com

#### Scenario: Install requires elevation

- **WHEN** Docker CLI is absent and the SSH user is not root and passwordless sudo is unavailable
- **THEN** gh sr MUST fail with the existing non-interactive sudo error message and MUST NOT proceed with setup

#### Scenario: Docker CLI already present

- **WHEN** `docker --version` succeeds during container-mode setup on Linux
- **THEN** gh sr MUST NOT invoke get.docker.com

### Requirement: Fresh install requires docker group and setup re-run

After installing Docker for the first time on a host, gh sr MUST add the SSH login user to the `docker` group and MUST stop container setup before image build, instructing the user to re-run setup so a new SSH session picks up group membership.

#### Scenario: Post-install group membership

- **WHEN** gh sr installs Docker via get.docker.com during container-mode setup
- **AND** the SSH user is identified from `hosts.*.addr` (user@host form)
- **THEN** gh sr MUST run `usermod -aG docker <ssh-user>` with root or `sudo -n`

#### Scenario: Re-run guidance after fresh install

- **WHEN** gh sr completes a fresh Docker install and adds the SSH user to the docker group
- **THEN** setup MUST exit before building the container runner image
- **AND** MUST print a message instructing the user to re-run `gh sr setup` (with applicable runner names)
- **AND** MUST NOT attempt `docker build` or `docker create` in the same invocation

#### Scenario: Second setup run succeeds

- **WHEN** the user re-runs `gh sr setup` after a fresh Docker install and the SSH user's new session has docker group membership
- **THEN** `docker info` MUST succeed
- **AND** setup MUST proceed normally through image build and container creation

#### Scenario: Root SSH user

- **WHEN** gh sr installs Docker and the SSH session is already root (`id -u` is 0)
- **THEN** gh sr MUST NOT require a setup re-run solely for docker group membership
- **AND** setup MAY proceed to image build in the same invocation if `docker info` succeeds

### Requirement: Stopped daemon recovery without reinstall

When the Docker CLI is present but the daemon is not reachable, gh sr MUST attempt to start the daemon without re-running get.docker.com.

#### Scenario: Daemon stopped

- **WHEN** `docker --version` succeeds but `docker info` fails during container-mode setup on Linux
- **AND** Docker was not just installed in the same invocation
- **THEN** gh sr MUST attempt `systemctl enable --now docker` when systemd is available
- **AND** MUST re-check `docker info` before proceeding or failing

#### Scenario: Permission denied after existing install

- **WHEN** `docker --version` succeeds but `docker info` fails with permission denied
- **AND** Docker was not just installed in the same invocation
- **THEN** gh sr MUST fail with a message indicating the SSH user needs membership in the `docker` group (including `usermod` guidance)
- **AND** MUST NOT invoke get.docker.com

### Requirement: EnsureSetup inherits Docker ensure behavior

`gh sr up` auto-setup (`EnsureSetup`) MUST run the same Docker detection and install logic as explicit `gh sr setup` for container-mode runners.

#### Scenario: Up triggers ensure on unprepared host

- **WHEN** `gh sr up` runs for a container-mode runner that is not yet set up
- **AND** Docker is missing on the Linux host
- **THEN** gh sr MUST install Docker and emit the same re-run guidance as `gh sr setup`
- **AND** MUST NOT start the runner container in the same invocation


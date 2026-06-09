# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

Release builds are tagged as `vMAJOR.MINOR.PATCH`. Each Git tag triggers a
[GitHub Release](https://github.com/an-lee/gh-sr/releases); the workflow also
generates release notes on publish. Use this changelog for a concise, curated
history when you want more than the auto-generated GitHub summary.

## [Unreleased]

### Added

- Added `gh sr disk usage`, `gh sr disk prune`, and `gh sr disk schedule` commands for managing the on-host disk footprint of runner instances under `~/.gh-sr/runners`. `usage` reports the per-instance breakdown of `_work`, `_temp`, and (for container/agentic runners) `docker-data`; `prune` reclaims `_work`/`_temp` on **idle** runners by default (inner Docker cache kept), with `--prune-cache` for deeper reclaim and `--include-orphans` to also remove directories not in `runners.yml`; `schedule install` registers a daily local prune timer on the operator machine (`03:00` by default, configurable; uses systemd on Linux, launchd on macOS, and Task Scheduler on Windows). `gh sr doctor` now warns when any instance directory exceeds 50 GiB. (#131)
- Added `BenchmarkLoad_Small`, `BenchmarkLoad_Large`, `BenchmarkValidate_Small`, and `BenchmarkValidate_Large` benchmarks in `internal/config` to establish a performance baseline for config loading and validation. Run with `make bench`. (#37)

### Changed

- `gh sr doctor` now runs host SSH checks and GitHub API checks (repos and orgs) concurrently, reducing wall-clock time from O(N × latency) to O(latency) for configurations with multiple hosts or targets. Output within each section is printed in sorted order. (#33)
- `gh sr service install`, `gh sr service uninstall`, and `gh sr service status` now open one SSH connection per host concurrently via `runPerHostParallel`, matching the behavior already in place for `up`, `down`, `restart`, `update`, and `status`. For single-host configs there is no observable difference; for multi-host configs completion time drops from O(N × SSH_latency) to O(SSH_latency). Output order across hosts is non-deterministic. (#30)

### Fixed

- Container-mode runners now pin their Docker MTU to the host's egress MTU, fixing workflow downloads (e.g. `actions/setup-go`, `actions/setup-node`) that failed with `Client network socket disconnected before secure TLS connection was established` on hosts whose real path MTU is below 1500 (cloud overlay networks such as GCP's 1460 default, VPN/WireGuard, nested virtualisation). The outer container's `eth0` and the inner `dockerd` bridge previously kept Docker's 1500 default; with PMTUD black-holed, large TLS-handshake packets were silently dropped while small packets (and the host itself) worked. `gh sr` auto-detects the host egress MTU at container-create time and `entrypoint.sh` applies it to both layers (plus an MSS clamp for forwarded inner traffic). Run `gh sr rebuild <name>` to adopt the fix. A `container_runner_image.mtu` override is available for hosts where a tunnel lowers the path MTU below the NIC MTU, and `gh sr doctor` flags a stale image as `container-mtu`.
- `gh sr disk prune` now escalates past root-owned files left inside container runners' `_work`/`_temp` host bind mounts. When the operator cannot delete a file directly, prune retries through `docker exec` against the runner container and, if that is unavailable, via passwordless sudo on the host. Previously, such files caused `permission denied` errors and prevented reclaim; busy runners continue to be skipped.

### Removed

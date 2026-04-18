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

### Changed

- `gh sr doctor` now runs host SSH checks and GitHub API checks (repos and orgs) concurrently, reducing wall-clock time from O(N × latency) to O(latency) for configurations with multiple hosts or targets. Output within each section is printed in sorted order. (#33)
- `gh sr service install`, `gh sr service uninstall`, and `gh sr service status` now open one SSH connection per host concurrently via `runPerHostParallel`, matching the behavior already in place for `up`, `down`, `restart`, `update`, and `status`. For single-host configs there is no observable difference; for multi-host configs completion time drops from O(N × SSH_latency) to O(SSH_latency). Output order across hosts is non-deterministic. (#30)

### Fixed

### Removed

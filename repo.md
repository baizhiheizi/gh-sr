---
name: repo
description: baizhiheizi/gh-sr — Go GitHub CLI extension for self-hosted runners
metadata:
  type: project
---

`baizhiheizi/gh-sr` — Go module (`go 1.25.9`). `cmd/gh-sr/` is the CLI entry (0% tested); `internal/` is the impl.

Packages and coverage (2026-07-04):

- `internal/agentic` 81.0% — agentic-workflow prereq validators
- `internal/autostart` **94.7%** — systemd/launchd/scheduled-task install (2026-07-04: Install 64.3→100, installSystemdUser 80→100, installSystemdSystem 77.8→100, installLaunchd 0→100, installWindowsTask 0→100; 2026-07-03: Start 0→91.7, Stop 0→95, Status 0→96.6, Uninstall 31.6→84.2)
- `internal/config` 83.9% — runners.yml parser (best-tested)
- `internal/diskschedule` 14.2% — local schedule install for `gh sr disk prune`
- `internal/doctor` 68.8% — health checks
- `internal/editor` 53.8% — editor picker
- `internal/host` 59.6% — SSH+local exec; `Executor` interface
- `internal/hostshell` 89.7% — shell-quoting + remote-write helpers
- `internal/hostshell/ps` 60.0%
- `internal/ops` **93.6%** — orchestration; `Update` 100%, `Remove` 93.3%, `ServiceCleanup` 92.5%, `ServiceInstall` 90.5%, `ServiceStatus` 80%, `ServiceUninstall` 75%, `Up` 77.8%, `RebuildImage` 76.9%
- `internal/runner` **56.7%** — container + native lifecycle (2026-07-02: formatContainerImageBuild → 100, windowsNativeConfigScript → 100, containerImageExtraApt → 100)
- `internal/table` 87.5%
- `internal/testutil` 88.2% — shared mocks
- `internal/tui` 15.0% — bubbletea TUI
- `cmd/gh-sr` 0.0% — tested via `internal/ops` end-to-end

Module path: `github.com/an-lee/gh-sr` (note: changed from `baizhiheizi/gh-sr`; remote URL still `baizhiheizi/gh-sr.git`).

CI: `.github/workflows/ci.yml` runs `go vet ./...` then `go test ./... -race -count=1` on `[self-hosted, linux]`.

Maintainer (an-lee) merges test-improver PRs regularly.

**Safeoutputs bridge status (2026-07-04):** 6th consecutive phantom run for `create_pull_request`. Patch + bundle preserved for each missing PR. Maintainer pattern: organic merges happen for ~half the runs despite phantom reports.

[[commands]] [[testing-notes]] [[backlog]]

---
name: repo
description: baizhiheizi/gh-sr — Go GitHub CLI extension for self-hosted runners
metadata:
  type: project
---

`baizhiheizi/gh-sr` — Go module (`go 1.25.9`). `cmd/gh-sr/` is the CLI entry (0% tested); `internal/` is the impl.

Packages and coverage (2026-06-18):

- `internal/agentic` 84.3% — agentic-workflow prereq validators
- `internal/autostart` 36.2% — systemd/launchd/scheduled-task install
- `internal/config` 83.9% — runners.yml parser (best-tested)
- `internal/diskschedule` 14.2% — local schedule install for `gh sr disk prune`
- `internal/doctor` 65.5% — health checks
- `internal/editor` 53.8% — editor picker
- `internal/host` 58.8% — SSH+local exec; `Executor` interface
- `internal/hostshell` 100.0% — shell-quoting + remote-write helpers (fully covered 2026-06-11)
- `internal/ops` 42.8% — orchestration; `runPerHostParallel`, `ResolveHostInfo`, `CollectHostMetrics` all 100%; `Down` 83.3%; `Restart` 85.7%; remaining orchestrators (Up/Update/Remove/RebuildImage/CollectStatus/Logs/CleanupOffline) still 0%
- `internal/runner` 43.7% — container + native lifecycle
- `internal/testutil` 88.2% — shared mocks (MockExecutor + per-call capture pattern)
- `internal/tui` 9.7% — bubbletea TUI
- `cmd/gh-sr` 0.0% — tested via `internal/ops` end-to-end

CI: `.github/workflows/ci.yml` runs `go vet ./...` then `go test ./... -race -count=1` on `[self-hosted, linux]`.

Maintainer (an-lee) merges test-improver PRs regularly. [[commands]] [[testing-notes]] [[backlog]]

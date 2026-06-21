---
name: repo
description: baizhiheizi/gh-sr — Go GitHub CLI extension for self-hosted runners
metadata:
  type: project
---

`baizhiheizi/gh-sr` — Go module (`go 1.25.9`). `cmd/gh-sr/` is the CLI entry (0% tested); `internal/` is the impl.

Packages and coverage (2026-06-21):

- `internal/agentic` 84.3% — agentic-workflow prereq validators
- `internal/autostart` 36.2% — systemd/launchd/scheduled-task install
- `internal/config` 83.9% — runners.yml parser (best-tested)
- `internal/diskschedule` 14.2% — local schedule install for `gh sr disk prune`
- `internal/doctor` 65.5% — health checks
- `internal/editor` 53.8% — editor picker
- `internal/host` 58.8% — SSH+local exec; `Executor` interface
- `internal/hostshell` 100.0% — shell-quoting + remote-write helpers
- `internal/ops` 54.6% — orchestration; `runPerHostParallel`, `ResolveHostInfo`, `CollectHostMetrics`, `removeHost` all 100%; `Remove` 94.1%; `Down` 83.3%; `Restart` 85.7%; `Up` 77.8%; still at 0%: `Update`, `RebuildImage`, `CollectStatus`, `CleanupOffline`
- `internal/runner` 49.3% — container + native lifecycle
- `internal/testutil` 88.2% — shared mocks
- `internal/tui` 9.7% — bubbletea TUI
- `cmd/gh-sr` 0.0% — tested via `internal/ops` end-to-end

Module path: `github.com/an-lee/gh-sr` (note: changed from `baizhiheizi/gh-sr`; remote URL still `baizhiheizi/gh-sr.git`).

CI: `.github/workflows/ci.yml` runs `go vet ./...` then `go test ./... -race -count=1` on `[self-hosted, linux]`.

Maintainer (an-lee) merges test-improver PRs regularly. [[commands]] [[testing-notes]] [[backlog]]

---
name: repo
description: baizhiheizi/gh-sr — Go GitHub CLI extension for self-hosted runners
metadata:
  type: project
---

`baizhiheizi/gh-sr` — Go module (`go 1.25.9`). `cmd/gh-sr/` is the CLI entry (0% tested); `internal/` is the impl.

Packages and coverage (2026-06-29):

- `internal/agentic` 81.0% — agentic-workflow prereq validators
- `internal/autostart` 43.7% — systemd/launchd/scheduled-task install
- `internal/config` 83.9% — runners.yml parser (best-tested)
- `internal/diskschedule` 14.2% — local schedule install for `gh sr disk prune`
- `internal/doctor` 68.8% — health checks
- `internal/editor` 53.8% — editor picker
- `internal/host` 59.6% — SSH+local exec; `Executor` interface
- `internal/hostshell` 89.7% — shell-quoting + remote-write helpers
- `internal/hostshell/ps` 60.0%
- `internal/ops` **90.9%** — orchestration; all orchestrators ≥75% except `Update` 53.8% and `ServiceCleanup` 67.5%
- `internal/runner` 55.0% — container + native lifecycle
- `internal/table` 77.3%
- `internal/testutil` 88.2% — shared mocks
- `internal/tui` 16.5% — bubbletea TUI
- `cmd/gh-sr` 0.0% — tested via `internal/ops` end-to-end

Module path: `github.com/an-lee/gh-sr` (note: changed from `baizhiheizi/gh-sr`; remote URL still `baizhiheizi/gh-sr.git`).

CI: `.github/workflows/ci.yml` runs `go vet ./...` then `go test ./... -race -count=1` on `[self-hosted, linux]`.

Maintainer (an-lee) merges test-improver PRs regularly.

[[commands]] [[testing-notes]] [[backlog]]
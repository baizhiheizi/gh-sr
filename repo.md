---
name: repo
description: an-lee/gh-sr — Go GitHub CLI extension for self-hosted runners
metadata:
  type: project
---

`an-lee/gh-sr` — Go module (`go 1.25.9`). `cmd/gh-sr/` is the CLI entry (0% tested); `internal/` is the impl.

Packages and coverage (2026-06-12):

- `internal/agentic` 83.9% — agentic-workflow prereq validators
- `internal/autostart` 17.2% — systemd/launchd/scheduled-task install
- `internal/config` 83.6% — runners.yml parser (best-tested)
- `internal/diskschedule` 12.8% — local schedule install for `gh sr disk prune`
- `internal/doctor` 62.1% — health checks
- `internal/editor` 53.8% — editor picker
- `internal/host` 58.8% — SSH+local exec; `Executor` interface
- `internal/hostshell` 100.0% — shell-quoting + remote-write helpers (fully covered 2026-06-11)
- `internal/ops` 33.4% — orchestration; pure helpers + `runPerHostParallel` + `ResolveHostInfo` 100%, orchestrators still need per-function coverage
- `internal/runner` 37.4% — container + native lifecycle
- `internal/testutil` 88.2% — shared mocks (MockExecutor + per-call capture pattern)
- `internal/tui` 4.9% — bubbletea TUI
- `cmd/gh-sr` 0.0% — tested via `internal/ops` end-to-end

CI: `.github/workflows/ci.yml` runs `go vet ./...` then `go test ./... -race -count=1` on `[self-hosted, linux]`.

Maintainer (an-lee) merges test-improver PRs regularly. [[commands]] [[testing-notes]] [[backlog]]

---
name: repo
description: an-lee/gh-sr — Go GitHub CLI extension for self-hosted runners
metadata:
  type: project
---

`an-lee/gh-sr` — Go module (`go 1.25.9`). `cmd/gh-sr/` is the CLI entry (0% tested); `internal/` is the impl.

Packages and coverage (2026-06-10):

- `internal/agentic` 83.9% — agentic-workflow prereq validators
- `internal/autostart` 18.5% — systemd/launchd/scheduled-task install
- `internal/config` 77.0% — runners.yml parser (best-tested)
- `internal/diskschedule` 15.6% — local schedule install for `gh sr disk prune`
- `internal/doctor` 62.1% — health checks
- `internal/editor` 53.8% — editor picker
- `internal/host` 57.0% — SSH+local exec; `Executor` interface
- `internal/hostshell` 23.1% — shell-quoting + remote-write helpers
- `internal/ops` 19.6% — orchestration (biggest gap; pure helpers now tested, orchestrators still need `ConnectHost` injection)
- `internal/runner` 36.9% — container + native lifecycle
- `internal/tui` 4.6% — bubbletea TUI
- `cmd/gh-sr` 0.0% — tested via `internal/ops` end-to-end

CI: `.github/workflows/ci.yml` runs `go vet ./...` then `go test ./... -race -count=1` on `[self-hosted, linux]`.

Maintainer (an-lee) merges test-improver PRs regularly (PRs #11, #31, #34, #40, #51, #54, #58, #68, #107, #108, #123, #127, #128, #131, #136 all merged). [[commands]] [[testing-notes]] [[backlog]]

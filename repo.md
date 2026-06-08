---
name: repo
description: an-lee/gh-sr — Go GitHub CLI extension for self-hosted runners
metadata:
  type: project
---

`an-lee/gh-sr` — Go module (`go 1.25.9`). `cmd/gh-sr/` is the CLI entry (0% tested); `internal/` is the impl.

Packages and coverage (2026-06-08):

- `internal/agentic` 83.9% — agentic-workflow prereq validators (improved from 60.5% this run)
- `internal/autostart` 18.5% — systemd/launchd/scheduled-task install
- `internal/config` 76.7% — runners.yml parser (best-tested)
- `internal/doctor` 58.8% — health checks
- `internal/editor` 53.8% — editor picker
- `internal/host` 57.0% — SSH+local exec; `Executor` interface
- `internal/hostshell` (new) — extracted remote-shell helpers (PR #110)
- `internal/ops` 7.0% — orchestration (biggest gap)
- `internal/runner` 32.8% — container + native lifecycle
- `internal/tui` 4.6% — bubbletea TUI
- `cmd/gh-sr` 0.0% — tested via `internal/ops` end-to-end

CI: `.github/workflows/ci.yml` runs `go vet ./...` then `go test ./... -race -count=1` on `[self-hosted, linux]`.

Maintainer (an-lee) merges test-improver PRs regularly (PRs #11, #31, #34, #40, #51, #54, #58, #68 all merged). [[commands]] [[testing-notes]] [[backlog]]

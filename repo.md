---
name: repo
description: baizhiheizi/gh-sr — Go GitHub CLI extension for self-hosted runners
metadata:
  type: project
---

Go module `github.com/an-lee/gh-sr` (remote `baizhiheizi/gh-sr.git`). CLI entry is `cmd/gh-sr/`; implementation under `internal/`.

Coverage snapshot 2026-07-08:
- `internal/agentic` 81.0%; `internal/autostart` 94.7%; `internal/config` 83.9%; `internal/diskschedule` 88.2%; `internal/doctor` 68.8%; `internal/editor` 53.8%; `internal/host` 59.6%; `internal/hostshell` 89.7%; `internal/hostshell/ps` 60.0%; `internal/ops` 93.6%; `internal/runner` 56.7%; `internal/table` 87.5%; `internal/testutil` 88.2%; `internal/tui` 15.0%; `cmd/gh-sr` 0.0%.

CI (`.github/workflows/ci.yml`) runs `go vet ./...`, `gofmt -l .`, and `go test ./... -race -count=1` on self-hosted linux. Bench job runs `go test ./... -run='^$' -bench=. -benchmem -count=1`.

Maintainer merges test-improver work regularly. By 2026-07-08, prior phantom patch work from 2026-06-30 through 2026-07-04 is present on `main`; monthly issue #306 now only tracks the new diskschedule PR intent plus duplicate #305.

Safeoutputs PR bridge 2026-07-08 again returned only patch+bundle for diskschedule: `/tmp/gh-aw/aw-test-assist-diskschedule-command-seams.patch`, `/tmp/gh-aw/aw-test-assist-diskschedule-command-seams.bundle`.

[[commands]] [[testing-notes]] [[backlog]]

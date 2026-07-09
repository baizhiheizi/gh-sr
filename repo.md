---
name: repo
description: baizhiheizi/gh-sr — Go GitHub CLI extension for self-hosted runners
metadata:
  type: project
---

Go module `github.com/an-lee/gh-sr` (remote `baizhiheizi/gh-sr.git`). CLI entry is `cmd/gh-sr/`; implementation under `internal/`.

Coverage snapshot 2026-07-09:
- `internal/agentic` 81.0%; `internal/autostart` 94.7%; `internal/config` 83.9%; `internal/diskschedule` 88.2%; `internal/doctor` 68.8%; `internal/editor` 53.8%; `internal/host` 59.6%; `internal/hostshell` 89.7%; `internal/hostshell/ps` 60.0%; `internal/ops` 93.6%; `internal/runner` 59.6%; `internal/table` 87.5%; `internal/testutil` 88.2%; `internal/tui` 17.4%; `cmd/gh-sr` 0.0%.

CI (`.github/workflows/ci.yml`) runs `go vet ./...`, `gofmt -l .`, and `go test ./... -race -count=1` on self-hosted linux. Bench job runs `go test ./... -run='^$' -bench=. -benchmem -count=1`.

Maintainer merges test-improver work regularly. By 2026-07-09, prior phantom patch work from 2026-06-30 through 2026-07-04 plus 2026-07-08 (#336) is present on `main`; the 2026-07-09 PR intent #337 phantom'd (patch saved). Monthly issue #306 now tracks the new runner-dispatch PR intent plus the stale duplicate #305.

Phantom PR pattern confirmed (5th consecutive: 2026-06-30, 2026-07-01, plus 2026-07-08 #336 landed anyway, 2026-07-09 #337 phantom): `safeoutputs.create_pull_request` reports success while the PR does not appear in `gh-sr/pulls`. Patch + bundle are always saved to `/tmp/gh-aw/aw-test-assist-*.{patch,bundle}`; the maintainer manually `git am`s when needed.

[[commands]] [[testing-notes]] [[backlog]]

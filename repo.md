---
name: repo
description: baizhiheizi/gh-sr — Go GitHub CLI extension for self-hosted runners
metadata:
  type: project
---

Go module `github.com/an-lee/gh-sr` (remote `baizhiheizi/gh-sr.git`). CLI entry is `cmd/gh-sr/`; implementation is under `internal/`.

Coverage snapshot 2026-07-18: `internal/runner` baseline on current `main` is 69.9%; current branch `test-assist/manager-remove-status-logs-orchestrators` (commit `2af02ce`) raises it to 72.7% via 8 tests in `runner_remove_status_logs_test.go`. New focused functions: `Manager.Remove` 85.7%, `Manager.Status` 100.0%, `Manager.Logs` 100.0%. PR #388 (run #29557450180, dirSizesWindows) is still open as draft.

CI (`.github/workflows/ci.yml`) runs `go vet ./...`, `gofmt -l .`, and `go test ./... -race -count=1` on self-hosted Linux. Bench job runs `go test ./... -run='^$' -bench=. -benchmem -count=1`. There is local coverage support but no CI coverage profile/artifact.

Prior Test Improver work through 2026-07-17 is present on `main`: PR #381 (setupNative/startNativeOnce/handleStaleRecovery), #362 (Manager Start/Stop probe branches), #343 (runner dispatcher/disk branches), #336 (diskschedule), #321/#316 (autostart), #311 (runner pure helpers), and #304 (ops Update). Monthly issues: #306 (July, active), #305 (July duplicate, still open despite redirect).

Safeoutputs may report only a patch/bundle without a live PR number. Persist the temporary ID, branch, commit, patch, and bundle, then verify GitHub state in a later run rather than claiming a PR exists. Current intent: branch `test-assist/dirsizes-windows-branch`, commit `6d44d5b`, patch `/tmp/gh-aw/aw-test-assist-dirsizes-windows-branch.patch` (12,349 bytes, 320 lines), bundle `/tmp/gh-aw/aw-test-assist-dirsizes-windows-branch.bundle` (5,855 bytes).

[[commands]] [[testing-notes]] [[backlog]] [[wip]]
---
name: repo
description: baizhiheizi/gh-sr — Go GitHub CLI extension for self-hosted runners
metadata:
  type: project
---

Go module `github.com/an-lee/gh-sr` (remote `baizhiheizi.git`). CLI entry is `cmd/gh-sr/`; implementation is under `internal/`.

Coverage snapshot 2026-08-07 on `main`: `internal/runner` 73.7%. After branch `test-assist/manager-remove-status-logs-v2` (commit `20c5ca7`, draft PR intent), `internal/runner` is 76.4% (+2.7 pp); `Manager.Remove` / `Manager.Status` / `Manager.Logs` all 0% → 100% each via 10 tests in `runner_remove_status_logs_test.go`. PR #402 (perf-disk-usage-batch-ssh) merged into `main` 2026-08-07.

Prior test-improver PRs merged into `main`: #381 (setupNative/startNativeOnce/handleStaleRecovery), #362 (Manager Start/Stop probe branches), #343 (runner dispatcher/disk), #336 (diskschedule), #321/#316 (autostart), #311 (runner pure helpers), #304 (ops Update), #388 (dirSizesWindows).

CI (`.github/workflows/ci.yml`) runs `go vet ./...`, `gofmt -l .`, and `go test ./... -race -count=1` on self-hosted Linux. There is no CI coverage profile/artifact step.

Safeoutputs may report only a patch/bundle without a live PR number. Persist the temporary ID, branch, commit, patch, and bundle, then verify GitHub state in a later run. Current intent: branch `test-assist/manager-remove-status-logs-v2`, commit `20c5ca7`, patch `/tmp/gh-aw/aw-test-assist-manager-remove-status-logs-v2.patch` (19,801 bytes, 568 lines), bundle `/tmp/gh-aw/aw-test-assist-manager-remove-status-logs-v2.bundle` (5,935 bytes).

[[commands]] [[testing-notes]] [[backlog]] [[wip]]

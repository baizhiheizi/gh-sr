---
name: repo
description: baizhiheizi/gh-sr — Go GitHub CLI extension for self-hosted runners
metadata:
  type: project
---

Go module `github.com/an-lee/gh-sr` (remote `baizhiheizi.git`). CLI entry is `cmd/gh-sr/`; implementation is under `internal/`.

Coverage snapshot 2026-08-21 on `main`: `internal/runner` 82.1% (after branch `test-assist/environment-orchestrators`, commit `93b4217`, draft PR intent). 11 tests in `environment_orchestrators_test.go` cover `ContainerEnvironment.Provision` / `Start` / `Reset` / `Destroy` 0% → 100% each. Patch `/tmp/gh-aw/aw-test-assist-environment-orchestrators.patch` (15,511 bytes, 435 lines), bundle `/tmp/gh-aw/aw-test-assist-environment-orchestrators.bundle` (4,379 bytes).

Prior test-improver PRs merged into `main`: #410 (Windows native branches — merged 2026-08-19), #381 (setupNative/startNativeOnce/handleStaleRecovery), #362 (Manager Start/Stop probe branches), #343 (runner dispatcher/disk), #336 (diskschedule), #321/#316 (autostart), #311 (runner pure helpers), #304 (ops Update), #388 (dirSizesWindows).

CI (`.github/workflows/ci.yml`) runs `go vet ./...`, `gofmt -l .`, and `go test ./... -race -count=1` on self-hosted Linux. There is no CI coverage profile/artifact step.

Safeoutputs may report only a patch/bundle without a live PR number. Persist the temporary ID, branch, commit, patch, and bundle, then verify GitHub state in a later run.

[[commands]] [[testing-notes]] [[backlog]] [[wip]]
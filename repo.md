---
name: repo
description: baizhiheizi/gh-sr — Go GitHub CLI extension for self-hosted runners
metadata:
  type: project
---

Go module `github.com/an-lee/gh-sr` (remote `baizhiheizi/gh-sr.git`). CLI entry is `cmd/gh-sr/`; implementation is under `internal/`.

Coverage snapshot 2026-07-14: `internal/runner` baseline on current `main` was 62.8%; run #29308107521 raises the branch to 63.8%. Current focused functions after the branch tests: `Manager.Start` 53.8%, `Manager.Stop` 42.9%, and `startAutostartWithDarwinFallback` 60%. `setupNative`, `startNativeOnce`, `handleStaleRegistration`, and `dirSizesWindows` remain 0%.

CI (`.github/workflows/ci.yml`) runs `go vet ./...`, `gofmt -l .`, and `go test ./... -race -count=1` on self-hosted Linux. Bench job runs `go test ./... -run='^$' -bench=. -benchmem -count=1`. There is local coverage support but no CI coverage profile/artifact.

Prior Test Improver work through 2026-07-09 is present on `main`: PR #343 (runner dispatcher/disk branches), #336 (diskschedule), #321/#316 (autostart), #311 (runner pure helpers), and #304 (ops Update). Monthly issue #306 is active; #305 remains an open duplicate despite its existing redirect comment.

Safeoutputs may report only a patch/bundle without a live PR number. Persist the temporary ID, branch, commit, patch, and bundle, then verify GitHub state in a later run rather than claiming a PR exists. Current intent: `#aw_probe`, branch `test-assist/native-start-stop-probe-branches`, commit `bf59188`.

[[commands]] [[testing-notes]] [[backlog]] [[wip]]

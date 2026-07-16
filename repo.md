---
name: repo
description: baizhiheizi/gh-sr — Go GitHub CLI extension for self-hosted runners
metadata:
  type: project
---

Go module `github.com/an-lee/gh-sr` (remote `baizhiheizi/gh-sr.git`). CLI entry is `cmd/gh-sr/`; implementation is under `internal/`.

Coverage snapshot 2026-07-16: `internal/runner` baseline on current `main` was 64.1%; run #29473958530 raises it to 69.7% via 9 new tests in `native_setup_test.go`. Current focused functions: `Manager.Start` 53.8%, `Manager.Stop` 42.9%, `startAutostartWithDarwinFallback` 60%, `setupNative` 73.8%, `startNativeOnce` 56.7%, `handleStaleRegistration` 75.0%, `EnsureSetup` 100.0%. Windows branches of those native helpers and `dirSizesWindows` remain 0%.

CI (`.github/workflows/ci.yml`) runs `go vet ./...`, `gofmt -l .`, and `go test ./... -race -count=1` on self-hosted Linux. Bench job runs `go test ./... -run='^$' -bench=. -benchmem -count=1`. There is local coverage support but no CI coverage profile/artifact.

Prior Test Improver work through 2026-07-09 is present on `main`: PR #343 (runner dispatcher/disk branches), #336 (diskschedule), #321/#316 (autostart), #311 (runner pure helpers), and #304 (ops Update). Monthly issue #306 is active; #305 remains an open duplicate despite its existing redirect comment.

Safeoutputs may report only a patch/bundle without a live PR number. Persist the temporary ID, branch, commit, patch, and bundle, then verify GitHub state in a later run rather than claiming a PR exists. Current intent: `#aw_probe`, branch `test-assist/native-start-stop-probe-branches`, commit `bf59188`.

[[commands]] [[testing-notes]] [[backlog]] [[wip]]

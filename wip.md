---
name: wip
description: Current test-improver work in progress
metadata:
  type: project
---

## Run #27346830049 (2026-06-11 12:39 UTC)

- **Branch:** `test-assist/hostshell-write-remote-bytes`
- **Commit:** `ab1f73f test(hostshell): cover WriteRemoteBytes across POSIX and Windows branches`
- **PR:** draft via `create_pull_request` safeoutputs; patch at `/tmp/gh-aw/aw-test-assist-hostshell-write-remote-bytes.patch` (~14 KB / 362 lines)
- **Coverage delta:** `internal/hostshell` **23.1% → 100.0%** (+76.9 pp); `WriteRemoteBytes` 0% → 100% (function-level)
- **New file:** `internal/hostshell/hostshell_remote_test.go` (362 lines, 4 top-level test functions, 16 sub-tests, all parallel)
- **Status:** build ✅, vet ✅, `go test ./... -race -count=1` ✅ (all 11 packages), `gofmt` ✅, coverage 100% on hostshell

**Pinned contracts (caught by tests, would break on silent refactor):**
- `WriteRemoteBytes` POSIX path uses `PosixSingleQuote`; round-trips through `'\\''` escape.
- `WriteRemoteBytes` Windows path uses `PowerShellSingleQuote`; round-trips through `''` doubling.
- Executor errors propagate unchanged from Run/RunShell.
- `Upload()` is never called — bytes stream via shell pipeline, no local tempfile.

**Prior off-cycle run (not in repo-memory before this run):** PR #149 merged 2026-06-11T04:08:54Z — escapePS 0% → 100%, internal/diskschedule 15.6% → 16.3% (+0.7 pp). Memory run-history backfilled accordingly.

**Next:** Backlog still #1 ops orchestrators (refactor), #2 ResolveHostInfo, #3 CollectHostMetrics — all blocked on the ConnectHost/hostFactory injection. diskschedule's 0% surface (Detect/Install/Uninstall/Status) needs an exec.Command injection refactor. hostshell is now done.

[[backlog]] [[run-history]]

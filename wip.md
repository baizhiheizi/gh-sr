---
name: wip
description: Current test-improver work in progress
metadata:
  type: project
---

## Run #27415213477 (2026-06-12 — this run)

- **Branch:** `test-assist/run-per-host-parallel`; commit `4db58a6`
- **PR:** draft via `create_pull_request` safeoutputs; patch at `/tmp/gh-aw/aw-test-assist-run-per-host-parallel.patch` (~21 KB)
- **Coverage:** `internal/ops` **19.6% → 24.2%** (+4.6 pp); `runPerHostParallel` **0% → 100%**
- **New file:** `internal/ops/run_per_host_parallel_test.go` (442 lines, 8 tests, parallel, race-clean)
- **Refactor:** unexported `var connectHostFn = ConnectHost`; 9 internal callsites updated; per-call local capture in `runPerHostParallel`; `connectHostMu` mutex.
- **Status:** build ✅, vet ✅, `go test ./... -race -count=1` ✅ (12/12), gofmt clean on touched files.

**Pinned contracts:** empty → fast path; N-runner/1-host → 1 connect + N sequential; N-host → N concurrent; connect error propagates; fn error aborts host runner loop; `h.Close()` after fn; nil `w` → `io.Discard`; `lockedWriter` serialises each Write.

**Next:** Up/Down/Restart/Update/Remove/RebuildImage/CollectStatus/CollectDiskUsage/PruneDisk still 0% but mechanically testable via `installMockConnectHost`. `ResolveHostInfo` and `CollectHostMetrics` use `connectHostFn` directly.

## Prior run #27346830049 (2026-06-11) — hostshell WriteRemoteBytes

- 0% → 100%; `internal/hostshell` 23.1% → 100.0% (+76.9 pp); **PR #156 merged 2026-06-12T02:51:50Z**

[[backlog]] [[run-history]]

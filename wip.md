---
name: wip
description: Current test-improver work in progress
metadata:
  type: project
---

## Run #27523706025 (2026-06-15)

- **Branch:** `test-assist/collect-host-metrics`; commit `6dce7da`
- **PR:** draft via `create_pull_request`; patch at `/tmp/gh-aw/aw-test-assist-collect-host-metrics.patch` (~18 KB)
- **Coverage:** `internal/ops` **33.4% → 37.6%** (+4.2 pp); `CollectHostMetrics` **0% → 100%**; `internal/ops/metrics.go` file 100%
- **New file:** `internal/ops/metrics_collect_test.go` (537 lines, 15 tests, sequential)
- **Status:** build ✅, vet ✅, race ✅, full suite ✅, gofmt ✅.

**Pinned contracts:** empty cfg, single-host happy path, filter (existing + missing), sorted multi-host order, connect error with warning, all-hosts-fail, nil-writer (both branches), host.Close() ordering, concurrent fan-out via barrier, warning serialization (8 concurrent), mixed success/failure, single connect per host, no orphaned warnings.

**Sequential tests note:** Dropped `t.Parallel()` because `connectHostMu` is held for full test duration. 19+ tests in queue would exceed 60s CI timeout. Wall-time impact ~0.01s.

**Next:** `internal/ops` orchestrators (`Up`/`Down`/...) — heavy `*runner.Manager` mocks. `internal/diskschedule` Detect/Install/Uninstall/Status — needs `exec.Command` refactor.

## Prior runs (one-line each)

- 2026-06-14: ResolveHostInfo 0%→100%; ops 24.2%→33.4% (+9.2 pp); PR `test-assist/resolve-host-info` open
- 2026-06-12: runPerHostParallel 0%→100%; ops 19.6%→24.2% (+4.6 pp); **PR #168 merged**
- 2026-06-11: hostshell WriteRemoteBytes 0%→100%; **PR #156 merged**

[[backlog]] [[run-history]]

---
name: wip
description: Current test-improver work in progress
metadata:
  type: project
---

## Run #27675407379 (2026-06-17)

- **Branch:** `test-assist/down-orchestrator`; commit `c3b0f62`
- **PR:** draft via `create_pull_request`; patch at `/tmp/gh-aw/aw-test-assist-down-orchestrator.patch` (~15 KB)
- **Coverage:** `internal/ops` **41.1% → 41.9%** (+0.8 pp); `Down` **0% → 83.3%**
- **New file:** `internal/ops/down_test.go` (422 lines, 10 tests, all `t.Parallel()`)
- **Status:** build ✅, vet ✅, race ✅, full suite ✅, gofmt ✅.

**Pinned contracts:** EmptyRunners (no-match filter short-circuits), SingleRunner, MultipleRunnersSameHost (SSH amortisation), MultiHostConcurrent, FilterByHost, FilterByNameArgs, NilWriter toleration, StopErrorPropagates, StopErrorOnOneHostDoesNotPoisonAnother (host isolation under error), WriterSerialisedAcrossHosts (substring-count check for torn-write detection).

**Pattern:** Reuses `installMockConnectHost` from PR #168 + a real `*runner.Manager{GitHub: nil}` (since `mgr.Stop` is the test target, not the mock). Native-mode `Stop` path drives `autostart.Detect` → `stopNative` via a `MockExecutor` that returns `not running` for the pid-file probe and empty for the systemd-detect probes.

**Next:** `internal/ops` orchestrators: `Up` / `Restart` / `Update` / `Remove` / `CollectStatus` / `Logs` / `CleanupOffline` — all still 0%. `Restart` is the next-best target: it composes `Stop` + `Start` and exercises the "ignore first error" pattern. `Up` adds `EnsureSetup` to the chain.

## Prior runs (one-line each)

- 2026-06-15: CollectHostMetrics 0%→100%; ops 33.4%→37.6% (+4.2 pp); **PR #189 merged 2026-06-15T06:04:18Z**
- 2026-06-14: ResolveHostInfo 0%→100%; ops 24.2%→33.4% (+9.2 pp); **PR #178 merged 2026-06-14T12:01:38Z**
- 2026-06-12: runPerHostParallel 0%→100%; ops 19.6%→24.2% (+4.6 pp); **PR #168 merged 2026-06-12T23:00:16Z**
- 2026-06-11: hostshell WriteRemoteBytes 0%→100%; **PR #156 merged 2026-06-12T02:51:50Z**

[[backlog]] [[run-history]]

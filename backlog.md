---
name: backlog
description: Prioritized testing opportunities for an-lee/gh-sr
metadata:
  type: project
---

## High value (next runs)

### 1. `internal/ops` user-facing orchestrators (still 0%)
- `runPerHostParallel`, `ResolveHostInfo`, `CollectHostMetrics` all 100% covered. `Down` at 83.3%.
- Still at 0%: `Up`, `Restart`, `Update`, `Remove`, `RebuildImage`, `CollectStatus`, `Logs`, `CleanupOffline`. All route through `connectHostFn` (refactored 2026-06-12) and are mechanically testable via `installMockConnectHost`.
- `Restart` is the next-best target: composes `Stop` + `Start` and exercises the "ignore first error" pattern.
- `Up` adds `EnsureSetup` to the chain.
- `Remove` / `RebuildImage` need `partitionRebuildTargets` (100% covered separately) and either a `Manager` mock or a real `Manager` with `GitHub: nil`.

### 2. `internal/diskschedule` — Detect/Install/Uninstall/Status (0%)
- `escapePS`, `parseAtTime`, `systemdQuoteArg`, `xmlEscapePlist` covered. Need `exec.Command` injection refactor for the rest.

### 3. `internal/ops/service.go` — `ServiceInstall` (30.4%), `ServiceUninstall`/`ServiceStatus` (0%)
- Use the same `installMockConnectHost` seam; one PR per function.

## Medium value

### 4. `internal/runner/container.go` (43.7% overall) — large lifecycle functions, real Docker work
### 5. `internal/runner/native.go` — 21 untested fns, mostly Docker exec wrappers

## Low value (defer)

### 6. `internal/tui/*` (9.7%) — UI rendering
### 7. `cmd/gh-sr/*` (0%) — tested via `internal/ops`
### 8. `internal/autostart` (36.2%) — side-effect install helpers

## Patterns proved (reusable)

- **Package-level factory var + per-call capture**: `var connectHostFn = ConnectHost` + `connect := connectHostFn` at function entry. Tests swap via mutex-serialised helper.
- **`installMockConnectHost(t, map[string]host.Executor)`**: builds `*host.Host` with `host.SetConn` pre-wired to a mock executor.
- **`newDownMockExecutor()` pattern**: wire the 4 production Stop-path branches (svc.sh probe, systemd-user probe, systemd-system probe, pid_file probe) to known responses; tests then exercise the real `autostart.Detect` → `stopNative` chain without real services.
- **Real `*runner.Manager{GitHub: nil}` over a mock Manager**: the test target is the orchestrator's use of `mgr.Stop`, not the Manager itself; mocking the Manager would test the mock.
- **Sequential (`t.Parallel()`-free) tests when many parallel tests would queue past CI timeout**: `connectHostMu` is held for the entire test duration. Drop `t.Parallel()` for new tests using the factory-swap pattern.
- **Substring-count assertion over length**: torn-write detection on concurrent output (catches mutex failure that a length check would miss).

## Completed (do not re-do)

- 2026-06-17: `Down` 0% → 83.3%; `internal/ops` 41.1% → 41.9% (+0.8 pp).
- 2026-06-15: `CollectHostMetrics` 0% → 100%; `internal/ops` 33.4% → 37.6% (+4.2 pp); **PR #189 merged**.
- 2026-06-14: `ResolveHostInfo` 0% → 100%; `internal/ops` 24.2% → 33.4% (+9.2 pp); **PR #178 merged**.
- 2026-06-12: `runPerHostParallel` 0% → 100%; `internal/ops` 19.6% → 24.2% (+4.6 pp); **PR #168 merged**.
- 2026-06-11: `WriteRemoteBytes` 0% → 100%; `internal/hostshell` 23.1% → 100.0% (+76.9 pp); **PR #156 merged**.
- 2026-06-11: `escapePS` 0% → 100%; `internal/diskschedule` 15.6% → 16.3% (+0.7 pp); **PR #149 merged**.
- 2026-06-10: six pure helpers in `internal/ops/disk.go` 0% → 100%; `internal/ops` 9.6% → 19.6% (+10.0 pp); **PR #147 merged**.
- 2026-06-08: ValidateAWFHygiene/Inner 0% → 100%; `internal/agentic` 60.5% → 83.9% (+23.4 pp).
- 2026-06-06: Five `Validate*` helpers 0% → 100%; `internal/agentic` 44.8% → 60.5%.

[[repo]] [[testing-notes]] [[wip]] [[run-history]]

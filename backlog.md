---
name: backlog
description: Prioritized testing opportunities for an-lee/gh-sr
metadata:
  type: project
---

## High value (next runs)

### 1. `internal/ops` user-facing orchestrators — Up/Down/Restart/Update/Remove/RebuildImage (still 0%)
- `runPerHostParallel` and `ResolveHostInfo` are now both 100% covered.
- All six orchestrators route through `connectHostFn` (refactored 2026-06-12) and are mechanically testable via `installMockConnectHost`. Each requires its own `*runner.Manager` mock — pick one per run.

### 2. `internal/ops` — `CollectHostMetrics` (0%)
- Concurrent per-host metrics via `h.CollectMetrics()`. Uses `connectHostFn`. Same injection pattern as `ResolveHostInfo`. Single-PR achievable.

### 3. `internal/diskschedule` — Detect/Install/Uninstall/Status (0%)
- `escapePS` (PR #149, 2026-06-11) already covered. Need `exec.Command` injection refactor.

## Medium value

### 5. `internal/runner/container.go` (37%) — large lifecycle functions, real Docker work
### 6. `internal/runner/native.go` — 21 untested fns, mostly Docker exec wrappers

## Low value (defer)

### 7. `internal/tui/*` (4.9%) — UI rendering
### 8. `cmd/gh-sr/*` (0%) — tested via `internal/ops`
### 9. `internal/autostart` (17.2%) — side-effect install helpers

## Patterns proved this run (reusable)

- **Package-level factory var + per-call capture**: `var connectHostFn = ConnectHost` + `connect := connectHostFn` at function entry. Tests swap via mutex-serialised helper. Goroutines use the captured local — per-call isolation under `t.Parallel()`.
- **`installMockConnectHost(t, map[string]host.Executor)`**: builds `*host.Host` with `host.SetConn` pre-wired to a mock executor.

## Completed (do not re-do)

- 2026-06-14: `ResolveHostInfo` 0% → 100%; `internal/ops` 24.2% → 33.4% (+9.2 pp); pure test addition reusing `installMockConnectHost` from PR #168.
- 2026-06-12: `runPerHostParallel` 0% → 100%; `internal/ops` 19.6% → 24.2% (+4.6 pp); refactor: `connectHostFn` + 9 callsites + per-call capture + `connectHostMu`. **PR #168 merged**.
- 2026-06-11: `WriteRemoteBytes` 0% → 100%; `internal/hostshell` 23.1% → 100.0% (+76.9 pp); **PR #156 merged**.
- 2026-06-11: `escapePS` 0% → 100%; `internal/diskschedule` 15.6% → 16.3% (+0.7 pp); **PR #149 merged**.
- 2026-06-10: six pure helpers in `internal/ops/disk.go` 0% → 100%; `internal/ops` 9.6% → 19.6% (+10.0 pp); **PR #147 merged**.
- 2026-06-08: ValidateAWFHygiene/Inner 0% → 100%; `internal/agentic` 60.5% → 83.9% (+23.4 pp).
- 2026-06-06: Five `Validate*` helpers 0% → 100%; `internal/agentic` 44.8% → 60.5%.

[[repo]] [[testing-notes]] [[wip]] [[run-history]]

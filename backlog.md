---
name: backlog
description: Prioritized testing opportunities for baizhiheizi/gh-sr
metadata:
  type: project
---

## High value (next runs)

### 1. `internal/ops` user-facing orchestrators (now 44.0%)
- `runPerHostParallel`, `ResolveHostInfo`, `CollectHostMetrics` all 100%; `Down` 83.3%; `Restart` 85.7%; **`Up` 77.8%** (2026-06-19).
- Still at 0%: `Update`, `Remove`, `RebuildImage`, `CollectStatus`, `Logs`, `CleanupOffline`. All route through `connectHostFn` and are mechanically testable via `installMockConnectHost`.
- `Update` is the natural next target after `Remove` — composes Remove + Setup + Start; needs `*runner.Manager{GitHub: client}` (httptest-backed) for Remove.
- `Remove` is the next natural target — `partitionRebuildTargets` already covered. Needs `httptest.Server`-backed `*runner.GitHubClient` (the `GitHub: nil` manager used by Up/Down/Restart/Logs panics in `removeNative`'s `m.GitHub.GetRemovalTokenScoped` call).
- `Up` 22.2% gap is the container-mode branch (too Docker-coupled without a new abstraction).
- `Logs` 7.1% gap is the `ResolveHostInfo` error branch (already covered indirectly by `CollectStatus` suite).

### 2. `internal/diskschedule` — Detect/Install/Uninstall/Status (0%)
- `escapePS`, `parseAtTime`, `systemdQuoteArg`, `xmlEscapePlist` covered. Need `exec.Command` injection refactor for the rest.

### 3. `internal/ops/service.go` — `ServiceInstall` (30.4%), `ServiceUninstall`/`ServiceStatus` (0%)
- Same `installMockConnectHost` seam; one PR per function.

## Medium value

### 4. `internal/runner/container.go` (43.7%) — large lifecycle, real Docker work
### 5. `internal/runner/native.go` — 21 untested fns, mostly Docker exec wrappers

## Low value (defer)

### 6. `internal/tui/*` (9.7%) — UI rendering
### 7. `cmd/gh-sr/*` (0%) — tested via `internal/ops`
### 8. `internal/autostart` (36.2%) — side-effect install helpers

## Patterns proved (reusable)

- **Package-level factory var + per-call capture**: `var connectHostFn = ConnectHost` + `connect := connectHostFn` at function entry.
- **`installMockConnectHost(t, map[string]host.Executor)`**: builds `*host.Host` with `host.SetConn` pre-wired to a mock executor.
- **`newDownMockExecutor()` / `newRestartMockExecutor()`**: wire Stop / Stop+Start paths to known responses.
- **Dual-path Stop+Start mock (Restart)**: share svc.sh + autostart probes; disambiguate by substring (`rm -f` only in Stop; `nohup` only in Start). Use `Ephemeral: true` to skip `autostart.Install`.
- **Real `*runner.Manager{GitHub: nil}` over a mock Manager**.
- **Sequential tests when many parallel tests would queue past CI timeout**: `connectHostMu` is held for the entire test duration.
- **Substring-count assertion over length**: torn-write detection on concurrent output.

## Completed (do not re-do)

- 2026-06-20: Logs 0%→92.9%; ops 50.2%→52.3% (+2.1 pp). **Patch preserved at `/tmp/gh-aw/aw-test-assist-logs-orchestrator.patch`** (bridge failed; same as Up).
- 2026-06-19: Up 0%→77.8%; ops 42.8%→44.0% (+1.2 pp). **Patch preserved at `/tmp/gh-aw/aw-test-assist-up-orchestrator.patch`** (bridge failed; same as Restart). Squashed into main via ff8d9cd.
- 2026-06-18: Restart 0%→85.7%; ops 41.9%→42.8% (+0.9 pp). **Squashed into main via ff8d9cd (2026-06-19)**.
- 2026-06-17: Down 0%→83.3%; ops 41.1%→41.9% (+0.8 pp). **Squashed into main via ff8d9cd (2026-06-19)**.
- 2026-06-15: CollectHostMetrics 0%→100%; ops 33.4%→37.6% (+4.2 pp); **PR #189 merged**.
- 2026-06-14: ResolveHostInfo 0%→100%; ops 24.2%→33.4% (+9.2 pp); **PR #178 merged**.
- 2026-06-12: runPerHostParallel 0%→100%; ops 19.6%→24.2% (+4.6 pp); **PR #168 merged**.
- 2026-06-11: WriteRemoteBytes 0%→100%; hostshell 23.1%→100.0% (+76.9 pp); **PR #156 merged**.
- 2026-06-10: six pure helpers in ops/disk.go 0%→100%; ops 9.6%→19.6% (+10.0 pp); **PR #147 merged**.
- 2026-06-08: ValidateAWFHygiene/Inner 0%→100%; agentic 60.5%→83.9% (+23.4 pp).

[[repo]] [[testing-notes]] [[wip]] [[run-history]]

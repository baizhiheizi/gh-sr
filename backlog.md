---
name: backlog
description: Prioritized testing opportunities for baizhiheizi/gh-sr
metadata:
  type: project
---

## High value (next runs)

### 1. `internal/ops` user-facing orchestrators (now 68.2%)
- `CleanupOffline`, `runPerHostParallel`, `ResolveHostInfo`, `CollectHostMetrics`, `removeHost` all 100%; `Remove` 93.3%; `Logs` 92.9%; `CollectStatus` 90.5%; `Restart` 85.7%; `Down` 83.3%; `ServiceStatus` 80.0%; `Up` 77.8%; `ServiceUninstall` 75.0%; `RebuildImage` 69.2%; `ServiceCleanup` 67.5%; `Setup` 56.2%; `Update` 53.8% (2026-06-24).
- Still at 0%: `ConnectHost`, `setupHost`, `CollectDiskUsage`, `PruneDisk`.
- `Setup` 56.2% is the next natural target (~44% gap = per-host setupHost branch).
- `Update` 53.8% gap; orchestrator composes Remove + Setup + Start.
- **Bug (out of scope):** `CollectStatus` panics on nil `mgr.GitHub`. Same family as nil-`w` panic in `Remove`/`Update`.

### 2. `internal/diskschedule` (14.2%) — `Detect`/`Install`/`Uninstall`/`Status` 0%
- Needs `exec.Command` injection refactor.

### 3. `internal/ops/service.go` partial — `ServiceCleanup` (67.5%), `ServiceUninstall` (75%)

## Patterns proved (reusable)

- **Package-level factory var + per-call capture**: `var connectHostFn = ConnectHost` + `connect := connectHostFn` at function entry.
- **`installMockConnectHost(t, map[string]host.Executor)`**: builds `*host.Host` with `host.SetConn` pre-wired to a mock executor.
- **Real `*runner.Manager{GitHub: g}` over a mock Manager** — `g` from `httptest.NewServer` answering the GitHub API.
- **`barrierMockExecutor`**: signals on Run + blocks on barrier — proves goroutines actually run in parallel.
- **`newStatusNativeRunningMock()`**: substring-matched `kill -0` → `running`.

## Completed (do not re-do)

- 2026-06-24: CollectStatus 0%→90.5%; ops 62.1%→68.2% (+6.1 pp). Patch at `/tmp/gh-aw/aw-test-assist-collect-status-orchestrator.patch` (bridge failed).
- 2026-06-21: Remove 0%→94.1%; ops 52.3%→54.6% (+2.3 pp). Patch at `/tmp/gh-aw/aw-test-assist-remove-orchestrator.patch` (bridge failed).
- 2026-06-20: Logs 0%→92.9%; ops 50.2%→52.3% (+2.1 pp). Patch at `/tmp/gh-aw/aw-test-assist-logs-orchestrator.patch` (bridge failed).
- 2026-06-19: Up 0%→77.8%; ops 42.8%→44.0% (+1.2 pp). Squashed via ff8d9cd.
- 2026-06-18: Restart 0%→85.7%; ops 41.9%→42.8% (+0.9 pp). Squashed via ff8d9cd.
- 2026-06-17: Down 0%→83.3%; ops 41.1%→41.9% (+0.8 pp). **PR #200 merged**.
- 2026-06-15: CollectHostMetrics 0%→100%; ops 33.4%→37.6% (+4.2 pp); **PR #189 merged**.
- 2026-06-14: ResolveHostInfo 0%→100%; ops 24.2%→33.4% (+9.2 pp); **PR #178 merged**.
- 2026-06-12: runPerHostParallel 0%→100%; ops 19.6%→24.2% (+4.6 pp); **PR #168 merged**.
- 2026-06-11: WriteRemoteBytes 0%→100%; hostshell 23.1%→100.0%; **PR #156 merged**.
- 2026-06-10: six pure helpers in ops/disk.go 0%→100%; ops 9.6%→19.6% (+10.0 pp); **PR #147 merged**.

[[repo]] [[testing-notes]] [[wip]] [[run-history]]
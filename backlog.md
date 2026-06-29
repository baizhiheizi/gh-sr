---
name: backlog
description: Prioritized testing opportunities for baizhiheizi/gh-sr
metadata:
  type: project
---

## High value (next runs)

### 1. `internal/ops` user-facing orchestrators (90.9% as of 2026-06-29)
- Remaining gaps: `Update` 53.8% (largest; Remove + Setup + Start), `RebuildImage` 76.9% (inner callback uncovered), `Up` 77.8% (`EnsureSetup` error branch), `ServiceCleanup` 67.5%, `ServiceUninstall` 75%, `ServiceStatus` 80%, `ServiceInstall` 90.5%, `Remove` 93.3%.
- Still at 0%: `ConnectHost` (SSH integration test).

### 2. `internal/diskschedule` (14.2%) — `Detect`/`Install`/`Uninstall`/`Status` 0%
- Needs `exec.Command` injection refactor.

### 3. `internal/autostart` (43.7%), `internal/runner/container.go` (~50%), `internal/tui/*` (16.5%) — broader package wins.

## Patterns proved (reusable)

- `installMockConnectHost(t, map[string]host.Executor)` and `installFailingConnectHost(t, sentinel)` — mock/sentinel-returning factory swap (uses `connectHostMu` for `-race` safety).
- `*runner.Manager{GitHub: runner.NewGitHubClientWithHTTP(pat, ts.Client(), ts.URL)}` — real Manager backed by httptest GitHub server.
- `barrierMockExecutor` (signals + blocks) and `newStatusNativeRunningMock()` (substring-matched `kill -0` → "running").

## Completed (do not re-do)

- 2026-06-29: Down→100, Restart→100, RebuildImage→76.9%; ops 90.4→90.9. Phantom patch.
- 2026-06-27: CollectDiskUsage 0→92.1%; PruneDisk 0→91.9%; ops +20.5 pp. **PR #280 merged**.
- 2026-06-26: Setup 56.2→100. **PR #270 merged**.
- 2026-06-24: CollectStatus 0→90.5. **PR #256 merged**.
- 2026-06-21: Remove 0→94.1. **PR #242 merged**.
- 2026-06-20: Logs 0→92.9. **PR #233 merged**.
- 2026-06-19: Up/Restart covered. **PR #224/#216 merged**.
- 2026-06-17: Down 0→83.3. **PR #200 merged**.
- Earlier: #189, #178, #168, #156, #147 (all merged).

[[repo]] [[testing-notes]] [[wip]] [[run-history]]

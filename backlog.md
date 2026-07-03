---
name: backlog
description: Prioritized testing opportunities for baizhiheizi/gh-sr
metadata:
  type: project
---

## High value (next runs)

### 1. `internal/autostart.installLaunchd` + `installWindowsTask` (both 0%)
- Reachable via the same Kind-dispatch pattern proved in Start/Stop/Status. Could close `internal/autostart` 84.9% → ~95%.

### 2. `internal/diskschedule` (14.2%) — needs `exec.Command` injection refactor
- Pure helpers (parseAtTime / systemdQuoteArg / PlistEscape / PowerShellSingleQuote) already at 100%.
- `Detect` linux/darwin branches testable via `t.Setenv("HOME", t.TempDir())` + `os.Stat`.

### 3. `internal/runner` (56.7%) — broader package wins
- Pure helpers covered this run. Orchestrators at 0% (setupNative, startNative, etc.) need real Manager + httptest — high complexity, lower marginal ROI.

### 4. `internal/tui/*` (15.0%) — bubbletea UI rendering (lower priority)

## Patterns proved (reusable)

- `installMockConnectHost(t, map[string]host.Executor)` factory swap (uses `connectHostMu` for `-race`).
- `*runner.Manager{GitHub: runner.NewGitHubClientWithHTTP(pat, ts.Client(), ts.URL)}` — real Manager over httptest.
- `barrierMockExecutor`, `newStatusNativeRunningMock()` — substring-matched mocks.
- **Pure-function table tests with `t.Parallel()`** (2026-07-02): 12 sub-cases, no mocks/SSH/exec, high leverage.
- **Kind-dispatch orchestrator tests** (2026-07-03): `TestStart`/`TestStop`/`TestStatus` table over 4 (os, kind) combos. Windows arms use `powershell.exe -EncodedCommand` substring (since `host.Host.RunShell` base64-encodes the script).

## Completed (do not re-do)

- 2026-07-03: Start 0→91.7, Stop 0→95, Status 0→96.6, Uninstall 31.6→84.2; autostart +22.2 pp. Phantom.
- 2026-07-02: formatContainerImageBuild 0→100, windowsNativeConfigScript 71.4→100, containerImageExtraApt 66.7→100; runner +1.1 pp. Phantom.
- 2026-07-01: Update 53.8→100; ops +1.0 pp. PR #304 landed.
- 2026-06-30: ServiceCleanup 67.5→92.5. Phantom.
- 2026-06-29: Down→100, Restart→100, RebuildImage→76.9. PR #293.
- 2026-06-27: CollectDiskUsage 0→92.1; PruneDisk 0→91.9. PR #280.
- 2026-06-26: Setup 56.2→100. PR #270.
- 2026-06-24: CollectStatus 0→90.5. PR #256.
- 2026-06-21: Remove 0→94.1. PR #242.
- 2026-06-20: Logs 0→92.9. PR #233.
- 2026-06-19: Up/Restart. PR #224/#216.
- 2026-06-17: Down 0→83.3. PR #200.
- Earlier: #189, #178, #168, #156, #147 (all merged).

[[repo]] [[testing-notes]] [[wip]] [[run-history]]

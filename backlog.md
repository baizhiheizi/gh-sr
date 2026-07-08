---
name: backlog
description: Prioritized testing opportunities for baizhiheizi/gh-sr
metadata:
  type: project
---

## Next priorities

1. **`internal/runner` (56.7%)** — next meaningful package win. Focus on orchestrators/lifecycle paths (`setupNative`, `startNative`, `removeContainer`, `rebuildContainerImage`) with real `*runner.Manager`, httptest GitHub API, and command mocks. Higher complexity than pure helpers.
2. **`internal/diskschedule` follow-ups (88.2% after 2026-07-08)** — core lifecycle now covered (`Detect` 90.9, `Install`/`Uninstall` 100, `Status` 94.1). Remaining gaps are mostly file/command write errors in systemd/launchd helpers; lower ROI.
3. **`internal/autostart` (94.7%)** — only incremental cleanup remains (`Uninstall` defaults, `instanceFromServiceBasename`, `IsStale`, `CleanupStale`).
4. **`internal/tui` (15%)** — bubbletea rendering; low priority unless coverage threshold appears.

## Reusable patterns

- `installMockConnectHost` + `connectHostMu` for race-clean `host.Executor` factory swaps.
- Real `*runner.Manager{GitHub: runner.NewGitHubClientWithHTTP(...)}` over mocks when testing orchestrator/Manager interaction.
- Pure helper table tests with `t.Parallel()` when no env/global seams.
- Autostart Windows tests assert `powershell.exe -EncodedCommand` wrapper because `host.Host.RunShell` base64-encodes scripts.
- 2026-07-08 diskschedule pattern: unexported package vars for `runtimeGOOS`, exec, and PowerShell seams; reset with `t.Cleanup`, no `t.Parallel()`.

## Completed (do not re-do)

- 2026-07-08: `internal/diskschedule` 14.2→88.2; PR intent `#aw_dskpr`, patch `/tmp/gh-aw/aw-test-assist-diskschedule-command-seams.patch`.
- 2026-07-04: autostart Install helpers; package 84.9→94.7. Present on main by 2026-07-08.
- 2026-07-03: autostart Start/Stop/Status/Uninstall; package +22.2 pp. Present on main by 2026-07-08.
- 2026-07-02: runner pure helpers; package 55.6→56.7. Present on main by 2026-07-08.
- 2026-07-01: ops Update 53.8→100. Present on main by 2026-07-08.
- Earlier merged/present: ServiceCleanup, Down/Restart/RebuildImage, CollectDiskUsage/PruneDisk, Setup, CollectStatus, Remove, Logs, Up/Restart, ResolveHostInfo, runPerHostParallel, WriteRemoteBytes.

[[repo]] [[testing-notes]] [[wip]] [[run-history]]

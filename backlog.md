---
name: backlog
description: Prioritized testing opportunities for baizhiheizi/gh-sr
metadata:
  type: project
---

## Next priorities

1. **`internal/runner` deep pipelines** (59.6%) — `setupNative`, `startNative` (each 0%) still need seam refactors similar to diskschedule's GOOS/runtime seams (#336) before they're testable without SSH/service side effects. `EnsureSetup` (runner.go:108, 0%) is now the simplest next target after that.
2. **`internal/runner` Windows shell branches** — `removeDirTree` and `PruneInstance`'s Windows dispatch paths still uncovered; the base64 `wrapCommand` for `RunShell` already works on remote addrs (verified by `TestClearWorkTemp_windowsBranch` in 2026-07-09 run).
3. **`internal/diskschedule` follow-ups** (88.2%) — command/file write error branches in systemd/launchd helpers remain, but core lifecycle behavior is now covered.
4. **`internal/autostart`** (94.7%) — only incremental cleanup remains (`Uninstall` defaults, `instanceFromServiceBasename`, `IsStale`, `CleanupStale`).
5. **`internal/tui`** (15%) — bubbletea rendering; low priority unless coverage threshold appears.

## Reusable patterns

- `installMockConnectHost` + `connectHostMu` for race-clean `host.Executor` factory swaps.
- Real `*runner.Manager{GitHub: runner.NewGitHubClientWithHTTP(...)}` over mocks when testing orchestrator/Manager interaction.
- Pure helper table tests with `t.Parallel()` when no env/global seams are mutated.
- Autostart Windows tests assert `powershell.exe -EncodedCommand` wrapper because `host.Host.RunShell` base64-encodes scripts.
- 2026-07-08 diskschedule pattern: unexported package vars for `runtimeGOOS`, exec, and PowerShell seams; reset with `t.Cleanup`, no `t.Parallel()`.
- 2026-07-09 `answerInstancePresence(...)` matcher: routes `RemoteBoolCheck` probes for both native (`test -d ... && test -f run.sh && test -f .runner`) and container (`docker inspect | grep -q '^/name$'`) presence checks to per-instance "yes"/"no" answers in a single `RunFn`. Useful whenever testing Manager-level `NeedsSetup`-style dispatchers with multiple `InstanceNames()`.

## Completed (do not re-do)

- 2026-07-09: `internal/runner` Manager-level dispatcher tests; `NeedsSetup`/`RebuildImage` 0% → 100%; `clearWorkTemp` 50% → 100%; package 57.6% → 59.6%.
- 2026-07-08: `internal/diskschedule` 14.2→88.2; PR #336 merged.
- 2026-07-04: autostart Install helpers; package 84.9→94.7. Present on main by 2026-07-08.
- 2026-07-03: autostart Start/Stop/Status/Uninstall; package +22.2 pp. Present on main by 2026-07-08.
- 2026-07-02: runner pure helpers; package 55.6→56.7. Present on main by 2026-07-08.
- 2026-07-01: ops Update 53.8→100. Present on main by 2026-07-08.
- Earlier merged/present: ServiceCleanup, Down/Restart/RebuildImage, CollectDiskUsage/PruneDisk, Setup, CollectStatus, Remove, Logs, Up/Restart, ResolveHostInfo, runPerHostParallel, WriteRemoteBytes.

[[repo]] [[testing-notes]] [[wip]] [[run-history]]

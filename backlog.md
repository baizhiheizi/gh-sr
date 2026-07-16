---
name: backlog
description: Prioritized testing opportunities for baizhiheizi/gh-sr
metadata:
  type: project
---

## Next priorities

1. **Windows runner branches in `setupNative`/`startNativeOnce`/`handleStaleRegistration`** — Linux covered on 2026-07-16 (run #29473958530) up to 56-100% per function; Windows paths still 0%. Reuse the non-local-address PowerShell wrapper pattern (`Addr: runner@vps` for Windows to trigger `wrapCommand`).
2. **Windows `dirSizesWindows`** (disk.go:290) — still 0%; covers the `powershell.exe` size-collection branch.
3. **Coverage infrastructure** — CI runs vet/format/full race but does not generate or retain a coverage profile. A separate non-gating coverage artifact job is reasonable after maintainer agreement; do not add a threshold without a policy.
4. **`internal/diskschedule` follow-ups** (88.2%) — command/file-write error branches remain, but core lifecycle behavior is covered.
5. **`internal/tui` rendering** — low priority unless a coverage policy or specific UI regression appears.

## Reusable patterns

- `installMockConnectHost` + `connectHostMu` for race-clean `host.Executor` factory swaps.
- Real `*runner.Manager{GitHub: runner.NewGitHubClientWithHTTP(...)}` over mocks when testing orchestrator/Manager interaction.
- Pure helper table tests with `t.Parallel()` when no env/global seams are mutated.
- OS command seams must reset with `t.Cleanup`; do not parallelize tests that mutate package globals.
- `answerInstancePresence(...)` routes native/container setup probes to per-instance yes/no results.
- For Windows `RunShell` branch tests, use a non-local address so `powershell.exe -EncodedCommand` wrapping is observable.
- 2026-07-14 lifecycle pattern: a sequence-aware `MockExecutor.RunFn` can pin combined probe → install → re-probe → autostart action order while matching stable command intent rather than complete generated scripts.

## Completed (do not re-do)

- 2026-07-14: Manager Start/Stop combined-probe lifecycle branches; `internal/runner` 62.8→63.8, Start 30.8→53.8, Stop 35.7→42.9; draft PR intent `#aw_probe`.
- 2026-07-09: Manager dispatcher/disk tests merged as PR #343; NeedsSetup/RebuildImage reached 100%; package 57.6→59.6.
- 2026-07-08: `internal/diskschedule` 14.2→88.2; PR #336 merged.
- 2026-07-04: autostart Install helpers; package 84.9→94.7; PR #321 merged.
- 2026-07-03: autostart Start/Stop/Status/Uninstall; package +22.2 pp; PR #316 merged.
- 2026-07-02: runner pure helpers; package 55.6→56.7; PR #311 merged.
- 2026-07-01: ops Update 53.8→100; PR #304 merged.
- Earlier merged/present: ServiceCleanup, Down/Restart/RebuildImage, CollectDiskUsage/PruneDisk, Setup, CollectStatus, Remove, Logs, Up/Restart, ResolveHostInfo, runPerHostParallel, WriteRemoteBytes.

[[repo]] [[testing-notes]] [[wip]] [[run-history]]

---
name: backlog
description: Prioritized testing opportunities for baizhiheizi/gh-sr
metadata:
  type: project
---

## Next priorities

1. **Windows runner branches in `setupNative`/`startNativeOnce`/`handleStaleRegistration`/`stopNative`/`removeNative`** — Linux covered 2026-07-16 (run #29473958530) up to 56-100% per function; Windows paths still 0%. Reuse the non-local-address PowerShell wrapper pattern (`Addr: runner@vps` triggers `host.Host.wrapCommand`) and the `decodeEncodedPowerShellCommand` helper from disk_test.go (UTF-16LE + base64) so tests can decode and assert against the real script body.
2. **`internal/tui` rendering** (18.9%) — low priority unless a coverage policy or specific UI regression appears.
3. **`internal/diskschedule` follow-ups** (88.2%) — command/file-write error branches remain, but core lifecycle behavior is covered.
4. **Coverage infrastructure** — CI runs vet/format/full race but does not generate or retain a coverage profile. A separate non-gating coverage artifact job is reasonable after maintainer agreement; do not add a threshold without a policy.

## Reusable patterns

- `installMockConnectHost` + `connectHostMu` for race-clean `host.Executor` factory swaps.
- Real `*runner.Manager{GitHub: runner.NewGitHubClientWithHTTP(...)}` over mocks when testing orchestrator/Manager interaction.
- Pure helper table tests with `t.Parallel()` when no env/global seams are mutated.
- OS command seams must reset with `t.Cleanup`; do not parallelize tests that mutate package globals.
- For Windows `RunShell` branch tests, use a non-local address (`Addr: runner@vps`) so `host.Host.wrapCommand` activates the `powershell -EncodedCommand` base64 wrapper. `decodeEncodedPowerShellCommand` (disk_test.go) mirrors `host.encodePowerShellScript` so tests can assert script content directly.
- 2026-07-14 lifecycle pattern: a sequence-aware `MockExecutor.RunFn` can pin combined probe → install → re-probe → autostart action order while matching stable command intent rather than complete generated scripts.
- 2026-07-18: `linuxInstanceProbe` V<version>+U marker pattern is reusable for `Manager.Status` orchestrator tests without re-pinning `statusNativeFromProbe`'s internal branches.

## Completed (do not re-do)

- 2026-07-18: `Manager.Remove` / `Manager.Status` / `Manager.Logs` orchestrators; `internal/runner` 69.9→72.7 (+2.8 pp); Manager.Remove 0→85.7; Manager.Status 0→100; Manager.Logs 0→100; draft PR intent `test-assist/manager-remove-status-logs-orchestrators`, commit `2af02ce`.
- 2026-07-17: `dirSizesWindows` + `dirSizes` Windows dispatch + `parseFourInt64s` Windows-emit paths; `internal/runner` 69.9→71.2, dirSizesWindows 0→100, dirSizes 75→100, parseFourInt64s 62.5→85; draft PR #388, commit `20aa85`.
- 2026-07-16: setupNative/startNativeOnce/handleStaleRecovery; `internal/runner` 64.1→69.7 (+5.6 pp); PR #381 merged.
- 2026-07-14: Manager Start/Stop combined-probe lifecycle branches; PR #362 merged.
- 2026-07-09: Manager dispatcher/disk tests merged as PR #343; NeedsSetup/RebuildImage reached 100%.
- 2026-07-08: `internal/diskschedule` 14.2→88.2; PR #336 merged.
- Earlier merged/present: autostart Install/Start/Stop/Status/Uninstall; runner pure helpers; ops Update/ServiceCleanup/Down/Restart/RebuildImage/CollectDiskUsage/PruneDisk/Setup/CollectStatus/Remove/Logs/Up/ResolveHostInfo/runPerHostParallel/WriteRemoteBytes.

[[repo]] [[testing-notes]] [[wip]] [[run-history]]
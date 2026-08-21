---
name: backlog
description: Prioritized testing opportunities for baizhiheizi/gh-sr
metadata:
  type: project
---

## Next priorities

1. **`containerAwaitHealthy` deadline branches** (52.4%) — the timeout-driven polling loop in `environment.go` requires a fake clock to exercise deadline expiration deterministically; otherwise the slow polling makes the test expensive.
2. **`internal/tui` rendering** (33.1%) — low priority unless a coverage policy or specific UI regression appears.
3. **`internal/host` connection/auth** (66.3%) — `connection.go` SSH auth methods (0%) require real SSH transport; out of scope for in-process tests.
4. **`internal/hostshell/ps`** (60.0%) — `Exec`/`CombinedOutput` 0% but require PowerShell runtime; Windows-only.
5. **`internal/runner/native.go` Windows branches** — `startNative` (0%) + `statusNativeOneshotNonLinux` (44.4%) + `statusNativeFromProbe` (52.9%) + `nativeRunnerVersion` (35.7%) + `removeNativeServices` (60%) + `logsNative` (66.7%); non-local Addr test seams already proven.
6. **`internal/runner/orphans.go`** — `PlanOrphanCleanup` (38.1%) + `CleanupOrphanInstance` (50%) + `instanceDirectoryExists` (0%).
7. **`internal/diskschedule` follow-ups** (88.2%) — command/file-write error branches remain, but core lifecycle behavior is covered.
8. **Coverage infrastructure** — CI runs vet/format/full race but does not generate or retain a coverage profile. A separate non-gating coverage artifact job is reasonable after maintainer agreement; do not add a threshold without a policy.

## Reusable patterns

- `installMockConnectHost` + `connectHostMu` for race-clean `host.Executor` factory swaps.
- Real `*runner.Manager{GitHub: runner.NewGitHubClientWithHTTP(...)}` over mocks when testing orchestrator/Manager interaction.
- Pure helper table tests with `t.Parallel()` when no env/global seams are mutated.
- OS command seams must reset with `t.Cleanup`; do not parallelize tests that mutate package globals.
- For Windows `RunShell` branch tests, use a non-local address (`Addr: runner@vps`) so `host.Host.wrapCommand` activates the `powershell -EncodedCommand` base64 wrapper. `decodeEncodedPowerShellCommand` (disk_test.go) mirrors `host.encodePowerShellScript` so tests can assert script content directly.
- 2026-07-14 lifecycle pattern: a sequence-aware `MockExecutor.RunFn` can pin combined probe → install → re-probe → autostart action order while matching stable command intent rather than complete generated scripts.
- 2026-07-18: `linuxInstanceProbe` V<version>+U marker pattern is reusable for `Manager.Status` orchestrator tests without re-pinning `statusNativeFromProbe`'s internal branches.
- 2026-08-07 container-status pattern: when the orchestrator delegates to a single h.Run shell script, have the mock simulate the script's stdout directly (e.g. `|configImage|digest|imageRev\n` for `containerLocalStatusOneShot`) rather than running individual `docker inspect` / `docker image inspect` sub-commands.
- 2026-08-14 Windows-branch pattern: the presence probe in `NativeRunnerConfigPresent` is the only Windows script that uses `Write-Output 'yes'/'no'` (start/stop use `Write-Host`); tests can use `Write-Output 'no'` as a unique fingerprint to disambiguate the probe response from other PowerShell scripts without depending on the full generated script body.
- 2026-08-21 container-environment pattern: `combinedGitHubStub` (single httptest server for both releases/latest + registration-token endpoints) + `envTestRig(t, mock, regTokStatus)` is reusable for any `ContainerEnvironment`-based test (next up: `containerAwaitHealthy` deadline branches via fake clock).

## Completed (do not re-do)

- 2026-08-21: `ContainerEnvironment.Provision` / `Start` / `Reset` / `Destroy` (0% → 100% each); `internal/runner` 81.2→82.1 (+0.9 pp); 11 tests in `internal/runner/environment_orchestrators_test.go`; branch `test-assist/environment-orchestrators`, commit `93b4217`. Helpers: `combinedGitHubStub`, `envTestRig`, `envTestRigVersionFail`.
- 2026-08-07: `Manager.Remove` / `Manager.Status` / `Manager.Logs` + `Manager.Out` fallback orchestrators; `internal/runner` 73.7→76.4 (+2.7 pp); Manager.Remove/Status/Logs 0→100 each; draft PR intent `test-assist/manager-remove-status-logs-v2`, commit `20c5ca7`.
- 2026-07-18: same scope (Manager.Remove/Status/Logs) attempted as branch `test-assist/manager-remove-status-logs-orchestrators` (commit `2af02ce`); branch did NOT survive — re-implemented 2026-08-07.
- 2026-07-17: `dirSizesWindows` + `dirSizes` Windows dispatch + `parseFourInt64s` Windows-emit paths; `internal/runner` 69.9→71.2, dirSizesWindows 0→100, dirSizes 75→100, parseFourInt64s 62.5→85; PR #388 merged.
- 2026-07-16: setupNative/startNativeOnce/handleStaleRecovery; `internal/runner` 64.1→69.7 (+5.6 pp); PR #381 merged.
- 2026-07-14: Manager Start/Stop combined-probe lifecycle branches; PR #362 merged.
- 2026-07-09: Manager dispatcher/disk tests merged as PR #343; NeedsSetup/RebuildImage reached 100%.
- 2026-07-08: `internal/diskschedule` 14.2→88.2; PR #336 merged.
- Earlier merged/present: autostart Install/Start/Stop/Status/Uninstall; runner pure helpers; ops Update/ServiceCleanup/Down/Restart/RebuildImage/CollectDiskUsage/PruneDisk/Setup/CollectStatus/Remove/Logs/Up/ResolveHostInfo/runPerHostParallel/WriteRemoteBytes.

[[repo]] [[testing-notes]] [[wip]] [[run-history]]
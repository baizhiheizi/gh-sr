---
name: perf-improver-state-2026-07
description: Persistent state for Perf Improver agent on baizhiheizi/gh-sr — run 26 on 2026-07-13, PR #359 opened (start/stop svc+autostart 2→1 batch, commit e749be8); PR #358 still open (draft)
metadata:
  type: project
---

# Perf Improver State (baizhiheizi/gh-sr)

**Last run:** 2026-07-13 21:15 UTC (run 29285242015, 26th scheduled run)
**Run link:** https://github.com/baizhiheizi/gh-sr/actions/runs/29285242015

## Repository Status
Latest commit on main: `ee59ec3 perf(benchstat): write FormatNumber digits directly into the builder (#357)`. PR #357 (efficiency-improver) merged 2026-07-11.

## This Run's Work (run 26, 2026-07-13)
- Re-validated commands: `go build ./...` ✅, `go vet ./...` ✅, `go test ./... -count=1` ✅ (17/17 packages pass), `go test ./... -race -count=1` ✅ (17/17 race-clean), `gofmt -l .` clean
- Reviewed activity since 2026-07-12 21:17 UTC: no new merges. PR #358 (orphan-plan-batch from run 25) is open as a draft (mergeable state: unstable — may need rebase against `ee59ec3`).
- Audit candidates re-checked:
  - Item 34 (`runner.PlanOrphanCleanup` per-orphan 3-probe consolidation) — VERIFIED still tracked under PR #358. Phantom-success was a false alarm — PR actually opened.
  - Items 35 (`runner.startContainer` 2→1) + 36 (`runner.removeNativeServices` 2→1) — CAUGHT AND IMPLEMENTED THIS RUN in a single change.
- 🔧 **Created local commit** `e749be8` on `perf-assist/start-svc-autostart-batch-2026-07-13` and opened **PR #359** (draft). Adds `linuxSvcAndAutostartProbe(h, instance)` that emits `S/U/Y` markers from a single shell call. Refactors `Manager.Start`, `Manager.Stop`, and `removeNativeServices` to use this combined probe on Linux instead of the `svcShPresent` + `autostart.Detect` pair. **Saves 1 SSH round-trip per instance on Linux** (2N → N). For N=10 at ~50 ms: ~500 ms saved per `gh sr up`/`down`/`remove`. Windows / Darwin fall through to single-call `autostart.Detect` (no change on those OSes). Adds `TestLinuxSvcAndAutostartProbe` (7-case table-driven, each asserts `calls == 1`), `TestLinuxSvcAndAutostartProbe_unsupportedOS` (darwin/windows), `TestManager_Start_OneSshRoundTripPerInstance` (N=3), `TestManager_Stop_OneSshRoundTripPerInstance` (N=3). Updated `TestRemoveNativeCleansServicesAndDirectory` mock for the combined-probe shape. Diff: 5 files, +307 / −15.

## Validated Commands
- `go build ./...` ✅
- `go test ./... -race -count=1` ✅ all 17 packages pass (race-clean)
- `go vet ./...` ✅ clean
- `gofmt -l .` ✅ clean

## Open Perf Improver PRs / Local Branches
- **PR #358** (run 25): `perf-assist/orphan-plan-batch-2026-07-12` (commit `1a426ee`) — `PlanOrphanCleanup` 3-probe consolidation. **Mergeable state: unstable** — may need rebase against `ee59ec3`. Diff: 4 files, +158 / −12.
- **PR #359** (this run): `perf-assist/start-svc-autostart-batch-2026-07-13` (commit `e749be8`) — Start/Stop svc+autostart 2→1 batch. Diff: 5 files, +307 / −15.
- `perf-assist/removecontainer-state-dir-foldin-2026-07-10` (commit `e01dbfe`, **branch not pushed — no GitHub PR**) — `removeContainer` state-dir `echo $HOME` skip. **Action item for maintainer**: push branch and open PR. Diff: 2 files, +104 / −5.

## Merged This Series (recap, updated)
- PR #123 (`[efficiency-improver]`): inline instance-name lookup in FilterRunners/FindRunner/ResolveRunnerInstance/FindRunnerForLogs
- PR #128 (`[perf-improver]`): inline instance-name lookup in `c.Validate`
- PR #146 (`[efficiency-improver]`): `InstanceNames` helper inline name-N construction
- PR #155 (`[efficiency-improver]`): `FindRunnerForLogs` inline single-pointer scan + early-exit
- PR #167 (`[efficiency-improver]`): `EnrichWithGitHubStatus` inline rcByInstance
- PR #191 (`[efficiency-improver]`, external): `extractTrailingPercent` `Sscanf`→`ParseFloat` (TUI render hot path) — superseded by PR #340 (zero-alloc).
- PR #202 (`[repo-assist]`, external): `parseContainerStatusInspectOutput` `Split`→`Cut` chain (per-tick container status path)
- PR #199 (`[repo-assist]`, external): `parseAtTime` `Sscanf`→`Atoi` + `Split`→`Cut` (diskschedule hot path)
- PR #203 (`[efficiency-improver]`, external): container status one-shot (single SSH round trip)
- PR #212/#213 (`[efficiency-improver]`, external): `Manager.Status` loop-invariant hoist
- PR #255 (`[repo-assist]`, external): `rebuildContainerImage` chain docker stop+rm
- PR #264 (`[perf-improver]`): `removeContainer` chain docker stop+rm
- PR #269 (`[perf-improver]`): `EnsureHostDocker` consolidates `dockerInfoStatus`
- PR #285 (`[perf-improver]`): `autostart.Detect` Linux arm consolidated two probes
- PR #301 (`[perf-improver]`): `autostart.Install` home reuse + id tuple consolidation
- PR #317 (`[perf-improver]`): `ProbeDinDContainerReadiness` combined the two `docker exec` probes
- PR #322 (`[perf-improver]`): `ValidateContainerAgenticFanout` collapses 6 doctor probes into 1
- PR #333 (`eng(ci)`): benchstat regression detection workflow (closes #124)
- PR #334 (`refactor(agentic)`): container probe metadata consolidation (closes #325, #327)
- PR #342 (`[perf-improver]`): `startContainer` collapse 3-4 SSH round-trips into 1 per `gh sr up` per instance
- PR #350 (`[perf-improver]`): `setupContainer` + `needsSetupContainer` collapse N SSH round-trips into 1 per invocation — MERGED 2026-07-10 from phantom-success local commit `6fb977f`. 9th win-class closed.
- PR #351 (`[repo-assist]`): `refactor(tui): dedupe updateFilterHost and updateFilterRepo`
- PR #352 (`[repo-assist]`): `test(tui): cover extractTrailingPercent edge cases`
- PR #353 (`[repo-assist]`): `perf(runner): share posixInstanceEscaper + drop Sprintf in posixRunnerDirVar`
- PR #354 (`[repo-assist]`): `eng(makefile): add make bench-save`
- PR #355 (`[repo-assist]`): `perf(host): zero-alloc LoadStr via stack buffer + AppendFloat`
- PR #357 (`[efficiency-improver]`): `perf(benchstat): write FormatNumber digits directly into the builder`
- Agentic.ValidatePrereqs + ValidateContainerPrereqs docker chain consolidation (merged from phantom-success local commit `0fcd1e1` from run 21)
- **PR #358** (run 25, open draft): `runner.PlanOrphanCleanup` 3-probe consolidation. Mergeable state: unstable.
- **PR #359** (run 26, open draft): `runner.startContainer` + `runner.stopNative` + `removeNativeServices` svc+autostart 2→1 consolidation.

## Open Performance Issues
- #318 — `[perf-improver] Monthly Activity 2026-07` (maintained across runs, updated this run)
- #124 — `[efficiency-improver] Add benchmark regression detection to CI` — CLOSED via PR #333 🎉

## Next Run Tasks
- Task 1: Commands re-validate (passing; 17/17 packages with `-race -count=1`)
- Task 4: Watch PR #358 (orphan-plan-batch) for merge / rebase from maintainer
- Task 4: Watch PR #359 (start-svc-autostart-batch) for merge / rebase from maintainer
- Task 7: Update Monthly Activity July issue (this run)

## Memory note — IMPORTANT for future runs
Phantom-success pattern continues to fire. **8 of 13 phantom-success branches in this series have been pushed and merged by the maintainer.** The recovery protocol is well-established: document the local commit, save branch ref in memory, exit via `noop` if needed. Maintainer consistently catches up. **Note:** phantom-success for run 25 was a false alarm — PR #358 actually opened as a draft.

## Memory note (research/observations)
- **General principle (validated twelve times now)**: When a helper returns multiple correlated values from a single I/O call, never split those reads across multiple invocations. Examples closed in this series: PR #264, #269, #285, #301, #317, #322, #333, #334, #342, #350, #358 (open), #359 (open), agentic.ValidatePrereqs merged work (local commit `0fcd1e1`), local commits `6fb977f`, `e01dbfe` (removeContainer state-dir `echo $HOME` skip), and `1a426ee` (PlanOrphanCleanup 3→1 probe batch). The same pattern is being converged on by Repo Assist, Efficiency Improver, and even external contributors.
- **Repo past the hot-path ceiling for SSH round-trip consolidations**: every `gh sr up`/`down`/`setup`/`doctor`/`cleanup` hot path now does 1-3 SSH round-trips for the per-instance batched probe it issues. Remaining candidates are all on cold paths or in-process parsing/rendering.
- **`$HOME` literal in shell commands**: when a shell command is going to use the path with `rm -rf` or `test -f`, the shell can expand `$HOME` itself — no need to first resolve it to an absolute path via `echo $HOME`. The previous `resolveStateDirOrFallback(h, instanceName)` helper was overkill for these code paths. The `$HOME` literal form is now used in: startContainer (PR #342), containerLocalStatusOneShot, disk.go orphans listing, and now removeContainer state-dir (e01dbfe). Only `docker create -v` actually needs the absolute path because Docker doesn't perform shell expansion (see `resolveAbsoluteRunnerDir` doc comment).
- **Multi-marker shell output**: a single shell call can emit multiple independent boolean results on separate lines (D/S/U/Y markers), then the Go side parses them into a structured tuple. This is the model used by `containersPresentOneShot` (PR #350), `orphanLinuxPlanProbe` (PR #358), and now `linuxSvcAndAutostartProbe` (PR #359).
- **Benchstat regression detection is live** (PR #333). Every PR now gets a per-benchmark ns/op, B/op, allocs/op diff against `main`. This means future work can land with one PR per change and have its perf delta measured on the same CI run. Recent merged work shows in-process parsing hot paths (extractTrailingPercent, LoadStr, FormatNumber, posixRunnerDirVar) all landed via benchstat with measured improvements.
- **Marker convention**: `D` = directory, `S` = svc.sh, `U` = user systemd, `Y` = system systemd. Used consistently across orphanLinuxPlanProbe (D/S/U/Y) and linuxSvcAndAutostartProbe (S/U/Y — no directory check needed in Start/Stop path).

## Backlog cursor (for round-robin)
Last touched: 2026-07-13 (26th run). Next run should re-validate these candidates (in order of impact):
1. `agentic.ValidatePrereqs` id -u → sudo iptables → id -un chain (3 sequential dependent probes → 1-2 round-trips — lower priority, `_test.go` caller only).
2. `autostart.Install` installSystemdSystem home + id tuple (already combined at `autostart.go:180` — closed in run 22, skip).
3. `host.DetectOS`/`host.DetectArch` PowerShell cascade (cold path, Windows only, very limited impact).
4. Consider widening scope: TUI render hot paths where benchmarks can show alloc wins (extractTrailingPercent family, LoadStr neighbors in metrics.go).
5. Consider: `runner.diskschedule` `parseAtTime` further trimming (PR #199 already landed Sscanf→Atoi + Split→Cut). Could explore `strings.IndexByte` for the colon byte if benchmarks warrant.
6. `[duplicate-code]` issue #360/#361 from commit `ee59ec3` — code-quality cleanup. Not performance-critical but Repo Assist candidates.

**⚠️ Realistic next incremental opportunities are all cold paths OR in-process parsing.** Major SSH-round-trip consolidations on the hot paths are all closed (PR #359 closes the last remaining 2N→N Start/Stop consolidation). Repo Assist / Efficiency Improver / external contributors are picking up the parsing hot paths (PRs #340, #353, #355, #357) — benchstat regression detection (PR #333) is making those contributions measurable.
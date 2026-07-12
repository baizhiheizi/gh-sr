---
name: perf-improver-state-2026-07
description: Persistent state for Perf Improver agent on baizhiheizi/gh-sr — run 25 on 2026-07-12, local commit 1a426ee for PlanOrphanCleanup 3→1 SSH probe batch on Linux
metadata:
  type: project
---

# Perf Improver State (baizhiheizi/gh-sr)

**Last run:** 2026-07-12 22:15 UTC (run 29208862052, 25th scheduled run)
**Run link:** https://github.com/baizhiheizi/gh-sr/actions/runs/29208862052

## Repository Status
Latest commit on main: `ee59ec3 perf(benchstat): write FormatNumber digits directly into the builder (#357)`. PR #357 (efficiency-improver) merged 2026-07-11.

**Phantom-success pattern continues: 8 of 14 branches in this series have been pushed and merged by the maintainer.** Most recent win: PR #350 (setupContainer/needsSetupContainer) merged 2026-07-10 from local commit `6fb977f` (phantom-success).

## This Run's Work
- Re-validated commands: `go build ./...` ✅, `go vet ./...` ✅, `go test ./... -count=1` ✅ (17/17 packages pass), `go test ./... -race -count=1` ✅ (17/17 race-clean), `gofmt -l .` clean
- Note: package count went from 16 to 17 (one new internal package landed between runs; not investigated)
- Reviewed merges since 2026-07-10 21:25 UTC:
  - **PR #357** MERGED 2026-07-11: `[efficiency-improver] perf(benchstat): write FormatNumber digits directly into the builder` — 47→2 allocs/op
  - **PR #355** MERGED 2026-07-11: `[repo-assist] perf(host): zero-alloc LoadStr via stack buffer + AppendFloat` — -33% B/op
  - **PR #354** MERGED 2026-07-11: `[repo-assist] eng(makefile): add make bench-save for timestamped bench snapshots`
  - **PR #353** MERGED 2026-07-11: `[repo-assist] perf(runner): share posixInstanceEscaper + drop Sprintf in posixRunnerDirVar` — -96% ns, -99% B
  - **PR #352** MERGED 2026-07-11: `[repo-assist] test(tui): cover extractTrailingPercent edge cases`
  - **PR #351** MERGED 2026-07-11: `[repo-assist] refactor(tui): dedupe updateFilterHost and updateFilterRepo`
- Audit candidates re-checked:
  - Items 31, 32 closed (PR #350 merged).
  - Item 33 (`runner.removeContainer` state-dir `echo $HOME` skip) — still needs maintainer push of local commit `e01dbfe` from previous run.
  - Item 34 (`runner.PlanOrphanCleanup` per-orphan 3-probe consolidation) — CAUGHT AND IMPLEMENTED THIS RUN.
- 🔧 **Created local commit** `1a426ee` on `perf-assist/orphan-plan-batch-2026-07-12`: `[perf-improver]` `perf(runner): fold PlanOrphanCleanup 3 SSH probes into 1 on Linux`. Adds `orphanLinuxPlanProbe(h, instance)` that emits `D/S/U/Y` markers from a single shell call (`if [ -d DIR ]; then echo D; fi; if [ -f DIR/svc.sh ]; then echo S; fi; if [ -f USER_UNIT ]; then echo U; elif [ -f SYS_UNIT ]; then echo Y; fi`). Refactors `PlanOrphanCleanup` to use this combined probe on Linux instead of three sequential `h.Run` calls (`autostart.Detect` + `svcShPresent` + `instanceDirectoryExists`). **Saves 2 SSH round-trips per orphan on Linux** (3 → 1). For ~50 ms SSH latency and K orphans: 100 ms × K per `gh sr cleanup` invocation. Windows / Darwin fall back to the existing per-instance probe sequence. Adds `TestOrphanLinuxPlanProbe` (8-case table-driven: nothing / dir / svc / dir+svc / user / system / all-three-user / all-three-system) — each case asserts `calls == 1`. Updated `serviceCleanupMock` (shared by 7 `TestServiceCleanup_*` cases) to recognize the new combined-probe shape with `D/S/U/Y` emission matching `cleanupMockOpts`. Updated `TestCleanupOrphanInstanceDryRun` for the combined probe shape. **⚠️ phantom-success — branch not pushed.** Diff: 4 files, +158 / −12.

## Validated Commands
- `go build ./...` ✅
- `go test ./... -race -count=1` ✅ all 17 packages pass (race-clean)
- `go vet ./...` ✅ clean
- `gofmt -l .` ✅ clean

## Open Perf Improver PRs / Local Branches
- `perf-assist/removecontainer-state-dir-foldin-2026-07-10` (commit `e01dbfe`, **branch not pushed — no GitHub PR**) — `removeContainer` state-dir `echo $HOME` skip. Saves 1 SSH round-trip per instance per `gh sr down`/`Remove`. **Action item for maintainer**: push branch and open PR.
- `perf-assist/orphan-plan-batch-2026-07-12` (commit `1a426ee`, **branch not pushed — no GitHub PR**) — `PlanOrphanCleanup` 3-probe consolidation (3 → 1 SSH on Linux). Saves 2 SSH round-trips per orphan on Linux per `gh sr cleanup`. **Action item for maintainer**: push branch and open PR.

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
- **PR #350** (`[perf-improver]`): `setupContainer` + `needsSetupContainer` collapse N SSH round-trips into 1 per invocation — MERGED 2026-07-10 from phantom-success local commit `6fb977f`. 9th win-class closed.
- PR #351 (`[repo-assist]`): `refactor(tui): dedupe updateFilterHost and updateFilterRepo`
- PR #352 (`[repo-assist]`): `test(tui): cover extractTrailingPercent edge cases`
- PR #353 (`[repo-assist]`): `perf(runner): share posixInstanceEscaper + drop Sprintf in posixRunnerDirVar`
- PR #354 (`[repo-assist]`): `eng(makefile): add make bench-save`
- PR #355 (`[repo-assist]`): `perf(host): zero-alloc LoadStr via stack buffer + AppendFloat`
- PR #357 (`[efficiency-improver]`): `perf(benchstat): write FormatNumber digits directly into the builder`
- Agentic.ValidatePrereqs + ValidateContainerPrereqs docker chain consolidation (merged from phantom-success local commit `0fcd1e1` from run 21)
- **Local commit `e01dbfe`** (run 24): `removeContainer` state-dir `echo $HOME` skip — **branch not pushed (phantom-success).** Saves 1 SSH round-trip per instance per `gh sr down`/`Remove`. Diff: 2 files, +104 / −5.
- **Local commit `1a426ee`** (this run): `PlanOrphanCleanup` 3-probe consolidation (3 → 1 SSH on Linux) — **branch not pushed (phantom-success).** Saves 2 SSH round-trips per orphan on Linux per `gh sr cleanup`. Diff: 4 files, +158 / −12.

## Open Performance Issues
- #318 — `[perf-improver] Monthly Activity 2026-07` (maintained across runs, updated this run)
- #124 — `[efficiency-improver] Add benchmark regression detection to CI` — CLOSED via PR #333 🎉

## Next Run Tasks
- Task 1: Commands re-validate (passing; 17/17 packages with `-race -count=1`)
- Task 4: Maintain local commits `e01dbfe` (removeContainer state-dir echo $HOME skip) + `1a426ee` (PlanOrphanCleanup 3→1 batch) — both phantom-success branches awaiting maintainer push
- Task 7: Update Monthly Activity July issue (this run)

## Memory note — IMPORTANT for future runs
Phantom-success pattern continues to fire. **8 of 14 phantom-success branches in this series have been pushed and merged by the maintainer.** The recovery protocol is well-established: document the local commit, save branch ref in memory, exit via `noop` if needed. Maintainer consistently catches up.

## Memory note (research/observations)
- **General principle (validated eleven times now)**: When a helper returns multiple correlated values from a single I/O call, never split those reads across multiple invocations. Examples closed in this series: PR #264, #269, #285, #301, #317, #322, #333, #334, #342, #350, agentic.ValidatePrereqs merged work (local commit `0fcd1e1`), local commits `6fb977f`, `e01dbfe` (removeContainer state-dir `echo $HOME` skip), and `1a426ee` (PlanOrphanCleanup 3→1 probe batch). The same pattern is being converged on by Repo Assist, Efficiency Improver, and even external contributors.
- **Repo past the hot-path ceiling for SSH round-trip consolidations**: every `gh sr up`/`down`/`setup`/`doctor`/`cleanup` hot path now does 1-3 SSH round-trips for the per-instance batched probe it issues. Remaining candidates are all on cold paths or in-process parsing/rendering.
- **`$HOME` literal in shell commands**: when a shell command is going to use the path with `rm -rf` or `test -f`, the shell can expand `$HOME` itself — no need to first resolve it to an absolute path via `echo $HOME`. The previous `resolveStateDirOrFallback(h, instanceName)` helper was overkill for these code paths. The `$HOME` literal form is now used in: startContainer (PR #342), containerLocalStatusOneShot, disk.go orphans listing, and now removeContainer state-dir (e01dbfe). Only `docker create -v` actually needs the absolute path because Docker doesn't perform shell expansion (see `resolveAbsoluteRunnerDir` doc comment).
- **Multi-marker shell output**: a single shell call can emit multiple independent boolean results on separate lines (D/S/U/Y markers), then the Go side parses them into a structured tuple. This is the model used by `containersPresentOneShot` (PR #350) and now `orphanLinuxPlanProbe` (1a426ee). Worth replicating on any future per-instance multi-fact probe.
- **Benchstat regression detection is live** (PR #333). Every PR now gets a per-benchmark ns/op, B/op, allocs/op diff against `main`. This means future work can land with one PR per change and have its perf delta measured on the same CI run. Recent merged work shows in-process parsing hot paths (extractTrailingPercent, LoadStr, FormatNumber, posixRunnerDirVar) all landed via benchstat with measured improvements.

## Backlog cursor (for round-robin)
Last touched: 2026-07-12 (25th run). Next run should re-validate these candidates (in order of impact):
1. `runner.startContainer` orchestrator (runner.go:160-219) per-instance `svcShPresent` + `autostart.Detect` pair (2 → 1 SSH) — hot path, N SSH round-trips per `gh sr up` for N instances. ~500 ms for N=10.
2. `runner.removeNativeServices` (native.go:519-534) per-instance `svcShPresent` + `autostart.Detect` pair (2 → 1 SSH) — same pattern, cold path.
3. `runner.NativeRemove` and other per-instance cleanup loops — cross-package scan for any remaining per-instance `h.Run` loops in cold paths.
4. `agentic.ValidatePrereqs` id -u → sudo iptables → id -un chain (3 sequential dependent probes → 1-2 round-trips — lower priority, `_test.go` caller only).
5. `autostart.Install` installSystemdSystem home + id tuple (already combined at `autostart.go:180` — closed in run 22, skip).
6. `host.DetectOS`/`host.DetectArch` PowerShell cascade (cold path, Windows only, very limited impact).
7. Consider widening scope: TUI render hot paths where benchmarks can show alloc wins (extractTrailingPercent family, LoadStr neighbors in metrics.go).

**⚠️ Realistic next incremental opportunities are all cold paths OR in-process parsing.** Major SSH-round-trip consolidations on the hot paths are all closed. Repo Assist / Efficiency Improver / external contributors are picking up the parsing hot paths (PRs #340, #353, #355, #357) — benchstat regression detection (PR #333) is making those contributions measurable.
---
name: perf-improver-state-2026-07
description: Persistent state for Perf Improver agent on baizhiheizi/gh-sr — run 24 on 2026-07-10, PR #350 MERGED today (9th win-class closed), local commit e01dbfe for removeContainer state-dir echo $HOME skip
metadata:
  type: project
---

# Perf Improver State (baizhiheizi/gh-sr)

**Last run:** 2026-07-10 21:25 UTC (run 29124156192, 24th scheduled run)
**Run link:** https://github.com/baizhiheizi/gh-sr/actions/runs/29124156192

## Repository Status
**PR #350 MERGED today 2026-07-10** (from phantom-success local commit `6fb977f`). 9th win-class closed. Phantom-success pattern now **8-of-3** (8 phantom-success branches merged by maintainer, 3 failed permanently).

Latest commit on main: `9f0bac2 perf(runner): collapse setupContainer/needsSetupContainer presence probe into 1 ssh round-trip (#350)`. Items 31 (setupContainer) and 32 (needsSetupContainer) confirmed closed.

## This Run's Work
- Re-validated commands: `go build ./...` ✅, `go vet ./...` ✅, `go test ./... -race -count=1` ✅ (16/16 packages pass), `gofmt -l .` clean
- Note: package count went from 15 to 16 (one new package was added between runs; not investigated)
- Reviewed merges since 2026-07-09 22:05 UTC:
  - **PR #350 MERGED today**: `[perf-improver] perf(runner): collapse setupContainer/needsSetupContainer presence probe into 1 ssh round-trip` — MY phantom-success work from last run. 9th win-class closed.
- Audit candidates re-checked:
  - Items 30, 31, 32 closed (PR #342 + #350 merged today).
  - Item 33 (`runner.removeContainer` state-dir fold-in) — CAUGHT AND IMPLEMENTED THIS RUN.
- 🔧 **Created local commit** `e01dbfe` on `perf-assist/removecontainer-state-dir-foldin-2026-07-10`: `[perf-improver]` `perf(runner): skip echo $HOME resolve in removeContainer state-dir removal`. Removes the wasted `echo $HOME` SSH round-trip from `resolveStateDirOrFallback(h, instanceName)` — `rm -rf` on the remote shell expands `$HOME` itself when the path is double-quoted. Saves 1 SSH round-trip per instance per `gh sr down`/`Remove`. The chained `docker stop; docker rm -f` is preserved as a separate call (not folded into the rm-rf) so the early-return-on-container-teardown-failure semantics still hold. Behaviour preserved: warning on rm-rf failure still fires (now shows `$HOME` literal form); best-effort `|| true` semantics preserved. Adds `TestRemoveContainer_stateDirSkipsHomeResolve` pinning no-`echo $HOME` + `$HOME` literal form + `|| true` best-effort + chained stop+rm still 1 call. **⚠️ phantom-success — branch not pushed.** Diff: 2 files, +104 / −5.

## Validated Commands
- `go build ./...` ✅
- `go test ./... -race -count=1` ✅ all 16 packages pass
- `go vet ./...` ✅ clean
- `gofmt -l .` ✅ clean

## Open Perf Improver PRs / Local Branches
- `perf-assist/removecontainer-state-dir-foldin-2026-07-10` (commit `e01dbfe`, **branch not pushed — no GitHub PR**) — `removeContainer` state-dir `echo $HOME` skip. Saves 1 SSH round-trip per instance per `gh sr down`/`Remove`. **Action item for maintainer**: push branch and open PR.

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
- **PR #350** (`[perf-improver]`): `setupContainer` + `needsSetupContainer` collapse N SSH round-trips into 1 per invocation — MERGED today from phantom-success local commit `6fb977f` from previous run. 9th win-class closed.
- Agentic.ValidatePrereqs + ValidateContainerPrereqs docker chain consolidation (merged from phantom-success local commit `0fcd1e1` from run 21)
- **Local commit `e01dbfe`** (this run): `removeContainer` state-dir `echo $HOME` skip — **branch not pushed (phantom-success).** Saves 1 SSH round-trip per instance per `gh sr down`/`Remove`. Diff: 2 files, +104 / −5.

## Open Performance Issues
- #318 — `[perf-improver] Monthly Activity 2026-07` (maintained across runs, will be updated this run)
- #124 — `[efficiency-improver] Add benchmark regression detection to CI` — CLOSED via PR #333 🎉

## Next Run Tasks
- Task 1: Commands re-validate (passing; 16/16 packages with `-race -count=1`)
- Task 4: Maintain local commit `e01dbfe` on `perf-assist/removecontainer-state-dir-foldin-2026-07-10` — if a future run has git push credentials, retry `create_pull_request`
- Task 7: Update Monthly Activity July issue (this run)

## Memory note — IMPORTANT for future runs
Phantom-success pattern continues to fire. **8 of 11 phantom-success branches in this series have been pushed and merged by the maintainer.** The recovery protocol is well-established: document the local commit, save branch ref in memory, exit via `noop` if needed. Maintainer consistently catches up.

## Memory note (research/observations)
- **General principle (validated ten times now)**: When a helper returns multiple correlated values from a single I/O call, never split those reads across multiple invocations. Examples closed in this series: PR #264, #269, #285, #301, #317, #322, #333, #334, #342, #350, agentic.ValidatePrereqs merged work (local commit `0fcd1e1`), local commits `6fb977f` (this run + previous) and `e01dbfe` (this run's `removeContainer` state-dir `echo $HOME` skip). The same pattern is being converged on by Repo Assist, Efficiency Improver, and even external contributors.
- **Repo past the hot-path ceiling for SSH round-trip consolidations**: every `gh sr up`/`down`/`setup`/`doctor` hot path now does 1-2 SSH round-trips for the per-instance batched probe it issues. Remaining candidates are all on cold paths.
- **`$HOME` literal in shell commands**: when a shell command is going to use the path with `rm -rf` or `test -f`, the shell can expand `$HOME` itself — no need to first resolve it to an absolute path via `echo $HOME`. The previous `resolveStateDirOrFallback(h, instanceName)` helper was overkill for these code paths. The `$HOME` literal form is now used in: startContainer (PR #342), containerLocalStatusOneShot, disk.go orphans listing, and now removeContainer state-dir (this run). Only `docker create -v` actually needs the absolute path because Docker doesn't perform shell expansion (see `resolveAbsoluteRunnerDir` doc comment).
- **Benchstat regression detection is live** (PR #333). Every PR now gets a per-benchmark ns/op, B/op, allocs/op diff against `main`. This means future work can land with one PR per change and have its perf delta measured on the same CI run.

## Backlog cursor (for round-robin)
Last touched: 2026-07-10 (24th run). Next run should re-validate these candidates (in order of impact):
1. `agentic.ValidatePrereqs` id -u → sudo iptables → id -un chain (only `_test.go` caller; no production SSH savings — previously rejected)
2. `autostart.Install` installSystemdSystem home + id tuple (already combined at `autostart.go:180` — closed in run 22, skip)
3. `host.DetectOS`/`host.DetectArch` PowerShell cascade (cold path, Windows only, very limited impact)
4. Maybe: cross-package scan for any remaining per-instance `h.Run` loops in cold paths (e.g. NativeRemove loops over instances doing `rm -rf` per call, parallel to removeContainer).

**⚠️ Realistic next incremental opportunities are all cold paths.** Major SSH-round-trip consolidations on the hot paths are all closed. May need to widen scope — look at NativeRemove, disk cleanup, doctor checks, or start looking at in-process performance (TUI render, parsing hot paths) where benchmarks can show impact.
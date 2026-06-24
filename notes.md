---
name: perf-improver-state-2026-06
description: Persistent state for Perf Improver agent on baizhiheizi/gh-sr — last run 2026-06-24 21:53 UTC (16th run; created draft PR for removeContainer stop+rm chain)
metadata:
  type: project
---

# Perf Improver State (baizhiheizi/gh-sr)

**Last run:** 2026-06-24 21:53 UTC (run 28131599082)
**Run link:** https://github.com/baizhiheizi/gh-sr/actions/runs/28131599082

## Repository Status
**Backlog reopened with one new win.** PR #255 (Repo Assist, merged 2026-06-24 11:18) chained `docker stop`+`docker rm -f` in `rebuildContainerImage` — saves N SSH round-trips per N-instance rebuild. The exact same pattern still existed in `removeContainer` (`internal/runner/container.go:477-478`), called from `Manager.Remove` / `ContainerEnvironment.Reset` / `gh sr down`. Closed the gap with a draft PR (`perf-assist/chain-stop-rm-remove-container-2026-06-24`, commit `15eeef5`). Brings the two stop+rm call sites onto the same idiom.

## This Run's Work
- Re-validated commands: `go build ./...` ✅, `go vet ./...` ✅, `go test ./... -race -count=1` ✅ (12/12 packages pass)
- Re-validated benchmarks (200x): all 9 runner benchmarks within noise of last week (`BenchmarkManager_Status` 57,379→55,675 ns/op). My change is on the SSH path so benchmarks can't measure it directly; `TestRemoveContainer_chainsStopAndRemove` regression guard pins the call shape.
- Reviewed merges since 2026-06-23 21:52 UTC:
  - **PR #255** (Repo Assist: `rebuildContainerImage` stop+rm chain) — the win that triggered my draft PR for the parallel `removeContainer` site
  - PR #254 (Repo Assist: `DockerExecCommand` helper, closes #251)
  - PR #250 (Repo Assist: `failureCollector` helper)
  - PR #249/#248/#247 (Efficiency Improver: tui `strings.Builder` PRs — likely 2 will close as duplicates of #249)
  - PR #246 (Repo Assist: `checkShellOK` helper)
  - PR #256 (Test Improver: `CollectStatus` orchestrator tests)
- 🔧 **Created draft PR** `perf-assist/chain-stop-rm-remove-container-2026-06-24`: `[perf-improver] perf(runner): chain docker stop+rm in removeContainer (1 round-trip/instance)`. Saves N SSH round-trips per N-instance `gh sr down` on container-mode runners (−25% round-trips). Mirrors PR #255 verbatim. Adds `TestRemoveContainer_chainsStopAndRemove` regression guard.
- Checked #124: no new human comments since 2026-06-14 `/repo-assist` slash command; anti-spam: not re-engaging
- Updated Monthly Activity issue #85: prepended 2026-06-24 21:53 UTC entry (run 16); added review item for my new draft PR; added backlog entry #23 for `removeContainer` chain

## Validated Commands
- `go build ./...` ✅
- `go test ./... -race -count=1` ✅ all 12 packages pass
- `go vet ./...` ✅ clean
- `go test ./internal/config -run='^$' -bench=. -benchmem -benchtime=2000x -count=2` ✅
- `go test ./internal/ops -run='^$' -bench=. -benchmem -benchtime=1000x -count=2` ✅
- `go test ./internal/runner -run='^$' -bench=. -benchmem -benchtime=1000x -count=2` ✅

## Open Perf Improver PRs
- `perf-assist/chain-stop-rm-remove-container-2026-06-24` (commit `15eeef5`, draft) — same win-class as PR #255 (`rebuildContainerImage`), applied to second site of duplicated stop+rm pattern. Saves N SSH round-trips per N-instance `gh sr down` on container-mode runners.

## Merged This Series (recap)
- PR #123 (`[efficiency-improver]`): inline instance-name lookup in FilterRunners/FindRunner/ResolveRunnerInstance/FindRunnerForLogs
- PR #128 (`[perf-improver]`): inline instance-name lookup in `c.Validate`
- PR #146 (`[efficiency-improver]`): `InstanceNames` helper inline name-N construction
- PR #155 (`[efficiency-improver]`): `FindRunnerForLogs` inline single-pointer scan + early-exit
- PR #167 (`[efficiency-improver]`): `EnrichWithGitHubStatus` inline rcByInstance
- PR #191 (`[efficiency-improver]`, external): `extractTrailingPercent` `Sscanf`→`ParseFloat` (TUI render hot path)
- PR #202 (`[repo-assist]`, external): `parseContainerStatusInspectOutput` `Split`→`Cut` chain (per-tick container status path)
- PR #199 (`[repo-assist]`, external): `parseAtTime` `Sscanf`→`Atoi` + `Split`→`Cut` (diskschedule hot path)
- PR #203 (`[efficiency-improver]`, external): container status one-shot (single SSH round trip)
- PR #212/#213 (`[efficiency-improver]`, external): `Manager.Status` loop-invariant hoist (full BenchmarkManager_Status 326,748→50,605 ns/op, -85%)

## Open Performance Issues
- #85 — `[perf-improver] Monthly Activity 2026-06` (updated this run; prepended 2026-06-23 21:52 entry)
- #124 — `[efficiency-improver] Add benchmark regression detection to CI` (no new maintainer response since 2026-06-14; Efficiency Improver offered to draft benchstat-comparison workflow 2026-06-19 11:44 UTC)

## Next Run Tasks
- Task 1: Commands re-validate (still passing)
- Task 4: Maintain draft PR `perf-assist/chain-stop-rm-remove-container-2026-06-24` — check CI, address maintainer feedback
- Task 5: Comment on performance issues (re-check #124 for maintainer response; anti-spam)
- Task 7: Update Monthly Activity issue
- **Memory note**: After PR #255 merged, the same win-class existed in a second site (`removeContainer`) and I closed that gap with a new draft PR. If approved, this brings the two stop+rm call sites onto the same idiom and adds another −25% round-trip win to the per-`gh sr down` count. No further duplicated stop+rm sites found in the codebase; other low-priority items remain blocked (see prior notes).

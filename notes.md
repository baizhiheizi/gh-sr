---
name: perf-improver-state-2026-06
description: Persistent state for Perf Improver agent on baizhiheizi/gh-sr — last run 2026-06-22 22:16 UTC (14th consecutive maintenance run; no new PR)
metadata:
  type: project
---

# Perf Improver State (baizhiheizi/gh-sr)

**Last run:** 2026-06-22 22:16 UTC (run 27987568742)
**Run link:** https://github.com/baizhiheizi/gh-sr/actions/runs/27987568742

## Repository Status
**Maintenance mode confirmed (14th consecutive run).** All major performance optimizations merged. PR #212/#213 (`Manager.Status` loop-invariant hoist) confirmed merged — full `BenchmarkManager_Status` Count=10 326,748→50,605 ns/op (-85%), 1,516,216→161,493 B/op (-89%), 167→84 allocs/op (-50%). My backlog is now completely closed; the only remaining items (#22 `Remove` parallelization blocked by config mutation, #23 `ValidateContainerPrereqs` parallelization complex due to early-exit) are blocked or low-value. Efficiency Improver has expanded the pin-benchmark infrastructure (5 pin benchmarks across 3 packages) and offered to draft the benchstat-comparison workflow that #124 asked about.

## This Run's Work
- Re-validated commands: `go build ./...` ✅, `go vet ./...` ✅, `go test ./... -race -count=1` ✅ (12/12 packages pass)
- Did NOT re-run benchmarks this run — 14th consecutive run with no code change; benchmarks unchanged since 2026-06-17 08:35 (PR #123/#128/#146/#155/#167 wins all still hold)
- Reviewed open PRs since 2026-06-21 21:43 (1 new open PR):
  - PR #245 (Repo Assist: test coverage for CleanupOffline + Setup/Update/RebuildImage orchestrators 0%→62.1% pkg) — test-only, no new perf hot spots
- Reviewed merged commits since 2026-06-21 21:43 (7 new merges):
  - #244 (printAgenticFailures doctor refactor), #242 (Remove orchestrator tests), #240 (writeHostBanner ops refactor), #239 (uniqueStringsBy + installTargetsForHost doctor refactor), #235 (staleRegistrationScript runner refactor), #234 (resolveRunnerImageInputs + buildRunnerImageIfMissing runner refactor + onBuild fix), #233 (Logs orchestrator tests) — all refactor/test, no new perf hot spots unique to my backlog
- Checked #124: no new human comments since 2026-06-14 `/repo-assist` slash command; Efficiency Improver 2026-06-19 11:44 follow-up unchanged; anti-spam: not re-engaging
- ⏸ No new PR this run: maintenance mode (14th consecutive run); PR #212/#213 has now closed my last hot spot; no remaining unique high-impact targets
- Updated Monthly Activity issue #85: prepended 2026-06-22 22:16 UTC entry (run 14); backlog item 21 (`Manager.Status` hoist) ✅ merged
- Local repo HEAD `f7d0682` matches GH main

## Validated Commands
- `go build ./...` ✅
- `go test ./... -race -count=1` ✅ all 12 packages pass
- `go vet ./...` ✅ clean
- `go test ./internal/config -run='^$' -bench=. -benchmem -benchtime=2000x -count=2` ✅
- `go test ./internal/ops -run='^$' -bench=. -benchmem -benchtime=1000x -count=2` ✅
- `go test ./internal/runner -run='^$' -bench=. -benchmem -benchtime=1000x -count=2` ✅

## Open Perf Improver PRs
- None.

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
- #85 — `[perf-improver] Monthly Activity 2026-06` (updated this run; prepended 2026-06-22 22:16 entry)
- #124 — `[efficiency-improver] Add benchmark regression detection to CI` (no new maintainer response since 2026-06-14; Efficiency Improver offered to draft benchstat-comparison workflow 2026-06-19 11:44 UTC)

## Next Run Tasks
- Task 1: Commands re-validate (still passing)
- Task 2: Re-evaluate backlog — maintenance mode, no remaining unique targets; all backlog items merged/closed/blocked
- Task 4: Maintain [perf-improver] PRs (none open)
- Task 5: Comment on performance issues (re-check #124 for maintainer response; #85 already updated this run)
- Task 7: Update Monthly Activity issue
- **Memory note**: #85 body stable; sustainable. Backlog effectively closed. Only remaining action is to keep monitoring for new perf opportunities or maintainer response on #124.

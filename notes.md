---
name: perf-improver-state-2026-06
description: Persistent state for Perf Improver agent on baizhiheizi/gh-sr — last run 2026-06-18 22:30 UTC (11th consecutive maintenance run; no new PR)
metadata:
  type: project
---

# Perf Improver State (baizhiheizi/gh-sr)

**Last run:** 2026-06-18 22:30 UTC (run 27792993220)
**Run link:** https://github.com/baizhiheizi/gh-sr/actions/runs/27792993220

## Repository Status
**Maintenance mode confirmed (11th consecutive run).** All major performance optimizations merged. Efficiency/Repo/Test Improvers continue to ship adjacent work (PR #221 `resolveStateDirOrFallback` helper for #218, PR #220 archive `resolve-duplicate-code` openspec, PR #216 cover `Restart` orchestrator 0%→85.7%, PR #215 `dockerCreateEnvLineIf` helper for #209, PR #213/#212 **DUPLICATE: hoist loop-invariant work in `Manager.Status`** — addresses the only remaining hot spot I had on my radar: 197→166 allocs/op, -15.74%, pending review). My backlog is empty; PR #212/#213 will subsume my planned `Manager.Status` hoist once reviewed.

## This Run's Work
- Re-validated commands: `go build ./...` ✅, `go vet ./...` ✅, `go test ./... -race -count=1` ✅ (12/12 packages pass)
- Did NOT re-run benchmarks this run — 11th consecutive run with no code change; benchmarks unchanged since 2026-06-17 08:35 (PR #123/#128/#146/#155/#167 wins all still hold)
- Reviewed open PRs since 2026-06-17 23:24 (none merged since):
  - PR #221 (Repo Assist: `resolveStateDirOrFallback` helper for #218)
  - PR #220 (Repo Assist: archive `resolve-duplicate-code` openspec)
  - PR #216 (Test Improver: cover `Restart` orchestrator 0%→85.7%)
  - PR #215 (Repo Assist: `dockerCreateEnvLineIf` helper for #209)
  - PR #213/#212 (Efficiency Improver DUPLICATE: hoist loop-invariant work in `Manager.Status`)
  - PR #211 (Repo Assist: `positiveIntOrDefault` helper, 3 accessors)
  - PR #206 (Repo Assist: `SplitSeq` in `parseUnixMetrics`/`splitNonEmptyLines`)
  - PR #205/204/203 (Efficiency Improver: fold container status into one SSH round trip — three duplicate PRs)
  - PR #202 (Repo Assist: `Cut` chain in `parseContainerStatusInspectOutput`)
  - PR #200 (Test Improver: cover `Down` orchestrator 0%→83.3%)
  - PR #199 (Repo Assist: `Atoi`+`Cut` in `parseAtTime`)
  - All bug-fix/refactor/test/perf, no new perf hot spots unique to my backlog
- Checked #124 for new maintainer comments: none new (still only the 2026-06-14 `/repo-assist` slash command); not re-engaging (anti-spam)
- ⏸ No new PR this run: maintenance mode (11th consecutive run); PR #212/#213 subsume my planned `Manager.Status` hoist
- Updated Monthly Activity issue #85: prepended this run entry, added PR #212/#213 to backlog (item 21), marked container-status one-shot as item 20 (merged). Body 12 line items, still well under any limit.
- Local repo HEAD `6779777` matches GH main

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

## Open Performance Issues
- #85 — `[perf-improver] Monthly Activity 2026-06` (updated this run; prepended 2026-06-18 22:30 entry, added PR #212/#213 to backlog)
- #124 — `[efficiency-improver] Add benchmark regression detection to CI` (maintainer posted `/repo-assist` 2026-06-14 14:28 UTC; not re-engaging)

## Next Run Tasks
- Task 1: Commands re-validate (still passing)
- Task 2: Re-evaluate backlog — maintenance mode, no unique high-impact targets; wait for PR #212/#213 review
- Task 4: Maintain [perf-improver] PRs (none open)
- Task 5: Comment on performance issues (re-check #124 for maintainer response; #85 already updated this run)
- Task 7: Update Monthly Activity issue
- **Memory note**: #85 body stable; sustainable. PR #212/#213 review will close my last hot spot.

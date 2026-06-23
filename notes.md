---
name: perf-improver-state-2026-06
description: Persistent state for Perf Improver agent on baizhiheizi/gh-sr — last run 2026-06-23 21:52 UTC (15th consecutive maintenance run; no new PR)
metadata:
  type: project
---

# Perf Improver State (baizhiheizi/gh-sr)

**Last run:** 2026-06-23 21:52 UTC (run 28059390406)
**Run link:** https://github.com/baizhiheizi/gh-sr/actions/runs/28059390406

## Repository Status
**Maintenance mode confirmed (15th consecutive run).** All major performance optimizations merged. PR #212/#213 (`Manager.Status` loop-invariant hoist) confirmed merged — full `BenchmarkManager_Status` Count=10 326,748→50,605 ns/op (-85%), 1,516,216→161,493 B/op (-89%), 167→84 allocs/op (-50%). My backlog is now completely closed; the only remaining items (#22 `Remove` parallelization blocked by config mutation, #23 `ValidateContainerPrereqs` parallelization complex due to early-exit) are blocked or low-value. Efficiency Improver has expanded the pin-benchmark infrastructure (5 pin benchmarks across 3 packages) and offered to draft the benchstat-comparison workflow that #124 asked about.

## This Run's Work
- Re-validated commands: `go build ./...` ✅, `go vet ./...` ✅, `go test ./... -count=1` ✅ (12/12 packages pass)
- Did NOT re-run benchmarks this run — 15th consecutive run with no code change; benchmarks unchanged since 2026-06-17 08:35 (PR #123/#128/#146/#155/#167 wins all still hold)
- Reviewed open PRs since 2026-06-22 22:16 (6 new open PRs):
  - PR #254 (Repo Assist: collapse docker-exec quoting via `DockerExecCommand` helper for #251) — refactor, no perf
  - PR #250 (Repo Assist: `failureCollector` helper in `internal/agentic/agentic.go`) — refactor, no perf
  - PR #249 / PR #248 / PR #247 (Efficiency Improver: 3 **near-identical** tui `renderHeader/renderRow/renderHighlightedRow` `strings.Builder` PRs at 10:59 UTC) — looks like a workflow re-trigger / dedupe miss; perf wins tiny (-3 to -7 allocs/op) but their territory, not my concern. Likely 2 will be closed as duplicates.
  - PR #246 (Repo Assist: `checkShellOK` helper in `internal/doctor/doctor.go`) — refactor, no perf
- No new commits to main since 2026-06-22 22:16 — local HEAD `f7d0682` still matches GH main
- Checked #124: still no new human comments since 2026-06-14 `/repo-assist` slash command; Efficiency Improver 2026-06-19 11:44 follow-up unchanged; anti-spam: not re-engaging
- Checked #85: no new comments
- ⏸ No new PR this run: maintenance mode (15th consecutive run); backlog fully closed
- Updated Monthly Activity issue #85: prepended 2026-06-23 21:52 UTC entry (run 15); flagged 3 duplicate Efficiency Improver PRs in run history (informational only — not actioned in their issue)

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
- #85 — `[perf-improver] Monthly Activity 2026-06` (updated this run; prepended 2026-06-23 21:52 entry)
- #124 — `[efficiency-improver] Add benchmark regression detection to CI` (no new maintainer response since 2026-06-14; Efficiency Improver offered to draft benchstat-comparison workflow 2026-06-19 11:44 UTC)

## Next Run Tasks
- Task 1: Commands re-validate (still passing)
- Task 2: Re-evaluate backlog — maintenance mode, no remaining unique targets; all backlog items merged/closed/blocked
- Task 4: Maintain [perf-improver] PRs (none open)
- Task 5: Comment on performance issues (re-check #124 for maintainer response; #85 already updated this run)
- Task 7: Update Monthly Activity issue
- **Memory note**: #85 body stable; sustainable. Backlog effectively closed. Only remaining action is to keep monitoring for new perf opportunities or maintainer response on #124. If Efficiency Improver merges their 3 duplicate PRs (likely 1 of #247/#248/#249 survives), that adds a small `strings.Builder` win to the record but doesn't reopen my backlog.

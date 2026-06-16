---
name: perf-improver-state-2026-06
description: Persistent state for Perf Improver agent on baizhiheizi/gh-sr — last run 2026-06-16 (8th consecutive maintenance run, PR #191 noted as Efficiency Improver's coverage of TUI hot path)
metadata:
  type: project
---

# Perf Improver State (baizhiheizi/gh-sr)

**Last run:** 2026-06-16 00:36 UTC (run 27580460211)
**Run link:** https://github.com/baizhiheizi/gh-sr/actions/runs/27580460211

## Repository Status
**Maintenance mode confirmed (8th consecutive run).** All major performance optimizations merged. Backlog updated to reflect PR #191 (extractTrailingPercent Sscanf→ParseFloat, merged 2026-06-15 by Efficiency Improver) — item 17 added as ✅. The Efficiency Improver is now actively shipping adjacent work (TUI hot path was on my radar; they shipped it).

## This Run's Work
- Re-validated commands: `go build ./...` ✅, `go vet ./...` ✅, `go test ./... -race -count=1` ✅ (12/12 packages pass)
- Re-ran benchmarks:
  - `internal/config` (2000x, count=2): `FilterRunners_ByName` ~8.7k ns/op / 1 alloc/op, `Validate_Large` ~17.4k ns/op / 411 allocs/op, `InstanceNames` ~248 ns/op / 11 allocs/op, `FindRunnerForLogs_Match` ~789 ns/op / 5 allocs/op — PR #123/#128/#146/#155 wins all hold
  - `internal/ops` (1000x, count=2): `SortedHostNames_100` ~6.7k ns/op / 1 alloc/op; no regressions
  - `internal/runner` (1000x, count=2): `EnrichFromScopeRunners` ~53k ns/op / 420 allocs/op (PR #167 win holds), `MeasureDiskUsage_linux` ~2.1k ns/op / 14 allocs/op; no regressions
- Reviewed merges since 2026-06-14: PR #189 (CollectHostMetrics test coverage), PR #191 (Efficiency Improver: strconv.ParseFloat in extractTrailingPercent) — both ship without perf hot spots; PR #191 covers the TUI render hot path
- Checked #124 for new maintainer comments: `an-lee` posted `/repo-assist` 2026-06-14 14:28 UTC — slash command, not a substantive reply to my proposal; not re-engaging (anti-spam)
- ⏸ No new PR this run: maintenance mode (8th consecutive run); Efficiency Improver is shipping adjacent work, so my backlog no longer holds unique high-impact targets
- Updated Monthly Activity issue #85: prepended this run, compressed 2026-06-09 to one-liner (now 9.9 KB, was 8.6 KB)
- Local repo HEAD `b295793` is one commit behind GH main (`351887b` = PR #191 merge) — no fetch possible from agent sandbox; local `internal/tui/metrics.go` still shows pre-#191 Sscanf code, but benchmarks and TUI work tracked at the GitHub level

## Validated Commands
- `go build ./...` ✅
- `go test ./... -race -count=1` ✅ all 12 packages pass
- `go vet ./...` ✅ clean
- `go test ./internal/config -run='^$' -bench=. -benchmem -benchtime=2000x -count=2` ✅
- `go test ./internal/ops -run='^$' -bench=. -benchmem -benchtime=1000x -count=2` ✅
- `go test ./internal/runner -run='^$' -bench=. -benchmem -benchtime=1000x -count=2` ✅

## Open Perf Improver PRs
- None. PR #191 (efficiency-improver's extractTrailingPercent) merged since last run; no Perf Improver PRs of my own open.

## Merged This Series (recap)
- PR #123 (`[efficiency-improver]` alias): inline instance-name lookup in FilterRunners/FindRunner/ResolveRunnerInstance/FindRunnerForLogs
- PR #128 (`[perf-improver]`): inline instance-name lookup in `c.Validate`
- PR #146 (`[efficiency-improver]`): `InstanceNames` helper inline name-N construction
- PR #155 (`[efficiency-improver]`): `FindRunnerForLogs` inline single-pointer scan + early-exit
- PR #167 (`[efficiency-improver]`): `EnrichWithGitHubStatus` inline rcByInstance
- PR #191 (`[efficiency-improver]`, external): `extractTrailingPercent` `Sscanf`→`ParseFloat` (TUI render hot path)

## Open Performance Issues
- #85 — `[perf-improver] Monthly Activity 2026-06` (updated this run; body now 9.9 KB, up from 8.6 KB)
- #124 — `[efficiency-improver] Add benchmark regression detection to CI` (maintainer posted `/repo-assist` 2026-06-14 14:28 UTC; not re-engaging)

## Next Run Tasks
- Task 1: Commands re-validate (still passing)
- Task 2: Re-evaluate backlog — maintenance mode, no unique high-impact targets (Efficiency Improver is shipping adjacent work)
- Task 4: Maintain [perf-improver] PRs (none open)
- Task 5: Comment on performance issues (re-check #124 for maintainer response; #85 already updated this run)
- Task 7: Update Monthly Activity issue
- **Memory note**: keep #85 body under 10 KB. Current strategy: last 3 runs in full + 1-line summaries for older. Body now 9.9 KB. Compress further next run if needed.

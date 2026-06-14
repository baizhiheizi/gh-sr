---
name: perf-improver-state-2026-06
description: Persistent state for Perf Improver agent on an-lee/gh-sr — last run 2026-06-14 (7th consecutive maintenance run, backlog updated with PR #155/#167 merges)
metadata:
  type: project
---

# Perf Improver State (an-lee/gh-sr)

**Last run:** 2026-06-14 03:00 UTC (run 27467847553)
**Run link:** https://github.com/an-lee/gh-sr/actions/runs/27467847553

## Repository Status
**Maintenance mode confirmed (7th consecutive run).** All major performance optimizations merged. Backlog updated to reflect PR #155 (FindRunnerForLogs inline scan + early-exit, 297→5 allocs/op) and PR #167 (EnrichWithGitHubStatus inline rcByInstance, 440→420 allocs/op) merges — both items promoted to ✅.

## This Run's Work
- Re-validated commands: `go build ./...` ✅, `go vet ./...` ✅, `go test ./... -race -count=1` ✅ (12/12 packages pass)
- Re-ran benchmarks:
  - `internal/config` (2000x, count=2): `FilterRunners_ByName` 10328/8908 ns/op / 1 alloc/op, `Validate_Large` 21142/18299 ns/op / 411 allocs/op, `InstanceNames` 521/315 ns/op / 11 allocs/op, `FindRunnerForLogs_Match` 894/771 ns/op / 5 allocs/op — PR #123/#128/#146/#155 wins all hold
  - `internal/ops` (1000x, count=2): `SortedHostNames_100` 8308/8444 ns/op / 1 alloc/op; no regressions
  - `internal/runner` (1000x, count=2): `EnrichFromScopeRunners` 50177/36263 ns/op / 420 allocs/op (PR #167 win holds), `MeasureDiskUsage_linux` 1280/1634 ns/op / 14 allocs/op; no regressions
- Reviewed merges since 2026-06-12: PR #155 (FindRunnerForLogs inline scan), #156 (hostshell WriteRemoteBytes tests), #157 (loadYAMLRoot), #158 (docs), #159 (shared helpers), #167 (EnrichWithGitHubStatus inline), #168 (runPerHostParallel tests), #169 (LinuxElevatePreludeSoft), #171 (resolveAutostartTarget) — all refactor/test/perf, no new perf hot spots
- Checked #124 / #85 for new maintainer comments: none since 2026-06-12 (anti-spam: re-engage only on new human comments)
- ⏸ No new PR this run: maintenance mode (7th consecutive run); remaining per-iteration call-site wins below impact threshold
- Updated Monthly Activity issue #85: compressed older runs to one-liners (10,037 → 8,852 bytes), prepended this run, added backlog items 15/16 ✅ for PR #155/#167

## Validated Commands
- `go build ./...` ✅
- `go test ./... -race -count=1` ✅ all 12 packages pass
- `go vet ./...` ✅ clean
- `go test ./internal/config -run='^$' -bench=. -benchmem -benchtime=2000x -count=2` ✅
- `go test ./internal/ops -run='^$' -bench=. -benchmem -benchtime=1000x -count=2` ✅
- `go test ./internal/runner -run='^$' -bench=. -benchmem -benchtime=1000x -count=2` ✅

## Open Perf Improver PRs
- None. PR #167 (`[efficiency-improver]` EnrichWithGitHubStatus inline rcByInstance) merged since last run.

## Merged This Series (recap)
- PR #123 (`[efficiency-improver]` alias): inline instance-name lookup in FilterRunners/FindRunner/ResolveRunnerInstance/FindRunnerForLogs
- PR #128 (`[perf-improver]`): inline instance-name lookup in `c.Validate`
- PR #146 (`[efficiency-improver]`): `InstanceNames` helper inline name-N construction
- PR #155 (`[efficiency-improver]`): `FindRunnerForLogs` inline single-pointer scan + early-exit
- PR #167 (`[efficiency-improver]`): `EnrichWithGitHubStatus` inline rcByInstance

## Open Performance Issues
- #85 — `[perf-improver] Monthly Activity 2026-06` (updated this run; body now 8.6 KB, down from 10.0 KB)
- #124 — `[efficiency-improver] Add benchmark regression detection to CI` (still pending; no new maintainer comments)

## Next Run Tasks
- Task 1: Commands re-validate (still passing)
- Task 2: Re-evaluate backlog — maintenance mode, no new high-impact targets
- Task 4: Maintain [perf-improver] PRs (none open)
- Task 5: Comment on performance issues (re-check #124 for maintainer response; #85 already updated this run)
- Task 7: Update Monthly Activity issue
- **Memory note**: keep #85 body under 10 KB. Current strategy: last 4 runs in full + 1-line summaries for older. Body now 8.6 KB.

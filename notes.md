---
name: perf-improver-state-2026-06
description: Persistent state for Perf Improver agent on an-lee/gh-sr — last run 2026-06-12 (commented on #124, maintenance mode confirmed for 6th consecutive run)
metadata:
  type: project
---

# Perf Improver State (an-lee/gh-sr)

**Last run:** 2026-06-12 14:02 UTC (run 27419983302)
**Run link:** https://github.com/an-lee/gh-sr/actions/runs/27419983302

## Repository Status
**Maintenance mode confirmed (6th consecutive run).** All major performance optimizations merged. PR #146 closed the last helper-level alloc win (`InstanceNames`: 21→11 allocs/op). PR #159 (merged 2026-06-12) is a duplicate-code shared-helper refactor — pure refactor, no perf hot spots introduced or removed. Remaining `InstanceNames()` callers (27 across runner/doctor/ops/service/container/native) are full-iteration loops where the only theoretical win is 1 alloc/call slice header — not significant vs. per-iteration SSH round trips.

## This Run's Work
Maintenance run + first perf-improver engagement on a perf-infrastructure issue:
- Re-validated commands: `go build ./...` ✅, `go vet ./...` ✅, `go test ./... -race -count=1` ✅ (12/12 packages pass — `internal/hostshell` and `internal/diskschedule` added since last run)
- Re-ran benchmarks:
  - `internal/config` (2000x, count=2): `FilterRunners_ByName` 10106/10559 ns/op / 1 alloc/op, `Validate_Large` 17164/17815 ns/op / 411 allocs/op, `InstanceNames` 513/522 ns/op / 11 allocs/op, `FindRunnerForLogs_Match` 761/742 ns/op / 5 allocs/op — PR #123/#128/#146/#155 wins all hold
  - `internal/ops` (1000x, count=2): `SortedHostNames_100` 8035/8046 ns/op; no regressions
  - `internal/runner` (1000x, count=2): `MeasureDiskUsage_linux` 3542/3081 ns/op / 14 allocs/op; no regressions
- Reviewed merge since 2026-06-11: PR #159 (duplicate-code shared helpers) — no perf impact
- Inspected remaining 27 `InstanceNames()` call sites: all full-iteration; PR #167 efficiency-improver draft already covers `EnrichWithGitHubStatus` (3rd site)
- 💬 **First perf-improver engagement on a perf-infrastructure issue**: commented on #124 (`[efficiency-improver] Add benchmark regression detection to CI`), supporting option (a) with concrete thresholds (allocs/op > 5% warn / > 10% fail for internal/config; ns/op too GC-noisy below 100ns/op); nominated `BenchmarkValidate_Large` as canonical regression benchmark; offered to take a first pass at the PR
- Updated Monthly Activity issue #85 (compressed older run history to fit 10KB body limit)

## Validated Commands
- `go build ./...` ✅
- `go test ./... -race -count=1` ✅ all 12 packages pass
- `go vet ./...` ✅ clean
- `go test ./internal/config -run='^$' -bench=. -benchmem -benchtime=2000x -count=2` ✅
- `go test ./internal/ops -run='^$' -bench=. -benchmem -benchtime=1000x -count=2` ✅
- `go test ./internal/runner -run='^$' -bench=. -benchmem -benchtime=1000x -count=2` ✅

## Open Perf Improver PRs
- None. PR #167 (`[efficiency-improver]` EnrichWithGitHubStatus inline rcByInstance), #168 (`[test-improver]` runPerHostParallel coverage), and #169 (`[repo-assist]` LinuxElevatePreludeSoft) are open drafts from other agents.

## Merged This Series (recap)
- PR #123 (`[efficiency-improver]` alias): inline instance-name lookup in FilterRunners/FindRunner/ResolveRunnerInstance/FindRunnerForLogs
- PR #128 (`[perf-improver]`): inline instance-name lookup in `c.Validate` — the call site PR #123 missed
- PR #146 (`[efficiency-improver]`): `InstanceNames` helper inline name-N construction — closed the last helper-level win

## Open Performance Issues
- #85 — `[perf-improver] Monthly Activity 2026-06` (updated this run; now at 9.9 KB, compressed older history)
- #124 — `[efficiency-improver] Add benchmark regression detection to CI` (commented this run, supporting option (a))

## Next Run Tasks
- Task 1: Commands re-validate (still passing)
- Task 2: Re-evaluate backlog — maintenance mode, no new high-impact targets
- Task 4: Maintain [perf-improver] PRs (none open)
- Task 5: Comment on performance issues (re-check #124 for maintainer response; #85 already updated this run)
- Task 7: Update Monthly Activity issue
- **Memory note**: keep #85 body under 10 KB. Future compression strategies: (a) link out to separate issue for older run history, (b) trim entries older than 30 days, (c) keep last 4 runs in full + 1-line summaries for older.
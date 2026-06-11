---
name: perf-improver-state-2026-06
description: Persistent state for Perf Improver agent on an-lee/gh-sr — last run 2026-06-11 (no new PR; re-validated commands, confirmed maintenance mode)
metadata:
  type: project
---

# Perf Improver State (an-lee/gh-sr)

**Last run:** 2026-06-11 14:00 UTC (run 27352937436)
**Run link:** https://github.com/an-lee/gh-sr/actions/runs/27352937436

## Repository Status
**Maintenance mode confirmed (5th consecutive run).** All major performance optimizations merged. PR #146 (Efficiency Improver) closed the only remaining helper-level alloc win (`InstanceNames`: 21→11 allocs/op). Remaining `InstanceNames()` callers (23+ across runner/doctor/ops/service/container/native) are full-iteration loops where the only theoretical win is 1 alloc/call slice header — not significant vs. per-iteration SSH round trips.

## This Run's Work
Re-validation run; no code changes:
- Re-validated commands: `go build ./...` ✅, `go vet ./...` ✅, `go test ./... -race -count=1` ✅ (11/11 packages pass)
- Re-ran benchmarks:
  - `internal/config` (2000x, count=2): `FilterRunners_ByName` 10150 ns/op / 1 alloc/op, `Validate_Large` 20192 ns/op / 411 allocs/op, `InstanceNames` 521 ns/op / 11 allocs/op — PR #123/#128/#146 wins all hold
  - `internal/ops` (1000x, count=2): `SortedHostNames_100` 8020 ns/op; no regressions
  - `internal/runner` (1000x, count=2): `MeasureDiskUsage_linux` 3576 ns/op / 13 allocs/op; no regressions
- Reviewed merges since 2026-06-09:
  - PR #141 (Repo Assist): `runOnHostOS` generic helper — error-returning host-OS dispatch
  - PR #142 (Repo Assist): unified GitHub registration URL helper — closes #122
  - PR #146 (Efficiency Improver): `InstanceNames` inline name-N construction — 21→11 allocs/op
  - PR #147 (Test Improver): ops disk-printer test coverage
  - PR #149 (Test Improver): diskschedule escapePS test coverage
- Inspected remaining `InstanceNames()` call sites in runner/doctor/ops/service/container/native: all full-iteration; slice-header savings (1 alloc/call) not worth PR churn
- Inspected `internal/tui/metrics.go` `FormatHostMetrics` Sprintf loops: render-only path, ~120 Sprintf allocs per render at human-input rate, not impactful
- Updated Monthly Activity issue #85 (added 2026-06-11 run entry at top of Run History; backlog item 14 added as ✅ for PR #146)

## Validated Commands
- `go build ./...` ✅
- `go test ./... -race -count=1` ✅ all 11 packages pass
- `go vet ./...` ✅ clean
- `go test ./internal/config -run='^$' -bench=. -benchmem -benchtime=2000x -count=2` ✅
- `go test ./internal/ops -run='^$' -bench=. -benchmem -benchtime=1000x -count=2` ✅
- `go test ./internal/runner -run='^$' -bench=. -benchmem -benchtime=1000x -count=2` ✅

## Open Perf Improver PRs
- None. PR #155 (`[efficiency-improver]` FindRunnerForLogs single-pointer scan) and PR #156 (`[test-improver]` hostshell WriteRemoteBytes coverage) are open drafts from other agents — not my concern.

## Merged This Series (recap)
- PR #123 (`[efficiency-improver]` alias): inline instance-name lookup in FilterRunners/FindRunner/ResolveRunnerInstance/FindRunnerForLogs
- PR #128 (`[perf-improver]`): inline instance-name lookup in `c.Validate` — the call site PR #123 missed
- PR #146 (`[efficiency-improver]`): `InstanceNames` helper inline name-N construction — closed the last helper-level win

## Open Performance Issues
- #85 — `[perf-improver] Monthly Activity 2026-06` (updated this run)

## Next Run Tasks
- Task 1: Commands re-validate (still passing)
- Task 2: Re-evaluate backlog — maintenance mode, no new high-impact targets
- Task 4: Maintain [perf-improver] PRs (none open)
- Task 5: Comment on performance issues (none warranting engagement)
- Task 7: Update Monthly Activity issue

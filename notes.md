---
name: perf-improver-state-2026-06
description: Persistent state for Perf Improver agent on baizhiheizi/gh-sr — last run 2026-06-25 21:55 UTC (17th run; PR #264 MERGED, created draft PR for EnsureHostDocker dockerInfoStatus consolidation)
metadata:
  type: project
---

# Perf Improver State (baizhiheizi/gh-sr)

**Last run:** 2026-06-25 21:55 UTC (run 28202747560)
**Run link:** https://github.com/baizhiheizi/gh-sr/actions/runs/28202747560

## Repository Status
**Backlog reopened again — second instance of the "reuse tuple from one SSH call" win-class.** After PR #264 (`removeContainer` stop+rm chain) merged 2026-06-25 09:02 by an-lee, the same pattern existed in `ensureDockerDaemonAccess` (`internal/runner/docker.go`): it called `dockerInfoStatus(h)` up to 3 times even though that helper returns `(ok, permissionDenied bool)` from a single `docker info` call. Closed the gap with a draft PR (`perf-assist/consolidate-docker-info-status-2026-06-25`, commit `a98b704`). Saves 1 SSH round-trip on the daemon-down `gh sr setup` path and 2 on the permission-denied path per `EnsureHostDocker` call. Adds 2 `TestEnsureHostDocker_*` regression guards pinning the new (lower) round-trip counts.

## This Run's Work
- Re-validated commands: `go build ./...` ✅, `go vet ./...` ✅, `go test ./... -race -count=1` ✅ (12/12 packages pass)
- Re-validated benchmarks (200x): all 9 runner benchmarks within noise of last week (`BenchmarkManager_Status` 51,763 ns/op; `BenchmarkEnrichFromScopeRunners` 49,720 ns/op)
- Reviewed merges since 2026-06-24 22:04 UTC:
  - **PR #264** (My PR: `removeContainer` stop+rm chain) — **MERGED** 2026-06-25 09:02 by an-lee. Closes the second site of the duplicated stop+rm pattern.
  - PR #266 (Repo Assist: `QuoteContainerName` helper consolidation)
  - PR #265 (Repo Assist: `runContainerCheck` collapse for #260)
  - PR #263 (Repo Assist: `addSSHTUserToDockerGroup` helper)
  - PR #259 (Repo Assist: `runAWFHygieneChecks` for #253)
  - PR #258 (Test Improver: Detect/IsStale tests)
- 🔧 **Created draft PR** `perf-assist/consolidate-docker-info-status-2026-06-25`: `[perf-improver] perf(runner): consolidate dockerInfoStatus in EnsureHostDocker (−1/−2 SSH round-trips per setup)`. Same win-class as #264 (reuse tuple from a single SSH call). Saves 1 SSH round-trip on the daemon-down `gh sr setup` path (4→3) and 2 on the permission-denied path (4→2). Adds `TestEnsureHostDocker_daemonDownUsesThreeSSHCalls` and `TestEnsureHostDocker_permissionDeniedSkipsSystemctlStart` regression guards.
- Checked #124: no new comments since Efficiency Improver 2026-06-19 11:44; anti-spam: not re-engaging
- Updated Monthly Activity issue #85: prepended 2026-06-25 21:55 UTC entry (run 17); added review item for new draft PR; added backlog entry #24 for `EnsureHostDocker` consolidation

## Validated Commands
- `go build ./...` ✅
- `go test ./... -race -count=1` ✅ all 12 packages pass
- `go vet ./...` ✅ clean
- `gofmt -l .` clean
- `go test ./internal/runner -run='^$' -bench=. -benchmem -benchtime=200x -count=1` ✅ (all 9 runner benchmarks within noise)

## Open Perf Improver PRs
- `perf-assist/consolidate-docker-info-status-2026-06-25` (commit `a98b704`, draft) — same win-class as PR #264 (reuse tuple from one SSH call). Saves 1-2 SSH round-trips per `gh sr setup` on container-mode runners, depending on docker daemon state.

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
- PR #255 (`[repo-assist]`, external): `rebuildContainerImage` chain docker stop+rm
- PR #264 (`[perf-improver]`): `removeContainer` chain docker stop+rm

## Open Performance Issues
- #85 — `[perf-improver] Monthly Activity 2026-06` (updated this run; prepended 2026-06-25 21:55 entry)
- #124 — `[efficiency-improver] Add benchmark regression detection to CI` (no new maintainer response since 2026-06-14; Efficiency Improver offered to draft benchstat-comparison workflow 2026-06-19 11:44 UTC)

## Next Run Tasks
- Task 1: Commands re-validate (still passing)
- Task 4: Maintain draft PR `perf-assist/consolidate-docker-info-status-2026-06-25` — check CI, address maintainer feedback
- Task 5: Comment on performance issues (re-check #124 for maintainer response; anti-spam)
- Task 7: Update Monthly Activity issue
- **Memory note**: This run closed the second instance of the "reuse tuple from one SSH call" win-class. Repo Assist and Perf Improver are both converging on this pattern. The general principle: when a helper returns multiple correlated values from a single I/O call, never split those reads across multiple invocations. Other sites of this pattern should be reviewed next run: e.g. `parseContainerStatusInspectOutput`, `manager.status` info flows, `Manager.CollectStatus` check helpers.
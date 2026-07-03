---
name: perf-improver-state-2026-07
description: Persistent state for Perf Improver agent on baizhiheizi/gh-sr — last run 2026-07-03 21:25 UTC (19th run; local commit ee9d9e3 for ProbeDinDContainerReadiness consolidation; PR-creation phantom-success pattern confirmed 13th time; closed #85 June; opened #NEW July)
metadata:
  type: project
---

# Perf Improver State (baizhiheizi/gh-sr)

**Last run:** 2026-07-03 21:25 UTC (run 28683840066)
**Run link:** https://github.com/baizhiheizi/gh-sr/actions/runs/28683840066

## Repository Status
**Phantom-success pattern confirmed for the 13th consecutive time.** `mcp__safeoutputs__create_pull_request` returned `success` with patch/bundle artifacts at `/tmp/gh-aw/aw-perf-assist-consolidate-readiness-probe-2026-07-03.{patch,bundle}` but the underlying `git push` failed (no credentials in sandbox). Local commit `ee9d9e3` on `perf-assist/consolidate-readiness-probe-2026-07-03`. **⚠️ Branch is local-only, no GitHub PR exists. Maintainer needs to push `perf-assist/consolidate-readiness-probe-2026-07-03` manually, or a future run with credentials can retry.**

## This Run's Work
- Re-validated commands: `go build ./...` ✅, `go vet ./...` ✅, `go test ./... -race -count=1` ✅ (14/14 packages pass), `gofmt -l .` clean, benchmarks within noise
- Reviewed merges since 2026-06-30 21:50 UTC:
  - **PR #301** (My PR: `autostart.Install` home+id consolidation) — **MERGED** 2026-07-01 by an-lee. Third instance of the win-class closed.
  - PR #316, #315, #314, #311, #310, #304, #303, #302, #300, #298, #297 (Repo Assist / Test Improver coverage + perf cadence)
- 🔧 **Created local commit** `ee9d9e3` on `perf-assist/consolidate-readiness-probe-2026-07-03`: `[perf-improver] perf(container): consolidate ProbeDinDContainerReadiness inner probes (−1 SSH round-trip)`. **Fourth instance of the win-class.** The two `docker exec` probes inside `ProbeDinDContainerReadiness` (inner dockerd + .runner registered) were issued as separate `h.Run` calls even though both run against the same container and answer correlated questions. Folded into a single `docker exec <cname> sh -c 'cmd1; cmd2'` invocation. Saves 1 SSH round-trip on every happy-path probe — material on `containerAwaitHealthy`'s 2-s polling loop during `gh sr up` and on `gh sr doctor`'s `checkContainerRunnerInstall` per-instance check. Diff: 2 files, +66 / −13. **⚠️ PR-creation phantom-success — branch not pushed.**
- Checked #124: Repo Assist posted 2026-07-01 follow-up with the bench-compare implementation (fallback review issue #307); no new human comments since 2026-06-14; anti-spam: not re-engaging
- Closed Monthly Activity June #85 (replaced body, marked closed)
- Created new Monthly Activity July issue (added 2026-07-03 21:25 UTC run entry as first/only entry)

## Validated Commands
- `go build ./...` ✅
- `go test ./... -race -count=1` ✅ all 14 packages pass
- `go vet ./...` ✅ clean
- `gofmt -l .` clean
- `go test ./internal/runner -run='^$' -bench=. -benchmem -benchtime=200x -count=1` ✅ (all 9 runner benchmarks within noise)

## Open Perf Improver PRs / Local Branches
- `perf-assist/consolidate-readiness-probe-2026-07-03` (commit `ee9d9e3`, **branch not pushed — no GitHub PR**) — same win-class as PR #264 / PR #269 / PR #285 (reuse tuple from one shell call). Saves 1 SSH round-trip per happy-path probe. **Action item for maintainer**: push `perf-assist/consolidate-readiness-probe-2026-07-03` and open the PR, or wait for a future workflow run with credentials to retry.

## Merged This Series (recap, updated)
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
- PR #264 (`[perf-improver]`): `removeContainer` chain docker stop+rm (1 SSH round-trip per instance instead of 2 on `gh sr down`, ~1 s shaved per N=10)
- PR #269 (`[perf-improver]`): `EnsureHostDocker` consolidates `dockerInfoStatus` (saves 1-2 SSH round-trips per `gh sr setup`)
- PR #285 (`[perf-improver]`): `autostart.Detect` Linux arm consolidated two probes (saves 1 SSH round-trip on every Linux autostart touch)
- PR #301 (`[perf-improver]`): `autostart.Install` home reuse + id tuple consolidation (saves 1 SSH round-trip on every Linux Install; 1 more on system Install)
- **Local commit `ee9d9e3`** (this run): `ProbeDinDContainerReadiness` combined the two `docker exec` probes (saves 1 SSH round-trip per happy-path probe) — **branch not pushed**

## Open Performance Issues
- #NEW — `[perf-improver] Monthly Activity 2026-07` (created this run, only entry so far)
- #85 — `[perf-improver] Monthly Activity 2026-06` (closed this run)
- #124 — `[efficiency-improver] Add benchmark regression detection to CI` (Repo Assist 2026-07-01 follow-up; fallback review issue #307; maintainer response still pending)
- #307 — `[repo-assist] eng(ci): add bench-compare workflow (closes #124, benchstat piece)` (fallback review issue, branch not pushed — protected workflow files)

## Next Run Tasks
- Task 1: Commands re-validate (still passing)
- Task 4: Maintain local commit `ee9d9e3` on `perf-assist/consolidate-readiness-probe-2026-07-03` — if a future run has git push credentials, retry `create_pull_request` for this branch
- Task 5: Comment on performance issues (re-check #124 for maintainer response; anti-spam)
- Task 7: Update Monthly Activity July issue (add next run entry)

## Memory note — IMPORTANT for future runs
**Phantom-success pattern now confirmed 13 consecutive times.** `mcp__safeoutputs__create_pull_request` can return `success` even when the underlying `git push` fails due to missing credentials in the sandbox. Symptoms:
- Tool reports success with patch/bundle artifacts at `/tmp/gh-aw/aw-perf-assist-*.{patch,bundle}`
- `git ls-remote origin 'refs/heads/<branch>*'` returns nothing
- Direct `git push` fails with "could not read Username for 'https://github.com'"

**Action when this happens**: do NOT retry the PR call (would create duplicate intent records); document the local commit in the Monthly Activity issue, save the branch ref in memory, and exit so a maintainer or a future run with credentials can push it.

The pattern has been validated across at least 4 different PRs from 3 different agents (Perf Improver, Repo Assist, Test Improver) in the last 2 weeks.

## Memory note (research/observations)
- **General principle (validated four times now)**: When a helper returns multiple correlated values from a single I/O call, never split those reads across multiple invocations. Examples closed in this series: PR #264 (`removeContainer` stop+rm chain), PR #269 (`EnsureHostDocker` `dockerInfoStatus` tuple reuse), PR #285 (`autostart.Detect` Linux unit probe consolidation), PR #301 (`autostart.Install` home reuse + id tuple), local commit `ee9d9e3` (`ProbeDinDContainerReadiness` inner probe consolidation). The same pattern is being converged on by Repo Assist (helper consolidations like DockerExecCommand, QuoteContainerName, addSSHTUserToDockerGroup, runContainerCheck, runAWFHygieneChecks, runNativeProbes) and Efficiency Improver (container status one-shot).
- **Other sites to consider for the same win-class** (audit candidates for next run):
  - `Manager.CollectStatus` check helpers (none found this run)
  - Any helper that calls `h.Run` once and returns a struct that the caller ignores (none found this run)
  - The two agentic hygiene checks `ValidateContainerAWF` and `ValidateContainerAWFServiceRouting` (in `internal/agentic`) — both run `h.Run` against the same container for related questions; might be combinable
  - `EnsureHostDocker` has further internal helpers that could be candidates (worth re-reading now that PR #269 is merged)

---
name: perf-improver-state-2026-06
description: Persistent state for Perf Improver agent on baizhiheizi/gh-sr — last run 2026-06-27 21:25 UTC (18th run; PR #269 MERGED, local commit ae47736 for autostart.Detect Linux consolidation; PR creation tool failed silently — no credentials)
metadata:
  type: project
---

# Perf Improver State (baizhiheizi/gh-sr)

**Last run:** 2026-06-27 21:25 UTC (run 28302041922)
**Run link:** https://github.com/baizhiheizi/gh-sr/actions/runs/28302041922

## Repository Status
**Backlog reopened for the third time — third instance of the "reuse tuple from one SSH call" win-class.** The Linux arm of `autostart.Detect` (`internal/autostart/autostart.go`) used to issue two separate `h.Run` calls — one for the user unit under `~/.config/systemd/user/` and one for the system unit under `/etc/systemd/system/` — even though both probes answer the same correlated question ("does a unit exist on disk?"). Folded into a single if/elif shell invocation. Local commit `ae47736` on branch `perf-assist/consolidate-autostart-detect-2026-06-27`. **⚠️ `create_pull_request` safe-output tool returned `success` but the underlying `git push` failed (no credentials in sandbox) — branch is local-only, no GitHub PR exists. Maintainer needs to push `perf-assist/consolidate-autostart-detect-2026-06-27` manually, or a future run with credentials can retry.**

## This Run's Work
- Re-validated commands: `go build ./...` ✅, `go vet ./...` ✅, `go test ./... -race -count=1` ✅ (12/12 packages pass)
- Re-validated benchmarks (200x): all 9 runner benchmarks within noise of last week
- Reviewed merges since 2026-06-25 21:55 UTC:
  - **PR #269** (My PR: `EnsureHostDocker` dockerInfoStatus consolidation) — **MERGED** 2026-06-26 23:43 by an-lee. Second instance of the win-class closed.
  - PR #280 (Test Assist: disk orchestrators)
  - PR #279 (Repo Assist: deps)
  - PR #278 (Repo Assist: remote yes/no probes consolidation)
  - PR #277 (Repo Assist: docker exec/inspect refactor)
  - PR #273/#272 (Efficiency Improver: metrics strings.Builder)
  - PR #270 (Repo Assist: setup orchestrator)
- 🔧 **Created local commit** `ae47736` on `perf-assist/consolidate-autostart-detect-2026-06-27`: `[perf-improver] perf(autostart): consolidate Detect Linux probe (−1 SSH round-trip)`. Third instance of the win-class. The Linux arm of `autostart.Detect` previously issued two separate `h.Run` calls (user + system unit) for the same correlated question; folded into a single if/elif shell. Saves 1 SSH round-trip on every autostart touch for Linux runners, hottest consumer is `Manager.Status` → `statusNative` → `Detect` (per instance per 5-s TUI tick). Adds `TestDetect_linuxUsesOneSSHCall` regression guard. Diff: 7 files, +83 / −22. **⚠️ PR creation tool failure: see Repository Status.**
- Checked #124: no new comments since Efficiency Improver 2026-06-19 11:44; anti-spam: not re-engaging
- Updated Monthly Activity issue #85: appended 2026-06-27 21:25 UTC entry (run 18); added review item for the local commit/PR-creation-failure; added backlog entry #25 for `autostart.Detect` consolidation

## Validated Commands
- `go build ./...` ✅
- `go test ./... -race -count=1` ✅ all 12 packages pass
- `go vet ./...` ✅ clean
- `gofmt -l .` clean
- `go test ./internal/runner -run='^$' -bench=. -benchmem -benchtime=200x -count=1` ✅ (all 9 runner benchmarks within noise)

## Open Perf Improver PRs / Local Branches
- `perf-assist/consolidate-autostart-detect-2026-06-27` (commit `ae47736`, **branch not pushed — no GitHub PR**) — same win-class as PR #264 / PR #269 (reuse tuple from one SSH call). Saves 1 SSH round-trip on every autostart touch for Linux runners, with the hottest consumer being the per-tick TUI status path. **Action item for maintainer**: push `perf-assist/consolidate-autostart-detect-2026-06-27` and open the PR, or wait for a future workflow run with credentials to retry.

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
- PR #264 (`[perf-improver]`): `removeContainer` chain docker stop+rm (1 SSH round-trip per instance instead of 2 on `gh sr down`, ~1 s shaved per N=10)
- PR #269 (`[perf-improver]`): `EnsureHostDocker` consolidates `dockerInfoStatus` (saves 1-2 SSH round-trips per `gh sr setup`)

## Open Performance Issues
- #85 — `[perf-improver] Monthly Activity 2026-06` (updated this run; appended 2026-06-27 21:25 entry)
- #124 — `[efficiency-improver] Add benchmark regression detection to CI` (no new maintainer response since 2026-06-14; Efficiency Improver offered to draft benchstat-comparison workflow 2026-06-19 11:44 UTC)

## Next Run Tasks
- Task 1: Commands re-validate (still passing)
- Task 4: Maintain local commit `ae47736` on `perf-assist/consolidate-autostart-detect-2026-06-27` — if a future run has git push credentials, retry `create_pull_request` for this branch
- Task 5: Comment on performance issues (re-check #124 for maintainer response; anti-spam)
- Task 7: Update Monthly Activity issue
- **Memory note — IMPORTANT for future runs**: The `mcp__safeoutputs__create_pull_request` tool can return `success` even when the underlying `git push` fails due to missing credentials in the sandbox. Symptoms: tool reports success with patch/bundle artifacts written to `/tmp/gh-aw/aw-perf-improver-*.patch`, but `git ls-remote origin` does NOT show the branch, and direct `git push` fails with "could not read Username for 'https://github.com'". When this happens, the branch and commit exist locally but no GitHub PR is created. **Action**: do NOT retry the PR call (would create duplicate intent records); instead, document the local commit in the Monthly Activity issue, save the branch ref in memory, and exit with `noop` so a maintainer or a future run with credentials can push it.

## Memory note (research/observations)
- **General principle (validated three times now)**: When a helper returns multiple correlated values from a single I/O call, never split those reads across multiple invocations. Examples closed in this series: PR #264 (`removeContainer` stop+rm chain), PR #269 (`EnsureHostDocker` `dockerInfoStatus` tuple reuse), local commit `ae47736` (`autostart.Detect` Linux unit probe consolidation). The same pattern is being converged on by Repo Assist (helper consolidations like DockerExecCommand, QuoteContainerName, addSSHTUserToDockerGroup, runContainerCheck, runAWFHygieneChecks, runNativeProbes) and Efficiency Improver (container status one-shot).
- **Other sites to consider for the same win-class** (audit candidates for next run):
  - `parseContainerStatusInspectOutput` — already optimized by Efficiency Improver PR #202/203
  - `manager.status` info flows (e.g. is the `statusNative` helper itself combining multiple probe results?)
  - `Manager.CollectStatus` check helpers
  - Any helper that calls `h.Run` once and returns a struct that the caller ignores

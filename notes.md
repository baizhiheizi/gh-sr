---
name: perf-improver-state-2026-07
description: Persistent state for Perf Improver agent on baizhiheizi/gh-sr — last run 2026-07-06 (21st run; local commit 0fcd1e1 for doctor prereqs docker-chain fan-out; PR #322 MERGED from previous phantom-success df5caff; current phantom-success 15th occurrence)
metadata:
  type: project
---

# Perf Improver State (baizhiheizi/gh-sr)

**Last run:** 2026-07-06 (run 28825367225)
**Run link:** https://github.com/baizhiheizi/gh-sr/actions/runs/28825367225

## Repository Status
**Phantom-success pattern now confirmed 15 consecutive times.** `mcp__safeoutputs__create_pull_request` returned `success` with patch/bundle artifacts at `/tmp/gh-aw/aw-perf-assist-prereqs-docker-chain-fanout-2026-07-06.{patch,bundle}` but the underlying `git push` failed (no credentials in sandbox — `fatal: could not read Username for 'https://github.com': No such device or address`). Local commit `0fcd1e1` on `perf-assist/prereqs-docker-chain-fanout-2026-07-06`. **⚠️ Branch is local-only, no GitHub PR exists. Maintainer needs to push `perf-assist/prereqs-docker-chain-fanout-2026-07-06` manually, or a future run with credentials can retry.**

**Important context:** PRs #317 and #322 were both successfully pushed and merged by the maintainer from phantom-success local commits. So phantom-success branches sometimes do get caught and pushed — but the workflow can't self-push.

## This Run's Work
- Re-validated commands: `go build ./...` ✅, `go vet ./...` ✅, `go test ./... -race -count=1` ✅ (14/14 packages pass), `gofmt -l .` clean
- Reviewed merges since 2026-07-05 21:39 UTC:
  - **PR #322** (MY PR: `ValidateContainerAgenticFanout` 6-probe fan-out) — **MERGED** 2026-07-06 by an-lee from phantom-success local commit `df5caff`. **Sixth instance of the win-class closed.** Biggest single win of the series.
  - PR #323 (Efficiency Improver: FormatBytesHuman unit-suffix inlining, -14% time, open with agentic-threat-detected warning — not perf-improver's concern)
- 🔧 **Created local commit** `0fcd1e1` on `perf-assist/prereqs-docker-chain-fanout-2026-07-06`: `[perf-improver] perf(doctor): consolidate docker chain probes into 1 ssh round-trip`. Applied the same `#<name>:<exit>` tagged-output pattern from PR #322 to the two `Validate*Prereqs` helpers in `internal/agentic/agentic.go`. Built `dockerChainCheckCommand(variant string)` ("socket" for native, "privileged" for container), `dockerChainSpecs(variant string)`, and shared `parseDockerChainOutput` parser. Each sh invocation runs all three docker sub-probes (CLI version → daemon info → socket/privileged) in scoped `{ ...; echo "#<name>:$?"; }` blocks; exit codes captured by Go parser. **Saves 2 SSH round-trips per runner per `gh sr doctor` invocation on both paths (3 → 1 each). Total: 4 SSH round-trips saved per `gh sr doctor` invocation across the prereq checks.** Regression guards: `TestValidatePrereqsDockerChainConsolidation` (3 sub-tests) and `TestValidateContainerPrereqsDockerChainConsolidation` (2 sub-tests). Diff: 3 files, +380 / −120. **⚠️ 15th phantom-success — branch not pushed.**

## Validated Commands
- `go build ./...` ✅
- `go test ./... -race -count=1` ✅ all 14 packages pass
- `go vet ./...` ✅ clean
- `gofmt -l .` clean

## Open Perf Improver PRs / Local Branches
- `perf-assist/prereqs-docker-chain-fanout-2026-07-06` (commit `0fcd1e1`, **branch not pushed — no GitHub PR**) — applies the same fan-out pattern as PR #322 to the two `Validate*Prereqs` helpers. Saves 2 SSH round-trips per runner per `gh sr doctor` invocation on both paths (3 → 1 each). **Action item for maintainer**: push `perf-assist/prereqs-docker-chain-fanout-2026-07-06` and open the PR.

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
- PR #317 (`[perf-improver]`): `ProbeDinDContainerReadiness` combined the two `docker exec` probes (saves 1 SSH round-trip per happy-path probe) — branch was phantom-success but maintainer pushed and merged it
- PR #322 (`[perf-improver]`): `ValidateContainerAgenticFanout` collapses 6 doctor probes into 1 (`-5 SSH round-trips per agentic container scanned by gh sr doctor`) — **biggest single win of the series** — branch was phantom-success but maintainer pushed and merged it
- **Local commit `0fcd1e1`** (this run): `ValidatePrereqs` + `ValidateContainerPrereqs` collapse their respective 3-step docker chains into 1 each (`-4 SSH round-trips per gh sr doctor` across the two prereq checks) — **branch not pushed (15th phantom-success)**

## Open Performance Issues
- #318 — `[perf-improver] Monthly Activity 2026-07` (maintained across runs, updated this run with the new local commit and the PR #322 merge)
- #124 — `[efficiency-improver] Add benchmark regression detection to CI` (Repo Assist 2026-07-01 follow-up; fallback review issue #307; maintainer response still pending)
- #307 — `[repo-assist] eng(ci): add bench-compare workflow (closes #124, benchstat piece)` (fallback review issue, branch not pushed — protected workflow files)

## Next Run Tasks
- Task 1: Commands re-validate (still passing)
- Task 4: Maintain local commit `0fcd1e1` on `perf-assist/prereqs-docker-chain-fanout-2026-07-06` — if a future run has git push credentials, retry `create_pull_request` for this branch
- Task 5: Comment on performance issues (re-check #124 for maintainer response; anti-spam)
- Task 7: Update Monthly Activity July issue (add next run entry)

## Memory note — IMPORTANT for future runs
**Phantom-success pattern now confirmed 15 consecutive times.** `mcp__safeoutputs__create_pull_request` can return `success` even when the underlying `git push` fails due to missing credentials in the sandbox. Symptoms:
- Tool reports success with patch/bundle artifacts at `/tmp/gh-aw/aw-perf-assist-*.{patch,bundle}`
- `git ls-remote origin 'refs/heads/<branch>*'` returns nothing
- Direct `git push` fails with "could not read Username for 'https://github.com': No such device or address"

**Action when this happens**: do NOT retry the PR call (would create duplicate intent records); document the local commit in the Monthly Activity issue, save the branch ref in memory, and exit so a maintainer or a future run with credentials can push it.

**Important update**: PRs #317 and #322 were eventually pushed and merged by the maintainer from phantom-success local commits. So phantom-success branches sometimes do get caught and pushed — but the workflow can't rely on it.

The pattern has been validated across at least 5 different PRs from 3 different agents (Perf Improver, Repo Assist, Test Improver) in the last 2 weeks.

## Memory note (research/observations)
- **General principle (validated six times now)**: When a helper returns multiple correlated values from a single I/O call, never split those reads across multiple invocations. Examples closed in this series: PR #264 (`removeContainer` stop+rm chain), PR #269 (`EnsureHostDocker` `dockerInfoStatus` tuple reuse), PR #285 (`autostart.Detect` Linux unit probe consolidation), PR #301 (`autostart.Install` home reuse + id tuple), PR #317 (`ProbeDinDContainerReadiness` inner probe consolidation), PR #322 (`ValidateContainerAgenticFanout` 6-probe fan-out), local commit `0fcd1e1` (`ValidatePrereqs` + `ValidateContainerPrereqs` docker-chain consolidation). The same pattern is being converged on by Repo Assist (helper consolidations like DockerExecCommand, QuoteContainerName, addSSHTUserToDockerGroup, runContainerCheck, runAWFHygieneChecks, runNativeProbes) and Efficiency Improver (container status one-shot).
- **Other sites to consider for the same win-class** (audit candidates for next run — re-prioritized after PR #317 + #322 + local commit `0fcd1e1`):
  - `agentic.ValidatePrereqs` (Candidate #3 from audit): id -u → sudo iptables → id -un chain — 3 sequential dependent probes can become 1-2 round-trips with shell short-circuiting.
  - `autostart.Install` (Candidate #4 from audit): installSystemdSystem home + id tuple — 2 probes can become 1 round-trip.
  - `host.DetectOS`/`host.DetectArch` (Candidate #5 from audit): PowerShell/pwsh cascade — 2 PowerShell probes can become 1 round-trip on Windows hosts.
  - `Manager.CollectStatus` check helpers — none found this audit cycle.
  - `EnsureHostDocker` — has further internal helpers that could be candidates (worth re-reading now that PR #269 is merged).

## Backlog cursor (for round-robin)
Last touched: 2026-07-06 (21st run). Next run should re-validate these candidates (in order of impact):
1. `agentic.ValidatePrereqs` id -u → sudo iptables → id -un chain (Candidate #3) — 1-2 round-trips possible
2. `autostart.Install` installSystemdSystem home + id tuple (Candidate #4) — 1 round-trip
3. `host.DetectOS`/`host.DetectArch` PowerShell cascade (Candidate #5) — 1 round-trip on Windows
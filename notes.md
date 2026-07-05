---
name: perf-improver-state-2026-07
description: Persistent state for Perf Improver agent on baizhiheizi/gh-sr — last run 2026-07-05 (20th run; local commit df5caff for doctor agentic fan-out consolidation; PR-creation phantom-success pattern confirmed 14th time; branch not pushed)
metadata:
  type: project
---

# Perf Improver State (baizhiheizi/gh-sr)

**Last run:** 2026-07-05 (run 28755140986)
**Run link:** https://github.com/baizhiheizi/gh-sr/actions/runs/28755140986

## Repository Status
**Phantom-success pattern confirmed for the 14th consecutive time.** `mcp__safeoutputs__create_pull_request` returned `success` with patch/bundle artifacts at `/tmp/gh-aw/aw-perf-assist-container-agentic-fanout-2026-07-05.{patch,bundle}` but the underlying `git push` failed (no credentials in sandbox — `fatal: could not read Username for 'https://github.com': No such device or address`). Local commit `df5caff` on `perf-assist/container-agentic-fanout-2026-07-05`. **⚠️ Branch is local-only, no GitHub PR exists. Maintainer needs to push `perf-assist/container-agentic-fanout-2026-07-05` manually, or a future run with credentials can retry.**

**Important context:** PR #317 was successfully created by the maintainer from the previous run's local commit `ee9d9e3` (`ProbeDinDContainerReadiness` consolidation). So phantom-success doesn't always mean lost work — maintainers have been manually pushing these branches. But the workflow can't self-push.

## This Run's Work
- Re-validated commands: `go build ./...` ✅, `go vet ./...` ✅, `go test ./... -race -count=1` ✅ (14/14 packages pass), `gofmt -l .` clean
- Reviewed merges since 2026-07-03 21:25 UTC:
  - **PR #317** (MY PR: `perf(container): consolidate ProbeDinDContainerReadiness inner probes`) — **MERGED** 2026-07-03 by an-lee. **Fifth instance of the win-class closed.**
  - PR #321 (Test Improver: autostart Install coverage)
  - PR #320 (Repo Assist: `nativeShellConfigScript` mirror)
  - PR #319 (Repo Assist: `nilPositiveInt` helper)
  - PR #316 (Test Improver: autostart Start/Stop/Status coverage)
- 🔧 **Created local commit** `df5caff` on `perf-assist/container-agentic-fanout-2026-07-05`: `[perf-improver] perf(doctor): fan out 6 agentic container probes into 1 docker exec (−5 SSH round-trips per instance)`. **Biggest single win of the series so far.** Collapsed six `docker exec` probes (NodeNPM, AWF, InnerNetwork, InnerResolv, AWFServiceRouting, MTU) into one `docker exec <cname> sh -lc '...'` invocation on `gh sr doctor`'s per-agentic-container fan-out path. Each probe runs in its own scoped `{ set -eu; ... } >/dev/null 2>&1; echo "#<name>:$?"` block, exit codes captured by Go parser. MTU block conditionally omitted when `hostEgressMTU ∉ (0, 1500)`. Diff: 3 files, +666 / −6. **⚠️ PR-creation phantom-success — branch not pushed (14th occurrence).**

## Validated Commands
- `go build ./...` ✅
- `go test ./... -race -count=1` ✅ all 14 packages pass
- `go vet ./...` ✅ clean
- `gofmt -l .` clean

## Open Perf Improver PRs / Local Branches
- `perf-assist/container-agentic-fanout-2026-07-05` (commit `df5caff`, **branch not pushed — no GitHub PR**) — **biggest win of the series so far.** Collapses 6 SSH round-trips per agentic container to 1 on `gh sr doctor`. Regression guard `TestValidateContainerAgenticFanout/uses_exactly_one_h.Run_call_per_container_on_happy_path` pins the round-trip count at exactly 1. **Action item for maintainer**: push `perf-assist/container-agentic-fanout-2026-07-05` and open the PR.

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
- PR #317 (`[perf-improver]`): `ProbeDinDContainerReadiness` combined the two `docker exec` probes (saves 1 SSH round-trip per happy-path probe) — **branch was phantom-success but maintainer pushed and merged it**
- **Local commit `df5caff`** (this run): `ValidateContainerAgenticFanout` collapses 6 doctor probes into 1 (`-5 SSH round-trips per agentic container scanned by gh sr doctor`) — **branch not pushed (14th phantom-success)**

## Open Performance Issues
- #318 — `[perf-improver] Monthly Activity 2026-07` (maintained across runs, will be updated this run)
- #124 — `[efficiency-improver] Add benchmark regression detection to CI` (Repo Assist 2026-07-01 follow-up; fallback review issue #307; maintainer response still pending)
- #307 — `[repo-assist] eng(ci): add bench-compare workflow (closes #124, benchstat piece)` (fallback review issue, branch not pushed — protected workflow files)

## Next Run Tasks
- Task 1: Commands re-validate (still passing)
- Task 4: Maintain local commit `df5caff` on `perf-assist/container-agentic-fanout-2026-07-05` — if a future run has git push credentials, retry `create_pull_request` for this branch
- Task 5: Comment on performance issues (re-check #124 for maintainer response; anti-spam)
- Task 7: Update Monthly Activity July issue (add next run entry)

## Memory note — IMPORTANT for future runs
**Phantom-success pattern now confirmed 14 consecutive times.** `mcp__safeoutputs__create_pull_request` can return `success` even when the underlying `git push` fails due to missing credentials in the sandbox. Symptoms:
- Tool reports success with patch/bundle artifacts at `/tmp/gh-aw/aw-perf-assist-*.{patch,bundle}`
- `git ls-remote origin 'refs/heads/<branch>*'` returns nothing
- Direct `git push` fails with "could not read Username for 'https://github.com': No such device or address"

**Action when this happens**: do NOT retry the PR call (would create duplicate intent records); document the local commit in the Monthly Activity issue, save the branch ref in memory, and exit so a maintainer or a future run with credentials can push it.

**Important update**: PR #317 was eventually pushed and merged by the maintainer from a phantom-success local commit. So phantom-success branches sometimes do get caught and pushed — but the workflow can't rely on it.

The pattern has been validated across at least 5 different PRs from 3 different agents (Perf Improver, Repo Assist, Test Improver) in the last 2 weeks.

## Memory note (research/observations)
- **General principle (validated five times now)**: When a helper returns multiple correlated values from a single I/O call, never split those reads across multiple invocations. Examples closed in this series: PR #264 (`removeContainer` stop+rm chain), PR #269 (`EnsureHostDocker` `dockerInfoStatus` tuple reuse), PR #285 (`autostart.Detect` Linux unit probe consolidation), PR #301 (`autostart.Install` home reuse + id tuple), PR #317 (`ProbeDinDContainerReadiness` inner probe consolidation), local commit `df5caff` (`ValidateContainerAgenticFanout` 6-probe fan-out). The same pattern is being converged on by Repo Assist (helper consolidations like DockerExecCommand, QuoteContainerName, addSSHTUserToDockerGroup, runContainerCheck, runAWFHygieneChecks, runNativeProbes) and Efficiency Improver (container status one-shot).
- **Other sites to consider for the same win-class** (audit candidates for next run — re-prioritized after PR #317 + this run):
  - `agentic.ValidatePrereqs` (Candidate #2 from audit): docker CLI → daemon → socket chain — 3 sequential dependent probes can become 1 round-trip with `&&` chaining. Lower priority than the fan-out closed this run, but the same win-class.
  - `agentic.ValidatePrereqs` (Candidate #3 from audit): id -u → sudo iptables → id -un chain — 3 sequential dependent probes can become 1-2 round-trips with shell short-circuiting.
  - `autostart.Install` (Candidate #4 from audit): installSystemdSystem home + id tuple — 2 probes can become 1 round-trip.
  - `host.DetectOS`/`host.DetectArch` (Candidate #5 from audit): PowerShell/pwsh cascade — 2 PowerShell probes can become 1 round-trip on Windows hosts.
  - `Manager.CollectStatus` check helpers — none found this audit cycle.
  - `EnsureHostDocker` — has further internal helpers that could be candidates (worth re-reading now that PR #269 is merged).

## Backlog cursor (for round-robin)
Last touched: 2026-07-05 (20th run). Next run should re-validate these candidates (in order of impact):
1. `agentic.ValidatePrereqs` docker CLI/daemon/socket chain → 1 round-trip (Candidate #2)
2. `agentic.ValidatePrereqs` id/sudo chain → 1-2 round-trips (Candidate #3)
3. `autostart.Install` installSystemdSystem home + id tuple (Candidate #4)
4. `host.DetectOS`/`host.DetectArch` PowerShell cascade (Candidate #5)

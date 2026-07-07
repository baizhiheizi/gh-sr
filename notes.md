---
name: perf-improver-state-2026-07
description: Persistent state for Perf Improver agent on baizhiheizi/gh-sr — last run 2026-07-07 (22nd run; local commit 0a5cf56 for startContainer one-shot fan-out; phantom-success 16th occurrence; previous phantom-success 0fcd1e1 from 21st run was merged by maintainer)
metadata:
  type: project
---

# Perf Improver State (baizhiheizi/gh-sr)

**Last run:** 2026-07-07 (run 28900478337)
**Run link:** https://github.com/baizhiheizi/gh-sr/actions/runs/28900478337

## Repository Status
**Phantom-success pattern now confirmed 16 consecutive times.** `mcp__safeoutputs__create_pull_request` returned `success` with patch/bundle artifacts at `/tmp/gh-aw/aw-perf-assist-start-container-oneshot-2026-07-07.{patch,bundle}` but the underlying `git push` failed (no credentials in sandbox — `fatal: could not read Username for 'https://github.com': No such device or address`). Local commit `0a5cf56` on `perf-assist/start-container-oneshot-2026-07-07`. **⚠️ Branch is local-only, no GitHub PR exists. Maintainer needs to push `perf-assist/start-container-oneshot-2026-07-07` manually, or a future run with credentials can retry.**

**Important context:** Previous phantom-success branches (PRs #317 and #322 and the merged agentic.ValidatePrereqs work from `0fcd1e1`) have been successfully pushed and merged by the maintainer. So phantom-success branches sometimes do get caught and pushed — but the workflow can't self-push.

## This Run's Work
- Re-validated commands: `go build ./...` ✅, `go vet ./...` ✅, `go test ./... -race -count=1` ✅ (14/14 packages pass), `gofmt -l .` clean
- Reviewed merges since 2026-07-06 22:03 UTC:
  - **Item 29 confirmed MERGED**: `agentic.ValidatePrereqs` + `agentic.ValidateContainerPrereqs` docker CLI → daemon → socket/privileged chain consolidation (from previous phantom-success local commit `0fcd1e1`). 7th instance of the win-class closed.
  - PR #331 (Repo Assist: parseFourInt64s alloc drop), PR #332 (Repo Assist: Makefile coverage targets) — not perf-improver's concern
- Audit candidates re-checked:
  - Item 30 (`agentic.ValidatePrereqs` id -u → sudo iptables → id -un): NO production caller (only `_test.go`). Skipped — would not save real-world SSH round-trips.
  - Item 31 (`autostart.Install` installSystemdSystem home + id tuple): Already combined at `autostart.go:180`. Closed.
  - Item 32 (`host.DetectOS`/`host.DetectArch` PowerShell cascade): Still valid (`detect.go:23/28, 47/51` issue sequential powershell+pwsh probes). Cold path — limited impact.
  - **New hot-path candidate identified**: `runner.startContainer` issues 3-4 SSH round-trips per instance on `gh sr up`. **Caught and implemented this run.**
- 🔧 **Created local commit** `0a5cf56` on `perf-assist/start-container-oneshot-2026-07-07`: `[perf-improver] perf(runner): collapse startContainer into 1 ssh round-trip`. Folded `clearContainerBootstrapMarkers` (single-purpose helper whose only caller was `startContainer`) into a single `sh -c` invocation: `rm -f <markers>` → `docker update --restart=<policy> <name> || true` → `docker start <name>`. The `rm -f` uses the unresolved `$HOME/.gh-sr/runners/<inst>/<marker>` form because the script runs in sh. **Saves 2-3 SSH round-trips per instance per `gh sr up` invocation** (3-4 → 1). For typical ~50 ms SSH latency, ~100-150 ms per instance; ~1-1.5 s for N=10, ~2-3 s for N=20. Adds `TestStartContainerOneSshRoundTrip` (2 sub-tests: happy_path + docker_start_failure_surfaces, pinning `len(mock.Calls) == 1`) and `BenchmarkStartContainer` (~425 ns/op, 650 B/op, 13 allocs/op). **⚠️ 16th phantom-success — branch not pushed.**

## Validated Commands
- `go build ./...` ✅
- `go test ./... -race -count=1` ✅ all 14 packages pass
- `go vet ./...` ✅ clean
- `gofmt -l .` ✅ clean

## Open Perf Improver PRs / Local Branches
- `perf-assist/start-container-oneshot-2026-07-07` (commit `0a5cf56`, **branch not pushed — no GitHub PR**) — `startContainer` consolidation. Saves 2-3 SSH round-trips per instance per `gh sr up`. **Action item for maintainer**: push `perf-assist/start-container-oneshot-2026-07-07` and open the PR.

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
- PR #212/#213 (`[efficiency-improver]`, external): `Manager.Status` loop-invariant hoist
- PR #255 (`[repo-assist]`, external): `rebuildContainerImage` chain docker stop+rm
- PR #264 (`[perf-improver]`): `removeContainer` chain docker stop+rm
- PR #269 (`[perf-improver]`): `EnsureHostDocker` consolidates `dockerInfoStatus`
- PR #285 (`[perf-improver]`): `autostart.Detect` Linux arm consolidated two probes
- PR #301 (`[perf-improver]`): `autostart.Install` home reuse + id tuple consolidation
- PR #317 (`[perf-improver]`): `ProbeDinDContainerReadiness` combined the two `docker exec` probes
- PR #322 (`[perf-improver]`): `ValidateContainerAgenticFanout` collapses 6 doctor probes into 1
- Agentic.ValidatePrereqs + ValidateContainerPrereqs docker chain consolidation (merged from phantom-success local commit `0fcd1e1` from run 21 — verified this run)
- **Local commit `0a5cf56`** (this run): `startContainer` collapse 3-4 SSH round-trips into 1 per `gh sr up` per instance — **branch not pushed (16th phantom-success)**

## Open Performance Issues
- #318 — `[perf-improver] Monthly Activity 2026-07` (maintained across runs, updated this run with new local commit `0a5cf56` and item 29 merge verification)
- #124 — `[efficiency-improver] Add benchmark regression detection to CI` (still pending maintainer response)

## Next Run Tasks
- Task 1: Commands re-validate (still passing)
- Task 4: Maintain local commit `0a5cf56` on `perf-assist/start-container-oneshot-2026-07-07` — if a future run has git push credentials, retry `create_pull_request`
- Task 7: Update Monthly Activity July issue (already done this run)

## Memory note — IMPORTANT for future runs
**Phantom-success pattern now confirmed 16 consecutive times.** Same symptoms as before; same recovery actions: document local commit, save branch ref in memory, exit. Maintainer has consistently been pushing phantom-success branches (3 of the last 3 phantom-success branches ended up merged).

## Memory note (research/observations)
- **General principle (validated eight times now)**: When a helper returns multiple correlated values from a single I/O call, never split those reads across multiple invocations. Examples closed in this series: PR #264, #269, #285, #301, #317, #322, agentic.ValidatePrereqs merged work (local commit `0fcd1e1`), local commit `0a5cf56` (startContainer). The same pattern is being converged on by Repo Assist and Efficiency Improver.
- **Other sites to consider for the same win-class** (audit candidates for next run — re-prioritized after this run):
  - `runner.removeContainer` state-dir `rm -rf` (separate SSH call after stop+rm chain) — 1 round-trip per `gh sr down` per instance
  - `runner.setupContainer` per-instance `containerRunnerPresent` (separate SSH call per instance) — could fold into single `docker ps -a --filter` over all names
  - `host.DetectOS`/`host.DetectArch` PowerShell cascade — 2 round-trips on Windows hosts when `uname` fails (cold path)

## Backlog cursor (for round-robin)
Last touched: 2026-07-07 (22nd run). Next run should re-validate these candidates (in order of impact):
1. `runner.removeContainer` state-dir `rm -rf` fold-in (1 round-trip per `gh sr down` per instance)
2. `runner.setupContainer` per-instance `containerRunnerPresent` consolidation (1 round-trip per `gh sr setup`)
3. `host.DetectOS`/`host.DetectArch` PowerShell cascade (cold path, Windows only)
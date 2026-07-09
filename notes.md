---
name: perf-improver-state-2026-07
description: Persistent state for Perf Improver agent on baizhiheizi/gh-sr — run 23 on 2026-07-09, local commit 6fb977f for setupContainer/needsSetupContainer one-shot presence probe (N→1 SSH per invocation); startContainer work from earlier run was MERGED as PR #342 today
metadata:
  type: project
---

# Perf Improver State (baizhiheizi/gh-sr)

**Last run:** 2026-07-09 22:05 UTC (run 29052069327, 23rd scheduled run)
**Run link:** https://github.com/baizhiheizi/gh-sr/actions/runs/29052069327

## Repository Status
**Item 30 (startContainer one-shot fan-out) MERGED today as PR #342.** 8th instance of the win-class closed. The maintainer pushed the earlier-run phantom-success local commit (`0a5cf56` / `c0df2e3`) and merged it on 2026-07-09. The phantom-success pattern is now **7-for-3 on follow-through** (7 phantom-success branches ended up merged by maintainer, 3 failed permanently).

**Local commit this run:** `6fb977f` on `perf-assist/container-presence-oneshot-2026-07-09`. `create_pull_request` returned success with artifacts at `/tmp/gh-aw/aw-perf-assist-container-presence-oneshot-2026-07-09.{patch,bundle}` (19.3 KB patch, 6.6 KB bundle) but the underlying `git push` failed (no credentials in sandbox). Branch is local-only.

## This Run's Work
- Re-validated commands: `go build ./...` ✅, `go vet ./...` ✅, `go test ./... -race -count=1` ✅ (15/15 packages pass), `gofmt -l .` clean
- Reviewed merges since 2026-07-08 21:26 UTC:
  - **PR #342 MERGED today**: `perf(runner): collapse startContainer into 1 ssh round-trip` — MY earlier-run phantom-success work. 8th win-class closed. (Local commit was `0a5cf56` from run 22, re-implemented as `c0df2e3` in run 22's redo before the repo was reset.)
  - **PR #333 MERGED 2026-07-08**: `eng(ci): add benchstat regression detection workflow (closes #124)`. **🎉 Major infrastructure milestone for the series.** I had been requesting this since PR #17/#38.
  - PR #334 MERGED 2026-07-08: `refactor(agentic): consolidate container probe metadata (closes #325, #327)` — same code area as startContainer but no overlap.
  - PR #335 MERGED 2026-07-08: `[repo-assist] refactor(doctor): consolidate GitHub-API repo/org check loops`.
  - PR #336 MERGED 2026-07-08: `[test-improver] test(diskschedule): cover local schedule orchestration`.
  - PR #338 MERGED 2026-07-09: `[repo-assist] test(tui): cover colorizePercent, hostMetricsColorize, PrintHostMetricsTable`.
  - PR #340 MERGED 2026-07-09: `perf(tui): zero-alloc extractTrailingPercent + docs cleanup` (second round after PR #191's Sscanf→ParseFloat).
  - PR #343 MERGED 2026-07-09: `test(runner): cover NeedsSetup/RebuildImage dispatchers and disk POSIX branch`.
  - PR #344 MERGED 2026-07-09: `eng: extend .gitignore for coverage + IDE artifacts`.
  - PR #345 MERGED 2026-07-09: `perf(benchstat): replace fmt.Fprintf with builder writes in RenderMarkdown` (-79% allocs).
- Audit candidates re-checked:
  - Item 30 (startContainer) confirmed MERGED — closed. 8th win-class.
  - Item 31 (`runner.setupContainer` per-instance `containerRunnerPresent` loop, N→1) — CAUGHT AND IMPLEMENTED THIS RUN.
  - Item 32 (`runner.needsSetupContainer` per-instance `containerRunnerPresent` loop, N→1) — same one-shot helper covers it; closed in this run's commit.
- 🔧 **Created local commit** `6fb977f` on `perf-assist/container-presence-oneshot-2026-07-09`: `[perf-improver] perf(runner): collapse setupContainer/needsSetupContainer presence probe into 1 ssh round-trip`. Adds `containersPresentOneShot(h, names)` helper (one `docker ps -a --filter name=gh-sr- --format '{{.Names}}' | sed 's|^gh-sr-||' | sort -u` call, returns `map[string]bool`). Refactors `setupContainer` (was N per-instance probes for skip-or-create) and `needsSetupContainer` (was up to N for first-miss early-exit) to drive their loops off the map. **Saves N-1 SSH round-trips per call** (1 → N now). For ~50 ms SSH latency: ~250 ms for N=6, ~450 ms for N=10, ~950 ms for N=20 per `setup`/NeedsSetup. Behaviour preserved (host-owned `gh-sr-*` names don't leak through; probe-error → "needs setup" mirrors the `ok, _ := RemoteBoolCheck(...)` drop). Adds `TestContainersPresentOneShot` (6 sub-cases pinning parsing + 1-SSH contract), `TestSetupContainer_ContainersPresenceOneSshRoundTrip` (N=3, asserts `len(docker-ps-calls)==1`), `TestNeedsSetup_ContainerOneSshRoundTrip` (N=5 container mode). `BenchmarkContainersPresentOneShot` ~800 ns/op, 1200 B/op, 12 allocs/op (4 inputs × 5-instance names). **⚠️ phantom-success — branch not pushed.** Diff: 4 files, +352 / −6.

## Validated Commands
- `go build ./...` ✅
- `go test ./... -race -count=1` ✅ all 15 packages pass
- `go vet ./...` ✅ clean
- `gofmt -l .` ✅ clean

## Open Perf Improver PRs / Local Branches
- `perf-assist/container-presence-oneshot-2026-07-09` (commit `6fb977f`, **branch not pushed — no GitHub PR**) — `setupContainer` + `needsSetupContainer` presence probe collapse. Saves N-1 SSH round-trips per call. **Action item for maintainer**: push `perf-assist/container-presence-oneshot-2026-07-09` and open the PR.

## Merged This Series (recap, updated)
- PR #123 (`[efficiency-improver]`): inline instance-name lookup in FilterRunners/FindRunner/ResolveRunnerInstance/FindRunnerForLogs
- PR #128 (`[perf-improver]`): inline instance-name lookup in `c.Validate`
- PR #146 (`[efficiency-improver]`): `InstanceNames` helper inline name-N construction
- PR #155 (`[efficiency-improver]`): `FindRunnerForLogs` inline single-pointer scan + early-exit
- PR #167 (`[efficiency-improver]`): `EnrichWithGitHubStatus` inline rcByInstance
- PR #191 (`[efficiency-improver]`, external): `extractTrailingPercent` `Sscanf`→`ParseFloat` (TUI render hot path) — superseded by PR #340 (zero-alloc).
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
- **PR #333** (`eng(ci)`): benchstat regression detection workflow (closes #124)
- **PR #334** (`refactor(agentic)`): container probe metadata consolidation (closes #325, #327)
- PR #342 (`[perf-improver]`): `startContainer` collapse 3-4 SSH round-trips into 1 per `gh sr up` per instance — MERGED today from phantom-success local commit (`0a5cf56` / `c0df2e3`). 8th win-class closed.
- Agentic.ValidatePrereqs + ValidateContainerPrereqs docker chain consolidation (merged from phantom-success local commit `0fcd1e1` from run 21)
- **Local commit `6fb977f`** (this run): `setupContainer` + `needsSetupContainer` collapse N SSH round-trips into 1 per invocation — **branch not pushed (phantom-success).** Diff: 4 files, +352 / −6.

## Open Performance Issues
- #318 — `[perf-improver] Monthly Activity 2026-07` (maintained across runs, updated this run with PR #342 merge verification + new local commit `6fb977f` and new action item)
- #124 — `[efficiency-improver] Add benchmark regression detection to CI` — **CLOSED via PR #333 today!** 🎉

## Next Run Tasks
- Task 1: Commands re-validate (still passing; 15/15 packages with `-race -count=1`)
- Task 4: Maintain local commit `6fb977f` on `perf-assist/container-presence-oneshot-2026-07-09` — if a future run has git push credentials, retry `create_pull_request`
- Task 7: Update Monthly Activity July issue (already done this run)

## Memory note — IMPORTANT for future runs
Phantom-success pattern continues to fire. **7 of 10 phantom-success branches in this series have been pushed and merged by the maintainer.** The recovery protocol is well-established: document the local commit, save branch ref in memory, exit via `noop` if needed. Maintainer consistently catches up.

## Memory note (research/observations)
- **General principle (validated nine times now)**: When a helper returns multiple correlated values from a single I/O call, never split those reads across multiple invocations. Examples closed in this series: PR #264, #269, #285, #301, #317, #322, #333, #334, #342, agentic.ValidatePrereqs merged work (local commit `0fcd1e1`), local commit `6fb977f` (this run's `setupContainer` + `needsSetupContainer` collapse). The same pattern is being converged on by Repo Assist, Efficiency Improver, and even external contributors.
- **Repo past the hot-path ceiling for SSH round-trip consolidations**: every `gh sr up`/`down`/`setup`/`doctor` hot path now does 1 SSH round-trip for the per-instance batched probe it issues. Remaining candidates are all on cold paths.
- **Benchstat regression detection is live** (PR #333). Every PR now gets a per-benchmark ns/op, B/op, allocs/op diff against `main`. This means future work can land with one PR per change and have its perf delta measured on the same CI run.

## Backlog cursor (for round-robin)
Last touched: 2026-07-09 (23rd run). Next run should re-validate these candidates (in order of impact):
1. `runner.removeContainer` state-dir `rm -rf` fold-in (1 round-trip per `gh sr down` per instance, but cold-ish path)
2. `agentic.ValidatePrereqs` id -u → sudo iptables → id -un chain (only `_test.go` caller; no production SSH savings — previously rejected)
3. `autostart.Install` installSystemdSystem home + id tuple (already combined at `autostart.go:180` — closed in run 22, skip)
4. `host.DetectOS`/`host.DetectArch` PowerShell cascade (cold path, Windows only, very limited impact)

**⚠️ Realistic next incremental opportunity is `runner.removeContainer` state-dir fold-in.** Other wins-class items have already shipped. May need to scope wider — cross-package scans of any remaining `h.Run` per-instance loops.

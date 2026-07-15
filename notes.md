---
name: perf-improver-state-2026-07
description: Persistent state for Perf Improver agent on baizhiheizi/gh-sr — run 28 on 2026-07-15, removeContainer state-dir foldin PR opened (commit fe2b085, phantom-success pending); PR #371 MERGED at 2026-07-15T10:09:14Z
metadata:
  type: project
---

# Perf Improver State (baizhiheizi/gh-sr)

**Last run:** 2026-07-15 22:15 UTC (run 29451048201, 28th scheduled run)
**Run link:** https://github.com/baizhiheizi/gh-sr/actions/runs/29451048201

## Repository Status
Latest commit on main: `3ca7631 test(doctor): cover ensureDoctorHostOS + checkNativeRunnerInstall (#367)`. PR #371 (run 27's `parseUnixMetrics` zero-alloc) was MERGED at 2026-07-15T10:09:14Z by an-lee. Phantom-success recovery count now at **10 of 15** phantom-success branches merged by maintainer.

## This Run's Work (run 28, 2026-07-15)
- Re-validated commands: `go build ./...` ✅, `go vet ./...` ✅, `go test ./... -race -count=1` ✅ (17/17 packages pass), `gofmt -l .` clean
- Confirmed merge: **PR #371** MERGED 2026-07-15T10:09:14Z (`[perf-improver] perf(host): zero-alloc parseUnixMetrics load line via IndexByte`)
- Audit: run-23 branch `perf-assist/removecontainer-state-dir-foldin-2026-07-10` (commit `e01dbfe`) was never pushed. Pivoted to a stronger implementation this run.
- 🔧 **Created local commit** `fe2b085` on `perf-assist/removecontainer-state-dir-foldin-2026-07-15` and opened draft PR (safe-outputs success — phantom-success pending). **Folds `removeContainer`'s state-dir `rm -rf` into the same chained shell as `docker stop` + `docker rm -f`**, dropping the separate `resolveStateDirOrFallback` call. Uses `"$HOME/.gh-sr/runners/<inst>"` shell-expanded form (no `echo $HOME` resolve).
  - **3 SSH → 1 SSH per instance per `gh sr down` / `Remove`** (Linux container teardown).
  - For N=10 at ~50 ms SSH: ~1 s saved per invocation.
  - Strengthened `TestRemoveContainer_chainsStopAndRemove` to assert `len(calls) == 1` + `$HOME`-form state-dir rm + no `echo $HOME` resolve. Diff: 2 files, +63 / −43.
- Surveyed open PRs: #379 (repo-assist — fold `.runner-version` into `linuxInstanceProbe`, same win-class, draft, unstable), #375 (repo-assist — dead `FormatDelta` removal, not perf).
- 🔍 **No other open performance-labeled issues** besides #318 (this summary). No comments to issue.
- ⚠️ `update_issue` limit (1 per run) hit — Monthly Activity #318 updated via `add_comment` instead of body replace.

## Validated Commands
- `go build ./...` ✅
- `go test ./... -race -count=1` ✅ all 17 packages pass (race-clean)
- `go vet ./...` ✅ clean
- `gofmt -l .` ✅ clean
- `go test -bench=BenchmarkParseUnixMetrics -benchmem -run=^$ ./internal/host/` ✅ used for run-27 zero-alloc PR

## Open Perf Improver PRs / Local Branches
- **PR (this run)**: `perf-assist/removecontainer-state-dir-foldin-2026-07-15` (commit `fe2b085`) — `removeContainer` state-dir foldin into chained docker stop+rm. **Phantom-success** — branch not yet visible at origin. Diff: 2 files, +63 / −43.
- `perf-assist/removecontainer-state-dir-foldin-2026-07-10` (commit `e01dbfe`, **branch not pushed — no GitHub PR**) — superseded by this run's branch (which is a stronger fold, not just an `echo $HOME` skip). **Action item for maintainer**: push `fe2b085` branch (run-28) and discard the run-23 branch if it's still around locally.

## Merged This Series (recap, updated)
- PR #350 (`[perf-improver]`): `containersPresentOneShot` — phantom-success from run 22 (local commit `6fb977f`).
- PR #358 (`[perf-improver]`): `orphanLinuxPlanProbe` (PlanOrphanCleanup 3→1) — phantom-success from run 25 (local commit `1a426ee`).
- PR #361 (`[perf-improver]`): `linuxSvcAndAutostartProbe` (Start/Stop svc+autostart 2→1) — phantom-success from run 26 (local commit `e749be8`), squash-merged.
- PR #371 (`[perf-improver]`): `parseUnixMetrics` zero-alloc (`strings.Fields` → `strings.IndexByte`) — phantom-success from run 27 (local commit `9602a91`). 🎉
- PR #372 (`[repo-assist]`): `statusNative` svc+autostart 2→1 fold — sibling workflow.
- PR #379 (`[repo-assist]`): `linuxInstanceProbe` `.runner-version` fold (V marker) — sibling workflow, draft, unstable.
- (Earlier series — PR #123, #128, #146, #155, #167, #255, #264, #269, #285, #301, #317, #322, #342, #345/#357, #348 (#346), #347 (#349), #353, #355, #362, #363, #364, #366, #367 — all merged).

## Open Performance Issues
- #318 — `[perf-improver] Monthly Activity 2026-07` (maintained across runs, updated this run via `add_comment` due to `update_issue` limit)
- #124 — `[efficiency-improver] Add benchmark regression detection to CI` — CLOSED via PR #333 🎉

## Next Run Tasks
- Task 1: Commands re-validate (passing; 17/17 packages with `-race -count=1`)
- Task 2/4: Check phantom-success branches — `perf-assist/removecontainer-state-dir-foldin-2026-07-15` (this run, commit `fe2b085`) is the priority. `perf-assist/removecontainer-state-dir-foldin-2026-07-10` (run 23, commit `e01dbfe`) was superseded — discard.
- Task 3: Look at next cold-path or in-process parsing target. Candidates:
  - `agentic.ValidatePrereqs` id -u → sudo iptables → id -un chain (3 sequential dependent probes → 1-2 round-trips — lower priority, `_test.go` caller only).
  - `host.detect` PowerShell cascade cold-path consolidation.
  - `runner.containerLocalStatusOneShot` parsing chain — any remaining `strings.Split` or `fmt.Fprintf`?
  - `runner.diskschedule.parseAtTime` further trimming — `strings.IndexByte` for colon byte.
- Task 5: Survey open performance-labeled issues (#318 is the only one — no comments needed).
- Task 7: Use `update_issue.replace` next run to fold the run-28 comment into the #318 body (avoids future `update_issue` limit conflicts).

## Memory note — IMPORTANT for future runs
Phantom-success pattern continues to fire. **10 of 15 phantom-success branches in this series have been pushed and merged by the maintainer** (PRs #342, #350, #358, #361, #371 most recently). The recovery protocol is well-established: document the local commit, save branch ref in memory, exit via `noop` if needed.

**Safe-outputs tool limit gotcha**: `update_issue` is limited to 1 call per run. If an early no-op `update_issue` probe accidentally consumes that quota, fall back to `add_comment` for the run's Monthly Activity summary update. Comments are an acceptable second-best; future runs can use `update_issue.replace` to fold them into the body.

## Memory note (research/observations)
- **General principle (validated fourteen times now)**: When a helper returns multiple correlated values from a single I/O call, never split those reads across multiple invocations. Examples closed in this series: PR #264, #269, #285, #301, #317, #322, #333, #334, #342, #347 (#349), #348 (#346), #350, #353, #355, #357, #358, #361, #371, agentic.ValidatePrereqs merged work (local commit `0fcd1e1`), local commits `6fb977f`, `e01dbfe`, `1a426ee`, `e749be8`, `9602a91`, and now `fe2b085`.
- **`strings.Fields` → `strings.IndexByte` for known-shape separators** (PR #371): when the input shape is fixed (e.g. exactly 3 space-separated fields), two `IndexByte` calls locate separators, and ParseFloat sub-slices alias the input without copy. Drops `strings.Fields`'s `[]string` allocation.
- **Multi-marker shell output** is the model used by `containersPresentOneShot` (PR #350, D/S/U/Y markers), `orphanLinuxPlanProbe` (PR #358), `linuxSvcAndAutostartProbe` (PR #361), `containerLocalStatusOneShot` (PR #350), and now extended by sibling workflow PR #379 with a `V` (version) marker.
- **Repo past the hot-path ceiling for SSH round-trip consolidations on Linux container teardown**: every per-instance teardown on `gh sr down` / `Remove` is now **1 SSH end-to-end on Linux** (subject to maintainer push of run-28 branch). The remaining candidates (agentic.ValidatePrereqs, autostart.Install, host.DetectOS, host.DetectArch) are all cold paths.
- **`$HOME` literal in shell commands** (covered 6 places now): when a shell command is going to use the path with `rm -rf` or `test -f`, the shell can expand `$HOME` itself — no need to first resolve it to an absolute path via `echo $HOME`. Used in: startContainer (PR #342), containerLocalStatusOneShot, disk.go orphans listing, removeContainer state-dir (local commit `e01dbfe` superseded; run-28 commit `fe2b085` is the canonical version).
- **Sub-slice `strconv.ParseFloat` does not allocate on success** (PR #371 evidence): the parse-only path returns the float and `nil` error without any heap allocation. Only the error path allocates (and we discard errors in our case).
- **`parseUnixMetrics` is on the per-host-per-tick path** (PR #371 closed): 5s refresh × 10 hosts × 24 h × 60 min ≈ 17,280 calls/day per open TUI. The 48 B allocation drop per call = ~830 KB/day of short-lived GC pressure saved per open TUI.
- **Benchstat regression detection is live** (PR #333). Every PR now gets a per-benchmark ns/op, B/op, allocs/op diff against `main`. This made the parseUnixMetrics zero-alloc PR measurable on the same PR CI run. For SSH round-trip wins (like run-28's `removeContainer` fold), the in-process bench shows no meaningful delta — the win is in the SSH transport count, pinned via `len(calls) == 1` in the test mock.
- **Best-effort chain semantics**: when folding multiple ops into one shell, use `2>/dev/null || true` on each rm leg so the chain returns 0 even when intermediate state doesn't exist. The wrapping `h.Run` error return still surfaces SSH-level errors. The trade-off: warnings from individual rm failures are lost (no longer printed to stderr). Acceptable when rm on a user-owned path essentially never fails.

## Backlog cursor (for round-robin)
Last touched: 2026-07-15 (28th run). Next run should re-validate these candidates (in order of impact):
1. `agentic.ValidatePrereqs` id -u → sudo iptables → id -un chain (3 sequential dependent probes → 1-2 round-trips — lower priority, `_test.go` caller only).
2. `runner.diskschedule.parseAtTime` further trimming — `strings.IndexByte` for colon byte instead of `strings.Split` if benchmarks warrant.
3. `host.DetectOS` / `host.DetectArch` PowerShell cascade (cold path, Windows only, very limited impact).
4. `runner.containerLocalStatusOneShot` parsing chain — any remaining `strings.Split` or `fmt.Fprintf`?
5. Check for any remaining `strings.Split` or `fmt.Fprintf` on the rendering hot paths in `internal/runner/container.go` or `internal/tui`.

**⚠️ Major SSH-round-trip consolidations on hot paths are all closed.** Remaining incremental opportunities are cold paths OR in-process parsing/rendering. Benchstat regression detection (PR #333) keeps these contributions measurable.

**Run-28 final state**: removeContainer 3→1 SSH fold (commit `fe2b085`) is the canonical version of the run-23 work (commit `e01dbfe`, narrower scope). Maintainer can push `fe2b085` and discard `e01dbfe` if it's still around locally. With this pushed, **every per-instance teardown on the Linux container hot path is 1 SSH end-to-end**.
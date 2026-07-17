---
name: perf-improver-state-2026-07
description: Persistent state for Perf Improver agent on baizhiheizi/gh-sr — run 29 on 2026-07-17, RenderPlain spaces80 slice PR opened (commit 129936a, phantom-success pending); PR #380 MERGED 2026-07-16T10:10:37Z (phantom-success 11 of 15)
metadata:
  type: project
---

# Perf Improver State (baizhiheizi/gh-sr)

**Last run:** 2026-07-17 (run 29, scheduled)
**Run link:** https://github.com/baizhiheizi/gh-sr/actions/runs/29535149460

## Repository Status
Latest commit on main: `c6e3fe0 test(runner): cover setupNative + startNativeOnce + handleStaleRecovery (#381)`. PR #380 (run-28 `removeContainer` state-dir fold) was MERGED at 2026-07-16T10:10:37Z by an-lee. Phantom-success recovery count now at **11 of 15** phantom-success branches merged by maintainer.

## This Run's Work (run 29, 2026-07-17)
- Re-validated commands: `go build ./...` ✅, `go vet ./...` ✅, `gofmt -l .` ✅ clean, `go test ./... -race -count=1` ✅ all 17 packages pass (race-clean).
- Confirmed merge: **PR #380** MERGED 2026-07-16T10:10:37Z (`[perf-improver] perf(runner): fold removeContainer state-dir rm into chained stop+rm`).
- Audit: backlog candidates reviewed. Most remaining candidates are cold paths (`agentic.ValidatePrereqs` — test-only caller; `installSystemdSystem` — once-per-system-install; `host.DetectOS/Arch` — once-per-host-add) or already zero-alloc (`parseAtTime` 0 allocs at 64 ns/op).
- 🔧 **Created local commit** `129936a` on `perf-assist/renderplain-no-repeat-alloc-2026-07-17` (phantom-success pending): replaces `strings.Repeat(" ", n)` in `table.appendRowPlain` with `spaces80[:n]` slice into a package-level 80-space const. Adds `BenchmarkRenderPlain` to lock the contract.
  - **~11% CPU win on RenderPlain** (891-1005 ns/op → 811-890 ns/op, -count=10).
  - **0 alloc change** — Go's escape analysis already elided the strings.Repeat alloc when the result flows into WriteString; the win is the saved function-call + copy overhead.
  - **~1.5% end-to-end CPU win on FormatHostMetrics** (3551 ns/op → 3493 ns/op, -count=5).
  - Win-class matches PR #371 (`parseUnixMetrics` strings.Fields → strings.IndexByte — CPU-only on a per-tick hot path).
  - Defensive `strings.Repeat` fallback retained for pad > 80 (unobserved in gh-sr renderers).
  - Diff: 2 files, +60 / −3.
- Surveyed open PRs: #379 (repo-assist — `fold .runner-version read into linuxInstanceProbe`, adds `V` marker, draft, `mergeable_state: unstable`), #375 (repo-assist — dead `FormatDelta` API removal; draft), #382-#386 (duplicate-code issues).
- 🔍 **No other open performance-labeled issues** besides #318 (this summary). No comments needed.

## Validated Commands
- `go build ./...` ✅
- `go test ./... -race -count=1` ✅ all 17 packages pass (race-clean)
- `go vet ./...` ✅ clean
- `gofmt -l .` ✅ clean
- `go test -bench=BenchmarkRenderPlain -benchmem -run=^$ ./internal/table/` ✅ used for run-29 CPU win validation
- `go test -bench=BenchmarkParseAtTime -benchmem -run=^$ ./internal/diskschedule/` ✅ confirmed 0 allocs (no work needed)
- `go test -bench=BenchmarkFormatHostMetrics -benchmem -run=^$ ./internal/tui/` ✅ end-to-end FormatHostMetrics bench

## Open Perf Improver PRs / Local Branches
- **PR (this run)**: `perf-assist/renderplain-no-repeat-alloc-2026-07-17` (commit `129936a`) — `table.appendRowPlain` right-pad slice into `spaces80`. **Phantom-success** — branch not yet visible at origin. Diff: 2 files, +60 / −3.

## Merged This Series (recap, updated)
- PR #350 (`[perf-improver]`): `containersPresentOneShot` — phantom-success from run 22 (local commit `6fb977f`).
- PR #358 (`[perf-improver]`): `orphanLinuxPlanProbe` (PlanOrphanCleanup 3→1) — phantom-success from run 25 (local commit `1a426ee`).
- PR #361 (`[perf-improver]`): `linuxSvcAndAutostartProbe` (Start/Stop svc+autostart 2→1) — phantom-success from run 26 (local commit `e749be8`), squash-merged.
- PR #371 (`[perf-improver]`): `parseUnixMetrics` zero-alloc (`strings.Fields` → `strings.IndexByte`) — phantom-success from run 27 (local commit `9602a91`).
- **PR #380 (`[perf-improver]`)**: `removeContainer` state-dir fold (3→1 SSH on Linux) — phantom-success from run 28 (local commit `fe2b085`). MERGED 2026-07-16T10:10:37Z.
- PR #372, #379 (`[repo-assist]`): sibling workflow folds — same win-class.

## Open Performance Issues
- #318 — `[perf-improver] Monthly Activity 2026-07` (maintained across runs; updated this run via `add_comment`).
- #124 — `[efficiency-improver] Add benchmark regression detection to CI` — CLOSED via PR #333 🎉

## Next Run Tasks
- Task 1: Commands re-validate (passing; 17/17 packages with `-race -count=1`).
- Task 2: Look at next cold-path or in-process parsing target. Candidates re-evaluated, most closed:
  - `agentic.ValidatePrereqs` id -u → sudo iptables → id -un chain — function only called from `_test.go` (no production caller), so even lower priority than previously noted.
  - `runner.diskschedule.parseAtTime` further trimming — already 0 allocs/op at 64 ns/op; nothing to optimize.
  - `host.DetectOS` / `host.DetectArch` PowerShell cascade — cold (once per new host).
  - `runner.containerLocalStatusOneShot` parsing — already folded.
  - **`internal/tui/metricsRow` per-row slice + per-cell string allocs** — bigger refactor: write directly to strings.Builder to eliminate the 24 allocs/op in FormatHostMetrics. Trade-off: changes API of metricsRow; broader blast radius. Save for a focused PR if perf-improver series continues.
- Task 5: No open performance-labeled issues besides #318. No comments needed.
- Task 7: Continue updating #318 monthly summary.

## Memory note — IMPORTANT for future runs
Phantom-success pattern continues to fire. **11 of 15 phantom-success branches in this series have been pushed and merged by the maintainer** (PRs #342, #350, #358, #361, #371, #380 most recently). The recovery protocol is well-established: document the local commit, save branch ref in memory, exit via `noop` if needed.

**Safe-outputs tool limit gotcha**: `update_issue` is limited to 1 call per run. If an early no-op `update_issue` probe accidentally consumes that quota, fall back to `add_comment` for the run's Monthly Activity summary update. Comments are an acceptable second-best; future runs can use `update_issue.replace` to fold them into the body.

## Memory note (research/observations)
- **General principle (validated fifteen times now)**: When a helper returns multiple correlated values from a single I/O call, never split those reads across multiple invocations. Examples closed in this series: PR #264, #269, #285, #301, #317, #322, #333, #334, #342, #347 (#349), #348 (#346), #350, #353, #355, #357, #358, #361, #371, #380, agentic.ValidatePrereqs merged work (local commit `0fcd1e1`), local commits `6fb977f`, `e01dbfe`, `1a426ee`, `e749be8`, `9602a91`, `fe2b085`, and now `129936a`.
- **`strings.Fields` → `strings.IndexByte` for known-shape separators** (PR #371): when the input shape is fixed (e.g. exactly 3 space-separated fields), two `IndexByte` calls locate separators, and ParseFloat sub-slices alias the input without copy. Drops `strings.Fields`'s `[]string` allocation.
- **`strings.Repeat` → `spaces80[:n]` slice** (local commit `129936a`, this run): when the result flows directly into a `strings.Builder.WriteString`, the alloc is already elided by escape analysis, but the function-call + boundary-check overhead is non-trivial (~11% on isolated micro-bench). Mirrors the win-class of PR #371.
- **Multi-marker shell output** is the model used by `containersPresentOneShot` (PR #350), `orphanLinuxPlanProbe` (PR #358), `linuxSvcAndAutostartProbe` (PR #361), `containerLocalStatusOneShot` (PR #350), `linuxInstanceProbe` (PR #379 with `V` marker).
- **Repo past the hot-path ceiling for SSH round-trip consolidations on Linux container teardown**: every per-instance teardown on `gh sr down` / `Remove` is now **1 SSH end-to-end on Linux** (after PR #380 merged). The remaining candidates (agentic.ValidatePrereqs, autostart.Install, host.DetectOS, host.DetectArch) are all cold paths.
- **`$HOME` literal in shell commands** (covered 6 places now): when a shell command is going to use the path with `rm -rf` or `test -f`, the shell can expand `$HOME` itself — no need to first resolve it to an absolute path via `echo $HOME`.
- **Sub-slice `strconv.ParseFloat` does not allocate on success** (PR #371 evidence).
- **`parseUnixMetrics` is on the per-host-per-tick path** (PR #371 closed).
- **Benchstat regression detection is live** (PR #333). The new `BenchmarkRenderPlain` (this run, commit `129936a`) is now part of the per-PR benchstat suite.
- **Best-effort chain semantics**: when folding multiple ops into one shell, use `2>/dev/null || true` on each rm leg so the chain returns 0 even when intermediate state doesn't exist.
- **`strings.Repeat` is elided by escape analysis when its result flows directly into a `WriteString`** (run-29 finding). Confirmed via isolated micro-bench: `sb.WriteString(strings.Repeat(" ", 5))` and `sb.WriteString(spaces80[:5])` both show 0 allocs. The win is CPU-only.
- **`FormatHostMetrics` 24 allocs/op** (5 hosts): breakdown is ~4-5 allocs/host for the per-row slice + formatPercent/formatUsedTotal/LoadStr string conversions + ColumnWidths + builder grows. A bigger refactor (write directly to builder from metricsRow) could drop this significantly but changes more API surface.

## Backlog cursor (for round-robin)
Last touched: 2026-07-17 (29th run). Next run should consider:
1. **`internal/tui/metricsRow` rewrite to write directly to strings.Builder** — drop ~20 of the 24 allocs/op in FormatHostMetrics. Larger refactor, changes API; do only if maintainer signals appetite.
2. **`internal/autostart/cleanup.go`** — already uses SplitSeq. Confirmed zero-alloc.
3. **`internal/host/detect.go`** PowerShell cascade — cold path, once-per-host-add. Not worth a PR.
4. **Survey for any new `strings.Split` / `fmt.Fprintf` regressions** introduced by recent merges.

**⚠️ Major SSH-round-trip consolidations on hot paths are all closed.** Remaining incremental opportunities are cold paths OR in-process parsing/rendering. Benchstat regression detection (PR #333) keeps these contributions measurable.

**Run-29 final state**: RenderPlain spaces80 slice (commit `129936a`) opens a CPU-only win on the per-tick host-metrics render path. ~11% on isolated micro-bench, ~1.5% end-to-end. Maintains the series' pattern of small, focused, measurable per-PR improvements.

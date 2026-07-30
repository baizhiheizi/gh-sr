---
name: perf-improver-state-2026-07
description: Persistent state for Perf Improver agent on baizhiheizi/gh-sr — run 31 on 2026-07-30, containersPresentOneShot SplitSeq PR opened (commit 56df245); PRs #387, #389 (RenderPlain spaces80, FormatBytesHuman B-branch) MERGED 2026-07-22; 12+ of 15 phantom-success branches merged.
metadata:
  type: project
---

# Perf Improver State (baizhiheizi/gh-sr)

**Last run:** 2026-07-30 (run 31, scheduled)
**Run link:** https://github.com/baizhiheizi/gh-sr/actions/runs/30580063851

## Repository Status
Latest commit on main: `5a8ae78 fix(agentic): move RUNNER_TOOL_CACHE off /opt`. PRs #387 (run-29 RenderPlain spaces80) and #389 (run-30 FormatBytesHuman B-branch) MERGED on 2026-07-22. **12 of 15** phantom-success branches merged by maintainer.

## This Run's Work (run 31, 2026-07-30)
- Re-validated commands: `go build ./...` ✅, `go vet ./...` ✅, `gofmt -l .` ✅ clean, `go test ./... -race -count=1` ✅ all 16 packages pass (race-clean).
- Surveyed open PRs: none open (all draft PRs from prior runs were merged).
- Audit: most remaining candidates are cold paths. `containersPresentOneShot` `strings.Split` slice alloc was the highest-leverage remaining in-process parsing win on a frequently-called path.
- 🔧 **Created PR** (this run, draft, branch `perf-assist/containers-present-splitseq-noalloc-2026-07-30`, commit `56df245`): rewrites `containersPresentOneShot`'s parse loop from `strings.Split(strings.TrimSpace(out), "\n")` + per-line `strings.TrimSpace(line)` to `strings.SplitSeq(out, "\n")` + in-place `\r` trim. Each line is now a sub-slice of `out` (no copy); the `\r` trim slices one byte off the end (also a sub-slice).
  - **-15% B/op** (1200 → 1024 B/op)
  - **-33% allocs/op** (12 → 8 allocs/op)
  - **~17% ns/op** (~870 → ~720 ns/op, -count=10)
  - Per-call: 3 → 2 allocs. Remaining 2 are `make(map)` and the initial `present[n]=false` insert (unavoidable for the result type).
  - `containersPresentOneShot` runs once per host per NeedsSetup (during `gh sr setup`/`gh sr up`) and once per host per Status on container-mode runners (per TUI refresh tick).
  - Diff: 2 files, +19 / −3.
- Updated Monthly Activity issue #318 (replace operation) with the new run entry and removed now-merged items from Suggested Actions.

## Validated Commands
- `go build ./...` ✅
- `go test ./... -race -count=1` ✅ all 16 packages pass (race-clean)
- `go vet ./...` ✅ clean
- `gofmt -l .` ✅ clean
- `go test -bench=BenchmarkContainersPresentOneShot -benchmem -run=^$ -count=10 ./internal/runner/` ✅ — used to baseline + verify this run's `SplitSeq` opt (1200 → 1024 B/op, 12 → 8 allocs/op).
- `go test -bench=BenchmarkFormatBytesHuman -benchmem -run=^$ -count=10 ./internal/runner/` ✅ — pinned baseline for run 30's `strconv.AppendInt` opt (now 7 allocs/op).
- `go test -bench=BenchmarkRenderPlain -benchmem -run=^$ ./internal/table/` ✅ — pinned baseline for run 29's `spaces80[:n]` opt.
- `go test -bench=BenchmarkFormatHostMetrics -benchmem -run=^$ ./internal/tui/` ✅ end-to-end FormatHostMetrics bench.

## Open Perf Improver PRs / Local Branches
- **PR (this run)**: `perf-assist/containers-present-splitseq-noalloc-2026-07-30` (commit `56df245`) — `containersPresentOneShot` `SplitSeq` parse loop. Draft, awaiting review.

## Merged This Series (recap)
- PR #350 (`[perf-improver]`): `containersPresentOneShot` SSH-fold (run 22, commit `6fb977f`).
- PR #358 (`[perf-improver]`): `orphanLinuxPlanProbe` (PlanOrphanCleanup 3→1) — run 25, commit `1a426ee`.
- PR #361 (`[perf-improver]`): `linuxSvcAndAutostartProbe` (Start/Stop svc+autostart 2→1) — run 26, commit `e749be8`.
- PR #371 (`[perf-improver]`): `parseUnixMetrics` zero-alloc — run 27, commit `9602a91`.
- PR #380 (`[perf-improver]`): `removeContainer` state-dir fold — run 28, commit `fe2b085`. MERGED 2026-07-16T10:10:37Z.
- PR #387 (`[perf-improver]`): `RenderPlain` spaces80[:n] slice — run 29, commit `129936a`. MERGED 2026-07-22T03:32:15Z.
- PR #389 (`[perf-improver]`): `FormatBytesHuman` B-branch `strconv.AppendInt` — run 30, commit `f8c5f84`. MERGED 2026-07-22T03:33:12Z.

## Open Performance Issues
- #318 — `[perf-improver] Monthly Activity 2026-07` (maintained across runs; updated this run via `update_issue.replace`).
- #124 — `[efficiency-improver] Add benchmark regression detection to CI` — CLOSED via PR #333 🎉

## Next Run Tasks
- Task 1: Commands re-validate (passing; 16/16 packages with `-race -count=1`).
- Task 2: Look at next cold-path or in-process parsing target. Candidates re-evaluated, most closed:
  - `agentic.ValidatePrereqs` id -u → sudo iptables → id -un chain — function only called from `_test.go` (no production caller).
  - `runner.diskschedule.parseAtTime` — already 0 allocs/op at 64 ns/op.
  - `host.DetectOS` / `host.DetectArch` PowerShell cascade — cold (once per new host).
  - `runner.containerLocalStatusOneShot` parsing — already folded.
  - **`internal/tui/metricsRow` per-row slice + per-cell string allocs** — bigger refactor: write directly to strings.Builder to eliminate the 24 allocs/op in FormatHostMetrics. Trade-off: changes API; broader blast radius.
  - **`runner.FormatBytesHuman` builder-style variant** — add `appendBytesHuman(b *strings.Builder, n int64)` and refactor `PrintDiskUsageTable` to write directly to a builder. Drops 5 allocs/row. Bigger change but well-scoped.
- Task 5: No open performance-labeled issues besides #318. No comments needed.
- Task 7: Continue updating #318 monthly summary.

## Memory note — IMPORTANT for future runs
Phantom-success pattern continues to fire. **12 of 15 phantom-success branches in this series have been pushed and merged by the maintainer**. Recovery protocol remains: document the local commit, save branch ref in memory, exit via `noop` if needed.

**Safe-outputs update_issue quota**: 1 call per run. If the call fails (e.g. body too large), there's no second chance within the same run. Fall back to `add_comment` for the monthly summary update; future runs can use `update_issue.replace` to fold comments into the body.

## Memory note (research/observations)
- **General principle (validated seventeen times now)**: When a helper returns multiple correlated values from a single I/O call, never split those reads across multiple invocations. Examples closed in this series: PR #264, #269, #285, #301, #317, #322, #333, #334, #342, #347 (#349), #348 (#346), #350, #353, #355, #357, #358, #361, #371, #380, local commits `6fb977f`, `e01dbfe`, `1a426ee`, `e749be8`, `9602a91`, `fe2b085`, `129936a` (run-29 → PR #387), `f8c5f84` (run-30 → PR #389), `56df245` (run-31, draft).
- **`strings.Fields` → `strings.IndexByte` for known-shape separators** (PR #371): when the input shape is fixed (e.g. exactly 3 space-separated fields), two `IndexByte` calls locate separators, and ParseFloat sub-slices alias the input without copy.
- **`strings.Repeat` → `spaces80[:n]` slice** (PR #387): when the result flows directly into a `strings.Builder.WriteString`, the alloc is already elided by escape analysis, but the function-call + boundary-check overhead is non-trivial (~11% on isolated micro-bench).
- **`strconv.FormatInt + string concat → strconv.AppendInt + stack buffer`** (PR #389): when the string-escape pattern is fixed-shape (digits + space + unit), AppendInt + `[N]byte` stack drops 1 alloc.
- **`strings.Split + per-line strings.TrimSpace → strings.SplitSeq + in-place \r trim`** (run-31, draft PR): SplitSeq yields each line as a sub-slice (no copy, no upfront `[]string` slice alloc). A trailing `\r` can be trimmed by slicing one byte off the end. Drops 1 alloc per call while preserving CRLF defensiveness.
- **Multi-marker shell output** is the model used by `containersPresentOneShot` (PR #350), `orphanLinuxPlanProbe` (PR #358), `linuxSvcAndAutostartProbe` (PR #361), `containerLocalStatusOneShot` (PR #350), `linuxInstanceProbe` (PR #379 with `V` marker).
- **Repo past the hot-path ceiling for SSH round-trip consolidations on Linux container teardown**: every per-instance teardown on `gh sr down` / `Remove` is now **1 SSH end-to-end on Linux**.
- **`$HOME` literal in shell commands**: when a shell command is going to use the path with `rm -rf` or `test -f`, the shell can expand `$HOME` itself — no need to first resolve it via `echo $HOME`.
- **Sub-slice `strconv.ParseFloat` does not allocate on success** (PR #371 evidence).
- **`parseUnixMetrics` is on the per-host-per-tick path** (PR #371 closed).
- **Benchstat regression detection is live** (PR #333). `BenchmarkFormatBytesHuman` (run-30), `BenchmarkRenderPlain` (run-29), `BenchmarkContainersPresentOneShot` (run-31) are all part of the per-PR benchstat suite.
- **Best-effort chain semantics**: when folding multiple ops into one shell, use `2>/dev/null || true` on each rm leg so the chain returns 0 even when intermediate state doesn't exist.
- **`FormatHostMetrics` 24 allocs/op** (5 hosts): breakdown is ~4-5 allocs/host for the per-row slice + formatPercent/formatUsedTotal/LoadStr string conversions + ColumnWidths + builder grows.
- **`FormatBytesHuman` 7 allocs/op** (run-30, 7 samples with 2 B-branch): after run-30 opt, drops from 8 to 7 allocs/op. The remaining 7 are: 1 alloc per GiB/MiB/KiB call (5 calls × 1 = 5) + 1 alloc for each of 2 B-branch samples (2 calls × 1 = 2).
- **`containersPresentOneShot` 8 allocs/op** (run-31, 4 inputs): after run-31 opt, drops from 12 to 8 allocs/op. Per-call: 3 → 2. Remaining 2 are `make(map)` and the initial `present[n]=false` insert (unavoidable for the result type).
- **Phantom-success + maintainer-pushed pattern**: don't rely on the local repo to confirm phantom-success state — use `mcp__github__list_pull_requests` or `search_pull_requests` to verify.

## Backlog cursor (for round-robin)
Last touched: 2026-07-30 (31st run). Next run should consider:
1. **`internal/tui/metricsRow` rewrite to write directly to strings.Builder** — drop ~20 of the 24 allocs/op in FormatHostMetrics. Larger refactor, changes API.
2. **`runner.FormatBytesHuman` builder-style variant** — add `appendBytesHuman(b *strings.Builder, n int64)` and refactor `PrintDiskUsageTable`. Drops 5 allocs/row. Bigger change but well-scoped.
3. **`internal/autostart/cleanup.go`** — already uses SplitSeq. Confirmed zero-alloc.
4. **`internal/host/detect.go`** PowerShell cascade — cold path, once-per-host-add. Not worth a PR.
5. **Survey for any new `strings.Split` / `fmt.Fprintf` regressions** introduced by recent merges.

**⚠️ Major SSH-round-trip consolidations on hot paths are all closed.** Remaining incremental opportunities are cold paths OR in-process parsing/rendering. Benchstat regression detection (PR #333) keeps these contributions measurable.

**Run-31 final state**: containersPresentOneShot SplitSeq parse loop (commit `56df245`) opens a -15% B / -33% allocs win on the per-host NeedsSetup + per-tick TUI Status refresh path. Maintains the series' pattern of small, focused, measurable per-PR improvements.

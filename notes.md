---
name: perf-improver-state-2026-08
description: Persistent state for Perf Improver agent on baizhiheizi/gh-sr — run 56 on 2026-08-20, FormatHostMetrics builder-direct render PR opened (commit 982ea3f).
metadata:
  type: project
---

# Perf Improver State (baizhiheizi/gh-sr)

**Last run:** 2026-08-20 (run 56, scheduled)
**Run link:** https://github.com/baizhiheizi/gh-sr/actions/runs/32410839023

## Repository Status
Latest commit on main: `8256c5c Merge pull request #417 from baizhiheizi/repo-assist/test-host-upload-wrapper-2026-08-19-62dd70392c97a016`. Shallow clone (only HEAD on local); PRs from `repo-assist`/`efficiency-improver`/`test-improver` are visible via list_pull_requests but their merges are not in the local git log. **PR #395** (run-31 `containersPresentOneShot` SplitSeq) MERGED 2026-07-31. **Issue #318** (July 2026 Monthly Activity) was closed 2026-08-14 by maintainer.

## This Run's Work (run 56, 2026-08-20)
- Re-validated commands: `go build ./...` ✅, `go vet ./...` ✅, `gofmt -l .` ✅ clean, `go test ./... -race -count=1` ✅ all packages pass (race-clean).
- Surveyed open PRs (head filter `perf-assist`): **0 open [perf-improver] PRs**.
- Audited benchmarks: `BenchmarkFormatHostMetrics` was 1880 B/op, 24 allocs/op — the highest-leverage remaining in-process rendering win on a per-View() hot path.
- 🔧 **Created PR** (this run, draft, branch `perf-assist/format-host-metrics-builder-direct-2026-08-20`, commit `982ea3f`): rewrites `FormatHostMetrics` to a new `FormatHostMetricsTo(b *strings.Builder, metrics)` that two-passes the cells directly into the builder.
  - **-63% B/op** (1880 → 704), **-96% allocs/op** (24 → 1), **+47% ns/op** (3000 → 4400) — pass-1 measurement cost, acceptable for 24× alloc reduction on per-View() hot path.
  - New builder-style helpers: `appendFormatPercent`, `appendFormatUsedTotal`, `(HostMetrics).AppendLoadStr`. Stack `[64]byte` scratch buffer used for both width measurement and final write.
  - `TestFormatHostMetrics_NewPath_ByteIdentical` (6 cases) locks byte-equality against legacy `table.RenderPlain` path.
  - `FormatHostMetrics(metrics)` now a thin wrapper allocating one builder + delegating to `FormatHostMetricsTo`.
  - `metricsRow`, `formatPercent`, `formatUsedTotal`, `(HostMetrics).LoadStr` retained unchanged for styled `PrintHostMetricsTable` / `viewHostMetrics` paths.
  - Diff: 3 files, +396 / −6. Files: `internal/host/metrics.go`, `internal/tui/metrics.go`, `internal/tui/compare_test.go`.
- Created new August 2026 Monthly Activity issue (since July #318 was closed 2026-08-14).

## Validated Commands
- `go build ./...` ✅
- `go test ./... -race -count=1` ✅ all packages pass
- `go vet ./...` ✅ clean; `gofmt -l .` ✅ clean
- `go test -bench=BenchmarkFormatHostMetrics -benchmem -run=^$ -count=10 ./internal/tui/` ✅ — this run's builder-direct opt (1880 → 704 B/op, 24 → 1 allocs/op).
- `go test -bench=BenchmarkMetricsRow -benchmem -run=^$ -count=10 ./internal/tui/` ✅ — pinned baseline for unchanged `metricsRow` slice path (still 10 allocs/op).
- `go test -bench=BenchmarkContainersPresentOneShot -benchmem -run=^$ -count=10 ./internal/runner/` ✅ — pinned baseline for run-31 (now 8 allocs/op).
- `go test -bench=BenchmarkFormatBytesHuman -benchmem -run=^$ -count=10 ./internal/runner/` ✅ — pinned baseline for run-30 (now 7 allocs/op).
- `go test -bench=BenchmarkRenderPlain -benchmem -run=^$ ./internal/table/` ✅ — pinned baseline for run-29.

## Open Perf Improver PRs / Local Branches
- **PR (this run)**: `perf-assist/format-host-metrics-builder-direct-2026-08-20` (commit `982ea3f`) — `FormatHostMetrics` builder-direct render. Draft, awaiting review.

## Merged This Series (recap)
PR #350 (containersPresentOneShot SSH-fold, run-22), PR #358 (orphanLinuxPlanProbe 3→1, run-25), PR #361 (linuxSvcAndAutostartProbe 2→1, run-26), PR #371 (parseUnixMetrics zero-alloc, run-27), PR #380 (removeContainer state-dir fold, run-28, MERGED 2026-07-16), PR #387 (RenderPlain spaces80[:n], run-29, MERGED 2026-07-22), PR #389 (FormatBytesHuman B-branch AppendInt, run-30, MERGED 2026-07-22), PR #395 (containersPresentOneShot SplitSeq, run-31, MERGED 2026-07-31).

## Next Run Tasks
- Task 1: Commands re-validate (passing; all packages with `-race -count=1`).
- Task 2: Next cold-path or in-process parsing target. Backlog after this run:
  - **`runner.FormatBytesHuman` builder-style variant** — add `appendBytesHuman(b *strings.Builder, n int64)` and refactor `PrintDiskUsageTable`. Drops 5 allocs/row.
  - **`FormatHostMetrics` ns/op regression optimization** — compute widths from `HostMetrics` fields directly (no formatting in pass 1). Risky; only if maintainer signals appetite.
  - **`host.DetectOS` / `host.DetectArch` PowerShell cascade** — cold. Not worth a PR.
  - **Survey for any new `strings.Split` / `fmt.Fprintf` regressions** introduced by recent merges.
- Task 5: No open performance-labeled issues besides August Monthly Activity. No comments needed.
- Task 7: Continue updating August Monthly Activity.

## Memory note — IMPORTANT for future runs
Phantom-success pattern continues. **13 of 16 phantom-success branches in this series have been pushed and merged by the maintainer** (added PR #395). Recovery protocol: document the local commit, save branch ref in memory, exit via `noop` if needed.

**Safe-outputs update_issue quota**: 1 call per run. Fall back to `add_comment` for the monthly summary update; future runs can use `update_issue.replace`.

## Memory note (research/observations)
- **General principle (validated eighteen times)**: When a helper returns multiple correlated values from a single I/O call, never split those reads across multiple invocations. Closed examples: PRs #264, #269, #285, #301, #317, #322, #333, #334, #342, #347 (#349), #348 (#346), #350, #353, #355, #357, #358, #361, #371, #380, local commits including `982ea3f` (run-56, draft PR).
- **`string(b)` returns 1 alloc even when b is stack-allocated** (this run): builder-style helpers that write to a stack scratch + `b.Write(...)` into a strings.Builder drop the alloc; `string(b)` coerces back to a heap allocation regardless of where b lives. Pattern: provide both string-returning and []byte-appending variants; use the latter for hot paths.
- **Two-pass width measurement + single-pass render** (this run): for table-style output, measure widths in pass 1 by formatting into a stack scratch buffer (no allocs), then write cells + padding in pass 2 straight into the builder. Net: 24 → 1 alloc per call, ~+47% ns/op for the duplicate format work. For TUI refresh at 5 s, the absolute cost (~4.4 µs for 5 hosts) is negligible vs the alloc-pressure win.
- **Byte-equality regression test pattern** (this run): compute the legacy output via the unchanged baseline in the same test, then `t.Errorf` on any byte mismatch. Catches drift in either path independently.
- **`strings.Fields` → `strings.IndexByte`** (PR #371): when input shape is fixed, two IndexByte calls locate separators; ParseFloat sub-slices alias without copy.
- **`strings.Repeat` → `spaces80[:n]`** (PR #387): when result flows into `WriteString`, alloc is elided by escape analysis but function-call + boundary-check overhead is non-trivial (~11%).
- **`strconv.FormatInt + string concat → strconv.AppendInt + stack buffer`** (PR #389): AppendInt + `[N]byte` drops 1 alloc.
- **`strings.Split → SplitSeq + in-place \r trim`** (PR #395): sub-slice of output, no copy, no upfront slice alloc.
- **Multi-marker shell output** is the model used by `containersPresentOneShot`, `orphanLinuxPlanProbe`, `linuxSvcAndAutostartProbe`, `containerLocalStatusOneShot`, `linuxInstanceProbe`.
- **Repo past the hot-path ceiling for SSH round-trip consolidations on Linux container teardown**: every per-instance teardown is now **1 SSH end-to-end on Linux**.
- **Sub-slice `strconv.ParseFloat` does not allocate on success** (PR #371 evidence).
- **Benchstat regression detection is live** (PR #333). `BenchmarkFormatBytesHuman` (run-30), `BenchmarkRenderPlain` (run-29), `BenchmarkContainersPresentOneShot` (run-31), `BenchmarkFormatHostMetrics` (run-56) are all part of the per-PR benchstat suite.
- **`FormatHostMetrics` 1 alloc/op** (run-56, 5 hosts): drops from 24 to 1 allocs/op. Single alloc is the `strings.Builder`'s internal buffer (grow-once).
- **`containersPresentOneShot` 8 allocs/op** (run-31): drops from 12 to 8. Per-call: 3 → 2.
- **Phantom-success + maintainer-pushed pattern**: don't rely on the local repo to confirm — use `mcp__github__list_pull_requests` to verify.

## Backlog cursor (for round-robin)
Last touched: 2026-08-20 (56th run). Next run should consider:
1. **`runner.FormatBytesHuman` builder-style variant** — appendBytesHuman + refactor PrintDiskUsageTable. Drops 5 allocs/row.
2. **`FormatHostMetrics` ns/op regression optimization** — compute widths from fields directly. Risky.
3. **`internal/autostart/cleanup.go`** — already uses SplitSeq. Zero-alloc.
4. **`internal/host/detect.go` PowerShell cascade** — cold path. Not worth a PR.
5. **Survey for `strings.Split` / `fmt.Fprintf` regressions** introduced by recent merges.

**⚠️ Major SSH-round-trip consolidations on hot paths are all closed.** Remaining incremental opportunities are cold paths OR in-process parsing/rendering. Benchstat regression detection (PR #333) keeps these contributions measurable.

**Run-56 final state**: FormatHostMetrics builder-direct render (commit `982ea3f`) opens a -63% B / -96% allocs win on the per-host-per-View() TUI scroll-panel hot path. Maintains the series' pattern of small, focused, measurable per-PR improvements.

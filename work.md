---
name: perf-improver-work
description: Work-in-progress, completed work, and run history for the perf-improver workflow on baizhiheizi/gh-sr.
metadata:
  type: project
---

# Work log

## In progress

(none)

## Completed

### 2026-09-24 (run 4) — TUI render low-alloc (PR draft, branch `perf-assist/tui-render-low-alloc`)

**PR opened**: `[perf-improver] perf(tui): drop per-row builder copy + "+ "\n"" concats in viewMain` (draft, commit `e287cde`).

**Change**: `internal/tui/table.go` — added `renderRowInto` / `renderHighlightedRowInto` (builder-appending variants of `renderRow` / `renderHighlightedRow`). The original functions now delegate to the `*Into` variants and remain thin wrappers so `PrintTable` / `PrintStatusTable` callers (`fmt.Fprintln`) are unchanged. `internal/tui/dashboard_view.go` — `viewMain` and `viewHostMetrics` switched to call `renderRowInto` / `renderHighlightedRowInto` directly against their own `strings.Builder`, followed by a single `b.WriteByte('\n')` instead of `+ "\n"` concat. Other `b.WriteString(<styled> + "\n")` patterns in the same file split into separate `WriteString` + `WriteByte('\n')` pairs.

**Benchmark (go1.26.0, AMD Ryzen AI 9 HX 370, -benchtime=1s -benchmem -count=10 on BenchmarkViewMain/one_status)**:

| Metric  | Before (main)              | After (perf-assist/tui-render-low-alloc) | Delta |
| ------- | -------------------------- | ----------------------------------------- | ----- |
| allocs  | 322 /op (10/10 consistent) | 315 /op (10/10 consistent)                | **-7 (-2.2%)** |
| bytes   | 10,062-10,131 B/op         | 9,468-9,517 B/op                          | **~-590 (-5.9%)** |
| time    | 35-53k ns/op               | 36-52k ns/op                              | within noise |

The byte saving is more meaningful than the alloc drop because eliminating the inner `b.String()` copy drops a ~600 byte heap allocation per viewMain call (the inner builder's final length-and-copy). Lipgloss's internal allocations are unchanged.

**New bench file entries**: `internal/tui/table_bench_test.go` gained `BenchmarkRenderHighlightedRowProduction` and `BenchmarkRenderRowProduction` — both run the production `runnerStatusColorize` callback (so styled branches like `statusRunning.Render` / `statusOnline.Render` actually fire) instead of the prior `colorizePassthrough`. Future TUI render changes can measure against the real colorize path.

**Verification**: build OK, vet OK, gofmt OK, full `go test ./... -race -count=1` OK (all 17 packages pass). Existing table tests (`TestPrintTable`, `TestRenderHeader_alignsColumnWidths`, `TestRenderHighlightedRow_contentMatches`) — no changes to visible output, assertions still pass.

**Insight (rejected approach)**: tried hoisting `Background(lipgloss.Color("8"))` out of `renderHighlightedRow`'s inner cell loop first — bench showed 322 allocs/op unchanged. Lipgloss internals do their own `termenv.Style` allocation per `Render()` call regardless of how we pre-build the intermediate `Style`. Not worth the readability hit. Reverted and noted in the PR body so future runs don't repeat it.

**Insight (net)**: this closes the ~5% of `viewMain` allocs that were under our control (intermediate `strings.Builder` + `b.String()` copy inside `renderRow` / `renderHighlightedRow`, and `+ "\n"` string concats). The remaining ~95% is still inside lipgloss. Caching rendered lines keyed on input hash would be the next step if a future run wants to push further, but it's still HIGH risk and would touch every consumer of `cellStyle` / `headerStyle`.

### 2026-09-03 (run 1) — Agentic SplitSeq (PR #458 — MERGED)

**PR opened**: `[perf-improver] perf(agentic): use SplitSeq for tagged-output parsers on doctor path` (draft, branch `perf-assist/agentic-splitseq-fe1dd4e41f768001`).

**Status**: Merged 2026-09-03T22:55:38Z (auto-merged by maintainer).

**Change**: `internal/agentic/agentic.go` — `parseContainerAgenticFanoutOutput` and `parseDockerChainOutput` switched from `strings.Split` to `strings.SplitSeq`. -38% to -46% on the four parser sub-benches; happy paths now 0-alloc.

### 2026-09-03 (run 2) — Runner probe SplitSeq (PR #463 — MERGED)

**PR opened**: `[perf-improver] perf(runner): use SplitSeq for per-instance probe parsers on Status path` (draft, branch `perf-assist/runner-probes-splitseq`, commit b1ffe5b).

**Status**: Merged by maintainer between run 2 and run 3.

**Change**: `internal/runner/container.go` (`ProbeDinDContainerReadiness`) and `internal/runner/linux_instance_probe.go` (`linuxInstanceProbe`) switched from `strings.Split` to `strings.SplitSeq`. These were the last two `strings.Split(out, "\n")` callers on the Status hot path.

**Benchmark (go1.25.9, AMD Ryzen AI 9 HX 370, -benchtime=500ms -count=5)**:

| Bench | Before | After | Δ |
| --- | --- | --- | --- |
| `BenchmarkLinuxInstanceProbe` | 504.9 ns/op, 770 B/op, 13 allocs | 485.2 ns/op, 703 B/op, 12 allocs | **-8.7% bytes, -1 alloc** |
| `BenchmarkLinuxInstanceProbe_WithDir` | 585.0 ns/op, 912 B/op, 16 allocs | 673.5 ns/op, 835 B/op, 15 allocs | **-8.4% bytes, -1 alloc** (timing noise) |
| `BenchmarkLinuxInstanceProbe_SystemdSystem` | 505.2 ns/op, 772 B/op, 13 allocs | 562.6 ns/op, 707 B/op, 12 allocs | **-8.4% bytes, -1 alloc** (timing noise) |
| `BenchmarkProbeDinDContainerReadiness` | 149.2 ns/op, 218 B/op, 3 allocs | 146.8 ns/op, 216 B/op, 3 allocs | wash (mock harness swamping) |

**New bench file**: `internal/runner/runner_probe_bench_test.go` — 5 sub-benches covering the typical + edge cases (running-healthy, inner-docker-down, user-level systemd, system-level systemd, with-D-marker for orphan cleanup).

**Verification**: build OK, vet OK, gofmt OK, full `go test ./... -race -count=1` OK (all packages pass).

**Insight**: `linuxInstanceProbe` shows the expected SplitSeq win (-1 alloc, ~9% bytes); `ProbeDinDContainerReadiness` shows no measurable change because the mock `DockerExecCommand` formatting + mock `Calls` slice growth swamp the split alloc. The change is still worth keeping for consistency with the package-wide SplitSeq rollout.

## Lessons learned this period

- `SplitSeq` migration is now complete across the repo: `internal/agentic`, `internal/autostart`, `internal/runner`, `internal/host`, `internal/cache`, `internal/config`, `internal/doctor`. Future code reviews should flag any new `strings.Split(out, "\n")` callers.
- When a microbench shows no change despite a theoretically-favorable refactor, check what other allocations are dwarfing the target — mock harness overhead can swamp real-world savings.
- Lipgloss's `Style.Render` does its own internal `termenv.Style` allocation per call regardless of how the caller pre-builds the `Style` (e.g. hoisting `Background(...)` out of an inner loop has no measurable impact). Optimizations on the TUI render path therefore must attack caller-side concat/builder patterns, not the intermediate `Style`.
- The `renderRow` / `renderHighlightedRow` `+ "\n"` concat pattern is the same shape across many helpers; once one panel (`viewMain`) is converted to a builder-into-builder pattern, the win can be applied to `viewHostMetrics` and any future panel that uses the same table-render primitives.
- A focus on byte savings (not just alloc count) often reveals larger GC wins — eliminating one intermediate `strings.Builder.String()` copy of ~600 bytes per `viewMain` call is more impactful than dropping several small per-cell allocations.
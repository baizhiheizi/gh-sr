---
name: perf-improver-opportunities
description: Performance optimization backlog for baizhiheizi/gh-sr, prioritized by impact and feasibility.
metadata:
  type: project
---

# Optimization backlog (prioritized)

Status as of 2026-09-24. The repo is small (~14k LOC) and already heavily optimized — most hot paths have dedicated benchmarks. The `strings.Split` → `strings.SplitSeq` rollout is now COMPLETE across `internal/agentic`, `internal/autostart`, `internal/runner`, `internal/host`, `internal/cache`, `internal/config`, `internal/doctor` (perf-improver PRs #458, #463 MERGED; repo-assist PR #461 and efficiency-improver PR #498 carry the remaining display-side callers — perf-improver is no longer the natural owner for the `strings.Split` backlog).

## Recently completed

- **[perf-improver] perf(agentic): use SplitSeq for tagged-output parsers on doctor path** — PR #458 (MERGED 2026-09-03T22:55:38Z). -38% to -46% on the four parser sub-benches; happy paths now 0-alloc. See work.md.
- **[repo-assist] refactor: use SplitSeq for line-iteration in non-agentic parsers** — PR #461 (draft). Covers `internal/cache`, `internal/config`, `internal/doctor`. Ships the rest of the package-wide SplitSeq rollout.
- **[perf-improver] perf(runner): use SplitSeq for per-instance probe parsers on Status path** — PR #463 (MERGED). Covers `internal/runner/container.go` (`ProbeDinDContainerReadiness`) and `internal/runner/linux_instance_probe.go` (`linuxInstanceProbe`). -8.7% bytes / -1 alloc on the linux probe; mock harness swamps the saving on the docker probe.
- **[efficiency-improver] perf(ui): use SplitSeq for FormatRemediation, wrapLines, and formatLaunchdDetail** — PR #498 (draft). Covers the last three display-side `strings.Split(out, "\n")` callers. Out of perf-improver's scope but completes the repo-wide SplitSeq rollout.
- **[perf-improver] perf(tui): drop per-row builder copy + "+ "\n"" concats in viewMain** — PR draft, branch `perf-assist/tui-render-low-alloc` (commit `e287cde`). Added `renderRowInto` / `renderHighlightedRowInto` builder-appending variants and switched `viewMain` / `viewHostMetrics` over to `WriteString` + `WriteByte('\n')` instead of `+ "\n"` concats. -7 allocs/op (-2.2%, 322→315), -590 B/op (-5.9%) on `BenchmarkViewMain/one_status`. See work.md.

## Backlog cursor

With SplitSeq complete and the easy TUI render caller-side allocs now addressed, the next runs should look at:

1. View() alloc reduction — remaining lipgloss cost (MEDIUM impact, MEDIUM risk) — the ~95% inside `lipgloss.Style.Render` is still untouched.
2. renderRow / renderHighlightedRow cell padding (MEDIUM impact, LOW risk) — still speculative; worth re-profiling after the *Into variants merge.
3. viewScroll `+ "\n"` concat (LOW impact, LOW risk) — same shape as the `viewMain` / `viewHostMetrics` fix; quick follow-up if a future run wants a small additional win.
4. Manager.Status further per-instance optimization (LOW priority).
5. Periodic `strings.Split` grep — keep checking for new offenders as code lands.

## Identified opportunities (priority order)

### 1. View() alloc reduction — remaining lipgloss cost (MEDIUM impact, MEDIUM risk)
- `BenchmarkViewMain/one_status`: 35-53k ns/op, 9,468-9,517 B/op, 315 allocs/op (after the *Into variants merge).
- ~95% of allocations still come from `lipgloss.Style.Render` and downstream `strings.Split` / `strings.Builder.WriteString` inside lipgloss (line-wrapping `strings.Split`, ANSI byte-buffer growth).
- Options: cache rendered cells keyed on content hash; replace `Width()` chain; emit ANSI directly for static-color cells.
- Risk: visual regression; lipgloss handles unicode/ANSI edge cases we don't want to reimplement.
- Status: caller-side allocs now closed (this run); library-side still ANALYZED, not started.

### 2. renderRow / renderHighlightedRow cell padding (MEDIUM impact, LOW risk)
- `BenchmarkRenderRow`: ~30k ns/op, ~6.2k B/op, ~248 allocs/op.
- ~32% of allocs from `strings.Split` inside lipgloss line-wrapping (per-cell).
- Could skip cells already at column width (no padding needed).
- Status: SPECULATIVE. Re-profile after the *Into variants merge to see whether padding-skip still gives measurable savings on top.

### 3. viewScroll `+ "\n"` concat (LOW impact, LOW risk)
- `internal/tui/dashboard_view.go` `viewScroll` has the `b.WriteString("  " + m.scrollLines[i] + "\n")` pattern repeated per line.
- Same shape as the `viewMain` / `viewHostMetrics` fix this run; mechanical conversion to `WriteString` + `WriteByte('\n')`.
- Status: NOT STARTED. Quick win if a future run wants to clear out the remaining TUI concat patterns.

### 4. Manager.Status further per-instance optimization (LOW impact)
- `BenchmarkManager_Status`: 28,607 ns/op, 120,343 B/op, 74 allocs/op.
- Already heavily optimized (mode/repo/labels hoisted per inline comment).
- Remaining allocs likely from per-instance `RunnerStatus` struct construction + mock SSH output parsing.
- Status: ANALYZED. Marginal gains expected.

### 5. Periodic SplitSeq grep
- `git grep -nE 'strings\.Split\b' -- '*.go'` — keep checking for new offenders as code lands.
- Status: ONGOING. Worth a 30-second sweep every few runs.

## Cross-cutting notes

- The `strings.SplitSeq` migration is now complete across the repo. Future code reviews should flag any new `strings.Split(out, "\n")` callers.
- TUI render performance is dominated by lipgloss internals — measurable optimizations all live behind a major library change.
- Benchmark infrastructure is comprehensive: `make bench` / `make bench-save` + bench-compare.yml CI workflow. New benchmarks fit the same shape (`b.ReportAllocs()` + `b.ResetTimer()` + `for i := 0; i < b.N; i++`).
- Lipgloss's `Style.Render` does its own internal `termenv.Style` allocation per call regardless of how the caller pre-builds the `Style`. Hoisting `Background(...)` out of an inner cell loop has no measurable impact.
- Byte savings from eliminating intermediate `strings.Builder.String()` copies often exceed the alloc-count savings in GC pressure terms — worth tracking both metrics, not just alloc count.
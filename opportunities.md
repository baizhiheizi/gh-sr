---
name: perf-improver-opportunities
description: Performance optimization backlog for baizhiheizi/gh-sr, prioritized by impact and feasibility.
metadata:
  type: project
---

# Optimization backlog (prioritized)

Status as of 2026-10-01. The repo is small (~14k LOC) and already heavily optimized — most hot paths have dedicated benchmarks. The `strings.Split` → `strings.SplitSeq` rollout is COMPLETE across all production callers (last offenders were `internal/runner/container.go` and `internal/runner/linux_instance_probe.go`, merged via PR #463 on 2026-09-27; PR #458/#461/#498 closed the rest). The TUI caller-side concat backlog is also largely closed (PR #502 merged 2026-09-27 closed `viewMain` / `viewHostMetrics`); only `viewScroll` remains as a mechanical follow-up.

## Recently completed

- **[perf-improver] perf(agentic): use SplitSeq for tagged-output parsers on doctor path** — PR #458 (MERGED 2026-09-03T22:55:38Z). -38% to -46% on the four parser sub-benches; happy paths now 0-alloc.
- **[repo-assist] refactor: use SplitSeq for line-iteration in non-agentic parsers** — PR #461 (MERGED 2026-09-27). `internal/cache`, `internal/config`, `internal/doctor`. Ships the non-agentic half of the SplitSeq rollout.
- **[perf-improver] perf(runner): use SplitSeq for per-instance probe parsers on Status path** — PR #463 (MERGED 2026-09-27). `internal/runner/container.go` (`ProbeDinDContainerReadiness`) and `internal/runner/linux_instance_probe.go` (`linuxInstanceProbe`). -8.7% bytes / -1 alloc on the linux probe; mock harness swamps the saving on the docker probe.
- **[efficiency-improver] perf(ui): use SplitSeq for FormatRemediation, wrapLines, and formatLaunchdDetail** — PR #498 (MERGED 2026-09-27). Last three display-side `strings.Split(out, "\n")` callers.
- **[perf-improver] perf(tui): drop per-row builder copy + "+ "\n"" concats in viewMain** — PR #502 (MERGED 2026-09-27). `renderRowInto` / `renderHighlightedRowInto` builder-appending variants; `viewMain` / `viewHostMetrics` switched to `WriteString` + `WriteByte('\n')`. -7 allocs/op (-2.2%, 322→315), -590 B/op (-5.9%) on `BenchmarkViewMain/one_status`.

## Backlog cursor

With SplitSeq + the TUI caller-side concat both complete, the next runs should look at:

1. View() alloc reduction — remaining lipgloss cost (MEDIUM impact, MEDIUM risk) — ~95% inside `lipgloss.Style.Render` is still untouched.
2. renderRow / renderHighlightedRow cell padding (MEDIUM impact, LOW risk) — still speculative; worth re-profiling on top of the merged *Into variants.
3. `viewScroll` `+ "\n"` concat (LOW impact, LOW risk) — same shape as the merged `viewMain` / `viewHostMetrics` fix; quick follow-up if a future run wants a small additional win. Deferred this run (run_id 36939788959) so it doesn't collide with the maintainer's recent refactor churn (#510/#509/#508/#511).
4. EnrichFromScopeRunners N×M GitHub-runner scan (MEDIUM impact, LOW-MEDIUM risk) — NEW. Largest remaining non-lipgloss alloc hotspot. Needs a fresh bench pass comparing map-build vs current scan before committing.
5. Manager.Status further per-instance optimization (LOW priority).
6. Periodic `strings.Split` grep — keep checking for new offenders as code lands.

## Identified opportunities (priority order)

### 1. View() alloc reduction — remaining lipgloss cost (MEDIUM impact, MEDIUM risk)
- `BenchmarkViewMain/one_status` (post-#502): 36,719 ns/op, 9,451 B/op, 315 allocs/op.
- ~95% of allocations still come from `lipgloss.Style.Render` and downstream `strings.Split` / `strings.Builder.WriteString` inside lipgloss (line-wrapping `strings.Split`, ANSI byte-buffer growth).
- Options: cache rendered cells keyed on content hash; replace `Width()` chain; emit ANSI directly for static-color cells.
- Risk: visual regression; lipgloss handles unicode/ANSI edge cases we don't want to reimplement.
- Status: caller-side allocs now closed (PR #502); library-side still ANALYZED, not started.

### 2. renderRow / renderHighlightedRow cell padding (MEDIUM impact, LOW risk)
- `BenchmarkRenderRow`: 35,462 ns/op, 6,304 B/op, 248 allocs/op.
- `BenchmarkRenderHighlightedRow`: 37,110 ns/op, 6,880 B/op, 284 allocs/op.
- ~32% of allocs from `strings.Split` inside lipgloss line-wrapping (per-cell).
- Could skip cells already at column width (no padding needed).
- Status: SPECULATIVE. Re-profile after PR #502 merge to see whether padding-skip still gives measurable savings on top.

### 3. viewScroll `+ "\n"` concat (LOW impact, LOW risk)
- `internal/tui/dashboard_view.go:250` has `b.WriteString("  " + m.scrollLines[i] + "\n")` repeated per line in `viewScroll`.
- Same shape as the `viewMain` / `viewHostMetrics` fix (PR #502); mechanical conversion to `WriteString` + `WriteByte('\n')`.
- Caveat: `viewScroll` only fires when the user is in scroll mode (NOT the per-tick refresh hot path), so this is more of a code-shape cleanup than a perf win.
- Status: NOT STARTED. Deferred from run 36939788959 to avoid colliding with the maintainer's recent refactor work; worth picking up on a quieter week.

### 4. EnrichFromScopeRunners N×M GitHub-runner scan (MEDIUM impact, LOW-MEDIUM risk) — NEW
- `BenchmarkEnrichFromScopeRunners` (20 repos × 10 instances): 50,363 ns/op, 117,529 B/op, 420 allocs/op.
- `BenchmarkEnrichFromScopeRunners_Small` (5 repos × 2 instances): 2,390 ns/op, 6,488 B/op, 28 allocs/op.
- Inner loop `for _, gr := range scopeRunners[key] { if gr.Name != statuses[i].Instance { continue } ... }` is O(N×M) per status scan over per-scope GitHub runners. For 200 statuses × 100 GitHub runners that's 20K string comparisons per call.
- Building a `map[string]GitHubRunner` per `scopeKey` would turn this into O(N+M). Cost: one extra map alloc per scope (≈ 20 small map allocs for the big fixture; 5 for the small fixture). For the small fixture the +alloc cost likely swamps the -CPU savings; for the big fixture the -CPU savings likely dominate.
- Alternative: cache `rcByInstance` + the scopeKey→runners map across `EnrichWithGitHubStatus` calls (cfg-change detection required). Higher code-complexity payoff.
- Status: NEEDS BENCHMARK before committing. Add a `BenchmarkEnrichFromScopeRunners_Map` variant comparing both strategies on the same fixtures (big + small), then decide.

### 5. Manager.Status further per-instance optimization (LOW impact)
- `BenchmarkManager_Status`: 48,939 ns/op, 167,457 B/op, 75 allocs/op.
- Already heavily optimized (mode/repo/labels hoisted per inline comment).
- Remaining allocs likely from per-instance `RunnerStatus` struct construction + mock SSH output parsing.
- Status: ANALYZED. Marginal gains expected.

### 6. Periodic SplitSeq grep
- `git grep -nE 'strings\.Split\b' -- '*.go'` — keep checking for new offenders as code lands.
- Last grep (2026-10-01): remaining production `strings.Split` callers are all legitimate slice-index uses (`cmd/gh-sr/main.go:300,308` `parts[1]:[0]`; `internal/doctor/lockfiles.go:98,99` `strings.Split(a, ".")` / `strings.Split(b, ".")` for version compare). No remaining `strings.Split(out, "\n")` callers in production code.
- Status: ONGOING. Worth a 30-second sweep every few runs.

## Cross-cutting notes

- The `strings.SplitSeq` migration is now complete across the repo. Future code reviews should flag any new `strings.Split(out, "\n")` callers.
- TUI render performance is dominated by lipgloss internals — measurable optimizations all live behind a major library change.
- Benchmark infrastructure is comprehensive: `make bench` / `make bench-save` + bench-compare.yml CI workflow. New benchmarks fit the same shape (`b.ReportAllocs()` + `b.ResetTimer()` + `for i := 0; i < b.N; i++`).
- Lipgloss's `Style.Render` does its own internal `termenv.Style` allocation per call regardless of how the caller pre-builds the `Style`. Hoisting `Background(...)` out of an inner cell loop has no measurable impact.
- Byte savings from eliminating intermediate `strings.Builder.String()` copies often exceed the alloc-count savings in GC pressure terms — worth tracking both metrics, not just alloc count.
- `GOTOOLCHAIN=auto` is required (go.mod requires go1.26.0; go1.25.9 is what's pre-installed locally).

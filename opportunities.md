---
name: perf-improver-opportunities
description: Performance optimization backlog for an-lee/gh-sr (was baizhiheizi/gh-sr), prioritized by impact and feasibility.
metadata:
  type: project
---

# Optimization backlog (prioritized)

Status as of 2026-11-27 (run 10). Module path is `github.com/an-lee/gh-sr` (GitHub repo remains `baizhiheizi/gh-sr`); repo is now 24 test packages and heavily optimized — most hot paths have dedicated benchmarks. Run-9's SplitSeq sweep merged as **#667** (2026-11-20, same day); run-8's enrich fix merged as **#610** (2026-11-18).

## Recently completed (run 10, 2026-11-27)

- **[perf-improver] perf(tui): drop responsive-layout scaffolding allocations from viewMain** — DRAFT PR, branch `perf-assist/responsive-layout-alloc-trim` (commit `4d71f97`). Closes backlog item #1: one_status 323→313 allocs (back below pre-#564 315), -6.7% B; twelve_statuses -22 allocs, -1.8% B. Two measured rejections recorded (title builder variant: bytes-unpredictable; planColumns buffer reuse: makes are stack-allocated, zero effect). See [[perf-improver-work]].

## Previously completed (run 9, 2026-11-20)

- **[perf-improver] perf: SplitSeq sweep of 7 one-shot line parsers** — MERGED as **#667** (2026-11-20; branch `perf-assist/splitseq-sweep-run9-b64f7b551c9c9867` with hash suffix). doctor.go:997, hostkeyalgos.go:68+109 (per-dial), disk_volumes.go:34+53, registry.go:500, toolcachesrc.go:113. Headline `hostKeyAlgosForDial`: -11% ns/op, -15/-16 allocs; +1 new benchmark per parser.
- **[perf-improver] perf(runner): drop per-instance key build in enrichFromScopeRunners** — MERGED as **#610** (2026-11-18). Big fixture -31% ns/op, -12.8% B/op, -49% allocs/op; small -28% ns/op, -46% allocs.

## Previously completed

- **[perf-improver] perf(tui): drop last "+ "\n"" concats in menu/scroll views via renderMenuItemsInto** — PR #537 + duplicate #536 (MERGED 2026-10-10). `BenchmarkViewActionMenu` -26% time, -24% bytes (-368 B), -13% allocs. `BenchmarkViewScroll` (200-line fixture) -24% time, -18% bytes (-6023 B), -77% allocs (-94).
- **[perf-improver] perf(tui): drop per-row builder copy + "+ "\n"" concats in viewMain** — PR #502 (MERGED 2026-09-27). `BenchmarkViewMain/one_status` -7 allocs/op (-2.2%), -590 B/op (-5.9%).
- **[perf-improver] perf(runner): use SplitSeq for per-instance probe parsers on Status path** — PR #463 (MERGED 2026-09-27).
- **[perf-improver] perf(agentic): use SplitSeq for tagged-output parsers on doctor path** — PR #458 (MERGED 2026-09-03).
- **[efficiency-improver] perf(ui): use SplitSeq for FormatRemediation, wrapLines, and formatLaunchdDetail** — PR #498 (MERGED 2026-09-27).
- **[repo-assist] refactor: use SplitSeq for line-iteration in non-agentic parsers** — PR #461 (MERGED 2026-09-27).

## Backlog cursor

After run 10's viewMain trim, the next run should pick up:

1. **renderRow / renderHighlightedRow cell padding** — now the top remaining item by seniority. MEDIUM impact, LOW risk. Still SPECULATIVE; `BenchmarkRenderRow` 248 allocs/op and `BenchmarkRenderHighlightedRow` 284 allocs/op unchanged. ~32% from lipgloss line-wrapping splits; could skip cells already at column width (no padding needed) — must verify padding-skip renders byte-identical output.
2. **Manager.Status further per-instance optimization** — LOW priority. Marginal gains expected.
3. **scopeRunners inner scan** — DEFERRED (map variant rejected run 6). Only revisit with a different shape (caching across refresh ticks).
4. **Periodic SplitSeq grep** — `git grep -nE 'strings\.Split\b' -- '*.go'` — keep checking for new offenders as code lands. Clean as of run 10: only legit slice-index uses remain (main.go:287+295, autostart.go:182, lockfiles.go:98-99, hostkeyalgos.go:130, columns.go rstripLines, benchstat script).
5. **lipgloss-internal cost** — ~82% of viewMain alloc events remain inside lipgloss Render internals (genSplit, Buffer.grow, termenv styles). Only reachable via caching rendered cells keyed on content hash or a major library change — HIGH risk, don't start without maintainer interest.

## Identified opportunities (priority order)

### 1. View() alloc reduction — remaining lipgloss cost (MEDIUM impact, MEDIUM risk)
- `BenchmarkViewMain/one_status` (current, post-run-10): ~35,581 ns/op, 9,511 B/op, 313 allocs/op — caller-side scaffolding closed by run 10's PR.
- ~82% of alloc events still come from `lipgloss.Style.Render` internals (`strings.genSplit` line-wrapping, `bytes.Buffer.grow`, `termenv.Style` per-call).
- Options: cache rendered cells keyed on content hash; replace `Width()` chain; emit ANSI directly for static-color cells.
- Risk: visual regression; lipgloss handles unicode/ANSI edge cases we don't want to reimplement.
- Status: caller-side allocs now closed (PR #502); library-side still ANALYZED, not started.

### 2. renderRow / renderHighlightedRow cell padding (MEDIUM impact, LOW risk)
- `BenchmarkRenderRow`: ~30k ns/op, ~6,340 B/op, 248 allocs/op.
- `BenchmarkRenderHighlightedRow`: ~32k ns/op, ~6,940 B/op, 284 allocs/op.
- ~32% of allocs from `strings.Split` inside lipgloss line-wrapping (per-cell).
- Could skip cells already at column width (no padding needed).
- Status: SPECULATIVE. Re-profile after PR #502 merge to see whether padding-skip still gives measurable savings on top.

### 3. Manager.Status further per-instance optimization (LOW impact)
- `BenchmarkManager_Status`: ~61k ns/op, ~170,800 B/op, 75 allocs/op.
- Already heavily optimized (mode/repo/labels hoisted per inline comment).
- Remaining allocs likely from per-instance `RunnerStatus` struct construction + mock SSH output parsing.
- Status: ANALYZED. Marginal gains expected.

### 4. GitHub-runner scan inside scopeRunners[key] (formerly item #5) (LOW priority)
- After the run-7 enrich fix, only the inner O(N×M) `gr.Name != statuses[i].Instance` scan remains.
- Map-lookup variant (per-scope `map[string]GitHubRunner`) was rejected in run 6 as a clear loss.
- Worth revisiting only with a different shape (caching, etc.).
- Status: DEFERRED.

### 5. Periodic SplitSeq grep
- `git grep -nE 'strings\.Split\b' -- '*.go'` — keep checking for new offenders as code lands.
- Last grep (2026-11-20, run 9): post-sweep production remainder is all legitimate slice-index uses (`cmd/gh-sr/main.go:287,295`; `internal/doctor/lockfiles.go:98,99`; `internal/host/hostkeyalgos.go:130` len-checked parts; `internal/tui/columns.go:353` rstripLines; `internal/autostart/autostart.go:183` one-shot install path — `strings.Cut` trivial follow-up if ever touched).
- Status: ONGOING. Worth a 30-second sweep every few runs.

## Cross-cutting notes

- **Escape analysis hides "obvious" allocation wins (run 10)**: before restructuring a hot path around a make, check whether the slice actually escapes. The per-iteration `nat/floor` makes in `fitKept` and the per-row `cells` makes in `viewHostMetrics` never escape, so Go stack-allocates them — "reuse the buffer" refactors there measured exactly zero on allocs/op AND B/op while adding plumbing.
- **strings.Builder grow-sequence coupling (run 10)**: writing content into a big shared Builder in small pieces (per-byte gap padding, several small WriteStrings) vs building it separately and appending one chunk shifts the Builder's internal grow chain — measured -427 B/op in one panel and +620 B/op in another for the same change. Small-write Into-style refactors near a large Builder are bytes-unpredictable; bench both allocs AND bytes at every call site.
- The `strings.SplitSeq` migration is complete across the repo (last offenders were `internal/runner/container.go` and `internal/runner/linux_instance_probe.go`, merged via PR #463 on 2026-09-27).
- TUI render performance is dominated by lipgloss internals — measurable optimizations all live behind a major library change.
- Benchmark infrastructure is comprehensive: `make bench` / `make bench-save` + bench-compare.yml CI workflow. New benchmarks fit the same shape (`b.ReportAllocs()` + `b.ResetTimer()` + `for i := 0; i < b.N; i++`).
- Lipgloss's `Style.Render` does its own internal `termenv.Style` allocation per call regardless of how the caller pre-builds the `Style`. Hoisting `Background(...)` out of an inner cell loop has no measurable impact.
- Byte savings from eliminating intermediate `strings.Builder.String()` copies often exceed the alloc-count savings in GC pressure terms — worth tracking both metrics, not just alloc count.
- `GOTOOLCHAIN=auto` no longer needed as of run 8 (2026-11-06): local Go 1.26.10 >= go.mod's 1.26.7.
- The workspace clone is shallow (depth-1, blob:none) — local git history is just the tip; use GitHub MCP for older history.
- Profile-driven investigation: when the bench shows a hotspot, run `go test -bench=<name> -benchtime=<large> -count=1 -memprofile=/tmp/x.mem ./pkg/` then `go tool pprof -alloc_objects -list '<fn>' /tmp/x.mem` to find the allocating lines directly. Found runner.go:451 dominated by line-attributed alloc events in run 7.
- Module path is `github.com/an-lee/gh-sr` (was `github.com/baizhiheizi/gh-sr`); update Go test/import paths in any follow-up commits.

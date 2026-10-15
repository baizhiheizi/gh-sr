---
name: perf-improver-opportunities
description: Performance optimization backlog for an-lee/gh-sr (was baizhiheizi/gh-sr), prioritized by impact and feasibility.
metadata:
  type: project
---

# Optimization backlog (prioritized)

Status as of 2026-10-15. Module path is `github.com/an-lee/gh-sr` (renamed from `baizhiheizi/gh-sr`); repo is small (~14k LOC) and already heavily optimized — most hot paths have dedicated benchmarks.

## Recently completed (run 7, 2026-10-15)

- **[perf-improver] perf(runner): drop per-instance key build in enrichFromScopeRunners** — DRAFT PR on branch `perf-assist/enrich-rc-by-base-name` (commit `e63fb3c`). Profile-driven change: `pprof -alloc_objects` showed line 451 (`name+"-"+strconv.Itoa(j)`) was 51% of total allocs in `BenchmarkEnrichFromScopeRunners`. New shape: map keyed on `rc.Name` (one entry per runner) instead of `name-N` (one entry per instance); per-status lookup extracts baseName via `strings.LastIndex(instName, "-")` + slice. `BenchmarkEnrichFromScopeRunners` big fixture: 420 → 212 allocs/op (-49.5%), 117,529 → 102,008 B/op (-13.2%). Small fixture: 28 → 15 allocs/op (-46.4%). Time is within high-variance noise. Behaviour-equivalent (same "last rc with that Name wins" pick for the multi-host same-name edge case). Drops `strconv` import.

## Previously completed

- **[perf-improver] perf(tui): drop last "+ "\n"" concats in menu/scroll views via renderMenuItemsInto** — PR #537 + duplicate #536 (MERGED 2026-10-10). `BenchmarkViewActionMenu` -26% time, -24% bytes (-368 B), -13% allocs. `BenchmarkViewScroll` (200-line fixture) -24% time, -18% bytes (-6023 B), -77% allocs (-94).
- **[perf-improver] perf(tui): drop per-row builder copy + "+ "\n"" concats in viewMain** — PR #502 (MERGED 2026-09-27). `BenchmarkViewMain/one_status` -7 allocs/op (-2.2%), -590 B/op (-5.9%).
- **[perf-improver] perf(runner): use SplitSeq for per-instance probe parsers on Status path** — PR #463 (MERGED 2026-09-27).
- **[perf-improver] perf(agentic): use SplitSeq for tagged-output parsers on doctor path** — PR #458 (MERGED 2026-09-03).
- **[efficiency-improver] perf(ui): use SplitSeq for FormatRemediation, wrapLines, and formatLaunchdDetail** — PR #498 (MERGED 2026-09-27).
- **[repo-assist] refactor: use SplitSeq for line-iteration in non-agentic parsers** — PR #461 (MERGED 2026-09-27).

## Backlog cursor

After the run-7 enrich fix, the next runs should look at:

1. **View() alloc reduction — remaining lipgloss cost** — MEDIUM impact, MEDIUM risk. ~95% still inside `lipgloss.Style.Render`. Attacking the library side touches every consumer of `cellStyle` / `headerStyle`.
2. **renderRow / renderHighlightedRow cell padding** — MEDIUM impact, LOW risk. ~32% of allocs from `strings.Split` inside lipgloss line-wrapping (per-cell). Could skip cells already at column width. Still speculative — re-profile after the run-7 enrich fix lands to see whether padding-skip still gives measurable savings on top.
3. **Manager.Status further per-instance optimization** — LOW priority. Marginal gains expected.
4. **Periodic `strings.Split` grep** — verified clean as of 2026-10-15: remaining production offenders are slice-index uses (`cmd/gh-sr/main.go:300,308`, `internal/doctor/lockfiles.go:98,99`). NEW finding (low priority): `internal/autostart/autostart.go:183` has `strings.Split(strings.TrimRight(whoOut, "\n"), "\n")` in the `Install` system-unit path — slice-index use, single SSH round-trip, not hot path. Worth a follow-up if someone touches that file.
5. **GitHub-runner scan inside scopeRunners[key]** — After the run-7 fix dropped the rcByInstance alloc hotspot, the inner O(N×M) `gr.Name != statuses[i].Instance` scan remains. The map-lookup variant was rejected (run 6, 2026-10-08) as a clear loss on every metric. Worth a second look only if a future run finds a different shape (e.g. cache results across refresh ticks).

## Identified opportunities (priority order)

### 1. View() alloc reduction — remaining lipgloss cost (MEDIUM impact, MEDIUM risk)
- `BenchmarkViewMain/one_status` (current): 36,635 ns/op, 9,549 B/op, 315 allocs/op.
- ~95% of allocations still come from `lipgloss.Style.Render` and downstream `strings.Split` / `strings.Builder.WriteString` inside lipgloss (line-wrapping `strings.Split`, ANSI byte-buffer growth).
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
- Last grep (2026-10-15): remaining production `strings.Split` callers are all legitimate slice-index uses (`cmd/gh-sr/main.go:300,308` `parts[1]:[0]`; `internal/doctor/lockfiles.go:98,99` `strings.Split(a, ".")` / `strings.Split(b, ".")` for version compare; `internal/autostart/autostart.go:183` slice-index for id -un/-gn output). New low-priority finding: autostart.go:183 is on a one-shot install path; converting to `strings.Cut` would be a trivial follow-up.
- Status: ONGOING. Worth a 30-second sweep every few runs.

## Cross-cutting notes

- The `strings.SplitSeq` migration is complete across the repo (last offenders were `internal/runner/container.go` and `internal/runner/linux_instance_probe.go`, merged via PR #463 on 2026-09-27).
- TUI render performance is dominated by lipgloss internals — measurable optimizations all live behind a major library change.
- Benchmark infrastructure is comprehensive: `make bench` / `make bench-save` + bench-compare.yml CI workflow. New benchmarks fit the same shape (`b.ReportAllocs()` + `b.ResetTimer()` + `for i := 0; i < b.N; i++`).
- Lipgloss's `Style.Render` does its own internal `termenv.Style` allocation per call regardless of how the caller pre-builds the `Style`. Hoisting `Background(...)` out of an inner cell loop has no measurable impact.
- Byte savings from eliminating intermediate `strings.Builder.String()` copies often exceed the alloc-count savings in GC pressure terms — worth tracking both metrics, not just alloc count.
- `GOTOOLCHAIN=auto` is required (go.mod requires go1.26.0; go1.25.9 is what's pre-installed locally).
- Profile-driven investigation: when the bench shows a hotspot, run `go test -bench=<name> -benchtime=<large> -count=1 -memprofile=/tmp/x.mem ./pkg/` then `go tool pprof -alloc_objects -list '<fn>' /tmp/x.mem` to find the allocating lines directly. Found runner.go:451 dominated by line-attributed alloc events in run 7.
- Module path is `github.com/an-lee/gh-sr` (was `github.com/baizhiheizi/gh-sr`); update Go test/import paths in any follow-up commits.

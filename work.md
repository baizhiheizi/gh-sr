---
name: perf-improver-work
description: Work-in-progress, completed work, and run history for the perf-improver workflow on baizhiheizi/gh-sr.
metadata:
  type: project
---

# Work log

## In progress

### 2026-10-15 (run 7) — drop per-instance key build in enrichFromScopeRunners (DRAFT PR on `perf-assist/enrich-rc-by-base-name`, commit `e63fb3c`)

**PR opened**: `[perf-improver] perf(runner): drop per-instance key build in enrichFromScopeRunners` (DRAFT).

**Status**: Draft — pushed to branch, awaiting maintainer review.

**Change**: `internal/runner/runner.go` —
- `rcByInstance[name+"-"+strconv.Itoa(j)]` map build replaced with `rcByName[rc.Name]` (one entry per runner, instead of `sum(Count_i)`).
- Per-status lookup: `dashIdx := strings.LastIndex(instName, "-")` then `rc := rcByName[instName[:dashIdx]]`. `instName[:dashIdx]` is a no-alloc slice on the underlying string.
- `strconv` import no longer needed (import removed).
- Function comment updated to document the lookup-by-truncation trick rather than the inline-vs-`InstanceNames()` rationale the old comment explained.

**Benchmark (go1.26.0, AMD Ryzen AI 9 HX 370, -benchtime=3s -benchmem)**:

| Bench | Before | After | Δ |
| --- | --- | --- | --- |
| `BenchmarkEnrichFromScopeRunners` big (20 rc × 10 count) allocs | 420 /op | 212 /op | **-208 (-49.5%)** |
| `BenchmarkEnrichFromScopeRunners` big bytes | 117,529 B/op | 102,008 B/op | **-15,521 (-13.2%)** |
| `BenchmarkEnrichFromScopeRunners_Small` (5 rc × 2 count) allocs | 28 /op | 15 /op | **-13 (-46.4%)** |
| `BenchmarkEnrichFromScopeRunners_Small` bytes | 6,488 B/op | 5,872 B/op | **-616 (-9.5%)** |

Time is within high-variance noise on this microbench (61k–106k ns/op pre-change); call it a wash on time. Alloc count is the headline win.

**Verification**: `go build ./...` OK, `go vet ./...` OK, `gofmt -l .` clean, `go test ./... -race -count=1 -short` OK across all 18 packages; race-clean specifically on `internal/runner`. Existing enrich tests `TestManager_EnrichWithGitHubStatus` (lines 16-49 of runner_test.go) and `TestManager_EnrichWithGitHubStatus_OS_mismatch_skips` (lines 51-83) still pass — both exercise the same `Count=1` single-instance lookup path.

**Profile-driven insight**: before changing the code, ran `go test -bench='BenchmarkEnrichFromScopeRunners$' -benchmem -benchtime=2s -count=1 -memprofile=/tmp/enrich.mem ./internal/runner/` and `go tool pprof -alloc_objects -list 'enrichFromScopeRunners' /tmp/enrich.mem`. That showed line 451 (`rcByInstance[name+"-"+strconv.Itoa(j)] = rc`) attributed 6.58M of the 12.78M total alloc events (51.49%). Total bench iterations × allocs/op: `BenchmarkEnrichFromScopeRunners` allocates 420/op and the bench loop iterates ~21k, so 8.86M alloc events expected from the bench itself; the remaining ~3.92M come from the makeEnrichStatuses test fixture (`rc.Name + "-" + strconv.Itoa(j)` at line 92 of bench_test.go). Confirms both production + test sites use the same shape — fixing production naturally eliminates the test-fixture alloc too once the harness reuses the new path.

**Rejected alternative**: tried the per-scope `map[string]GitHubRunner` lookup in run 6 (2026-10-08) for the O(N×M) `gr.Name != statuses[i].Instance` inner scan — clear loss on every metric (big fixture: +26% time, +33% bytes, +83 allocs; small: +72% time, +73% bytes, +10 allocs). The map-build overhead dominated. Closed: the inner O(N×M) scan is the right shape; further wins require caching across refreshes (out of scope, bigger refactor).

## Completed

### 2026-10-08 (run 6) — TUI menu/scroll builder-into-builder sweep (PRs #536/#537 MERGED 2026-10-10)

**PRs opened**: `[perf-improver] perf(tui): drop last "+ "\n"" concats in menu/scroll views via renderMenuItemsInto`. Two duplicate PRs (#536 and #537) because the workflow ran twice and both were merged within ~28 seconds. Both contain the same change with different branches (`perf-assist/menu-render-builder-into-{hash}`); final on-disk state is the merge of #536 (commit `6b7e047`).

**Change**: `internal/tui/dashboard_view.go` —
- Added `renderMenuItemsInto(b *strings.Builder, items []string, cursor int)` mirroring the `renderRowInto` shape. Original `renderMenuItems(items, cursor)` delegates via `var b strings.Builder; renderMenuItemsInto(&b, items, cursor); return b.String()` so existing test callers are unchanged.
- All four menu views (`viewActionMenu`, `viewGlobalMenu`, `viewFilterMenu`, `viewFilterList`) call `renderMenuItemsInto` instead of concatenating the helper's return value.
- Trailing `helpStyle.Render(...) + "\n"` in each menu view split into separate `WriteString` + `WriteByte('\n')` calls.
- `viewScroll` per-line `"  " + line + "\n"` split into `b.WriteString("  ")` + `b.WriteString(line)` + `b.WriteByte('\n')`.

**Benchmark (go1.26.0, AMD Ryzen AI 9 HX 370, -benchtime=500ms -benchmem -count=5)**:

| Bench | Before | After | Δ |
| --- | --- | --- | --- |
| `BenchmarkViewActionMenu` | 7,252 ns/op, 1,537 B/op, 39 allocs | 5,373 ns/op, 1,169 B/op, 34 allocs | **-26% time, -24% bytes (-368 B), -13% allocs** |
| `BenchmarkViewScroll` (200-line fixture) | 14,560 ns/op, 32,601 B/op, 122 allocs | 11,042 ns/op, 26,578 B/op, 28 allocs | **-24% time, -18% bytes (-6,023 B), -77% allocs (-94)** |
| `BenchmarkRenderMenuItems` (helper) | 365 ns/op, 576 B/op, 13 allocs | 365 ns/op, 576 B/op, 13 allocs | wash (expected — `String()` is no-copy) |
| `BenchmarkRenderMenuItemsInto` (helper) | n/a | 365 ns/op, 576 B/op, 13 allocs | identical to wrapper, as expected |

Output byte-identical to main. Existing menu tests still pass.

**Verification**: `go build ./...` OK, `go vet ./...` OK, `gofmt -l .` clean, `go test ./... -race -count=1 -short` OK across all 17 packages.

### 2026-09-24 (run 4) — TUI render low-alloc (PR #502 — MERGED 2026-09-27)

**PR opened**: `[perf-improver] perf(tui): drop per-row builder copy + "+ "\n"" concats in viewMain` (draft, branch `perf-assist/tui-render-low-alloc`, commit `e287cde`).

**Status**: Merged by maintainer 2026-09-27T23:42:55Z (closed_at and merged_at identical).

**Change**: `internal/tui/table.go` — added `renderRowInto` / `renderHighlightedRowInto` (builder-appending variants of `renderRow` / `renderHighlightedRow`). The original functions now delegate to the `*Into` variants and remain thin wrappers so `PrintTable` / `PrintStatusTable` callers (`fmt.Fprintln`) are unchanged. `internal/tui/dashboard_view.go` — `viewMain` and `viewHostMetrics` switched to call `renderRowInto` / `renderHighlightedRowInto` directly against their own `strings.Builder`, followed by a single `b.WriteByte('\n')` instead of `+ "\n"` concat. Other `b.WriteString(<styled> + "\n")` patterns in the same file split into separate `WriteString` + `WriteByte('\n')` pairs.

**Benchmark (go1.26.0, AMD Ryzen AI 9 HX 370, -benchtime=1s -benchmem -count=10 on BenchmarkViewMain/one_status)**:

| Metric  | Before (main)              | After (perf-assist/tui-render-low-alloc) | Delta |
| ------- | -------------------------- | ----------------------------------------- | ----- |
| allocs  | 322 /op (10/10 consistent) | 315 /op (10/10 consistent)                | **-7 (-2.2%)** |
| bytes   | 10,062-10,131 B/op         | 9,468-9,517 B/op                          | **~-590 (-5.9%)** |
| time    | 35-53k ns/op               | 36-52k ns/op                              | within noise |

The byte saving is more meaningful than the alloc drop because eliminating the inner `b.String()` copy drops a ~600 byte heap allocation per viewMain call (the inner builder's final length-and-copy). Lipgloss's internal allocations are unchanged.

**Post-merge confirmation (run 5, 2026-10-01)**: `BenchmarkViewMain/one_status` on `main` reads 36,719 ns/op, 9,451 B/op, 315 allocs/op — matches the PR's measured numbers. No regressions from the maintainer's intervening refactors (#510 dispatchAction, #509 forEachCacheHost, #508 fanOutHosts, #511 statusNativeState).

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

**Status**: Merged by maintainer 2026-09-27T23:42:47Z.

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
- Mechanical follow-up fixes (e.g. the `viewScroll` `+ "\n"` concat) are best deferred when the maintainer is in a refactor-consolidation phase — the PR churn isn't worth a 1-line change. Schedule such cleanups for quieter weeks.
- The TUI render builder-into-builder pattern (PR #502, PR #537) generalises to every helper that uses `b.WriteString(<rendered> + "\n")` or `"  " + line + "\n"`. Net for the four helpers: -26% to -77% allocs/op, -18% to -24% bytes/op.
- `strings.Builder.String()` is a no-copy refcount bump, so a helper-level bench that swaps the string form for the builder-into form shows NO change. The actual saving only shows up at the call site where the helper's result would have been concatenated back into a larger `strings.Builder` — bench the compositing path, not the helper.
- For a simple `O(N×M)` slice iteration vs a `map[string]T` build, the map wins ONLY when M is large enough that the map build cost is amortised across the lookups. On small/medium fixtures the map build cost (header + bucket array per scopeKey) dominates and the slice iteration is the right shape. Don't pre-optimise; bench with realistic sizes first.
- **Profile-driven allocation reduction**: when the bench shows high alloc count but the call site looks innocuous, `-memprofile` + `pprof -alloc_objects -list` finds the exact allocating line. The line attribution is precise: for `BenchmarkEnrichFromScopeRunners`, line 451 (`name+"-"+strconv.Itoa(j)`) was attributed 51% of total alloc events despite being 4 lines of obvious-looking code. The fix — keying the map on `rc.Name` instead of `name-N` — was measurable immediately (big fixture: -49.5% allocs/op; small: -46.4% allocs/op).
- **`strings.LastIndex` + slice for suffix extraction**: when the lookup key is `<base>-<suffix>` and the caller already has the full key, extract the base via `LastIndex` + slice rather than concatenating `<base>+"-"+<suffix>` to index a precomputed map. The slice is a no-alloc pointer+length on the underlying string. Benchmark validated on `enrichFromScopeRunners`.
- **Import cuts follow key-shape changes**: removing `strconv` from runner.go (no longer needed once we stopped using `strconv.Itoa(j)` for map keys) is a free, low-risk simplification that comes with the actual perf change.
- **Behaviour-equivalence for "map[key collisions pick the last writer"**: the prior `rcByInstance` and the new `rcByName` both pick whichever `*RunnerConfig` is written to the map last for a given key (the prior picked the last `name-N` insert for runners sharing a base name, the new picks the last base-name insert; same downstream effect because lookup keys collapse). The validation rule `runner instance must be unique per host` (config_test.go) makes the multi-host same-name case a non-issue in practice.

### 2026-10-08 (run 6) — TUI menu/scroll builder-into-builder sweep (draft PR on `perf-assist/menu-render-builder-into`)

**PR opened**: `[perf-improver] perf(tui): drop last "+ "\n"" concats in menu/scroll views via renderMenuItemsInto` (DRAFT, branch `perf-assist/menu-render-builder-into`, commit `697b2d4`).

**Status**: Draft — pushed to branch, awaiting maintainer review.

**Change**: `internal/tui/dashboard_view.go` —
- Added `renderMenuItemsInto(b *strings.Builder, items []string, cursor int)` mirroring the `renderRowInto` shape. The original `renderMenuItems(items, cursor)` now delegates (`var b strings.Builder; renderMenuItemsInto(&b, items, cursor); return b.String()`) so existing test callers are unchanged.
- All four menu views (`viewActionMenu`, `viewGlobalMenu`, `viewFilterMenu`, `viewFilterList`) call `renderMenuItemsInto(&b, ...)` instead of concatenating the helper's return value.
- Trailing `helpStyle.Render("...") + "\n"` in each menu view split into separate `b.WriteString(helpStyle.Render("..."))` + `b.WriteByte('\n')` calls.
- `viewScroll` per-line `"  " + line + "\n"` split into `b.WriteString("  ")` + `b.WriteString(line)` + `b.WriteByte('\n')`.

`internal/tui/dashboard_view_bench_test.go` — added 4 new benchmarks:
- `BenchmarkRenderMenuItems` + `BenchmarkRenderMenuItemsInto` (pair so the helper-only difference is quantifiable; intentionally identical because `strings.Builder.String()` is no-copy)
- `BenchmarkViewActionMenu` (full action-menu compositing, captures the caller-side saving)
- `BenchmarkViewScroll` + `scrollFixture()` (200-line scroll fixture for the `viewScroll` per-line fix)

**Benchmark (go1.26.0, AMD Ryzen AI 9 HX 370, -benchtime=500ms -benchmem -count=10)**:

| Bench | Before (main) | After (`perf-assist/menu-render-builder-into`) | Δ |
| --- | --- | --- | --- |
| `BenchmarkViewActionMenu` | 7,252 ns/op, 1,537 B/op, 39 allocs | 5,373 ns/op, 1,169 B/op, 34 allocs | **-26% time, -24% bytes (-368 B), -13% allocs** |
| `BenchmarkViewScroll` (200-line fixture) | 14,560 ns/op, 32,601 B/op, 122 allocs | 11,042 ns/op, 26,578 B/op, 28 allocs | **-24% time, -18% bytes (-6,023 B), -77% allocs (-94)** |
| `BenchmarkRenderMenuItems` (helper) | 365 ns/op, 576 B/op, 13 allocs | 365 ns/op, 576 B/op, 13 allocs | wash (expected — `String()` is no-copy) |
| `BenchmarkRenderMenuItemsInto` (helper) | n/a | 365 ns/op, 576 B/op, 13 allocs | identical to wrapper, as expected |

Output is byte-identical to main (no visible-text changes). Existing menu tests (`TestViewActionMenu`, `TestViewGlobalMenu`, etc.) still pass.

**Verification**: `go build ./...` OK, `go vet ./...` OK, `gofmt -l .` clean, `go test ./... -race -count=1 -short` OK across all 17 packages.

**Also tried (REJECTED): `enrichFromScopeRunners` map-lookup variant**. Built a per-scope `map[string]GitHubRunner` to turn the inner O(N×M) `gr.Name != statuses[i].Instance` scan into an O(1) lookup. Bench was a CLEAR LOSS on every metric:
- Big fixture (20 repos × 10 instances): 49,054 → 61,713 ns/op (+26%), 117,528 → 156,576 B/op (+33%), 420 → 503 allocs (+83)
- Small fixture (5 repos × 2 instances): 2,478 → 4,266 ns/op (+72%), 6,488 → 11,208 B/op (+73%), 28 → 38 allocs (+10)

The map allocation overhead (one map header + bucket array per `scopeKey`) dominates the saved string compares on every fixture size. The function's alloc count is dominated by the `name+"-"+strconv.Itoa(j)` strings in the `rcByInstance` build (~200 per call on the big fixture), NOT by the GitHub-runner scan. **Conclusion**: the O(N×M) slice iteration is the right shape. Caching `rcByInstance` across `EnrichWithGitHubStatus` calls with cfg-change detection would be a bigger refactor and isn't worth the complexity for the per-tick cost. Backlog item #5 closed; conclusion recorded in `opportunities.md`.

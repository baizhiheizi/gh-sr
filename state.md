---
name: efficiency-improver-state
description: Efficiency Improver persistent state — last run, work in progress, discovered commands, and validated optimization patterns for baizhiheizi/gh-sr.
metadata:
  type: project
---

## Last Run

- Date: 2026-10-14
- Run ID: 38642960211
- Workflow: efficiency-improver

## Work This Run

- Re-validated build/test/vet/fmt against `go 1.26.0` via `GOTOOLCHAIN=auto`. All 17 packages green.
- Identified 2 candidates via backlog (both HIGH); both live in the same file `internal/agentic/agentic.go`. Picked the more impactful one: dedup of `containerCheckDefs` rebuild on the fanout + per-check paths.
- Created branch `efficiency/agentic-dedup-defs` and draft PR. Extracted 6 per-def constructors (`innerHostDockerInternalDef`, `nodeNPMDef`, `dockerSocketUserDef`, `zstdDef`, `cacheEnvDef`, `mtuDef`) so both `containerCheckDefs` (slice-of-all) and `containerCheckDefByName` (single-name lookup) compose from one source of truth. Added `containerAgenticFanoutCheckCommandFromDefs(outerContainer, defs)` and `containerAgenticFanoutSpecsFromDefs(defs)`. `ValidateContainerAgenticFanout` now builds the defs slice once and feeds both downstream helpers from it.
- 3 new tests: `TestContainerCheckDefByName_MatchesFanoutDefs` (walks every slice entry, asserts byName returns the same fields — pins against drift between the two paths after the per-def split), `TestContainerCheckDefByName_GatedDefReturnsFalseOffGate` (cache-env + MTU gates, plus unknown-name returns false), `TestValidateContainerAgenticFanout_BuildDefsOnce` (production wrapper cmd/specs match what the slice builder produces via FromDefs).
- Updated `[efficiency-improver] Monthly Activity 2026-10` issue #529 with new run-history entry + replaced the two addressed candidates in the backlog.

## Measurements (this run)

- `BenchmarkValidateContainerAgenticFanout_Setup/always_on/shared` (5 reps, AMD Ryzen AI 9 HX 370):
  - Before (split): 3124–3670 ns/op, 9492 B/op, 49 allocs/op
  - After  (shared): 2050–2289 ns/op, 6640 B/op, 28 allocs/op
  - **-35% wall-clock, -30% bytes, -43% allocs**
- `BenchmarkValidateContainerAgenticFanout_Setup/all_gated/shared`:
  - Before (split): 5488–7169 ns/op, 14939 B/op, 81 allocs/op
  - After  (shared): 2941–4020 ns/op, 10005 B/op, 44 allocs/op
  - **-43% wall-clock, -33% bytes, -46% allocs**
- `BenchmarkContainerCheckDefByName/always_on_inner`:
  - Before: 959–1164 ns/op, 1906 B/op, 19 allocs/op
  - After:  371–417 ns/op, 624 B/op, 5 allocs/op
  - **-64% wall-clock, -67% bytes, -74% allocs**
- `BenchmarkContainerCheckDefByName/gated_cache_env`:
  - Before: 1377–1623 ns/op, 3043 B/op, 24 allocs/op
  - After:  224–253 ns/op, 368 B/op, 4 allocs/op
  - **-84% wall-clock, -88% bytes, -83% allocs**
- `BenchmarkContainerCheckDefByName/gated_mtu`:
  - Before: 1568–2060 ns/op, 3620 B/op, 31 allocs/op
  - After:  539–594 ns/op, 945 B/op, 11 allocs/op
  - **-70% wall-clock, -74% bytes, -65% allocs**

## Open Items

- Draft PR `efficiency/agentic-dedup-defs` awaiting maintainer review.
- Draft PR `efficiency/cache-dedup-gateway-ssh` (from 2026-10-07) still awaiting review.
- `[efficiency-improver] Monthly Activity 2026-10` issue #529 open.

## Discovered Commands (validated)

```bash
# Build / test / vet / format
GOTOOLCHAIN=auto go build ./...
GOTOOLCHAIN=auto go test ./... -race -count=1
GOTOOLCHAIN=auto go vet ./...
gofmt -l .   # must be empty

# Benchmarking via Makefile
make bench          # go test ./... -run='^$' -bench=. -benchmem -count=3
make bench-save     # writes bench-results/bench-<UTC-stamp>.txt

# Local CI mirror
make ci             # vet + fmt + test

# Targeted perf bench for cache work
GOTOOLCHAIN=auto go test ./internal/cache/... -bench BenchmarkInspect -benchmem -count=3 -run='^$'

# Targeted perf bench for agentic work
GOTOOLCHAIN=auto go test ./internal/agentic/... \
  -bench 'BenchmarkContainerCheckDefs|BenchmarkValidateContainerAgenticFanout_Setup|BenchmarkContainerCheckDefByName' \
  -benchmem -count=5 -run='^$'
```

Notes:
- `go.mod` requires go 1.26.0; local toolchain is 1.25.9. Build needs `GOTOOLCHAIN=auto` (or running in an environment with 1.26.x pre-installed) so `go` auto-downloads 1.26.0.
- `make bench-save` is the canonical "save before / after" workflow — pair with `scripts/benchstat` to diff snapshots.
- SSH round-trip count (via `MockExecutor.Calls`) is the right proxy metric for cache work — wall-clock microbenchmarks with mocks understate network wins.
- For the agentic work the wall-clock microbenchmarks with mocks are the right proxy — the dedup is pure CPU/alloc work; there is no network round-trip removed.

## Validated Optimization Patterns

- **`strings.Split` → `strings.SplitSeq`** for newline-delimited iteration: drops the upfront `[]string` slice allocation; project's convention since PR #458 / #461 / #463 (perf-improver / repo-assist) and #498 (efficiency-improver).
- For bounded-line capture (≤ N lines), pre-size the destination: `make([]string, 0, N)`. Growing from `nil` via `append` regresses for short inputs because each append can reallocate the backing array.
- For early-stop after N lines, break out of the `SplitSeq` loop — avoids materialising the tail slice entirely.
- **Extract a private `xxxWithPreResolved(dep)` helper** when two callers both need to resolve the same external resource (e.g. gateway IP) and one already has it: public `xxx(h)` resolves-and-forwards, callers that already paid the cost pass the resolved value to the private helper. Mirrors the `localURL(gw)` pattern that already existed in `internal/cache`.
- Only resolve external resources when actually needed — adding `if s.URLOverride == "" && (s.BindAddr == "" || s.BindAddr == "0.0.0.0")` guard avoids wasted calls in the short-circuit branches. Saves 100% (not just 50%) when applicable.
- **Extract per-item constructors when an aggregate builder and a single-item lookup share metadata**. Both `containerCheckDefs` (build all) and `containerCheckDefByName` (build one) needed identical per-probe `Message`/`Remediation` strings. Splitting each def into its own helper (e.g. `innerHostDockerInternalDef`, `cacheEnvDef`) lets the slice builder compose `[]containerCheckDef{def1, def2, ...}` and the byName lookup `switch name { case "...": return helper(), true }` share the same source — eliminating the "build all to return one" cost (was 14 allocs/op for an always-on probe; now 5) and the "build twice in different helpers" cost (was 49 allocs/op for the fanout setup; now 28).
- When the slice builder and a *From* helper both want the same upstream slice, introduce `xxxFromDefs(defs)` as the primary path and keep the legacy `xxx(args)` as a thin wrapper that calls `containerCheckDefs` once and delegates. New callers that already have the slice use the From form; legacy test fixtures and existing call sites keep working unchanged.
- Pin the dedup with a test that walks every slice entry and asserts the per-item helper returns the same fields — guards against typo-level drift after the split.
- Existing benchmarks live alongside production code; follow the `MockExecutor` pattern when adding new ones. Add paired `/split` vs `/shared` sub-benchmarks when measuring a dedup — the gap between them is the energy the refactor saves.

## Backlog

- `internal/cache/cache.go` `Prune` line 469 — gateway resolve once per call; not worth caching unless many prunes per session. LOW.
- Continue `strings.Split` audit on remaining callers (`scripts/benchstat/main.go:56` — CLI flag splitting, low impact). LOW.
- Audit `dashboard.View()` / TUI rendering for further allocation reductions (mostly covered by PR #502, `viewScroll` is the remaining sibling). LOW.
- Review cache TTL/expiry policies in `internal/cache` and `internal/autostart` for data efficiency.
- The two HIGH-priority `internal/agentic/agentic.go` items were addressed in this run's PR (`efficiency/agentic-dedup-defs`).
---
name: efficiency-improver-state
description: Efficiency Improver persistent state — last run, work in progress, discovered commands, and validated optimization patterns for baizhiheizi/gh-sr.
metadata:
  type: project
---

## Last Run

- Date: 2026-10-07
- Run ID: 37700742259
- Workflow: efficiency-improver

## Work This Run

- Re-validated build/test/vet/fmt against `go 1.26.0` via `GOTOOLCHAIN=auto`. All 17 packages green.
- Identified 4 candidates via Explore agent; picked the duplicate docker0 gateway SSH resolve in `internal/cache/cache.go` (`Inspect` lines 407/410 and `Ensure` lines 293/306).
- Created branch `efficiency/cache-dedup-gateway-ssh` and draft PR. Extracted private `runnerURL(gatewayIP)` helper, kept public `RunnerURL(h)` signature, made `Inspect` and `Ensure` resolve the gateway once and share with `runnerURL`/`localURL`. BindAddr-set and URLOverride paths now resolve zero times (was one wasted resolve); auto-bind paths drop from 2 to 1 SSH round-trip.
- Added `BenchmarkInspect` + `TestInspect_gatewayResolvedOnce` (4 sub-tests) + `TestDeploy_gatewayResolvedOnce` (2 sub-tests) pinning the exact SSH-call count for every configuration.
- Created `[efficiency-improver] Monthly Activity 2026-10` issue.

## Measurements (this run)

- `BenchmarkInspect` (3 reps, mock executor, AMD Ryzen AI 9 HX 370):
  - Before: 1242–1538 ns/op, 1406–1476 B/op, 17 allocs/op
  - After:  1012–1114 ns/op, 1110–1175 B/op, 16 allocs/op
  - ~30% wall-clock, ~24% memory, –1 alloc on the local path; one full round-trip eliminated per `Inspect`/`Ensure` on real network (5–50 ms each on contended hosts).

## Open Items

- Draft PR `efficiency/cache-dedup-gateway-ssh` awaiting maintainer review.
- `[efficiency-improver] Monthly Activity 2026-10` issue open.

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
```

Notes:
- `go.mod` requires go 1.26.0; local toolchain is 1.25.9. Build needs `GOTOOLCHAIN=auto` (or running in an environment with 1.26.x pre-installed) so `go` auto-downloads 1.26.0.
- `make bench-save` is the canonical "save before / after" workflow — pair with `scripts/benchstat` to diff snapshots.
- SSH round-trip count (via `MockExecutor.Calls`) is the right proxy metric for cache work — wall-clock microbenchmarks with mocks understate network wins.

## Validated Optimization Patterns

- **`strings.Split` → `strings.SplitSeq`** for newline-delimited iteration: drops the upfront `[]string` slice allocation; project's convention since PR #458 / #461 / #463 (perf-improver / repo-assist) and #498 (efficiency-improver).
- For bounded-line capture (≤ N lines), pre-size the destination: `make([]string, 0, N)`. Growing from `nil` via `append` regresses for short inputs because each append can reallocate the backing array.
- For early-stop after N lines, break out of the `SplitSeq` loop — avoids materialising the tail slice entirely.
- **Extract a private `xxxWithPreResolved(dep)` helper** when two callers both need to resolve the same external resource (e.g. gateway IP) and one already has it: public `xxx(h)` resolves-and-forwards, callers that already paid the cost pass the resolved value to the private helper. Mirrors the `localURL(gw)` pattern that already existed in `internal/cache`.
- Only resolve external resources when actually needed — adding `if s.URLOverride == "" && (s.BindAddr == "" || s.BindAddr == "0.0.0.0")` guard avoids wasted calls in the short-circuit branches. Saves 100% (not just 50%) when applicable.
- Existing benchmarks live alongside production code; follow the `MockExecutor` pattern when adding new ones. The `ip -4 -o addr show docker0` substring count is a clean, mechanical way to assert the exact SSH-call count without instrumenting production code.

## Backlog

- `internal/agentic/agentic.go:379-380` — `containerCheckDefs` rebuilt twice per fanout call (once in `containerAgenticFanoutCheckCommand`, once in `containerAgenticFanoutSpecs`). Build once, share. MEDIUM.
- `internal/agentic/agentic.go:345-352` — `containerCheckDefByName` builds ALL defs to return one. Switch on `name` to build only the requested def. MEDIUM.
- `internal/cache/cache.go` `Prune` line 469 — gateway resolve once per call; not worth caching unless many prunes per session. LOW.
- Continue `strings.Split` audit on remaining callers (test fixtures, `scripts/benchstat/main.go:56` — CLI flag splitting, low impact).
- Audit `dashboard.View()` / TUI rendering for further allocation reductions (mostly covered by PR #502).
- Review cache TTL/expiry policies in `internal/cache` and `internal/autostart` for data efficiency.
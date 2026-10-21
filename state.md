---
name: efficiency-improver-state
description: Efficiency Improver persistent state — last run, work in progress, discovered commands, validated optimization patterns, and the energy-efficiency backlog for baizhiheizi/gh-sr.
metadata:
  type: project
---

## Last Run

- Date: 2026-10-21
- Run ID: 39681773081
- Workflow: efficiency-improver
- Round-robin slot this run: **Task 6 (measurement infrastructure)**. Tasks 2+3 were done on 2026-10-14.

## Work This Run

- Added a new benchmark metric, `round-trips/op`, that measures the **SSH round trips** (remote command invocations) an operation costs. Rationale: for `gh-sr` the dominant energy cost is the remote round trip (network hop + remote shell start + remote probe CPU), and the existing suite (ns/op, B/op, allocs/op) only measures local CPU/allocations.
- `internal/testutil/roundtrip.go` — new `RoundTripCounter` executor: counts `Run` calls but does **not** retain the command strings (retaining them would add per-iteration allocation churn inside the measured window and inflate allocs/op).
- New benchmark files: `internal/runner/roundtrip_bench_test.go`, `internal/agentic/roundtrip_bench_test.go`, `internal/cache/roundtrip_bench_test.go`.
- `scripts/benchstat/benchstat.go` — parse `round-trips/op`, compare it (absolute rule: any increase = fail), render a dedicated column (only when at least one row carries the metric, so existing report assertions are unchanged). New file `scripts/benchstat/roundtrip_test.go` with 7 tests.
- `Makefile` — documented the new metric on the `bench` target.
- Created draft PR on branch `efficiency/measure-ssh-round-trips` (PR number assigned after the run).
- Filed an issue: "benchstat drops every sub-benchmark row, so the CI regression gate never sees them" (with the no-op-diff noise measurements).
- Updated the `[efficiency-improver] Monthly Activity 2026-10` issue #529 by **appending** a dated update block (the issue body is unreadable through the MCP GitHub tools — see "Tooling limits" below — so a replace would have destroyed maintainer checkbox edits).

## Measurements (this run)

Round trips per operation (proxy metric = SSH round trips; exact counts, not wall-clock):

| Operation | Round trips |
| --------- | ----------- |
| `Manager.Status`, 1 native instance | 2 |
| `Manager.Status`, 4 native instances | 8 |
| `Manager.Status`, 16 native instances | 32 |
| `Manager.Status`, 4 container instances | 4 |
| `MeasureDiskUsageBatch`, 1 instance | 1 |
| `MeasureDiskUsageBatch`, 8 instances | 1 |
| `ValidateContainerAgenticFanout` (cache off / on) | 1 / 1 |
| `cache.Inspect` auto-bind / URL override | 5 / 4 |

Why native Status costs 2 per instance: `linuxInstanceProbe` (1) + `statusNativeState`'s systemctl probe (1, taken when the S marker is present). Container Status costs 1 (`containerLocalStatusOneShot` folds bootstrap-marker + docker inspect).

Metric stability: two back-to-back CI-equivalent bench runs (`go test ./... -run='^$' -bench=. -benchmem -count=3 -benchtime=500ms`) with **no code change** →
- `round-trips/op`: **0 % delta on every row**
- `ns/op`: up to **+47 %** on identical code, 5 rows over the fail threshold

## Tooling limits (important for future runs)

- **Issue #529 (the Monthly Activity issue) cannot be read** through the MCP GitHub tools — it is filtered by the integrity policy ("has lower integrity than agent requires"), as are the issue bodies created by this workflow's safeoutputs calls. `mcp__github__issue_read` and `search_issues` both return nothing for them.
  - Consequence: use `update_issue` with `operation: append` for the monthly issue, never `replace`, so maintainer checkbox edits survive.
  - Consequence: PR/issue numbers created via safeoutputs are **not visible** during the run — reference them by branch name / title instead of number.
- `mcp__github__ui_get` is denied in this run's permission mode; `mcp__github__list_label` works for label discovery.

## Open Items

- Draft PR on branch `efficiency/measure-ssh-round-trips` — measure SSH round trips per operation as an energy proxy (awaiting maintainer review).
- New issue: benchstat drops every sub-benchmark row (awaiting maintainer input on the noise-hardening approach).
- `[efficiency-improver] Monthly Activity 2026-10` issue #529 open.
- PR #570 (`[repo-assist] perf(doctor): batch the per-instance container probe into one SSH round-trip`) and PR #566 are other agents' work — not ours to maintain.

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

# CI-equivalent bench (what bench-compare.yml runs)
GOTOOLCHAIN=auto go test ./... -run='^$' -bench=. -benchmem -count=3 -benchtime=500ms | tee bench.txt

# Round-trip metric benchmarks only
GOTOOLCHAIN=auto go test ./internal/runner/ ./internal/agentic/ ./internal/cache/ \
  -run='^$' -bench='RoundTrips' -benchmem -count=3 -benchtime=500ms

# Diff two snapshots
go run scripts/benchstat/main.go -base base.txt -head head.txt -packages internal/runner -output out.md
```

Notes:
- `go.mod` requires go 1.26.0; local toolchain is older. Build needs `GOTOOLCHAIN=auto`.
- The full CI-equivalent bench run takes ~4 minutes on this box — run it in the background, not in the foreground of a 60 s Bash call.
- `scripts/benchstat/main.go` has a `//go:build ignore` tag; run it with `go run scripts/benchstat/main.go` (as the workflow does), not as part of `go build ./...`.

## Validated Optimization Patterns

- **`strings.Split` → `strings.SplitSeq`** for newline-delimited iteration (repo convention since PR #458/#461/#463/#498).
- For bounded-line capture (≤ N lines), pre-size the destination: `make([]string, 0, N)`.
- **Extract a private `xxxWithPreResolved(dep)` helper** when two callers resolve the same external resource.
- **Extract per-item constructors when an aggregate builder and a single-item lookup share metadata** (the `containerCheckDefs` split, PR #545).
- **Measure SSH round trips, not just CPU**, for any `gh-sr` operation that touches a host. `MockExecutor` records the command strings in `Calls`; a counting executor that does not retain them avoids polluting the alloc metrics.
- **A deterministic count needs no threshold.** The round-trip count is exact (fixed mock output → fixed call sequence), so the comparison is absolute: any increase fails. Wall-clock metrics still need the percentage thresholds.
- **Benchmark names must be flat.** `scripts/benchstat`'s regex `^Benchmark(\w+)(?:-\d+)?\s+(\d+)\s+(.*)$` does not match names containing `/`, so sub-benchmark rows are silently dropped from the CI comparison. Name new CI-relevant benchmarks flat (`BenchmarkFooCase1`) and put the case in the name.
- **Measure the noise before widening a CI gate.** Two identical bench runs are enough to tell a real regression from runner contention.

## Backlog

- **HIGH — Network/IO:** batch `Manager.Status` across the instances of one host. Today 2 round trips per native instance (32 for a 16-runner host, on the TUI's per-tick refresh path). Target shape already exists: `buildDirSizesBatchPOSIXScript` does 1 round trip for 8 instances. Baseline numbers are now measured by `ManagerStatusRoundTrips*`.
- **HIGH — Network/IO:** `cache.Inspect` costs 5 round trips per host (docker inspect, `echo $HOME`, gateway resolve, health curl, `du -sb`) → could be folded into one script like the other batched probes. Baseline measured by `InspectRoundTrips*`.
- **MEDIUM — Measurement:** close the `scripts/benchstat` sub-benchmark hole with noise hardening (issue filed this run). Unblocks gating for every table-driven benchmark.
- **MEDIUM — Measurement:** `gh sr doctor` round trips per host are not yet measured — `runHostChecks` needs a realistic mock fixture (the count depends on which checks short-circuit), so the benchmark was deferred.
- LOW — Code-Level: `internal/cache/cache.go` `Prune` line 469 — gateway resolve once per call.
- LOW — Code-Level: continue the `strings.Split` audit on remaining callers (`scripts/benchstat/main.go:56`).
- LOW — Code-Level: audit `dashboard.View()` / TUI rendering for further allocation reductions (`viewScroll` is the remaining sibling of PR #502).
- LOW — Data: review cache TTL / expiry policies in `internal/cache` and `internal/autostart`.
- The two HIGH-priority `internal/agentic/agentic.go` items were addressed by PR #545 (merged 2026-10-15).

## Last-Run Task Rotation

| Task | Last run | Status |
| ---- | -------- | ------ |
| 1 — discover/validate commands | 2026-10-14 | stable, no changes needed |
| 2 — identify opportunities | 2026-10-14 | done (agentic dedup) |
| 3 — implement improvements | 2026-10-14 | done (PR #545, merged) |
| 4 — maintain PRs | 2026-10-21 | no open `[efficiency-improver]` PRs of ours to maintain |
| 5 — comment on efficiency issues | 2026-10-21 | no open human-authored efficiency issues found (search for performance/slow/energy/optimization returned 0) — no comment posted, by design |
| 6 — measurement infrastructure | 2026-10-21 | done (round-trips/op metric) |
| 7 — monthly activity summary | 2026-10-21 | done (appended to #529) |

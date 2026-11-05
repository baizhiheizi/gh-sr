---
name: efficiency-improver-state
description: Efficiency Improver persistent state — last run, work in progress, discovered commands, validated optimization patterns, and the energy-efficiency backlog for baizhiheizi/gh-sr.
metadata:
  type: project
---

## Last Run

- Date: 2026-11-05
- Run ID: 41936221772
- Workflow: efficiency-improver
- Round-robin slot this run: **Tasks 2+3 (implement a backlog item)** + quick Task 4 verification + Task 7.

## Work This Run

- **cache.Inspect round-trip fold (5→1)** — the second HIGH Network/IO backlog item. `Inspect` issued five separate `h.Run` probes per host (docker inspect, `echo $HOME`, `ip -4 -o addr show docker0`, `curl /health`, `du -sb`); all five now run in one POSIX script via `inspectOneShotScript`, emitting tag-prefixed sections (`S:` state|image, `H:` home, `G:` docker0 ip line, `K:` health 1/0, `D:` du bytes) parsed by `parseInspectOneShot` (internal/cache/cache.go).
- Behavior preserved branch-for-branch: auto-bind extracts the gateway in-shell via `${gw#*inet }` parameter expansion (mirrors `parseGatewayIPOutput`) so the health curl targets the same address as the runner URL; no docker0 → loopback fallback; explicit BindAddr/URLOverride omit the `ip` probe entirely; container absent → script skips curl/du; empty `$HOME` still errors "resolving home dir".
- Scripts validated end-to-end in real shells: dumped the exact generated scripts via a throwaway test, `sh -n`/`bash -n` syntax check, then ran them with shimmed `docker`/`ip`/`curl` binaries — asserted the health probe URL derivation (`http://172.17.0.1:27420/health`) and the absent/no-docker0 fallbacks. Throwaway test deleted before commit.
- Updated per-substring mocks to composed-blob mocks: added `oneShotOutput` helper in internal/cache/cache_test.go; rewrote internal/cache/inspect_test.go, inspect_bench_test.go, roundtrip_bench_test.go, internal/doctor `reachabilityMock`, internal/ops `inspectingStateMock`.
- Created draft PR on branch `efficiency/cache-inspect-one-shot` (number not visible during run).
- Created `[efficiency-improver] Monthly Activity 2026-11` issue (this run). The 2026-10 issue #529 is **no longer open** (verified via list_issues: open issues are #595, #597–#600, all integrity-filtered) — no close needed.
- Task 4 check: PR #575 (`efficiency/measure-ssh-round-trips`, the round-trips/op metric) was **merged** (files on main; PR closed 2026-10-23). No open `[efficiency-improver]` PRs remain.

## Measurements (this run)

`cache.Inspect`, proxy metric = SSH round trips (exact), ns/B/allocs are mock-local:

| Benchmark | Metric | Before (main) | After (branch) |
| --------- | ------ | ------------- | -------------- |
| `InspectRoundTripsAutoBind` | round-trips/op | 5.000 | **1.000** |
| `InspectRoundTripsURLOverride` | round-trips/op | 4.000 | **1.000** |
| `InspectRoundTripsAutoBind` | ns/op | ~830 | ~1479 |
| `InspectRoundTripsAutoBind` | B/op | 688 | 2867 |
| `InspectRoundTripsURLOverride` | ns/op | ~548 | ~1016 |
| `BenchmarkInspect` | ns/op | ~930 | ~1495 |

The local ns/B regression (~+0.6 µs, +2.2 KB per call) buys four eliminated SSH round trips per host per invocation (~0.1–0.6 s wall on a 20–100 ms RTT VPS). Documented as the explicit trade-off in the PR.

Prior-run round-trip baselines (unchanged): `Manager.Status` 2 RT per native instance (32 for 16), `MeasureDiskUsageBatch` 1 RT, agentic fanout 1 RT.

Metric stability (2026-10-21 run, still valid): `round-trips/op` 0 % delta across identical runs; `ns/op` up to +47 % noise.

## Tooling limits (important for future runs)

- **safeoutputs-created issues/PRs cannot be read** through the MCP GitHub tools — integrity policy filters them. `issue_read`/`search_issues` return nothing for them; `list_issues` DOES reveal their **numbers** in the filter-removal note (e.g. "#595 … has lower integrity").
  - Consequence: use `update_issue` with `operation: append` for monthly issues, never `replace`.
  - Consequence: PR/issue numbers created via safeoutputs are **not visible** during the run — reference them by branch name / title instead of number.
- `mcp__github__ui_get` denied in this permission mode; `mcp__github__list_label` works for label discovery.
- search_pull_requests `fields` enum: `head` and `merged` are NOT valid (fails schema validation); use `number/title/state/draft/html_url/updated_at/closed_at`.

## Open Items

- Draft PR on branch `efficiency/cache-inspect-one-shot` — cache.Inspect 5→1 fold (awaiting maintainer review).
- Open issue (2026-10-21): benchstat drops every sub-benchmark row (awaiting maintainer input on the noise-hardening approach; it is one of the filtered open issues #595–#600).
- `[efficiency-improver] Monthly Activity 2026-11` issue open (created this run). #529 (2026-10) closed.
- PR #570 / #566 are other agents' work — not ours to maintain.

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
- **Multi-probe folds: emit tag-prefixed single-line sections** (`S:`/`H:`/`G:`/`K:`/`D:`), not sentinel-delimited blocks — prefix matching survives empty values and order is self-describing. Multi-line raw outputs (like `ip` output) get `head -n 1` in-shell, then the existing Go parser runs on the single line.
- **Mirror Go parsing in shell with parameter expansion, not awk/sed**: `${gw#*inet }` → `${g%% *}` → `${g%%/*}` reproduces `parseGatewayIPOutput` exactly, no quoting hazards, verified against real `ip -o` output.
- **Validate generated scripts end-to-end**: dump the exact script via a throwaway `go test`, `sh -n`/`bash -n` it, then run it with shimmed binaries on PATH (each shim a 1-2 line `#!/bin/sh` echoer) to assert URLs/fallbacks. Delete the throwaway before committing.
- **One-shot scripts must exit 0**: end on a `printf`/`fi` (false `if` with no else exits 0 in bash); `session.Run` errors on non-zero exit and the fold wraps that as the operation error.
- **When a fold removes per-probe `h.Run` calls, rewrite the per-substring mocks as composed-blob mocks** (helper like `oneShotOutput(st, home, gw, body, du)` per test package). Script-text substring assertions (e.g. gatewayCallCount on `ip -4 -o addr show docker0`) become STRONGER — they now assert the probe is absent from the script itself.

## Backlog

- **HIGH — Network/IO:** batch `Manager.Status` across the instances of one host. Today 2 round trips per native instance (32 for a 16-runner host, on the TUI's per-tick refresh path). Target shape already exists: `buildDirSizesBatchPOSIXScript` does 1 round trip for 8 instances. Baseline numbers are measured by `ManagerStatusRoundTrips*`. This is now the top remaining backlog item — the natural next Task 3 candidate (bigger refactor; budget a whole run for it).
- **MEDIUM — Measurement:** close the `scripts/benchstat` sub-benchmark hole with noise hardening (issue filed 2026-10-21). Unblocks gating for every table-driven benchmark.
- **MEDIUM — Measurement:** `gh sr doctor` round trips per host are not yet measured — `runHostChecks` needs a realistic mock fixture (the count depends on which checks short-circuit), so the benchmark was deferred.
- LOW — Code-Level: `internal/cache/cache.go` `Prune` — gateway resolve once per call.
- LOW — Code-Level: continue the `strings.Split` audit on remaining callers (`scripts/benchstat/main.go:56`).
- LOW — Code-Level: audit `dashboard.View()` / TUI rendering for further allocation reductions (`viewScroll` is the remaining sibling of PR #502).
- LOW — Data: review cache TTL / expiry policies in `internal/cache` and `internal/autostart`.
- DONE: cache.Inspect fold (draft PR this run, branch `efficiency/cache-inspect-one-shot`); agentic dedup (PR #545, merged 2026-10-15); round-trips/op metric (PR #575, merged 2026-10-23).

## Last-Run Task Rotation

| Task | Last run | Status |
| ---- | -------- | ------ |
| 1 — discover/validate commands | 2026-10-14 | stable, no changes needed |
| 2 — identify opportunities | 2026-10-14 | done (backlog driven since) |
| 3 — implement improvements | 2026-11-05 | done (cache.Inspect 5→1 fold, draft PR) |
| 4 — maintain PRs | 2026-11-05 | no open `[efficiency-improver]` PRs; verified #575 merged |
| 5 — comment on efficiency issues | 2026-11-05 | all 5 open issues (#595–#600) are agent-created (integrity-filtered) — no human-authored efficiency issues; no comment by design |
| 6 — measurement infrastructure | 2026-10-21 | done (round-trips/op metric, merged as #575) |
| 7 — monthly activity summary | 2026-11-05 | done (created Monthly Activity 2026-11; #529/2026-10 already closed) |

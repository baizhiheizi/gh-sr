---
name: perf-improver-runs
description: Round-robin task history — which tasks ran when, plus backlog cursor for next run.
metadata:
  type: project
---

# Round-robin task history

Use this to spread work across runs: prefer tasks that haven't run for the longest.

## 2026-09-03 (run_id 33764880799) — First perf-improver run

All 7 tasks completed in a single run (fresh repo, no prior state).

- ✅ Task 1 (Discover commands): validated `go build/test/vet`, `gofmt -l .`, `make bench/bench-save`, `make ci`. Stored in [[perf-improver-commands]].
- ✅ Task 2 (Identify opportunities): profiled View() (322 allocs/op from lipgloss internals), found two SplitSeq offenders in agentic package. Stored in [[perf-improver-opportunities]].
- ✅ Task 3 (Implement improvements): opened PR #458 (`[perf-improver] perf(agentic): use SplitSeq for tagged-output parsers on doctor path`). See [[perf-improver-work]].
- ✅ Task 4 (Maintain PRs): no prior perf-improver PRs to maintain.
- ✅ Task 5 (Comment on perf issues): no open performance-labelled issues.
- ✅ Task 6 (Measurement infra): repo already has CI bench-compare workflow + `make bench/bench-save`. No infra work needed.
- ✅ Task 7 (Monthly summary): created issue #459 `[perf-improver] Monthly Activity 2026-09`.

## 2026-09-03 (run_id 33809625237) — Second perf-improver run

- ✅ Task 1 (Discover commands): re-validated `go build`, `gofmt -l .`, `go vet` all OK. Commands in [[perf-improver-commands]] still current.
- ✅ Task 2 (Identify opportunities): picked up at top of [[perf-improver-opportunities]]. After agentic SplitSeq merged and repo-assist covered non-agentic SplitSeq, only two `strings.Split(out, "\n")` callers remained on the Status hot path: `ProbeDinDContainerReadiness` (container.go) and `linuxInstanceProbe` (linux_instance_probe.go). Backlog updated.
- ✅ Task 3 (Implement improvements): opened draft PR `[perf-improver] perf(runner): use SplitSeq for per-instance probe parsers on Status path` on branch `perf-assist/runner-probes-splitseq`. `LinuxInstanceProbe` shows -8.7% bytes, -1 alloc; `ProbeDinDContainerReadiness` wash (mock harness swamping the saving). See [[perf-improver-work]].
- ✅ Task 4 (Maintain PRs): prior PR #458 was MERGED at 2026-09-03T22:55:38Z (between this run and the prior one). No other perf-improver PRs to maintain.
- ✅ Task 5 (Comment on perf issues): no open performance-labelled issues besides the Monthly Activity issue.
- ✅ Task 6 (Measurement infra): added `internal/runner/runner_probe_bench_test.go` (5 sub-benches covering healthy/inner-down/user-systemd/system-systemd/with-dir cases). Side-effect of Task 3.
- ✅ Task 7 (Monthly summary): updating issue #459 this run — removes merged PR #458 action item, adds new SplitSeq probe PR action item, prepends run history entry.

## 2026-09-24 (run_id 36067889544) — Fourth perf-improver run

Took the **TUI render low-alloc** path (backlog item #1 / item #2 from prior runs). The remaining `strings.Split(out, "\n")` offenders flagged at run 3 (`autostart.go`, `agentic.go`, `lockfiles.go`, `main.go`, `tui/dashboard.go:799` `wrapLines`) are all legitimate slice-index uses (or are now covered by the efficiency-improver's PR #498), so the SplitSeq backlog is closed from perf-improver's perspective.

- ✅ Task 1 (Discover commands): re-validated `go build`, `gofmt -l .`, `go vet` all OK. Commands in [[perf-improver-commands]] still current. Note that `go.mod` requires `go >= 1.26.0`; use `GOTOOLCHAIN=auto` so the toolchain auto-fetches.
- ✅ Task 2 (Identify opportunities): picked up at top of [[perf-improver-opportunities]]. Profiled `BenchmarkViewMain/one_status` (322 allocs/op, 96% from lipgloss internals). Decided to attack the ~5% of allocs under our control (intermediate `strings.Builder` + `b.String()` copy inside `renderRow` / `renderHighlightedRow`, and `+ "\n"` string concats in `viewMain` / `viewHostMetrics`). Backlog updated; item #1 partially de-risked, item #2 (cell padding) still speculative.
- ✅ Task 3 (Implement improvements): opened draft PR `[perf-improver] perf(tui): drop per-row builder copy + "+ "\n"" concats in viewMain` on branch `perf-assist/tui-render-low-alloc` (commit `e287cde`). See [[perf-improver-work]] for full benchmark table.
- ✅ Task 4 (Maintain PRs): prior PRs #458 and #463 are MERGED. The only outstanding automation-labelled PR in the repo is efficiency-improver's #498 (out of scope). No perf-improver PRs needed maintenance.
- ✅ Task 5 (Comment on perf issues): no open performance-labelled issues besides the Monthly Activity issue.
- ✅ Task 6 (Measurement infra): added two production-colorize benchmarks in `internal/tui/table_bench_test.go` so future TUI render changes can be measured against the real `runnerStatusColorize` path. Side-effect of Task 3. No infra work otherwise.
- ✅ Task 7 (Monthly summary): updating issue #459 this run — appends new run-history entry, references the new draft PR.

## 2026-10-01 (run_id 36939788959) — Fifth perf-improver run

First run after PR #502 landed (merged 2026-09-27). Maintainer has been doing consolidation refactors (#510 dispatchAction, #509 forEachCacheHost, #508 fanOutHosts, #511 statusNativeState). No PR this run — deferred the trivial `viewScroll` `+ "\n"` mechanical follow-up so it lands after the refactor churn settles. Closed the September #459 issue and rolled over to a new October summary issue.

- ✅ Task 1 (Discover commands): re-validated `go build`, `go vet`, `gofmt -l .` all OK. `go test ./... -race -count=1 -short` passes across all 17 packages. Commands in [[perf-improver-commands]] still current; noted `GOTOOLCHAIN=auto` requirement (still need to fetch go1.26.0).
- ✅ Task 2 (Identify opportunities): picked up at top of [[perf-improver-opportunities]]. Re-ran the bench suite (`internal/{runner,tui,ops,autostart,host,agentic}`); `BenchmarkViewMain/one_status` now at 315 allocs/op on main (PR #502 merged). All other benchmarks within expected noise — no regressions from the recent maintainer refactors. New finding: `BenchmarkEnrichFromScopeRunners` is now the largest non-lipgloss alloc hotspot (420 allocs/op at 200 statuses) because the inner O(N×M) `gr.Name != statuses[i].Instance` scan against `scopeRunners[key]` dominates. Adding as new backlog item #5; needs a fresh bench pass to weigh the +alloc map-build against the -CPU scan trade-off before committing.
- ⏭️ Task 3 (Implement improvements): **skipped** this run. Backlog items #3 (`viewScroll` `+ "\n"`) and #5 (`enrichFromScopeRunners` N×M scan) are the only remaining low-risk wins; #3 is mechanical but might collide with the maintainer's recent refactor churn, #5 needs more bench data. Not worth a marginal PR this period.
- ✅ Task 4 (Maintain PRs): no open perf-improver PRs. Prior PRs #458, #463, #502 are all merged.
- ✅ Task 5 (Comment on perf issues): no open performance-labelled issues besides the Monthly Activity issues.
- ✅ Task 6 (Measurement infra): no infra work needed. Existing bench suite (4 viewMain, 2 metricsRow, 5 runner probe, etc.) covers the hot paths. Could add a `BenchmarkEnrichFromScopeRunners_Map` variant next time item #5 is picked up.
- ✅ Task 7 (Monthly summary): closed September issue #459 (with rollover comment) and opened the new `[perf-improver] Monthly Activity 2026-10` issue. Updated [[perf-improver-opportunities]] with item #5 (EnrichFromScopeRunners N×M scan) and re-prioritised #1–#4 against the new state.

## Backlog cursor for next run

Pick up at the top of [[perf-improver-opportunities]]:

1. **View() alloc reduction — remaining lipgloss cost** — was HIGH/HIGH; now MEDIUM/MEDIUM after PR #502 (merged 2026-09-27) closed the caller-side alloc opportunities. The remaining ~95% is inside `lipgloss.Style.Render` (line-wrapping `strings.Split`, ANSI byte-buffer growth). HIGH risk, only attempt if a future run wants to push further.
2. **renderRow / renderHighlightedRow cell padding** — MEDIUM impact, LOW risk. Still speculative. Worth profiling after the *Into variants are merged to see whether padding-skip still gives measurable savings on top.
3. **`viewScroll` `+ "\n"` concat** — `internal/tui/dashboard_view.go:250` has `b.WriteString("  " + m.scrollLines[i] + "\n")` repeated per line. Same shape as the merged `viewMain` / `viewHostMetrics` fix (PR #502). LOW impact (only fires when scroll mode is active, not the per-tick hot path). LOW risk. Quick mechanical follow-up if a future run wants to clear it out; deferred this run so it doesn't collide with the maintainer's recent refactor churn (#510/#509/#508/#511).
4. **Manager.Status further per-instance optimization** — LOW priority. Marginal gains expected.
5. **EnrichFromScopeRunners N×M GitHub-runner scan** — NEW this run. `BenchmarkEnrichFromScopeRunners` 420 allocs/op, 117 KB/op at 200 statuses (the dominant non-lipgloss alloc hotspot on the TUI refresh path). Hot loop does O(N×M) `gr.Name != statuses[i].Instance` comparisons against `scopeRunners[key]`. Building a `map[string]GitHubRunner` per scopeKey would turn this into O(N+M) at the cost of one map-alloc per scope. MEDIUM impact for big configs (where the comparison cost dominates), probably neutral for small configs (10 statuses × 10 runners). NEEDS a fresh bench pass comparing both strategies on the same fixture before committing. Could also be neutralised by caching `rcByInstance` + the scopeKey→runners map across `EnrichWithGitHubStatus` calls (cfg-change detection required — adds complexity).
6. **Periodic `strings.Split` grep** — DONE for current code; worth re-running if new code lands.
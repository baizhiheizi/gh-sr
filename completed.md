---
name: completed
description: "Completed efficiency work (PRs, outcomes)"
metadata:
  node_type: memory
  type: project
  originSessionId: 04e58c81-0a51-4f35-bd75-e67ad9ba414d
---

# Completed Work

## 2026-06-11 — FindRunnerForLogs single-pointer + early-exit (this run)

Branch: `efficiency/config-find-runner-for-logs` (commit 2eb47b6).
PR title: `[efficiency-improver] perf(config): inline single-pointer scan + early-exit in FindRunnerForLogs`.
Patch: `/tmp/gh-aw/aw-efficiency-config-find-runner-for-logs.patch` (awaiting PR URL).

Files: internal/config/config.go (FindRunnerForLogs), internal/config/bench_test.go (+2 benchmarks).

`BenchmarkFindRunnerForLogs_Match` 5906→790 ns/op (-86%), 6430→144 B/op (-98%), 297→5 allocs/op (-98%).
`BenchmarkFindRunnerForLogs_Ambiguous` 3987→150 ns/op (-96%), 5254→144 B/op (-97%), 66→5 allocs/op (-92%).

Three layered fixes:
1. Drop dead-code `map[*RunnerConfig]bool` (each `r := &c.Runners[i]` is unique — `seen[r]` was always false).
2. Replace allocating `r.InstanceNames()` with the existing allocation-free `matchesRunnerInstanceName` helper.
3. Single `*RunnerConfig` + `string otherHost` state, break on second match (result already known).

Trade-off: ambiguous-host error message format changed from `[h1 h2 h3]` to `(h1, h2)` — short-circuits after two matches. Actionable instruction identical ("specify --host"). Test only asserts error is returned, not format, so it passes.

## 2026-06-10 — InstanceNames helper fmt.Sprintf → strconv.Itoa (merged as #146)

Branch: `efficiency/instance-names-strconv-itoa` (commit 1cc51e5). MERGED 2026-06-11T04:06:49Z as PR #146.
PR title: `[efficiency-improver] perf(config): inline name-N construction in InstanceNames helper`.

## 2026-06-09 — single du walk in dirSizesPOSIX (merged as #136)

Branch: `efficiency/disk-single-du-walk` (commit d0e357d).
PR title: `[efficiency-improver] perf(runner): single du walk in dirSizesPOSIX (4 round trips → 1)`.
Status: MERGED 2026-06-09 (commit 46b6278 on main).

Local 7.9 GB: 0.769s → 0.727s (5% wall-clock); byte-identical output. On 50 GB remote: ~9-15s saved per instance (3 fewer SSH round trips + 3 fewer tree walks).

Extracted `buildDirSizesPOSIXScript(instance)` so the structural test calls the real production function. `du --max-depth=1` (GNU) / `du -d 1` (BSD) detected at script start. Output format unchanged, so parser + mocks needed only string-matcher updates.

## Prior (now merged)

- PR #128 `[perf-improver] perf(config): inline instance-name lookup in Validate` — merged 2026-06-09. Validate_Large 711→411 allocs/op (-42%), 49,643→18,529 ns/op (-63%).
- PR #123 `[efficiency-improver] perf(config): inline instance-name lookup` — merged 2026-06-09. FilterRunners_ByName 503→1 allocs/op (502×).

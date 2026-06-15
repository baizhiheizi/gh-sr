---
name: completed
description: "Completed efficiency work (PRs, outcomes)"
metadata:
  type: project
---

# Completed Work

## 2026-06-15 — TUI extractTrailingPercent ParseFloat (this run)

Branch: `efficiency/tui-parsefloat-percentage-parse` (commit 8ae6038). Patch: `/tmp/gh-aw/aw-efficiency-tui-parsefloat-percentage-parse.patch`.
PR title: `[efficiency-improver] perf(tui): use strconv.ParseFloat in extractTrailingPercent`.

`BenchmarkExtractTrailingPercent` 3806→452 ns/op (-88%), 818→160 B/op (-80%), 36→6 allocs/op (-83%). `fmt.Sscanf` → `strconv.ParseFloat`. Hot path: per colored host-metrics cell per Bubble Tea View() call. First TUI-side bench.

## 2026-06-12 — EnrichWithGitHubStatus inline rcByInstance (merged as #167)

PR #167 MERGED 2026-06-12T22:59:59Z. `EnrichFromScopeRunners_Small` 33→28 allocs/op (-15%). Third `InstanceNames()` discard-site closed (after #146 and #155).

## 2026-06-11 — FindRunnerForLogs (merged as #155)

PR #155 MERGED 2026-06-12T02:51:36Z. `FindRunnerForLogs_Match` 5906→790 ns/op (-86%), 297→5 allocs/op (-98%). Removed dead-code map + replaced allocating InstanceNames() with allocation-free helper + single-pointer state machine.

## 2026-06-10 — InstanceNames helper (merged as #146)

PR #146 MERGED 2026-06-11T04:06:49Z. fmt.Sprintf → `name + "-" + strconv.Itoa(i)`. 21→11 allocs/op, 1239→~430 ns/op. Helper called 23+ times across codebase.

## 2026-06-09 — single du walk in dirSizesPOSIX (merged as #136)

PR #136 MERGED 2026-06-09. 4 `du -sk` → 1 `du --max-depth=1` walk. 4 SSH round trips → 1. On 50 GB remote: ~9-15s saved per instance.

## Prior (merged)

- PR #128 Validate_Large 711→411 allocs/op (-42%) — MERGED 2026-06-09
- PR #123 FilterRunners_ByName 503→1 allocs/op (502×) — MERGED 2026-06-09

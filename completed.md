---
name: completed
description: "Completed efficiency work (PRs, outcomes)"
metadata:
  type: project
---

# Completed Work

## 2026-07-09 — scripts/benchstat RenderMarkdown (-51% time / -79% allocs) — **DRAFT PR pending**

Branch `efficiency/benchstat-format-fprintf-replace` (34c4908). Issue: `fmt.Fprintf(&sb, ...)` triggers reflection (no Fprintf fast-path on `*strings.Builder`).

| Bench | Before | After | Δ |
|---|---|---|---|
| `BenchmarkRenderMarkdown` | 12,877 ns/op · 6,205 B/op · **226 allocs/op** | 6,248 ns/op · 2,800 B/op · **47 allocs/op** | **−51.5% / −54.9% / −79.2%** |
| `BenchmarkFullPipeline` | 42,309 ns/op · 29,045 B/op · 324 allocs/op | 31,427 ns/op · 25,637 B/op · 145 allocs/op | **−25.7% / −11.7% / −55.2%** |

Fix: `sb.WriteString(...)` + `strconv.AppendFloat([24]byte, ...)` / `strconv.AppendFloat([32]byte, ...)`, `sb.Grow(128 + 96*len(rows))`. New helpers: `formatDeltaTo([]byte, float64)`, `writeMetricCell`. 3 new bench funcs added (picked up by bench-compare.yml baseline on next main run). 14/14 existing benchstat_test.go + 16/16 packages pass.

## 2026-07-07 — FormatBytesHuman inline unit suffix (-14.3%) — **MERGED PR #323**

Branch commit 9337d8c; tree 14d1edb. `BenchmarkFormatBytesHuman` (5×50000x): avg 443.5 → 379.9 ns/op; per-call ~79 → ~65 ns/op (-18%).

## Earlier merged

- #322 doctor fan-out, #226 ContainerImageLayoutRevision hoist (-85%/-89%/-50%), #213 Manager.Status hoist (-15.74% allocs), #203 container status SSH (3→1), #191 extractTrailingPercent (-88%), #167 EnrichWithGitHubStatus (-15%), #155 FindRunnerForLogs (-86%/-98%), #146 InstanceNames (-48%), #136 dirSizesPOSIX single du walk, #128 Validate_Large, #123 FilterRunners (502×), #249 TUI render, TUI metrics (squash 2373126).

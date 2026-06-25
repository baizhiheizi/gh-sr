---
name: completed
description: "Completed efficiency work (PRs, outcomes)"
metadata:
  type: project
---

# Completed Work

## 2026-06-25 — TUI metrics strconv.FormatFloat (no PR — silent tool failure, 3rd consecutive)

Branch: `efficiency/metrics-strconv-formatfloat-2026-06-25` (commit 09ca658). `metricsRow` (3 cells) + `LoadStr` + `FormatHostMetrics` (header + cells) swapped from `fmt.Sprintf` to `strconv.FormatFloat` + `strings.Builder`. New `appendHostCell` helper inlines the column padding/separator. New tests `TestLoadStr` + `TestFormatHostMetrics`.

**Energy evidence** (5×10000x on AMD Ryzen AI 9 HX 370):
- `BenchmarkMetricsRow`: ~1968→~1500 ns/op (**-24%**), 472→456 B/op (-3.4%), **20→16 allocs/op (-20%)**.
- `BenchmarkFormatHostMetrics` (new): ~2627 ns/op, 1856 B/op, 35 allocs/op.

**Test status**: build/vet/format/race clean, 12/12 packages pass.

**PR creation failure**: `safe-outputs create_pull_request` reported success 3× but no PR opened. Patch + bundle at `/tmp/gh-aw/aw-efficiency-metrics-strconv-formatfloat-2026-06-25.{patch,bundle}`. Recurring intermittent failure.

## 2026-06-24 — TUI render strings.Builder (PR #249 MERGED)

Branch: `efficiency/tui-render-strings-builder`. `renderHeader`/`renderRow`/`renderHighlightedRow` swapped from `var line string + line +=` to `var b strings.Builder`. **Energy**: allocs 161/255/291→158/248/284 (-2-3%); B/op -5-7%. **PR history**: 3× silent failure → eventually merged as PR #249 on 2026-06-24T02:01:58Z.

## Earlier (merged)

- PR #226 (ContainerImageLayoutRevision hoist) MERGED 2026-06-19. 326,748→50,605 ns/op (-85%), 167→84 allocs/op (-50%).
- PR #213 (Manager.Status loop-invariant hoist) MERGED 2026-06-19. 197→166 allocs/op (-15.74%, p=0.008).
- PR #203 (container status SSH consolidation) MERGED 2026-06-18. 3/2 → 1 SSH round trip.
- PR #191 (TUI extractTrailingPercent ParseFloat) MERGED 2026-06-15. 3806→452 ns/op (-88%).
- PR #167 (EnrichWithGitHubStatus) MERGED 2026-06-12. 33→28 allocs/op (-15%).
- PR #155 (FindRunnerForLogs) MERGED 2026-06-12. 5906→790 ns/op (-86%).
- PR #146 (InstanceNames) MERGED 2026-06-11. 21→11 allocs/op.
- PR #136 (single du walk) MERGED 2026-06-09.
- PR #128 (Validate_Large) MERGED 2026-06-09.
- PR #123 (FilterRunners_ByName) MERGED 2026-06-09. 503→1 allocs/op (502×).

---
name: completed
description: "Completed efficiency work (PRs, outcomes)"
metadata:
  type: project
---

# Completed Work

## 2026-06-23 — TUI render strings.Builder (no PR — silent tool failure, 3rd consecutive)

Branch: `efficiency/tui-render-strings-builder` (commit e430457). `internal/tui/table.go` — `renderHeader`/`renderRow`/`renderHighlightedRow` swapped from `var line string + line +=` (O(N²) string concat) to `var b strings.Builder` + `b.WriteString(...)`. New `internal/tui/table_bench_test.go` pins per-call cost.

**Energy evidence** (5×5000x on AMD Ryzen AI 9 HX 370): RenderHeader 161→158 allocs/op (-1.9%); RenderRow 255→248 allocs/op (-2.7%); RenderHighlightedRow 291→284 allocs/op (-2.4%); -5-7% B/op; ns/op within noise (lipgloss `Render()` dominates).

**Test status**: build/vet/format clean, 12/12 packages pass.

**PR creation failure**: `safe-outputs create_pull_request` reported success 3× but no PR opened (GET #247 → 404). Reported `incomplete`. Patch + bundle at `/tmp/gh-aw/aw-efficiency-tui-render-strings-builder.{patch,bundle}`. Intermittent — both 2026-06-17 (→ PR #203) and 2026-06-19 (→ PR #226) eventually merged after the original report_incomplete.

## 2026-06-19 — ContainerImageLayoutRevision hoist (PR #226 MERGED)

Additive to PR #213. BenchmarkManager_Status: 326,748 → 50,605 ns/op (**-85%**), 1,516,216 → 161,493 B/op (**-89%**), 167 → 84 allocs/op (**-50%**).

## 2026-06-18 — Manager.Status loop-invariant hoist (PR #213 MERGED)

`BenchmarkManager_Status` 197→166 allocs/op (-15.74%, p=0.008).

## 2026-06-17 — Container status SSH consolidation (PR #203 MERGED)

**SSH round trips per call: 3/2 → 1 (always).** `BenchmarkContainerLocalStatusImageAndRevision` = 1,429 ns/op, 902 B/op, 6 allocs/op.

## 2026-06-15 — TUI extractTrailingPercent ParseFloat (PR #191 MERGED)

`BenchmarkExtractTrailingPercent` 3806→452 ns/op (-88%), 36→6 allocs/op (-83%).

## Earlier (merged)

- PR #167 (EnrichWithGitHubStatus) MERGED 2026-06-12. 33→28 allocs/op (-15%).
- PR #155 (FindRunnerForLogs) MERGED 2026-06-12. 5906→790 ns/op (-86%).
- PR #146 (InstanceNames) MERGED 2026-06-11. 21→11 allocs/op.
- PR #136 (single du walk) MERGED 2026-06-09.
- PR #128 (Validate_Large) MERGED 2026-06-09.
- PR #123 (FilterRunners_ByName) MERGED 2026-06-09. 503→1 allocs/op (502×).
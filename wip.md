---
name: wip
description: Current optimization work in progress
metadata:
  type: project
---

# Work in Progress

- **TUI metrics `strconv.FormatFloat`** (this run, no PR): branch `efficiency/metrics-strconv-formatfloat-2026-06-25` (commit 09ca658). `metricsRow`/`LoadStr`/`FormatHostMetrics` swapped from `fmt.Sprintf("%.<prec>f ...", ...)` to `strconv.FormatFloat` + `strings.Builder`. New `appendHostCell` helper for column padding/separator. New tests `TestLoadStr`/`TestFormatHostMetrics`. Build/test/vet/race clean. `BenchmarkMetricsRow` 20→16 allocs/op (-20%), ~1968→~1500 ns/op (-24%), 472→456 B/op (-3.4%). `safe-outputs create_pull_request` reported success 3× but no PR opened (recurring intermittent failure — TUI strings.Builder PR #249 eventually merged 2026-06-24 despite same failure). Patch + bundle at `/tmp/gh-aw/aw-efficiency-metrics-strconv-formatfloat-2026-06-25.{patch,bundle}`.

## Resolved

- PR #249 (TUI strings.Builder) MERGED 2026-06-24T02:01:58Z. allocs: 161/255/291 → 158/248/284 (-2-3%); B/op: -5-7%; ns/op within noise (lipgloss `Render()` dominates).
- PR #203 (container status SSH consolidation) MERGED 2026-06-18T02:31:23Z.
- PR #213 (Manager.Status loop-invariant hoist) MERGED 2026-06-19T02:42:34Z. 197→166 allocs/op (-15.74%).
- PR #226 (ContainerImageLayoutRevision hoist) MERGED 2026-06-19T12:05:05Z. 326,748→50,605 ns/op (-85%), 167→84 allocs/op (-50%).
- PR #123 / #128 / #131 / #136 / #146 / #155 / #167 / #191 — all merged.
- Issue #124 open: benchstat comparison on PRs. 6 prereq benches in tree.
- Issue #125 (Monthly Activity 2026-06) — updated this run with metrics strconv PR.

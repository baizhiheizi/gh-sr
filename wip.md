---
name: wip
description: Current optimization work in progress
metadata:
  type: project
---

# Work in Progress

- **TUI render strings.Builder** (this run, no PR): branch `efficiency/tui-render-strings-builder` (commit e430457). `renderHeader`/`renderRow`/`renderHighlightedRow` in `internal/tui/table.go` swapped from `var line string + line +=` (O(N²) string concat) to `var b strings.Builder` + `b.WriteString(...)`. New `internal/tui/table_bench_test.go` pins per-call cost. Build/test/vet/race clean. RenderHeader 161→158 allocs/op (-1.9%); RenderRow 255→248 allocs/op (-2.7%); RenderHighlightedRow 291→284 allocs/op (-2.4%); -5-7% B/op; ns/op within noise (lipgloss `Render()` dominates). **`safe-outputs create_pull_request` reported success 3× but no PR opened on GitHub** (3rd consecutive run with this failure; eventually retries succeed). Patch + bundle preserved at `/tmp/gh-aw/aw-efficiency-tui-render-strings-builder.{patch,bundle}`. Reported `incomplete`.

## Resolved

- PR #203 (container status SSH consolidation) MERGED 2026-06-18T02:31:23Z.
- PR #213 (Manager.Status loop-invariant hoist) MERGED 2026-06-19T02:42:34Z. 197→166 allocs/op (-15.74%).
- PR #226 (ContainerImageLayoutRevision hoist) MERGED 2026-06-19T12:05:05Z. 326,748→50,605 ns/op (-85%), 167→84 allocs/op (-50%).
- PR #123 / #128 / #131 / #136 / #146 / #155 / #167 / #191 — all merged.
- Issue #124 open: benchstat comparison on PRs. 5 prereq benches in tree.
- Issue #125 open: Monthly Activity 2026-06 — updated this run.
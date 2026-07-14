---
name: wip
description: Work in progress for efficiency-improver
metadata:
  type: project
---

# Work in Progress

- **Draft PR pending (2026-07-14)**: branch `efficiency/tui-render-sprintf-and-closure` (commit ef19ba6). Title: `[efficiency-improver] perf(tui): hoist dashboard footer strings to constants — drop per-View() Sprintf`. FooterMain/idle: 11→10 allocs/op; FooterMain/loading: 12→10 allocs/op; ViewMain: 341→340 allocs/op. Patch at `/tmp/gh-aw/aw-efficiency-tui-render-sprintf-and-closure.patch`.

## Re-apply candidate

- **PR #357** (commit 46ed29d, branch `efficiency/benchstat-formatnumber-write-direct`): opened 2026-07-11 09:57 UTC, **closed 2026-07-11 23:44 UTC** (not merged). `scripts/benchstat` writeNumber: RenderMarkdown 47→2 allocs/op (-95.7%), FullPipeline 145→100 allocs/op (-31%). Disposition unclear. If close was stylistic, re-apply on fresh branch.

## Negative results documented 2026-07-14

- `formatContainerImageBuild` closure: Go escape analysis already elides it. 0 allocs/op — sentinel kept.

## Open issues

- Issue #324 (monthly activity): update_issue budget exhausted 2026-07-14 (1/1 used; truncated body call counted). **Next run must update #324 cleanly.**
- Issue #132: btrfs/reflink design — HIGH per-host disk + pull energy; still open since 2026-06-15.
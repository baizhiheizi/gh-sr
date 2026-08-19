---
name: wip
description: Work in progress for efficiency-improver
metadata:
  type: project
---

# Work in Progress

- **Draft PR via safeoutputs (2026-08-19)**: branch `efficiency/tui-footermain-consts` (commit 2747f3c). Title: `[efficiency-improver] perf(tui): hoist dashboard footer strings to constants — drop per-View() Sprintf`. FooterMain/idle: 5,803→0.21 ns, 11→0 allocs (-100%); FooterMain/loading: 6,545→0.21 ns, 12→0 allocs (-100%); ViewMain: 40,454→39,058 ns, 333→322 allocs (-3.3%). Patch at `/tmp/gh-aw/aw-efficiency-tui-footermain-consts.patch`. Bundle at `/tmp/gh-aw/aw-efficiency-tui-footermain-consts.bundle`. PR number not yet verified — MCP github guard unavailable after PR creation.

## Re-apply candidate

- **PR #357** (commit 46ed29d, branch `efficiency/benchstat-formatnumber-write-direct`): opened 2026-07-11 09:57 UTC, **closed 2026-07-11 23:44 UTC** (not merged). `scripts/benchstat` writeNumber: RenderMarkdown 47→2 allocs/op (-95.7%), FullPipeline 145→100 allocs/op (-31%). Disposition unclear. If close was stylistic, re-apply on fresh branch.

## Negative results documented 2026-07-14

- `formatContainerImageBuild` closure: Go escape analysis already elides it. 0 allocs/op — sentinel kept.

## Open issues

- Monthly activity issue: MCP github server unavailable 2026-08-19 — cannot list/read/update existing issue or create new one. PR was created via safeoutputs successfully, but the post-PR read for monthly update is blocked. **Next run must retry** and update/close the monthly issue cleanly.
- Issue #132: btrfs/reflink design — HIGH per-host disk + pull energy; still open since 2026-06-15.
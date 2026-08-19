---
name: run-history
description: Efficiency Improver run history for round-robin scheduling
metadata:
  type: project
---

- 2026-08-19 (32290291823): TUI dashboard `footerMain` Sprintf → package `var` strings (2 states). Branch `efficiency/tui-footermain-consts` (2747f3c). FooterMain/idle: 5,803→0.21 ns, 11→0 allocs (-100%); FooterMain/loading: 6,545→0.21 ns, 12→0 allocs (-100%); ViewMain: 333→322 allocs (-3.3%). PR via safeoutputs. MCP github guard unavailable post-PR for monthly issue update.
- 2026-07-14 (29319792354): Same optimization attempt with smaller win (const vs var; `helpStyle.Render` returns non-const string). Branch not on origin/main this run. Re-implemented as 2026-08-19 above.
- 2026-07-11 (29146206472): `scripts/benchstat` writeNumber (-95.7% allocs/op). **PR #357 closed without merge 2026-07-11 23:44 UTC**. Re-apply candidate still open.
- 2026-07-09 (29010944815): scripts/benchstat RenderMarkdown rewrite (-79.2% allocs/op) → PR #345.
- 2026-07-06 (28787663140): FormatBytesHuman inline unit suffix (-14.3%) → PR #323.
- 2026-07-03 (28652519952): HostMetrics.LoadStr stack-buffer REGRESSED (553 vs 385 ns/op; `strings.Builder.String()` is zero-copy). Reverted.
- 2026-06-26 (28231593128): TUI metrics `strconv.AppendFloat`/`[24]byte` — squash 2373126.
- 2026-06-19 (27822093719): ContainerImageLayoutRevision hoist (-85%) → PR #226.
- 2026-06-18 (27755003134): Manager.Status loop-invariant hoist → PR #213.
- 2026-06-17 (27685462756): container status SSH consolidation (3→1) → PR #203.
- 2026-06-15 (27535126807): extractTrailingPercent ParseFloat (-88%) → PR #191.
- 2026-06-11 (27339307382): FindRunnerForLogs (-86%) → PR #155.
- 2026-06-09 (27198579456): dirSizesPOSIX single du walk → PR #136.
- 2026-06-08 (27130562074): FilterRunners/FindRunner → PR #123.
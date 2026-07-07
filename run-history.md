---
name: run-history
description: Round-robin tracking of which tasks ran when
metadata:
  type: project
---

# Run History

- 2026-07-07 (28858688416): Verified PR #323 merge (squash 14d1edb, branch 9337d8c). Corrected "silent failure" framing — `safe-outputs create_pull_request success` IS reliable, the apparent 404 is a delayed squash-merge. Scanned for new opportunities — none of the win-class remaining in TUI hot paths. Updated Issue #324 (Monthly Activity 2026-07).
- 2026-07-06 (28787663140): FormatBytesHuman inline unit suffix — multi-sample **avg 443.5 → 379.9 ns/op (-14.3%)**; per-call ~79 → ~65 ns/op (-18%). Branch `efficiency/format-bytes-human-inlined` (9337d8c). 14/14 packages clean.
- 2026-07-03 (28652519952): Negative-result: LoadStr stack-buffer regressed 553 vs 385 ns/op — strings.Builder.String() is zero-copy. Reverted.
- 2026-06-26 (28231593128): TUI metrics strconv — BenchmarkMetricsRow 20→10 allocs/op (-50%); squash 2373126.
- 2026-06-19 (27822093719): ContainerImageLayoutRevision hoist (-85%) → PR #226.
- 2026-06-18 (27755003134): Manager.Status loop-invariant hoist → PR #213.
- 2026-06-17 (27685462756): container status SSH consolidation (3→1) → PR #203.
- 2026-06-15 (27535126807): extractTrailingPercent ParseFloat (-88%) → PR #191.
- 2026-06-11 (27339307382): FindRunnerForLogs (-86%) → PR #155.
- 2026-06-09 (27198579456): dirSizesPOSIX single du walk → PR #136.
- 2026-06-08 (27130562074): FilterRunners/FindRunner → PR #123; issue #124 created.
---
name: run-history
description: Round-robin tracking of which tasks ran when
metadata:
  type: project
---

# Run History

- 2026-07-03 (28652519952): FormatBytesHuman inline unit suffix — 8→7 allocs/op, 56→48 B/op (-14%), ~444→~367 ns/op avg (-17%). Per-call 2→1 alloc. Branch `efficiency/format-bytes-human-single-alloc` (commit 9b384c8). PR created. **Negative-result**: `HostMetrics.LoadStr` stack-buffer regressed to 553 ns/op (vs 385 baseline) — `strings.Builder.String()` is zero-copy. Reverted.
- 2026-06-26 (28231593128): TUI metrics strconv — `BenchmarkMetricsRow` 20→10 allocs/op (-50%). Branch `efficiency/metrics-strconv-formatfloat-2026-06-26` (commit 6f0b887). PR tool failed but work merged via `aw: upgrade & update` squash (commit 2373126).
- 2026-06-25 (28162208483): Same pattern, smaller delta (20→16). Superseded.
- 2026-06-23 (28019485668): TUI render strings.Builder; merged as PR #249.
- 2026-06-19 (27822093719): ContainerImageLayoutRevision hoist (-85%); PR #226 MERGED.
- 2026-06-18 (27755003134): Manager.Status loop-invariant hoist; PR #213 MERGED.
- 2026-06-17 (27685462756): container status SSH consolidation (3→1); PR #203 MERGED.
- 2026-06-15 (27535126807): TUI extractTrailingPercent ParseFloat (-88%); PR #191 MERGED.
- 2026-06-11 (27339307382): FindRunnerForLogs (-86%); PR #155 MERGED.
- 2026-06-10 (27268656760): InstanceNames; PR #146 MERGED.
- 2026-06-09 (27198579456): dirSizesPOSIX single du walk; PR #136 MERGED.
- 2026-06-08 (27130562074): FilterRunners/FindRunner (PR #123); issue #124 created.
- 2026-06-07 (27093653174): full sweep, no new high-value targets.
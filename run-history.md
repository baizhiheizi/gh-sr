---
name: run-history
description: Round-robin tracking of which tasks ran when
metadata:
  type: project
---

# Run History

- 2026-06-26 (run 28231593128): TUI metrics strconv.FormatFloat — `BenchmarkMetricsRow` 20→10 allocs/op (**-50%**), ~1800→~1380 ns/op (-23%), 472→424 B/op (-10%). Branch `efficiency/metrics-strconv-formatfloat-2026-06-26` (commit 6f0b887). `safe-outputs create_pull_request` returned success 2× but no PR opened (5th recurring failure). Reported `incomplete`. Monthly Activity #125 updated.
- 2026-06-25 (run 28162208483): Same pattern, smaller delta (20→16). Superseded by 2026-06-26.
- 2026-06-23 (run 28019485668): TUI render strings.Builder — allocs/op: -2-3%; B/op: -5-7%. Eventually merged as PR #249.
- 2026-06-19 (run 27822093719): ContainerImageLayoutRevision hoist (-85% time); PR #226 MERGED.
- 2026-06-18 (run 27755003134): Manager.Status loop-invariant hoist; PR #213 MERGED.
- 2026-06-17 (run 27685462756): container status SSH consolidation (3→1); PR #203 MERGED.
- 2026-06-15 (run 27535126807): TUI extractTrailingPercent ParseFloat (-88%); PR #191 MERGED.
- 2026-06-11 (run 27339307382): FindRunnerForLogs (-86% time); PR #155 MERGED.
- 2026-06-10 (run 27268656760): InstanceNames helper; PR #146 MERGED.
- 2026-06-09 (run 27198579456): dirSizesPOSIX single du walk; PR #136 MERGED.
- 2026-06-08 (run 27130562074): FilterRunners/FindRunner alloc hotspot (PR #123), benchmark-CI infra proposal #124.
- 2026-06-07 (run 27093653174): full sweep, no new high-value targets.

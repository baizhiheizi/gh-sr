---
name: run-history
description: Round-robin tracking of which tasks ran when
metadata:
  type: project
---

# Run History

- 2026-06-25 (run 28162208483): TUI metrics strconv.FormatFloat — `BenchmarkMetricsRow` 20→16 allocs/op (-20%), ~1968→~1500 ns/op (-24%), 472→456 B/op (-3.4%). `LoadStr`/`metricsRow`/`FormatHostMetrics` swapped from `fmt.Sprintf` to `strconv.FormatFloat` + `strings.Builder`. New `BenchmarkFormatHostMetrics` + tests `TestLoadStr`/`TestFormatHostMetrics`. Branch `efficiency/metrics-strconv-formatfloat-2026-06-25` (commit 09ca658). `safe-outputs create_pull_request` returned success 3× but no PR opened (recurring). Patch + bundle at `/tmp/gh-aw/aw-efficiency-metrics-strconv-formatfloat-2026-06-25.{patch,bundle}`. Monthly Activity #125 updated.
- 2026-06-23 (run 28019485668): TUI render strings.Builder — allocs/op: 161/255/291 → 158/248/284 (-2-3%); B/op: 4438/6731/7355 → 4229/6296/6874 (-5-7%). Eventually opened and merged as PR #249 on 2026-06-24T02:01:58Z.
- 2026-06-19 (run 27822093719): ContainerImageLayoutRevision hoist (-85% time, -89% bytes, -50% allocs); PR #226 MERGED 2026-06-19T12:05:05Z.
- 2026-06-18 (run 27755003134): Manager.Status loop-invariant hoist; PR #213 MERGED 2026-06-19T02:42:34Z.
- 2026-06-17 (run 27685462756): container status SSH consolidation (3→1 round trip); PR #203 MERGED 2026-06-18T02:31:23Z.
- 2026-06-15 (run 27535126807): TUI extractTrailingPercent ParseFloat (-88% time); PR #191 MERGED.
- 2026-06-11 (run 27339307382): FindRunnerForLogs (-86% time, -98% allocs); PR #155 MERGED.
- 2026-06-10 (run 27268656760): InstanceNames helper; PR #146 MERGED.
- 2026-06-09 (run 27198579456): dirSizesPOSIX single du walk; PR #136 MERGED.
- 2026-06-08 (run 27130562074): FilterRunners/FindRunner allocation hotspot (PR #123), benchmark-CI infra proposal #124.
- 2026-06-07 (run 27093653174): full sweep, no new high-value targets.

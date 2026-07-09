---
name: run-history
description: Round-robin tracking of which tasks ran when
metadata:
  type: project
---

# Run History

- 2026-07-09 (29010944815): scripts/benchstat RenderMarkdown rewrite — `fmt.Fprintf(&sb, ...)` reflection → direct builder writes + `strconv.AppendFloat` + `[24]byte` / `[32]byte` stack buffers. **RenderMarkdown: -51.5% time, -54.9% B/op, -79.2% allocs.** FullPipeline -25.7%/-55.2%. Branch `efficiency/benchstat-format-fprintf-replace` (34c4908) + draft PR. 16/16 packages OK.
- 2026-07-07 (28858688416): Verified PR #323 merge (squash 14d1edb); corrected "silent failure" framing.
- 2026-07-06 (28787663140): FormatBytesHuman inline unit suffix (-14.3%) → PR #323 → merged.
- 2026-07-03 (28652519952): HostMetrics.LoadStr stack-buffer REGRESSED (553 vs 385 ns/op; `strings.Builder.String()` is zero-copy). Reverted; documented.
- 2026-06-26 (28231593128): TUI metrics `strconv.AppendFloat`/`[24]byte` — squash 2373126.
- 2026-06-19 (27822093719): ContainerImageLayoutRevision hoist (-85%) → PR #226.
- 2026-06-18 (27755003134): Manager.Status loop-invariant hoist → PR #213.
- 2026-06-17 (27685462756): container status SSH consolidation (3→1) → PR #203.
- 2026-06-15 (27535126807): extractTrailingPercent ParseFloat (-88%) → PR #191.
- 2026-06-11 (27339307382): FindRunnerForLogs (-86%) → PR #155.
- 2026-06-09 (27198579456): dirSizesPOSIX single du walk → PR #136.
- 2026-06-08 (27130562074): FilterRunners/FindRunner → PR #123; issue #124 created.

---
name: run-history
description: Round-robin tracking of which tasks ran when
metadata:
  type: project
---

# Run History

- 2026-06-19 (run 27822093719): ContainerImageLayoutRevision hoist in Manager.Status (-85% wall-clock, -89% bytes, -50% allocs on BenchmarkManager_Status); new BenchmarkContainerImageLayoutRevision pin (30µs, 150KB, 12 allocs/op). PR creation tool returned success but no PR opened on GitHub (3 retries, 2 distinct approaches — same persistent failure as 2026-06-17; reported incomplete). Branch `efficiency/hoist-container-image-layout-revision` (commit 15a59dc) preserved locally. Additive to PR #213 (hoist mode/repo/labels/host/isContainer — merged 2026-06-19T02:42:34Z).
- 2026-06-17 (run 27685462756): container status SSH consolidation (2-3 → 1 round trip per container per Status tick); new TestContainerLocalStatusImageAndRevision_one_ssh_round_trip + BenchmarkContainerLocalStatusImageAndRevision (1,429 ns/op, 902 B/op, 6 allocs/op). PR creation tool returned success but no PR opened on GitHub — reported incomplete. Negative results: enrichFromScopeRunners O(N·M)→O(N+M) regressed at 20×10 scale; EqualFold short-circuit unmeasurable. Branch `efficiency/container-status-one-shot` (commit ef6beab) preserved locally.
- 2026-06-15 (run 27535126807): TUI extractTrailingPercent fmt.Sscanf → strconv.ParseFloat (-88% time, -80% bytes, -83% allocs); added BenchmarkExtractTrailingPercent + BenchmarkMetricsRow (first TUI benches); commented on #132 btrfs/reflink design.
- 2026-06-11 (run 27339307382): FindRunnerForLogs dead-code map + per-iter InstanceNames() allocations + missing early-exit; Match 5906→790 ns/op (-86%), 297→5 allocs/op (-98%); Ambiguous 3987→150 ns/op (-96%), 66→5 allocs/op (-92%); PR #155 MERGED 2026-06-12T02:51:36Z.
- 2026-06-10 (run 27268656760): `InstanceNames` helper fmt.Sprintf → strconv.Itoa; 21→11 allocs/op, 1239→~430 ns/op; PR #146 MERGED 2026-06-11T04:06:49Z.
- 2026-06-09 (run 27198579456): dirSizesPOSIX 4 du calls → 1 du --max-depth=1 walk (PR #136, merged); PR #123, #128 confirmed merged.
- 2026-06-08 (run 27143755302): fixed c.Validate() per-iter InstanceNames() (711→411 allocs/op) — propagated as PR #128
- 2026-06-08 (run 27130562074): FilterRunners/FindRunner allocation hotspot (PR #123), benchmark-CI infra proposal #124
- 2026-06-07 (run 27093653174): full sweep, no new high-value targets

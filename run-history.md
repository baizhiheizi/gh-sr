---
name: run-history
description: Round-robin tracking of which tasks ran when
metadata:
  node_type: memory
  type: project
  originSessionId: 04e58c81-0a51-4f35-bd75-e67ad9ba414d
---

# Run History

- 2026-06-11 (run 27339307382): FindRunnerForLogs dead-code map + per-iter InstanceNames() allocations + missing early-exit; Match 5906→790 ns/op (-86%), 297→5 allocs/op (-98%); Ambiguous 3987→150 ns/op (-96%), 66→5 allocs/op (-92%); PR patch created.
- 2026-06-10 (run 27268656760): `InstanceNames` helper fmt.Sprintf → strconv.Itoa; 21→11 allocs/op, 1239→~430 ns/op; PR #146 MERGED 2026-06-11T04:06:49Z.
- 2026-06-09 (run 27198579456): dirSizesPOSIX 4 du calls → 1 du --max-depth=1 walk (PR #136, merged); PR #123, #128 confirmed merged.
- 2026-06-08 (run 27143755302): fixed c.Validate() per-iter InstanceNames() (711→411 allocs/op) — propagated as PR #128
- 2026-06-08 (run 27130562074): FilterRunners/FindRunner allocation hotspot (PR #123), benchmark-CI infra proposal #124
- 2026-06-07 (run 27093653174): full sweep, no new high-value targets

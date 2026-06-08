---
name: backlog
description: Identified energy-efficiency opportunities for gh-sr
metadata: 
  node_type: memory
  type: project
  originSessionId: 04e58c81-0a51-4f35-bd75-e67ad9ba414d
---

# Efficiency Backlog

| Priority | Focus Area | Opportunity | Estimated Impact |
|----------|------------|-------------|------------------|
| MEDIUM | Data | `BenchmarkLoad_Large` allocates 3.3k allocs/op — YAML loading hotspot, ~473µs/op | TBD |
| MEDIUM | Code-Level | `BenchmarkValidate_Large` allocates 711 allocs/op — same `InstanceNames` per-iter pattern as filter; fix in `c.Validate()` | Likely 80%+ alloc reduction |
| LOW | Code-Level | TUI re-render hotspots (bubbletea v2) — audit `View()` implementations | TBD |
| TBD | Network | Audit repeated `gh run` calls for batching | TBD |
| TBD | Network | TUI status refresh polling vs event-driven | TBD |

## Completed

- ✅ `FilterRunners_ByName` 503→1 allocs/op (PR `[efficiency-improver] perf(config): inline instance-name lookup`) — branch `efficiency/inline-instance-name-lookup`, commit 4d8a04f
- ✅ `FindRunner` instance-name scan: 0 allocs/op (new benchmarks added)

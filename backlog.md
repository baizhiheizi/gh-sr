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
| LOW | Data | `BenchmarkLoad_Large` ~3.3k allocs/op — YAML loading hotspot, ~473µs/op | TBD (yaml.v3 internals) |
| LOW | Code-Level | TUI re-render hotspots (bubbletea v2) — audit `View()` implementations | TBD |
| TBD | Network | Audit repeated `gh run` calls for batching | TBD |
| TBD | Network | TUI status refresh polling vs event-driven | TBD |
| LOW | Code | `Remove` parallelization (per-host) | Rare op, config-mutation concerns |
| LOW | Code | `ValidateContainerPrereqs` parallelization | ~150ms savings; complex early-exit |

## Completed

- ✅ `FilterRunners_ByName` 503→1 allocs/op — PR #123
- ✅ `FindRunner` instance-name scan: 0 allocs/op — PR #123
- ✅ `Validate_Large` 711→411 allocs/op — PR #128
- ✅ `dirSizesPOSIX` 4 `du -sk` → 1 `du --max-depth=1` walk (this run) — 4 SSH round trips per instance → 1

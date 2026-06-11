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
| LOW | Data | `BenchmarkLoad_Large` ~3.1k allocs/op — YAML loading hotspot, ~230µs/op | TBD (yaml.v3 internals) |
| ~~LOW~~ | Code-Level | ~~TUI `viewMain` allocates `[]string{9}` cells slice per row × render; `computeWidths` duplicates the same work — could fold into one pass~~ | **Skipped — escape analysis stack-allocates the `cells` slice (1 alloc/op); actual hotspot is lipgloss Render. Not worth chasing.** |
| TBD | Network | Audit repeated `gh run` calls for batching | TBD |
| TBD | Network | TUI status refresh polling vs event-driven | TBD |
| LOW | Code | `Remove` parallelization (per-host) | Rare op, config-mutation concerns |
| LOW | Code | `ValidateContainerPrereqs` parallelization | ~150ms savings; complex early-exit |
| MEDIUM | Infra | Issue #124 — benchstat comparison comment on PRs (bench already runs on PRs; only the comparison step is missing) | Unblocks future detection |

## Completed

- ✅ `FilterRunners_ByName` 503→1 allocs/op — PR #123
- ✅ `FindRunner` instance-name scan: 0 allocs/op — PR #123
- ✅ `Validate_Large` 711→411 allocs/op — PR #128
- ✅ `dirSizesPOSIX` 4 `du -sk` → 1 `du --max-depth=1` walk — PR #136 (4 SSH round trips → 1)
- ✅ `InstanceNames` helper 21→11 allocs/op, 1239→~430 ns/op — this run's PR; helper called 23+ times across the codebase

---
name: backlog
description: Identified energy-efficiency opportunities for gh-sr
metadata:
  type: project
---

# Efficiency Backlog

| Priority | Focus Area | Opportunity | Estimated Impact |
|----------|------------|-------------|------------------|
| TBD | Network | TUI status refresh polling (5s) — `EnrichWithGitHubStatus` fires K parallel `ListRunnersScoped` HTTP calls per refresh. Could cache or use ETag. | TBD |
| TBD | Code | `diskschedule.parseAtTime` uses `fmt.Sscanf("%d", &hour)` — cold-path. | LOW |
| TBD | Network | Audit repeated `gh run` calls for batching | TBD |
| LOW | Data | `BenchmarkLoad_Large` ~3.1k allocs/op — YAML loading hotspot | yaml.v3 internals |
| LOW | Code | `Remove` parallelization (per-host) | Rare op |
| LOW | Code | `ValidateContainerPrereqs` parallelization | ~150ms |
| MEDIUM | Infra | Issue #124 — benchstat comparison on PRs (5 prereq benches in tree now) | Unblocks future detection |
| MEDIUM | Storage | Issue #132 — btrfs loop + reflink seed for shared inner Docker cache | HIGH per-host disk + pull energy |

## Completed (recent)

- ✅ `FilterRunners_ByName` 503→1 allocs/op — PR #123
- ✅ `dirSizesPOSIX` 4 SSH round trips → 1 — PR #136
- ✅ `InstanceNames` helper 21→11 allocs/op — PR #146
- ✅ `FindRunnerForLogs_Match` 5906→790 ns/op (-86%) — PR #155
- ✅ `EnrichFromScopeRunners_Small` 33→28 allocs/op (-15%) — PR #167
- ✅ `extractTrailingPercent` 3806→452 ns/op (-88%) — PR #191
- ✅ `containerLocalStatusImageAndRevision` 2-3 → 1 SSH round trip — PR #203
- ✅ `Manager.Status` loop-invariant hoist — PR #213
- ✅ `ContainerImageLayoutRevision` hoist (-85% time, -89% bytes) — PR #226
- ✅ TUI render `var line string + line +=` → `strings.Builder` — branch `efficiency/tui-render-strings-builder` (commit e430457), preserved locally, awaiting manual push (PR creation tool silent failure). RenderHeader 161→158, RenderRow 255→248, RenderHighlightedRow 291→284 allocs/op; -5-7% B/op; ns/op within noise.
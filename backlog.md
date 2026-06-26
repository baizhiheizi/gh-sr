---
name: backlog
description: Identified energy-efficiency opportunities for gh-sr
metadata:
  type: project
---

# Efficiency Backlog

| Priority | Focus Area | Opportunity | Estimated Impact |
|----------|------------|-------------|------------------|
| ⏳ | Code | TUI `metricsRow` + `FormatHostMetrics` + `host.LoadStr` `fmt.Sprintf` → `strconv.FormatFloat` + `strings.Builder`. Branch `efficiency/metrics-strconv-formatfloat-2026-06-26` (commit 6f0b887). `BenchmarkMetricsRow` 20→10 allocs/op (**-50%**), ~1800→~1380 ns/op (-23%), 472→424 B/op (-10%). `safe-outputs create_pull_request` reported success 2× but no PR opened (recurring). Patch + bundle at `/tmp/gh-aw/aw-efficiency-metrics-strconv-formatfloat-2026-06-26.{patch,bundle}`. | MEDIUM (-50% allocs, -23% ns) |
| TBD | Network | TUI refresh polling (5s) — `EnrichWithGitHubStatus` fires K parallel `ListRunnersScoped` HTTP. ETag cache opportunity. | TBD |
| TBD | Network | Audit repeated `gh run` calls for batching | TBD |
| LOW | Code | `diskschedule.parseAtTime` uses `fmt.Sscanf` — cold-path | LOW |
| LOW | Data | `BenchmarkLoad_Large` ~3.1k allocs/op — YAML loading hotspot | yaml.v3 internals |
| LOW | Code | `Remove` parallelization (per-host) | Rare op |
| LOW | Code | `ValidateContainerPrereqs` parallelization | ~150ms |
| MEDIUM | Infra | Issue #124 — benchstat on PRs (5 prereq benches in tree) | Unblocks future detection |
| MEDIUM | Storage | Issue #132 — btrfs loop + reflink seed | HIGH per-host disk + pull energy |

## Completed (recent)

- ✅ `FilterRunners_ByName` 503→1 allocs/op — PR #123
- ✅ `dirSizesPOSIX` 4 SSH round trips → 1 — PR #136
- ✅ `InstanceNames` 21→11 allocs/op — PR #146
- ✅ `FindRunnerForLogs_Match` 5906→790 ns/op (-86%) — PR #155
- ✅ `EnrichFromScopeRunners_Small` 33→28 allocs/op (-15%) — PR #167
- ✅ `extractTrailingPercent` 3806→452 ns/op (-88%) — PR #191
- ✅ `containerLocalStatusImageAndRevision` 2-3 → 1 SSH round trip — PR #203
- ✅ `Manager.Status` loop-invariant hoist — PR #213
- ✅ `ContainerImageLayoutRevision` hoist (-85% time, -89% bytes) — PR #226
- ✅ TUI render `var line string + line +=` → `strings.Builder` — PR #249
- ✅ TUI metrics `strconv.FormatFloat` + `strings.Builder` (commit 6f0b887, no PR — silent tool failure; preserved locally)

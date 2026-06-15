---
name: backlog
description: Identified energy-efficiency opportunities for gh-sr
metadata:
  type: project
---

# Efficiency Backlog

| Priority | Focus Area | Opportunity | Estimated Impact |
|----------|------------|-------------|------------------|
| LOW | Data | `BenchmarkLoad_Large` ~3.1k allocs/op — YAML loading hotspot, ~230µs/op | TBD (yaml.v3 internals) |
| TBD | Network | TUI status refresh polling (5s) — `EnrichWithGitHubStatus` runs once per tick; for K scopes this fires K parallel `ListRunnersScoped` HTTP calls per refresh. Could cache for `~refresh_window` or use conditional requests (ETag) | TBD |
| TBD | Code | `Manager.Status` allocates via `rc.InstanceNames()` + `strings.Join(EffectiveLabelsForInstance(...))` per call — runs in the inner per-instance loop on every status refresh | TBD |
| TBD | Code | TUI `renderHeader` / `renderRow` / `renderHighlightedRow` use `line += cellStyle.Width(...).Render(styled)` in a loop (O(n²) string concatenation in Go). `Render()` dominates, so likely small per-render win — worth measuring on a 10-host panel. | TBD |
| TBD | Code | `diskschedule.parseAtTime` uses `fmt.Sscanf("%d", &hour)` — same ParseInt vs Sscanf pattern, but cold-path (one-shot install). LOW priority. | TBD |
| TBD | Network | Audit repeated `gh run` calls for batching | TBD |
| LOW | Code | `Remove` parallelization (per-host) | Rare op, config-mutation concerns |
| LOW | Code | `ValidateContainerPrereqs` parallelization | ~150ms savings; complex early-exit |
| MEDIUM | Infra | Issue #124 — benchstat comparison comment on PRs (bench already runs on PRs; only the comparison step is missing). 2 TUI benches added this run partially fills the prerequisite. | Unblocks future detection |
| MEDIUM | Storage | Issue #132 — btrfs loop + reflink seed for shared inner Docker cache. Reduces per-runner image pull energy by N× via dedup. Could be the biggest single-host energy win in the project. Commented on it this run. | HIGH per-host disk + pull energy |

## Completed

- ✅ `FilterRunners_ByName` 503→1 allocs/op — PR #123
- ✅ `FindRunner` instance-name scan: 0 allocs/op — PR #123
- ✅ `Validate_Large` 711→411 allocs/op — PR #128
- ✅ `dirSizesPOSIX` 4 `du -sk` → 1 `du --max-depth=1` walk — PR #136 (4 SSH round trips → 1)
- ✅ `InstanceNames` helper 21→11 allocs/op, 1239→~430 ns/op — PR #146
- ✅ `FindRunnerForLogs_Match` 5906→790 ns/op (-86%), 297→5 allocs/op (-98%) — PR #155
- ✅ `FindRunnerForLogs_Ambiguous` 3987→150 ns/op (-96%), 66→5 allocs/op (-92%) — PR #155
- ✅ `EnrichFromScopeRunners_Small` 33→28 allocs/op (-15%), ~50% wall-clock reduction at median — PR #167
- ✅ `EnrichFromScopeRunners` 440→420 allocs/op (-4.5%), -3.2 KB/op — PR #167
- ✅ `extractTrailingPercent` (TUI) 3806→452 ns/op (-88%), 818→160 B/op (-80%), 36→6 allocs/op (-83%) — this run's PR. fmt.Sscanf → strconv.ParseFloat. First TUI-side bench.

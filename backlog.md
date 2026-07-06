---
name: backlog
description: Identified energy-efficiency opportunities for gh-sr
metadata:
  type: project
---

# Efficiency Backlog

| Priority | Focus Area | Opportunity | Estimated Impact |
|----------|------------|-------------|------------------|
| TBD | Network | TUI refresh polling (5s) — `EnrichWithGitHubStatus` fires K parallel `ListRunnersScoped` HTTP. ETag cache opportunity. | TBD |
| TBD | Network | Audit repeated `gh run` calls for batching | TBD |
| LOW | Code | `diskschedule.parseAtTime` uses `fmt.Sscanf` — cold-path | LOW |
| LOW | Data | `BenchmarkLoad_Large` ~3.1k allocs/op — YAML loading hotspot | yaml.v3 internals |
| LOW | Code | `Remove` parallelization (per-host) | Rare op |
| LOW | Code | `ValidateContainerPrereqs` parallelization | ~150ms |
| MEDIUM | Infra | Issue #124 — benchstat on PRs (5 prereq benches in tree) | Unblocks future detection |
| MEDIUM | Storage | Issue #132 — btrfs loop + reflink seed | HIGH per-host disk + pull energy |

## Completed (recent)

- ✅ `FormatBytesHuman` inline unit suffix — avg **443.5 → 379.9 ns/op (-14.3%)** multi-sample; per-call ~79 → ~65 ns/op (-18%). Branch `efficiency/format-bytes-human-inlined` (commit 9337d8c). PR tool reported success but no PR #323 on GitHub (404) — recurring silent-failure pattern; branch + patch + bundle preserved for manual push.
- ✅ TUI metrics `strconv.AppendFloat` + `[24]byte` stack buffer (merged via squash 2373126, in tree as of 2026-07-03).
- ✅ `FilterRunners_ByName` 503→1 allocs/op — PR #123.
- ✅ `dirSizesPOSIX` 4 SSH round trips → 1 — PR #136.
- ✅ `InstanceNames` 21→11 allocs/op — PR #146.
- ✅ `FindRunnerForLogs_Match` 5906→790 ns/op (-86%) — PR #155.
- ✅ `EnrichFromScopeRunners_Small` 33→28 allocs/op (-15%) — PR #167.
- ✅ `extractTrailingPercent` 3806→452 ns/op (-88%) — PR #191.
- ✅ `containerLocalStatusImageAndRevision` 2-3 → 1 SSH round trip — PR #203.
- ✅ `Manager.Status` loop-invariant hoist — PR #213.
- ✅ `ContainerImageLayoutRevision` hoist (-85% time, -89% bytes) — PR #226.
- ✅ TUI render `var line string + line +=` → `strings.Builder` — PR #249.
- ✅ Doctor 6 agentic container probes → 1 docker exec — PR #322 (perf-improver agent).

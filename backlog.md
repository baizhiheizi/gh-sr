---
name: backlog
description: Efficiency Improver backlog of opportunities
metadata:
  type: project
---

# Efficiency Backlog

| Priority | Focus | Opportunity | Impact |
|----------|-------|-------------|--------|
| ⏳ | Code | TUI `dashboardModel.footerMain` per-View() `fmt.Sprintf` → package consts. FooterMain/idle: 11→10 allocs/op, -9.9% bytes; FooterMain/loading: 12→10 allocs/op, -11.2% bytes. Draft PR pending (branch `efficiency/tui-render-sprintf-and-closure`, commit ef19ba6). | LOW/MED |
| TBD | Net | TUI refresh polling (5s) — `EnrichWithGitHubStatus` fires K parallel `ListRunnersScoped`. ETag cache. | TBD |
| TBD | Net | Audit repeated `gh run` calls for batching | TBD |
| LOW | Code | `diskschedule.parseAtTime` uses `fmt.Sscanf` — cold-path | LOW |
| LOW | Data | `BenchmarkLoad_Large` ~3.1k allocs/op — YAML loading hotspot | yaml.v3 internals |
| LOW | Code | `Remove` parallelization (per-host) | Rare |
| LOW | Code | `ValidateContainerPrereqs` parallelization | ~150ms |
| LOW | Code | `agentic.FormatRemediation` 4× `fmt.Fprintf(&sb, …)` reflection — cold-path | LOW |
| ❌ | Code | `scripts/benchstat` writeNumber direct-to-builder — **47 → 2 allocs/op (-95.7%)** RenderMarkdown; FullPipeline 145→100 allocs/op (-31%). **PR #357 closed without merge 2026-07-11 23:44 UTC** — re-apply on fresh branch if close was stylistic. | HIGH |
| MED | Infra | Sentinels added 2026-07-14: `BenchmarkFooterMain`, `BenchmarkViewMain`, `BenchmarkFormatContainerImageBuild`. `BenchmarkWriteNumber` added 2026-07-11 (PR #357 closed). | Regression detection |
| MED | Storage | Issue #132 — btrfs loop + reflink seed for shared inner Docker cache | HIGH per-host disk + pull energy |

## Recently closed (merged)

- ✅ `scripts/benchstat` RenderMarkdown rewrite (PR #345, -79.2% allocs).
- ✅ FormatBytesHuman inline unit suffix (PR #323, -14.3%).
- ✅ doctor fanout (PR #322, 6→1 SSH); PR #361 Start/Stop combined probe (perf-improver, merged 2026-07-13).
- Earlier wins: PR #226 ContainerImageLayoutRevision; #213 Manager.Status hoist; #203 container SSH (3→1); #191 extractTrailingPercent; #167 EnrichWithGitHubStatus; #155 FindRunnerForLogs; #146 InstanceNames; #136 dirSizesPOSIX; #128 Validate_Large; #123 FilterRunners; #249 TUI render.

## Recently closed (NOT merged)

- ❌ PR #357 `scripts/benchstat` writeNumber — closed 2026-07-11 23:44 UTC without merge.

## Negative results documented 2026-07-14

- `formatContainerImageBuild` closure (`short := func(s string) string`): Go escape analysis already elides it. 0 allocs/op — sentinel kept.
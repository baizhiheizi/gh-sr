# Efficiency Backlog

| Priority | Focus Area | Opportunity | Estimated Impact |
|----------|------------|-------------|------------------|
| TBD | Network | TUI refresh polling (5s) — `EnrichWithGitHubStatus` fires K parallel `ListRunnersScoped` HTTP. ETag cache opportunity. | TBD |
| TBD | Network | Audit repeated `gh run` calls for batching | TBD |
| LOW | Code | `diskschedule.parseAtTime` uses `fmt.Sscanf` — cold-path | LOW |
| LOW | Data | `BenchmarkLoad_Large` ~3.1k allocs/op — YAML loading hotspot | yaml.v3 internals |
| LOW | Code | `Remove` parallelization (per-host) | Rare op |
| LOW | Code | `ValidateContainerPrereqs` parallelization | ~150ms |
| LOW | Code | `LoadStr` stack-buffer — Repo Assist re-tried (PR #355) reports -26% ns / -33% B on median. Marginal win. | LOW |
| LOW | Code | `agentic.FormatRemediation`/`FormatAllRemediations` 4× `fmt.Fprintf(&sb, ...)` reflection calls — cold-path | LOW |
| MEDIUM | Infra | `BenchmarkWriteNumber` per-cell sentinel — picked up by `bench` and `bench-compare` going forward | Enables regression detection |
| MEDIUM | Storage | Issue #132 — btrfs loop + reflink seed for shared inner Docker cache | HIGH per-host disk + pull energy |

## Recently closed (merged)

- ✅ `scripts/benchstat` writeNumber direct-to-builder — 47 → 2 allocs/op on RenderMarkdown (-95.7%); PR pending (branch `efficiency/benchstat-formatnumber-write-direct`, commit 46ed29d).
- ✅ `scripts/benchstat` RenderMarkdown rewrite (PR #345, -79.2% allocs).
- ✅ FormatBytesHuman inline unit suffix (PR #323, -14.3%).
- Earlier wins (full list in run-history.md + completed.md): PR #322 doctor fan-out; #226 ContainerImageLayoutRevision; #213 Manager.Status hoist; #203 container status SSH (3→1); #191 extractTrailingPercent (-88%); #167 EnrichWithGitHubStatus; #155 FindRunnerForLogs; #146 InstanceNames; #136 dirSizesPOSIX; #128 Validate_Large; #123 FilterRunners; #249 TUI render.

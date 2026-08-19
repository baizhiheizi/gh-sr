---
name: completed
description: PRs and outcomes from efficiency-improver runs
metadata:
  type: project
---

# Completed Work

## 2026-08-19 — TUI dashboard footerMain Sprintf → package vars — **DRAFT PR via safeoutputs (pending number)**

Branch `efficiency/tui-footermain-consts` (2747f3c). Per-View() `fmt.Sprintf` → 2 package `var` strings.

| Bench | Before | After | Δ |
|---|---|---|---|
| `FooterMain/idle` | 5,803 ns · 1,145 B · 11 allocs | 0.21 ns · 0 B · 0 allocs | −100% all |
| `FooterMain/loading` | 6,545 ns · 1,289 B · 12 allocs | 0.21 ns · 0 B · 0 allocs | −100% all |
| `ViewMain/one_status` | 40,454 ns · 11,189 B · 333 allocs | 39,058 ns · 10,003 B · 322 allocs | −3.3% allocs |

Added `TestFooterMain_idleAndLoading` (ANSI-stripped pin). Sentinels `BenchmarkFooterMain`, `BenchmarkViewMain`. 17/17 packages OK; race suite OK. MCP github unavailable post-PR — number unverified.

## 2026-07-14 — Same optimization, weaker win (const vs var) — **BRANCH LOST**

Branch `efficiency/tui-render-sprintf-and-closure` (ef19ba6). FooterMain/idle: 11→10 allocs; FooterMain/loading: 12→10 allocs. Negative result: `formatContainerImageBuild` closure already elided. Re-implemented 2026-08-19 with `var` (lipgloss returns non-const string).

## 2026-07-11 — scripts/benchstat writeNumber — **PR #357 CLOSED WITHOUT MERGE**

PR #357 (46ed29d). RenderMarkdown 47→2 allocs/op (-95.7%). Closed 2026-07-11 23:44 UTC. Re-apply candidate.

## Merged PRs (backlog highlights)

- #345 RenderMarkdown rewrite (-79.2%); #323 FormatBytesHuman (-14.3%); #322 doctor fanout (6→1 SSH); #361 Start/Stop combined; #226 ContainerImageLayoutRevision hoist (-85%); #213 Manager.Status hoist; #203 container SSH 3→1; #191 extractTrailingPercent (-88%); #167 EnrichWithGitHubStatus; #155 FindRunnerForLogs (-86%); #146 InstanceNames; #136 dirSizesPOSIX; #128 Validate_Large; #123 FilterRunners; #249 TUI render; TUI metrics squash 2373126.
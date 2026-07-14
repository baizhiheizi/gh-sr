---
name: completed
description: PRs and outcomes from efficiency-improver runs
metadata:
  type: project
---

# Completed Work

## 2026-07-14 — TUI dashboard footerMain Sprintf → package consts — **DRAFT PR pending**

Branch `efficiency/tui-render-sprintf-and-closure` (ef19ba6). Replaced per-View() `fmt.Sprintf` with two package `const` strings (`footerMainIdle`, `footerMainLoading`).

| Bench | Before | After | Δ |
|---|---|---|---|
| `FooterMain/idle` | 7,888 ns · 1,145 B · 11 allocs | 7,263 ns · 1,032 B · 10 allocs | −7.9% / −9.9% / −9.1% |
| `FooterMain/loading` | 7,873 ns · 1,289 B · 12 allocs | 8,119 ns · 1,144 B · 10 allocs | ~ / −11.2% / −16.7% |
| `ViewMain` (1 status) | 52,446 ns · 11,487 B · 341 allocs | 53,681 ns · 11,379 B · 340 allocs | ~ / −0.9% / −0.3% |

Added `TestFooterMain_idleAndLoading` + `TestFooterMain_constantsMatch`. Sentinels: `BenchmarkFooterMain`, `BenchmarkViewMain`, `BenchmarkFormatContainerImageBuild`.

**Negative result**: `formatContainerImageBuild` closure already elided by Go escape analysis (0 allocs/op).

16 existing + 3 new tests pass; 17/17 packages OK; race suite OK.

## 2026-07-11 — scripts/benchstat writeNumber direct-to-builder — **PR #357 CLOSED WITHOUT MERGE**

Branch `efficiency/benchstat-formatnumber-write-direct` (46ed29d). `FormatNumber(f) string` → `writeNumber(sb, f)`. RenderMarkdown 47→2 allocs/op (-95.7%); FullPipeline 145→100 allocs/op (-31%); new `BenchmarkWriteNumber` sentinel.

**Outcome**: PR #357 created 2026-07-11 09:57 UTC, **closed without merge 2026-07-11 23:44 UTC**. Patch at `/tmp/gh-aw/aw-efficiency-benchstat-formatnumber-write-direct.patch`.

## 2026-07-09 — scripts/benchstat RenderMarkdown (-79.2% allocs/op) — **MERGED PR #345**

PR #345 (squash-merge). RenderMarkdown 12,877 → 6,248 ns/op, 6,205 → 2,800 B/op, 226 → 47 allocs/op.

## 2026-07-07 — FormatBytesHuman inline unit suffix (-14.3%) — **MERGED PR #323**

PR #323 (squash-merge 14d1edb). BenchmarkFormatBytesHuman 443.5 → 379.9 ns/op.

## Earlier merged (full list in run-history.md)

PR #361 · #345 · #322 · #249 · #226 · #213 · #203 · #191 · #167 · #155 · #146 · #136 · #128 · #123 · TUI metrics (squash 2373126).
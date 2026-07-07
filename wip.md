---
name: wip
description: Current optimization work in progress
metadata:
  type: project
---

# Work in Progress

- **Monthly Activity issue #324 update** (this run, 2026-07-07): Rewrote body to correct the prior "silent failure" framing for PR #323 — GitHub MCP `GET /pulls/323` returned 404 in the same agent loop, but the squash-merge landed anyway on 2026-07-07 as commit `14d1edb` (branch commit `9337d8c`). **The "silent failure" pattern is in fact a delayed squash-merge** — `safe-outputs create_pull_request` reporting `success` IS reliable; what fails is the *immediate verify step* via GitHub MCP.

## Resolved

- **FormatBytesHuman inline unit suffix** MERGED 2026-07-07 as **PR #323** via squash (branch 9337d8c → tree 14d1edb). `BenchmarkFormatBytesHuman` (5×50000x, AMD Ryzen AI 9 HX 370): avg **443.5 → 379.9 ns/op (-14.3%)**; per-call ~79 → ~65 ns/op (**-18%**). Allocs/op unchanged. 14/14 packages clean.
- TUI metrics `strconv.AppendFloat` + `[24]byte` stack buffer — merged via squash 2373126.
- PR #249 / #213 / #226 / #123 / #128 / #131 / #136 / #146 / #155 / #167 / #191 / #203 — all merged.
- Issue #124 open: benchstat on PRs (5 prereq benches in tree; repo-assist phantom-PR ready at `repo-assist/eng-bench-compare-2026-07-01`).
- Issue #132 open: `gh sr storage` btrfs/reflink design — HIGH per-host disk + pull energy.
---
name: wip
description: Current optimization work in progress
metadata:
  type: project
---

# Work in Progress

- PR open (this run): `[efficiency-improver] perf(tui): use strconv.ParseFloat in extractTrailingPercent` (branch `efficiency/tui-parsefloat-percentage-parse`, commit 8ae6038). Patch `/tmp/gh-aw/aw-efficiency-tui-parsefloat-percentage-parse.patch`. `BenchmarkExtractTrailingPercent` 3806→452 ns/op (-88%), 36→6 allocs/op (-83%). Awaiting PR URL from this run's create_pull_request.
- Issue #124 open: benchstat comparison on PRs (CI `bench` job already gates on `pull_request: [main]`).
- Issue #125 open: Monthly Activity 2026-06 — updated this run.

## Resolved

- PR #123 (inline instance-name lookup) — MERGED 2026-06-09T03:48:43Z.
- PR #128 (perf-improver alias of Validate fix) — MERGED 2026-06-09T03:49:34Z.
- PR #131 (an-lee, disk commands) — MERGED 2026-06-09T04:52:18Z. Introduced the disk.go code audited.
- PR #136 (single du walk) — MERGED 2026-06-09 (commit 46b6278).
- PR #146 (InstanceNames Sprintf → strconv.Itoa) — MERGED 2026-06-11T04:06:49Z.
- PR #155 (FindRunnerForLogs single-pointer + early-exit) — MERGED 2026-06-12T02:51:36Z.
- PR #167 (EnrichWithGitHubStatus inline rcByInstance) — MERGED 2026-06-12T22:59:59Z.

---
name: run-history
description: Test-improver run history (recent only — see wip.md for current)
metadata:
  type: project
---

Keep the last 4 runs in detail; older runs are summarised in one line each.

## 2026-06-15 — Run 27523706025

- Branch: `test-assist/collect-host-metrics`; commit 6dce7da
- Patch: `/tmp/gh-aw/aw-test-assist-collect-host-metrics.patch` (~18 KB)
- Coverage: `internal/ops` 33.4% → 37.6% (+4.2 pp); `CollectHostMetrics` 0% → 100%
- New file: `internal/ops/metrics_collect_test.go` (537 lines, 15 tests, sequential)

## 2026-06-14 — Run 27465943133

- Branch: `test-assist/resolve-host-info`; commit 1831f3e
- Patch: `/tmp/gh-aw/aw-test-assist-resolve-host-info.patch` (~24 KB)
- Coverage: `internal/ops` 24.2% → 33.4% (+9.2 pp); `ResolveHostInfo` 0% → 100%

## 2026-06-12 — Run 27415213477

- Branch: `test-assist/run-per-host-parallel`; commit 4db58a6
- Coverage: `internal/ops` 19.6% → 24.2% (+4.6 pp); `runPerHostParallel` 0% → 100%
- Refactor: `connectHostFn` factory + 9 callsites + per-call capture + `connectHostMu`
- **PR #168 merged 2026-06-12T23:00:16Z**

## 2026-06-11 — Run 27346830049

- `WriteRemoteBytes` 0% → 100%; `internal/hostshell` 23.1% → 100.0% (+76.9 pp)
- **PR #156 merged 2026-06-12T02:51:50Z**

## Earlier (one-line archive)

- #149 (2026-06-11) `escapePS` in `internal/diskschedule` 0% → 100%
- #108 / #107 ValidateContainer* helpers in `internal/agentic` (44.8% → 60.5%); closed as duplicate
- #68, #58, #54, #51, #40, #31, #11 — earlier merges
- Monthly Activity issues: #4 (April, closed), #69 (May, closed), #109 (June, active)

[[wip]] [[backlog]]

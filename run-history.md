---
name: run-history
description: Test-improver run history (recent only — see wip.md for current)
metadata:
  type: project
---

Keep the last 4 runs in detail; older runs are summarised in one line each.

## 2026-06-17 — Run 27675407379

- Branch: `test-assist/down-orchestrator`; commit `c3b0f62`
- Patch: `/tmp/gh-aw/aw-test-assist-down-orchestrator.patch` (~15 KB)
- Coverage: `internal/ops` 41.1% → 41.9% (+0.8 pp); `Down` 0% → 83.3%
- New file: `internal/ops/down_test.go` (422 lines, 10 tests, all `t.Parallel()`)

## 2026-06-15 — Run 27523706025

- Branch: `test-assist/collect-host-metrics`; commit `6dce7da`
- Patch: `/tmp/gh-aw/aw-test-assist-collect-host-metrics.patch` (~18 KB)
- Coverage: `internal/ops` 33.4% → 37.6% (+4.2 pp); `CollectHostMetrics` 0% → 100%
- New file: `internal/ops/metrics_collect_test.go` (537 lines, 15 tests, sequential)
- **PR #189 merged 2026-06-15T06:04:18Z**

## 2026-06-14 — Run 27465943133

- Branch: `test-assist/resolve-host-info`; commit `1831f3e`
- Patch: `/tmp/gh-aw/aw-test-assist-resolve-host-info.patch` (~24 KB)
- Coverage: `internal/ops` 24.2% → 33.4% (+9.2 pp); `ResolveHostInfo` 0% → 100%
- **PR #178 merged 2026-06-14T12:01:38Z**

## 2026-06-12 — Run 27415213477

- Branch: `test-assist/run-per-host-parallel`; commit `4db58a6`
- Coverage: `internal/ops` 19.6% → 24.2% (+4.6 pp); `runPerHostParallel` 0% → 100%
- Refactor: `connectHostFn` factory + 9 callsites + per-call capture + `connectHostMu`
- **PR #168 merged 2026-06-12T23:00:16Z**

## Earlier (one-line archive)

- 2026-06-11: hostshell WriteRemoteBytes 0%→100%; **PR #156 merged 2026-06-12**
- 2026-06-11: diskschedule escapePS 0%→100%; **PR #149 merged**
- 2026-06-10: six pure helpers in ops/disk.go 0%→100%; ops 9.6%→19.6% (+10.0 pp); **PR #147 merged 2026-06-11**
- 2026-06-08: agentic ValidateAWFHygiene/Inner 0%→100%; agentic 60.5%→83.9%
- 2026-06-06: agentic Validate* helpers 0%→100%; agentic 44.8%→60.5%
- Monthly Activity issues: #4 (April, closed), #69 (May, closed), #109 (June, active)

[[wip]] [[backlog]]

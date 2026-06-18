---
name: run-history
description: Test-improver run history (recent only — see wip.md for current)
metadata:
  type: project
---

Keep the last 4 runs in detail; older runs are summarised in one line each.

## 2026-06-18 — Run 27743420099

- Branch: `test-assist/restart-orchestrator`; commit `600dcda`
- Patch: `/tmp/gh-aw/aw-test-assist-restart-orchestrator.patch` (~25 KB)
- Coverage: `internal/ops` 41.9% → 42.8% (+0.9 pp); `Restart` 0% → 85.7%
- New file: `internal/ops/restart_test.go` (673 lines, 12 contract tests)

## 2026-06-17 — Run 27675407379

- Branch: `test-assist/down-orchestrator`; commit `c3b0f62`
- Patch: `/tmp/gh-aw/aw-test-assist-down-orchestrator.patch` (~15 KB)
- Coverage: `internal/ops` 41.1% → 41.9% (+0.8 pp); `Down` 0% → 83.3%
- New file: `internal/ops/down_test.go` (422 lines, 10 tests)

## 2026-06-15 — Run 27523706025

- Branch: `test-assist/collect-host-metrics`; commit `6dce7da`
- Coverage: `internal/ops` 33.4% → 37.6% (+4.2 pp); `CollectHostMetrics` 0% → 100%
- New file: `internal/ops/metrics_collect_test.go` (537 lines, 15 tests)
- **PR #189 merged 2026-06-15T06:04:18Z**

## 2026-06-14 — Run 27465943133

- Branch: `test-assist/resolve-host-info`; commit `1831f3e`
- Coverage: `internal/ops` 24.2% → 33.4% (+9.2 pp); `ResolveHostInfo` 0% → 100%
- **PR #178 merged 2026-06-14T12:01:38Z**

## Earlier (one-line archive)

- 2026-06-12: runPerHostParallel 0%→100%; ops 19.6%→24.2% (+4.6 pp); **PR #168 merged 2026-06-12T23:00:16Z**.
- 2026-06-11: hostshell WriteRemoteBytes 0%→100%; **PR #156 merged 2026-06-12**.
- 2026-06-10: six pure helpers in ops/disk.go 0%→100%; ops 9.6%→19.6% (+10.0 pp); **PR #147 merged 2026-06-11**.
- 2026-06-08: ValidateAWFHygiene/Inner 0%→100%; agentic 60.5%→83.9% (+23.4 pp).
- 2026-06-06: agentic Validate* helpers 0%→100%; agentic 44.8%→60.5% (+15.7 pp).
- Monthly Activity issues: #4 (April, closed), #69 (May, closed), #109 (June, active).

[[wip]] [[backlog]]

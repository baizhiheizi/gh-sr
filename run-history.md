---
name: run-history
description: Test-improver run history (recent only — see wip.md for current)
metadata:
  type: project
---

Keep the last 4 runs in detail; older runs are summarised in one line each.

## 2026-06-19 — Run 27814296911

- Branch: `test-assist/up-orchestrator`; commit `c2719e5`
- Patch: `/tmp/gh-aw/aw-test-assist-up-orchestrator.patch` (~25 KB, 680 lines)
- Bundle: `/tmp/gh-aw/aw-test-assist-up-orchestrator.bundle` (~6 KB)
- Coverage: `internal/ops` 42.8% → 44.0% (+1.2 pp); `Up` 0% → 77.8%
- New file: `internal/ops/up_test.go` (631 lines, 12 tests)
- PR: NOT created (safeoutputs bridge failed; preserve patch)

## 2026-06-18 — Run 27743420099

- Branch: `test-assist/restart-orchestrator`; commit `600dcda`
- Patch: `/tmp/gh-aw/aw-test-assist-restart-orchestrator.patch` (~25 KB)
- Coverage: `internal/ops` 41.9% → 42.8% (+0.9 pp); `Restart` 0% → 85.7%
- New file: `internal/ops/restart_test.go` (673 lines, 12 tests)
- PR: NOT created via safeoutputs, but **squashed into main via ff8d9cd (2026-06-19)** by maintainer manual merge

## 2026-06-17 — Run 27675407379

- Branch: `test-assist/down-orchestrator`; commit `c3b0f62`
- Patch: `/tmp/gh-aw/aw-test-assist-down-orchestrator.patch` (~15 KB)
- Coverage: `internal/ops` 41.1% → 41.9% (+0.8 pp); `Down` 0% → 83.3%
- New file: `internal/ops/down_test.go` (422 lines, 10 tests)
- PR: #200 created via safeoutputs (earlier PR system); **squashed into main via ff8d9cd (2026-06-19)**

## 2026-06-15 — Run 27523706025

- Branch: `test-assist/collect-host-metrics`; commit `6dce7da`
- Coverage: `internal/ops` 33.4% → 37.6% (+4.2 pp); `CollectHostMetrics` 0% → 100%
- New file: `internal/ops/metrics_collect_test.go` (537 lines, 15 tests)
- **PR #189 merged 2026-06-15T06:04:18Z**

## Earlier (one-line archive)

- 2026-06-14: ResolveHostInfo 0%→100%; ops 24.2%→33.4% (+9.2 pp); **PR #178 merged 2026-06-14T12:01:38Z**.
- 2026-06-12: runPerHostParallel 0%→100%; ops 19.6%→24.2% (+4.6 pp); **PR #168 merged**.
- 2026-06-11: hostshell WriteRemoteBytes 0%→100%; **PR #156 merged 2026-06-12**.
- 2026-06-10: six pure helpers in ops/disk.go 0%→100%; ops 9.6%→19.6% (+10.0 pp); **PR #147 merged 2026-06-11**.
- 2026-06-08: ValidateAWFHygiene/Inner 0%→100%; agentic 60.5%→83.9% (+23.4 pp).
- 2026-06-06: agentic Validate* helpers 0%→100%; agentic 44.8%→60.5% (+15.7 pp).
- Monthly Activity issues: #4 (April, closed), #69 (May, closed), #109 (June, active).

[[wip]] [[backlog]]
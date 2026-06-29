---
name: run-history
description: Test-improver run history (recent only — see wip.md for current)
metadata:
  type: project
---

Keep the last 4 runs in detail; older runs are summarised in one line each.

## 2026-06-29 — Run 28355199319
- Branch: `test-assist/orchestrator-connect-errors`; commit `a3c3b89`
- Coverage: `internal/ops` 90.4% → 90.9% (+0.5 pp); `Down` 83.3% → 100% (closed); `Restart` 85.7% → 100% (closed); `RebuildImage` 69.2% → 76.9% (+7.7 pp)
- New file: `internal/ops/orchestrator_connect_errors_test.go` (127 lines, 3 tests)
- PR: phantom — bridge failure recurring.

## 2026-06-27 — Run 28280956385 — disk-orchestrators, ops 70.2%→90.7% (+20.5 pp). **PR #280 merged 2026-06-27**.

## 2026-06-26 — Run 28221781038 — setup-orchestrator, ops 68.2%→70.2% (+2.0 pp). **PR #270 merged 2026-06-26**.

## 2026-06-24 — Run 28079915247 — collect-status-orchestrator, ops 62.1%→68.2% (+6.1 pp). **PR #256 merged 2026-06-24**.

## Earlier (one-line archive)
- 2026-06-21: Remove 0%→94.1%; **PR #242 merged**.
- 2026-06-20: Logs 0%→92.9%; **PR #233 merged 2026-06-21**.
- 2026-06-19: Up 0%→77.8%; **PR #224 merged**. Restart 0%→85.7%; **PR #216 merged**.
- 2026-06-17: Down 0%→83.3%; **PR #200 merged**.
- 2026-06-15: CollectHostMetrics 0%→100%; **PR #189 merged**.
- 2026-06-14: ResolveHostInfo 0%→100%; **PR #178 merged**.
- 2026-06-12: runPerHostParallel 0%→100%; **PR #168 merged**.
- 2026-06-11: WriteRemoteBytes 0%→100%; **PR #156 merged**.
- 2026-06-10: six pure helpers in ops/disk.go 0%→100%; **PR #147 merged**.
- Monthly Activity issues: #4 (April, closed), #69 (May, closed), #109 (June, active).

[[wip]] [[backlog]]

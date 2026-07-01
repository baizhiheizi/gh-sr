---
name: run-history
description: Test-improver run history (recent only — see wip.md for current)
metadata:
  type: project
---

Keep the last 4 runs in detail; older runs are summarised in one line each.

## 2026-07-01 — Run 28499581092
- Branch: `test-assist/update-orchestrator`; commit `bddc280`
- Coverage: `internal/ops` 92.6% → 93.6% (+1.0 pp); `Update` 53.8% → 100% (closed; largest single-function gap in `internal/ops`)
- New file: `internal/ops/orchestrator_update_test.go` (385 lines, 5 tests + 2 helpers)
- PR: phantom — bridge failure (3rd consecutive). All safeoutputs (create_pull_request, update_issue, create_issue, add_comment) reported success but did not land.

## 2026-06-30 — Run 28425497071 — ServiceCleanup 67.5%→92.5%; ops 90.4%→92.1% (+1.7 pp). **PR phantom** (2nd consecutive).

## 2026-06-29 — Run 28355199319 — Down→100, Restart→100, RebuildImage→76.9%; ops 90.4%→90.9% (+0.5 pp). **PR #293 landed** (despite phantom report).

## 2026-06-27 — Run 28280956385 — disk-orchestrators, ops 70.2%→90.7% (+20.5 pp). **PR #280 merged 2026-06-27**.

## Earlier (one-line archive)
- 2026-06-26: Setup 56.2→100. **PR #270 merged 2026-06-26**.
- 2026-06-24: CollectStatus 0→90.5. **PR #256 merged 2026-06-24**.
- 2026-06-21: Remove 0%→94.1%; **PR #242 merged**.
- 2026-06-20: Logs 0%→92.9; **PR #233 merged 2026-06-21**.
- 2026-06-19: Up/Restart covered. **PR #224/#216 merged**.
- 2026-06-17: Down 0→83.3. **PR #200 merged**.
- 2026-06-15: CollectHostMetrics 0%→100%; **PR #189 merged**.
- 2026-06-14: ResolveHostInfo 0%→100%; **PR #178 merged**.
- 2026-06-12: runPerHostParallel 0%→100%; **PR #168 merged**.
- 2026-06-11: WriteRemoteBytes 0%→100%; **PR #156 merged 2026-06-12**.
- 2026-06-10: six pure helpers in ops/disk.go 0%→100%; **PR #147 merged 2026-06-11**.
- Monthly Activity issues: #4 (April, closed), #69 (May, closed), #109 (June, attempted close 2026-07-01, phantom).

[[wip]] [[backlog]]

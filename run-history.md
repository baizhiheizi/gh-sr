---
name: run-history
description: Test-improver run history (recent only — see wip.md for current)
metadata:
  type: project
---

## 2026-08-07 — Run 31209652644
- 10 tests in `internal/runner/runner_remove_status_logs_test.go` for `Manager.Remove` / `Manager.Status` / `Manager.Logs` + `Manager.Out` fallback.
- Branch `test-assist/manager-remove-status-logs-v2`, commit `20c5ca7`. Draft PR intent recorded.
- Coverage: `internal/runner` 73.7→76.4 (+2.7 pp); Manager.Remove/Status/Logs 0→100 each.

## 2026-07-18 — Run 29631854277
- 8 tests in `runner_remove_status_logs_test.go` (same scope as 2026-08-07). Branch `test-assist/manager-remove-status-logs-orchestrators`, commit `2af02ce` — but the commit never made it into `main`; the 2026-08-07 run re-implemented the work.

## 2026-07-17 — Run 29557450180
- 6 tests (4 subtests) covering `dirSizesWindows` / `dirSizes` Windows dispatch / `parseFourInt64s`. PR #388 merged 2026-07-22.

## 2026-07-16 — Run 29473958530
- 9 tests for `setupNative` / `startNativeOnce` / `handleStaleRegistration` / `EnsureSetup`. PR #381 merged (commit `c6e3fe0`).

## Earlier
- 2026-07-14: Manager Start/Stop probe branches; PR #362 merged.
- 2026-07-09: runner dispatch/disk; PR #343 merged.
- 2026-07-08: diskschedule 14.2→88.2; PR #336 merged.
- 2026-07-04: autostart Install; PR #321 merged.
- 2026-07-03: autostart Start/Stop/Status/Uninstall; PR #316 merged.
- 2026-07-02: runner pure helpers; PR #311 merged.
- 2026-07-01: ops Update; PR #304 merged.
- Monthly: #4 Apr, #69 May, #109 June, #306/#305 July closed-or-superseded; August issue opened by 2026-08-07 run.

[[wip]] [[backlog]]

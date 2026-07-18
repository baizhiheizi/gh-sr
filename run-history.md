---
name: run-history
description: Test-improver run history (recent only — see wip.md for current)
metadata:
  type: project
---

## 2026-07-18 — Run 29631854277
- 8 tests in `internal/runner/runner_remove_status_logs_test.go` for `Manager.Remove` / `Manager.Status` / `Manager.Logs`.
- Branch `test-assist/manager-remove-status-logs-orchestrators`, commit `2af02ce`. PR intent persisted; verify remote in next run.
- Coverage: `internal/runner` 69.9→72.7 (+2.8 pp); Manager.Remove 0→85.7; Manager.Status 0→100; Manager.Logs 0→100.

## 2026-07-17 — Run 29557450180
- 6 tests (4 subtests) covering `dirSizesWindows` / `dirSizes` Windows dispatch / `parseFourInt64s`. PR #388 open as draft (agentic-threat-detected).

## 2026-07-16 — Run #29473958530
- 9 tests for `setupNative` / `startNativeOnce` / `handleStaleRegistration` / `EnsureSetup`. PR #381 merged (commit `c6e3fe0`).

## Earlier
- 2026-07-14: Manager Start/Stop probe branches; PR #362 merged.
- 2026-07-09: runner dispatch/disk; PR #343 merged.
- 2026-07-08: diskschedule 14.2→88.2; PR #336 merged.
- 2026-07-04: autostart Install; PR #321 merged.
- 2026-07-03: autostart Start/Stop/Status/Uninstall; PR #316 merged.
- 2026-07-02: runner pure helpers; PR #311 merged.
- 2026-07-01: ops Update; PR #304 merged.
- Monthly: #4 Apr, #69 May, #109 June closed; #306 July active; #305 July duplicate still open.

[[wip]] [[backlog]]
---
name: wip
description: Current test-improver work in progress
metadata:
  type: project
---

## Run #28642393250 (2026-07-03)

- Branch: `test-assist/autostart-start-stop-status`; commit `c7aabdb`
- PR: **NOT CREATED** — safeoutputs phantom (**5th consecutive**). Patch `/tmp/gh-aw/aw-test-assist-autostart-start-stop-status.patch`; bundle `/tmp/gh-aw/aw-test-assist-autostart-start-stop-status.bundle`. Maintainer: `git am` + push.
- Coverage: `Start` **0.0% → 91.7%**; `Stop` **0.0% → 95.0%**; `Status` **0.0% → 96.6%**; `Uninstall` **31.6% → 84.2%**; package `internal/autostart` **62.7% → 84.9% (+22.2 pp — largest package jump in this run's history)**.
- New file: `internal/autostart/autostart_orchestrator_test.go` (475 lines, 4 test groups).
- Status: build ✅, vet ✅, race ✅, full suite ✅, gofmt ✅.

**Pattern (reusable):** Kind-dispatch table-driven tests with the existing `newMockHost` + `testutil.MockExecutor`. One table per orchestrator covers 4 dispatch arms (systemd-user, systemd-system, launchd, windows-task). Windows arms verify the `powershell.exe -EncodedCommand` wrapper fired (since `host.Host.RunShell` base64-encodes the script).

**Windows RunShell gotcha (new):** `host.Host.RunShell` base64-encodes the script via `host.wrapCommand` on Windows hosts. The `testutil.MockExecutor` sees the *wrapper*, not the literal PowerShell — substring-matching the action script content doesn't work. Solution: return `"yes"` for every call on Windows (Detect resolves to `KindWindowsTask`), and assert the wrapper substring `powershell.exe` + `-EncodedCommand` on the last call.

**Safeoutputs bridge (CRITICAL):** 5th consecutive phantom. `create_pull_request` reported success but no PR landed remotely. Patch + bundle preserved for each missing PR. Maintainer pattern: organic merges happen for ~half the runs despite phantom reports.

**Next:** `internal/autostart.installLaunchd` + `installWindowsTask` (both 0%) — reachable via the same pattern. Then `internal/diskschedule` (needs `exec.Command` injection refactor) or back to `internal/tui` colorize helpers.

[[backlog]] [[run-history]]
---
name: wip
description: Current test-improver work in progress
metadata:
  type: project
---

## Run #28697055488 (2026-07-04)

- Branch: `test-assist/autostart-install`; commit `f0a38bf`
- PR: **PHANTOM (6th consecutive)** — safeoutputs returned patch+bundle metadata only, no PR number. Patch `/tmp/gh-aw/aw-test-assist-autostart-install.patch`; bundle `/tmp/gh-aw/aw-test-assist-autostart-install.bundle`. Maintainer: `git am` + push.
- Coverage:
  - `Install` **64.3% → 100.0%**
  - `installSystemdUser` **80.0% → 100.0%**
  - `installSystemdSystem` **77.8% → 100.0%**
  - `installLaunchd` **0.0% → 100.0%**
  - `installWindowsTask` **0.0% → 100.0%**
  - `remoteHome` **44.4% → 88.9%**
  - package `internal/autostart` **84.9% → 94.7%** (+9.8 pp)
- New code: `internal/autostart/autostart_orchestrator_test.go` (TestInstall group added, 4-arm dispatch table + 8 subtests for orchestrator + error branches).
- Status: build ✅, vet ✅, race ✅, full suite ✅, gofmt ✅.

**Pattern (reusable):** WriteRemoteBytes streams base64 over `h.Run` (POSIX) / `h.RunShell` (Windows), NOT via the SSH `Upload` API. Tests assert on the SSH command substring (`base64 -d`/powershell) — the real install path matches the mock path.

**Safeoutputs bridge (CRITICAL — 6th consecutive phantom):** `create_pull_request` reported success but no PR landed remotely. Patch + bundle preserved for each missing PR. Maintainer pattern: organic merges happen for ~half the runs despite phantom reports.

**Next:** `internal/diskschedule` (14.2%, needs `exec.Command` injection refactor) — or revisit `internal/runner` orchestrators (setupNative, startNative — 0%, high complexity, lower marginal ROI).

[[backlog]] [[run-history]]

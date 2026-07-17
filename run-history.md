---
name: run-history
description: Test-improver run history (recent only — see wip.md for current)
metadata:
  type: project
---

## 2026-07-17 — Run 29557450180
- 6 tests (4 subtests) in `internal/runner/disk_test.go` covering `dirSizesWindows`, `dirSizes` Windows dispatch, `parseFourInt64s` Windows-emit paths.
- Branch `test-assist/dirsizes-windows-branch`, commit `6d44d5b`; patch + bundle persisted at `/tmp/gh-aw/aw-test-assist-dirsizes-windows-branch.{patch,bundle}`.
- New helper `decodeEncodedPowerShellCommand` (UTF-16LE + base64) mirrors `host.encodePowerShellScript`.
- Coverage: `internal/runner` 69.9→71.2; `dirSizesWindows` 0→100; `dirSizes` 75→100; `parseFourInt64s` 62.5→85.

## 2026-07-16 — Run #29473958530
- 9 tests in `internal/runner/native_setup_test.go` covering `setupNative`/`startNativeOnce`/`handleStaleRegistration`/`EnsureSetup`.
- Branch `test-assist/native-setup-and-stale-recovery`, commit `dbd34dc`. PR #381 merged (commit `c6e3fe0`).
- Coverage: `internal/runner` 64.1→69.7; setupNative 0→73.8; startNativeOnce 0→56.7; handleStaleRegistration 0→75; EnsureSetup 0→100.

## Earlier
- 2026-07-14: Manager Start/Stop probe branches; PR #362 merged.
- 2026-07-09: runner dispatch/disk; PR #343 merged.
- 2026-07-08: diskschedule 14.2→88.2; PR #336 merged.
- 2026-07-04: autostart Install; PR #321 merged.
- 2026-07-03: autostart Start/Stop/Status/Uninstall; PR #316 merged.
- 2026-07-02: runner pure helpers; PR #311 merged.
- 2026-07-01: ops Update; PR #304 merged.
- Older: #296, #293, #280, #270, #256, #242, #233, #224/#216, #200, #189, #178, #168, #156, #147.
- Monthly: #4 Apr, #69 May, #109 June closed; #306 July active; #305 July duplicate still open.

[[wip]] [[backlog]]
---
name: run-history
description: Test-improver run history (recent only — see wip.md for current)
metadata:
  type: project
---

## 2026-07-14 — Run 29308107521
- Reviewed recent PR #361's combined native lifecycle probe and added two sequence-aware Manager tests for install→re-probe→autostart Start and autostart Stop dispatch.
- Branch `test-assist/native-start-stop-probe-branches`, commit `bf59188`; draft PR intent `#aw_probe`; patch `/tmp/gh-aw/aw-test-assist-native-start-stop-probe-branches.patch`, bundle `/tmp/gh-aw/aw-test-assist-native-start-stop-probe-branches.bundle`.
- Coverage: `internal/runner` 62.8→63.8; Start 30.8→53.8; Stop 35.7→42.9; `startAutostartWithDarwinFallback` 0→60.
- Verified focused tests, full race suite, build, vet, gofmt, and diff check. Monthly issue #306 updated.

## Recent
- 2026-07-09 runner dispatch/disk branches 57.6→59.6; PR #343 merged.
- 2026-07-08 diskschedule 14.2→88.2; PR #336 merged.
- 2026-07-04 autostart Install helpers 84.9→94.7; PR #321 merged.
- 2026-07-03 autostart Start/Stop/Status/Uninstall +22.2 pp; PR #316 merged.
- 2026-07-02 runner pure helpers 55.6→56.7; PR #311 merged.
- 2026-07-01 ops Update 53.8→100; PR #304 merged.
- Earlier merged: #296, #293, #280, #270, #256, #242, #233, #224/#216, #200, #189, #178, #168, #156, #147.
- Monthly: #4 Apr, #69 May, #109 June closed; #306 July active; #305 July duplicate still open.

[[wip]] [[backlog]]

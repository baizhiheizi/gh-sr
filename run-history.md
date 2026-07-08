---
name: run-history
description: Test-improver run history (recent only — see wip.md for current)
metadata:
  type: project
---

## 2026-07-08 — Run 28920234933
- `internal/diskschedule` 14.2→88.2; `Detect` 0→90.9, `Install` 0→100, `Uninstall` 0→100, `Status` 56.2→94.1.
- Branch `test-assist/diskschedule-command-seams`, commit `54b8881`; PR intent `#aw_dskpr`; patch `/tmp/gh-aw/aw-test-assist-diskschedule-command-seams.patch`.
- Verified gofmt, diff check, build, vet, package coverage/race, full race.

## Recent
- 2026-07-04 autostart Install helpers; package 84.9→94.7. Present on main by 2026-07-08.
- 2026-07-03 autostart Start/Stop/Status/Uninstall; package 62.7→84.9. Present on main by 2026-07-08.
- 2026-07-02 runner pure helpers; package 55.6→56.7. Present on main by 2026-07-08.
- 2026-07-01 ops Update 53.8→100; ops +1.0 pp. Present on main by 2026-07-08.
- 2026-06-30 ServiceCleanup 67.5→92.5. Present on main by 2026-07-08.
- Earlier merged: #293, #280, #270, #256, #242, #233, #224/#216, #200, #189, #178, #168, #156, #147.
- Monthly: #4 Apr, #69 May, #109 June closed; #305/#306 July duplicate pair (#306 active).

[[wip]] [[backlog]]

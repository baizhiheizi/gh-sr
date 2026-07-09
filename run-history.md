---
name: run-history
description: Test-improver run history (recent only — see wip.md for current)
metadata:
  type: project
---

## 2026-07-09 — Run 28998803505
- `internal/runner` 57.6% → 59.6%; `NeedsSetup` 0% → 100%; `RebuildImage` 0% → 100%; `clearWorkTemp` 50% → 100%; `removeDirTree` 25% → 50%; `PruneInstance` 54.1% → 64.9%.
- Branch `test-assist/runner-orchestrator-dispatch`, commit `4487f30`; PR intent `#337`; **phantom PR** (safeoutputs reported success but the PR did not land in the live repo). Patch `/tmp/gh-aw/aw-test-assist-runner-orchestrator-dispatch.patch`, bundle `/tmp/gh-aw/aw-test-assist-runner-orchestrator-dispatch.bundle`.
- Verified gofmt, diff check, build, vet, package coverage/race, full race.

## Recent
- 2026-07-08 diskschedule 14.2→88.2; PR #336 merged 2026-07-08.
- 2026-07-04 autostart Install helpers; package 84.9→94.7. Present on main by 2026-07-08.
- 2026-07-03 autostart Start/Stop/Status/Uninstall; package +22.2 pp. Present on main by 2026-07-08.
- 2026-07-02 runner pure helpers; package 55.6→56.7. Present on main by 2026-07-08.
- 2026-07-01 ops Update 53.8→100. Present on main by 2026-07-08.
- 2026-06-30 ServiceCleanup 67.5→92.5. Present on main by 2026-07-08.
- Earlier merged: #293, #280, #270, #256, #242, #233, #224/#216, #200, #189, #178, #168, #156, #147.
- Monthly: #4 Apr, #69 May, #109 June closed; #305/#306 July duplicate pair (#306 active).

[[wip]] [[backlog]]

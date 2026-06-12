---
name: run-history
description: Test-improver run history (recent)
metadata:
  type: project
---

## 2026-06-12 — Run 27415213477

- Branch: `test-assist/run-per-host-parallel`; commit 4db58a6
- Patch: `/tmp/gh-aw/aw-test-assist-run-per-host-parallel.patch`
- Coverage: `internal/ops` 19.6% → 24.2% (+4.6 pp); `runPerHostParallel` 0% → 100%
- New file: `internal/ops/run_per_host_parallel_test.go` (442 lines, 8 tests, race-clean)
- Refactor: `var connectHostFn = ConnectHost` + 9 callsites; per-call local capture; `connectHostMu` mutex

## 2026-06-11 — Run 27346830049

- Branch: `test-assist/hostshell-write-remote-bytes`; commit ab1f73f
- Coverage: `internal/hostshell` 23.1% → 100.0% (+76.9 pp); `WriteRemoteBytes` 0% → 100%
- **PR #156 merged 2026-06-12T02:51:50Z**

## 2026-06-10 — Run 27275755111

- Branch: `test-assist/disk-printer-helpers`; commit 5508526
- Coverage: `internal/ops` 9.6% → 19.6% (+10.0 pp); six helpers 0% → 100%
- **PR #147 merged 2026-06-11T04:08:14Z**

## 2026-06-08 — Run 27138355567

- Branch: `test-assist/awf-hygiene-validators`
- Coverage: `internal/agentic` 60.5% → 83.9% (+23.4 pp)

## 2026-06-06 — Run 27061368592

- Branch: `test-assist/agentic-validation-helpers`
- Coverage: `internal/agentic` 44.8% → 60.5% (+15.7 pp)
- Closed as duplicate #107/#108

## Earlier (merged; archive)

- #149 (2026-06-11) — `escapePS` in `internal/diskschedule` 0% → 100%
- #156 (2026-06-12) — hostshell WriteRemoteBytes
- #147 (2026-06-11) — disk-printer helpers
- #108 / #107 — ValidateContainer* helpers in `internal/agentic`
- #68 (2026-05-31), #58 (2026-04-29), #54, #51 (2026-04-24), #40 (2026-04-19), #31 (2026-04-17), #11

Monthly Activity issues: #4 (April, closed), #69 (May, closed), #109 (June, active).

[[wip]] [[backlog]]

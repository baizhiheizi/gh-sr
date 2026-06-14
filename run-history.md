---
name: run-history
description: Test-improver run history (recent only — see wip.md for current)
metadata:
  type: project
---

Keep the last 4 runs in detail; older runs are summarised in one line each.

## 2026-06-14 — Run 27465943133

- Branch: `test-assist/resolve-host-info`; commit 1831f3e
- Patch: `/tmp/gh-aw/aw-test-assist-resolve-host-info.patch` (~24 KB)
- Coverage: `internal/ops` 24.2% → 33.4% (+9.2 pp); `ResolveHostInfo` 0% → 100%

## 2026-06-12 — Run 27415213477

- Branch: `test-assist/run-per-host-parallel`; commit 4db58a6
- Coverage: `internal/ops` 19.6% → 24.2% (+4.6 pp); `runPerHostParallel` 0% → 100%
- Refactor: `connectHostFn` factory + 9 callsites + per-call capture + `connectHostMu` mutex
- **PR #168 merged 2026-06-12T23:00:16Z**

## 2026-06-11 — Run 27346830049 — `WriteRemoteBytes` 0% → 100%; `internal/hostshell` 23.1% → 100.0% (+76.9 pp); **PR #156 merged 2026-06-12T02:51:50Z**

## 2026-06-10 — Run 27275755111 — six disk-printer helpers 0% → 100%; `internal/ops` 9.6% → 19.6% (+10.0 pp); **PR #147 merged 2026-06-11T04:08:14Z**

## Earlier (one-line archive)

- #149 (2026-06-11) — `escapePS` in `internal/diskschedule` 0% → 100%
- #108 / #107 — ValidateContainer* helpers in `internal/agentic` (44.8% → 60.5%); closed as duplicate
- #68 (2026-05-31), #58 (2026-04-29), #54, #51, #40, #31, #11 — earlier merges
- Monthly Activity issues: #4 (April, closed), #69 (May, closed), #109 (June, active)

[[wip]] [[backlog]]

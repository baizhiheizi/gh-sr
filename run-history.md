---
name: run-history
description: Test-improver run history (recent only — see wip.md for current)
metadata:
  type: project
---

Keep the last 4 runs in detail; older runs are summarised in one line each.

## 2026-06-21 — Run 27897086810

- Branch: `test-assist/remove-orchestrator`; commit `3521f2b`
- Patch: `/tmp/gh-aw/aw-test-assist-remove-orchestrator.patch` (~22 KB, 633 lines)
- Coverage: `internal/ops` 52.3% → 54.6% (+2.3 pp); `Remove` 0% → 94.1%; `removeHost` 0% → 100%
- New file: `internal/ops/remove_test.go` (593 lines, 12 tests)
- PR: NOT created via safeoutputs (bridge failed; same recurring issue as prior 3 runs)

## 2026-06-20 — Run 27863360393

- Branch: `test-assist/logs-orchestrator`; commit `d0bca1c`
- Patch: `/tmp/gh-aw/aw-test-assist-logs-orchestrator.patch` (~14 KB, 406 lines)
- Coverage: `internal/ops` 50.2% → 52.3% (+2.1 pp); `Logs` 0% → 92.9%
- PR: NOT created via safeoutputs (bridge failed)

## 2026-06-19 — Run 27814296911

- Branch: `test-assist/up-orchestrator`; commit `c2719e5`
- Coverage: `internal/ops` 42.8% → 44.0% (+1.2 pp); `Up` 0% → 77.8%
- **Squashed into main via ff8d9cd (2026-06-19)** by maintainer manual merge

## 2026-06-18 — Run 27743420099

- Branch: `test-assist/restart-orchestrator`; commit `600dcda`
- Coverage: `internal/ops` 41.9% → 42.8% (+0.9 pp); `Restart` 0% → 85.7%
- **Squashed into main via ff8d9cd (2026-06-19)** by maintainer manual merge

## Earlier (see backlog.md for merged PRs)

- Monthly Activity issues: #4 (April, closed), #69 (May, closed), #109 (June, active).

[[wip]] [[backlog]]

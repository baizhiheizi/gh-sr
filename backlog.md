---
name: backlog
description: Prioritized testing opportunities for an-lee/gh-sr
metadata:
  type: project
---

## High value (likely next runs)

### 1. `internal/ops` orchestrators — `runPerHostParallel` (still 0%)
- Concurrency primitive for every multi-host op. Refactor: extract a `hostFactory func(name string, cfg config.HostConfig) (*host.Host, error)` parameter.
- 2026-06-10 covered the testable pure helpers in `internal/ops/disk.go` (6 functions 0% → 100%); orchestrators still need the refactor.
- PR #113 (repo-assist) already extracted `groupRunnersByHost`.

### 2. `internal/ops/detect.go` — `ResolveHostInfo` (0%)
### 3. `internal/ops/metrics.go` — `CollectHostMetrics` (0%)

Both need the same `hostFactory` injection as #1.

### 4. `internal/diskschedule` — Detect/Install/Uninstall/Status (0%)
- `escapePS` (PR #149, 2026-06-11), `parseAtTime`, `systemdQuoteArg`, `xmlEscapePlist` already covered.
- `Detect`/`Install`/`Uninstall`/`Status` need an `exec.Command` injection refactor — too many platform-system-call branches to mock at the boundary.

## Medium value

### 5. `internal/runner/container.go` — large lifecycle functions (37% coverage)
Real Docker work; integration-tested implicitly. Hard to unit-test.

### 6. ~~`internal/hostshell` — `WriteRemoteBytes` (0%)~~ **DONE 2026-06-11** (run #27346830049) — 0% → 100%; package 23.1% → 100.0%

### 7. `internal/runner/native.go` — 21 untested fns
Mostly Docker exec wrappers needing SSH.

## Low value (defer)

### 8. `internal/tui/*` (4.6%) — UI rendering
### 9. `cmd/gh-sr/*` (0%) — tested via `internal/ops`
### 10. `internal/autostart` (18.5%) — side-effect install helpers

## Test infrastructure ideas (Task 6 candidates)

- **Mock Executor consolidation** — open issue "Mock Executor implementations across test files" from the duplicate-code detector. A shared `internal/host/hosttest` package with one mock + exec-scripting DSL would prevent drift between the three per-package mocks. Note: as of 2026-06-11 there are now FOUR per-package mocks (host/mock_test.go, autostart, agentic, hostshell) — the hostshell one (`recordingExecutor`) is a near-clone of host's `mockExecutor`. Worth a follow-up.
- **Test-only constants for emitted commands** — lift exact shell command strings into named test constants. Pattern already used in `agentic_awf_hygiene_test.go`.

## Completed (do not re-do)

- 2026-06-11: `WriteRemoteBytes` in `internal/hostshell/hostshell.go` — 0% → 100%; `internal/hostshell` 23.1% → 100.0% (+76.9 pp)
- 2026-06-11: `escapePS` in `internal/diskschedule/diskschedule.go` — 0% → 100%; `internal/diskschedule` 15.6% → 16.3% (+0.7 pp) (PR #149)
- 2026-06-10: columnWidths, printRow, printPruneResult, PrintDiskUsageTable, diskHostInstanceKey, rcByInstanceForHost in `internal/ops/disk.go` — 0% → 100%; `internal/ops` 9.6% → 19.6% (+10.0 pp)
- 2026-06-08: ValidateAWFHygiene, ValidateAWFHygieneInner in `internal/agentic` — 0% → 100%; `internal/agentic` 60.5% → 83.9% (+23.4 pp)
- 2026-06-06: Five `Validate*` helpers in `internal/agentic/agentic.go` — 0% → 100%; `internal/agentic` 44.8% → 60.5%

[[repo]] [[testing-notes]] [[wip]] [[run-history]]

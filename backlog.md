---
name: backlog
description: Prioritized testing opportunities for an-lee/gh-sr
metadata:
  type: project
---

## High value (likely next runs)

### 1. `internal/ops/ops.go` — `runPerHostParallel` (0%)
- Concurrency primitive for every multi-host op (`Up`/`Down`/`Restart`/`Update`/`CollectStatus`).
- Refactor: extract a `hostFactory func(name string, cfg config.HostConfig) (*host.Host, error)` parameter so it's testable without SSH. Or make `ConnectHost` a package-level var.
- **Note:** repo-assist PR #113 already extracted `groupRunnersByHost` from the body. The fan-out itself still needs a `ConnectHost` injection.

### 2. `internal/ops/detect.go` — `ResolveHostInfo` (0%)
### 3. `internal/ops/metrics.go` — `CollectHostMetrics` (0%)

Both need the same `ConnectHost` injection as #1.

## Medium value

### 4. `internal/runner/container.go` — large lifecycle functions
`setupContainer`, `createContainerInstance`, `startContainer`, `stopContainer`, `removeContainer`, `containerLocalStatusImageAndRevision`, `logsContainer`, `rebuildContainerImage`, `needsSetupContainer`, `buildAgenticRunnerImage` (all 0%). Real Docker work; integration-tested implicitly.

### 5. `internal/runner/native.go` — 21 untested fns
Mostly Docker exec wrappers needing SSH. Lower priority.

## Low value (defer)

### 6. `internal/tui/*` (4.6%)
UI rendering. Hard to test without snapshot testing.

### 7. `cmd/gh-sr/*` (0%)
Tested via `internal/ops`; direct unit tests would need refactoring `loadConfig` / `newManager` to be injectable.

## Completed (do not re-do)

- `ValidateAWFHygiene` and `ValidateAWFHygieneInner` in `internal/agentic` — June 2026 run, branch `test-assist/awf-hygiene-validators`. Coverage 60.5% → 83.9% (+23.4 pp).
- Five `Validate*` helpers in `internal/agentic/agentic.go` — June 2026 run, branch `test-assist/agentic-validation-helpers`. Coverage 44.8% → 60.5%.

[[repo]] [[testing-notes]] [[wip]] [[run-history]]

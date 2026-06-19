## ADDED Requirements

### Requirement: Disk usage mode uses EffectiveRunnerMode

When measuring disk usage for a configured runner, `DiskUsageEntry.Mode` MUST be set from `RunnerConfig.EffectiveRunnerMode()` rather than hand-assigned `"container"` / `"native"` strings. Orphan instances (nil config) MUST retain mode `"unknown"`.

#### Scenario: Container runner reports container mode

- **WHEN** `MeasureDiskUsage` is called with a runner config where `EffectiveRunnerMode()` returns `"container"`
- **THEN** `DiskUsageEntry.Mode` MUST be `"container"`

#### Scenario: Agentic profile reports container mode

- **WHEN** `MeasureDiskUsage` is called with an agentic runner (profile resolves to container via `EffectiveRunnerMode()`)
- **THEN** `DiskUsageEntry.Mode` MUST be `"container"`, not `"native"`

#### Scenario: Orphan directory reports unknown mode

- **WHEN** `MeasureDiskUsage` is called with `rc == nil`
- **THEN** `DiskUsageEntry.Mode` MUST be `"unknown"` and `Orphan` MUST be true

### Requirement: Container prune dispatch is localized in disk.go

Adjacent container-mode checks in `PruneInstance` disk pruning (work/temp clear and inner Docker cache prune) MUST be consolidated into a single helper or function block within `internal/runner/disk.go` so the container-vs-native policy is documented in one place.

#### Scenario: Prune policy is readable from one location

- **WHEN** a maintainer needs to understand which prune actions run for container vs native runners
- **THEN** the branching logic MUST be findable in one helper in `disk.go` without hunting multiple scattered `IsContainerMode()` blocks within that function

## MODIFIED Requirements

<!-- None: no pre-existing openspec requirements for runner mode display -->

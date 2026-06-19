## Context

The gh-sr codebase uses an agentic Duplicate Code Detector workflow that files GitHub issues when it finds >10 lines or 3+ instances of similar patterns. Six open issues remain after seventeen were resolved via small helper extractions (notably `internal/hostshell` from #96, `containerEscalation`/`passwordlessSudo` from #134).

Current duplication hotspots:

| Issue | Location | Pattern |
|-------|----------|---------|
| #154 | `autostart`, `diskschedule` | `xmlEscapePlist`, inlined `launchdBootoutScript`, `escapePS` |
| #144 | `runner/disk.go` | `set -e` + `posixRunnerDirVar` header repeated inconsistently |
| #152 | `tui/*` | Column-width loop (3×), runner-status cells (3×); partial fix via `computeWidths` in `dashboard_view.go` only |
| #143 | `*_test.go` (5 files) | Local `mockExecutor` variants implementing `host.Executor` |
| #145 | `runner`, `doctor`, `ops`, `disk` | `IsContainerMode()` branching; `DiskUsageEntry.Mode` uses hand-assigned strings |

Established convention: cross-package shell quoting lives in `internal/hostshell`. Domain-specific helpers stay in their package when not shared.

## Goals / Non-Goals

**Goals:**

- Eliminate all duplication called out in open issues #143–145, #152, #154
- Preserve byte-identical runtime behavior (scripts, TUI output, disk measurements)
- Follow existing refactor patterns (small PRs, `Closes #N`, full test suite)
- Close meta issue #93 when sub-issues are resolved

**Non-Goals:**

- Strategy/handler abstraction for all 23 `IsContainerMode()` call sites (#145 architectural scope)
- New typed `Mode` enum beyond existing `RunnerModeNative` / `RunnerModeContainer` / `EffectiveRunnerMode()`
- Refactoring closed duplicate-code items (#92–#153 already addressed)
- Changing duplicate-code-detector workflow configuration
- Consolidating TUI render loops beyond width + cell derivation (colorization stays at call sites)

## Decisions

### 1. Plist escape → `hostshell.PlistEscape`

**Decision:** Add `PlistEscape(s string) string` to `internal/hostshell`, delete duplicates from `autostart/generate.go` and `diskschedule/diskschedule.go`.

**Rationale:** Matches #96 precedent (`PosixSingleQuote`, `PowerShellSingleQuote`). Both autostart and diskschedule already import or can import `hostshell`.

**Alternative rejected:** Export from `autostart` — creates wrong dependency direction (`diskschedule` → `autostart` for a generic XML helper).

### 2. Launchd bootout → export from `autostart`

**Decision:** Rename `launchdBootoutScript` → `LaunchdBootoutScript` (exported). `diskschedule.uninstallLaunchd` calls it with `hostshell.PosixSingleQuote(labelBase)` for the label argument, matching `autostart.autostart.go:295`.

**Rationale:** Script template is identical; hand-rolled quote escaping in diskschedule is a drift hazard.

**Alternative rejected:** Move to `hostshell` — bootout is launchd-specific, belongs with autostart domain.

### 3. PowerShell escaping in diskschedule

**Decision:** Remove `escapePS`; use `hostshell.PowerShellSingleQuote` at call sites. Adjust `fmt.Sprintf` templates to use `%s` without extra surrounding quotes where the helper already wraps.

**Rationale:** `escapePS` only doubles inner quotes; `PowerShellSingleQuote` is the canonical form.

### 4. POSIX script header in `disk.go`

**Decision:** Add unexported `posixScriptHeader(instance string) string` returning `"set -e\n" + posixRunnerDirVar(instance) + "\n"`. Use in `buildDirSizesPOSIX`, `clearWorkTempPOSIX`, and `removeDirTreePOSIX`.

**Rationale:** Fixes missing `set -e` asymmetry in clear/remove scripts (#144). Optional `posixRunnerDirLayout` constant deferred unless grep shows >2 hard-coded literals.

### 5. TUI helpers → `internal/tui/table.go`

**Decision:**

```go
func computeColumnWidths(headers []string, rows [][]string) []int
func runnerStatusCells(s runner.RunnerStatus) []string
```

Remove `computeWidths` from `dashboard_view.go` (merge into the two helpers). Colorization and lipgloss styling remain at each render site.

**Rationale:** Generic width helper serves metrics tables; `runnerStatusCells` encapsulates the 9-field contract and empty→`-` substitution. Completes what #94 started incompletely.

### 6. Test mocks → `internal/testutil`

**Decision:** New package `internal/testutil` with exported `MockExecutor` supporting: static `Output`/`RunErr`, `RunFn`, sequential `Responses`, and `Calls []string` recording. Migrate package-local mocks; delete dead `uploadErr`/`closeErr` paths unless a test exercises them.

**Rationale:** Go convention for shared test helpers. Avoids `host` importing test code. `host/mock_test.go` migrates last or re-exports for backward compat within package tests.

**Alternative rejected:** `host.NewMockExecutor()` — blurs production/test boundary.

### 7. Runner mode display (#145 — limited scope)

**Decision:**

- `MeasureDiskUsage`: set `entry.Mode` from `rc.EffectiveRunnerMode()` when `rc != nil`; keep `"unknown"` for orphans
- `PruneInstance` in `disk.go`: extract small helper for container-mode prune actions (clear + cache) to collapse adjacent checks

**Rationale:** `EffectiveRunnerMode()` already resolves agentic→container. Full dispatch abstraction is not duplication — it's intentional branching.

**Alternative rejected:** Typed `Mode` enum — redundant with existing string constants; high churn for minimal gain.

## Risks / Trade-offs

| Risk | Mitigation |
|------|------------|
| TUI output drift after refactor | Run existing `internal/tui/status_test.go`; manual spot-check dashboard if needed |
| POSIX script behavior change from adding `set -e` | Existing `TestDirSizesPOSIXScript_*` tests; add assertion that all three scripts include `set -e` |
| `PowerShellSingleQuote` quote wrapping breaks diskschedule templates | Run `internal/diskschedule/...` tests; verify generated PS matches before/after |
| `testutil` import cycles | `testutil` depends only on `host`; no imports from runner/autostart |
| Detector re-files TUI issues | Hit all 6 call sites in one PR (#152 lesson) |

## Migration Plan

Implement in four sequential PRs (can be one branch with commits, or separate PRs):

1. **PR 1:** `hostshell` + `autostart` + `diskschedule` + `disk.go` headers → Closes #154, #144
2. **PR 2:** `tui/table.go` → Closes #152
3. **PR 3:** `testutil/MockExecutor` → Closes #143
4. **PR 4:** `EffectiveRunnerMode` in disk usage + prune helper → Closes #145, #93

Each PR: `go build ./...`, `go test ./...`, `go test -race ./...`, `go vet ./...`.

Rollback: revert individual PRs; no migrations or data changes.

## Open Questions

- None blocking. Optional: hoist `$HOME/.gh-sr/runners` to a constant if a follow-up grep finds additional literals outside `posixRunnerDirVar`.

## Why

The daily Duplicate Code Detector workflow has filed six open `[duplicate-code]` issues (#143–145, #152, #154, plus grouping parent #93). Each identifies real copy-paste or structural duplication that increases maintenance cost and drift risk. Seventeen similar issues were already closed via focused refactors (e.g. `internal/hostshell`, `containerEscalation`), but the remaining items span TUI rendering, cross-package launchd/plist helpers, POSIX shell templates, test mocks, and runner-mode dispatch. Resolving them in one coordinated change prevents partial fixes that cause the detector to reopen the same patterns (as happened with #94 → #152).

## What Changes

- Extract shared shell/plist/launchd helpers into `internal/hostshell` and export launchd bootout from `internal/autostart` (#154)
- Add `posixScriptHeader` helper and consistent `set -e` usage in `internal/runner/disk.go` POSIX scripts (#144)
- Create `internal/tui/table.go` with column-width and runner-status cell helpers; replace all six duplicated call sites (#152)
- Create `internal/testutil` with a canonical `MockExecutor`; migrate five package-local test mocks (#143)
- Use `EffectiveRunnerMode()` for disk-usage mode display and consolidate adjacent container-mode checks in `disk.go` (#145 — targeted only, no strategy-pattern refactor)
- Close GitHub issues #143, #144, #145, #152, #154, and meta issue #93 when complete

No user-facing CLI behavior changes. No breaking API changes to exported packages.

## Capabilities

### New Capabilities

- `shared-shell-helpers`: Centralized plist XML escaping, launchd bootout scripts, and POSIX script headers used by autostart, diskschedule, and runner disk operations
- `tui-table-rendering`: Shared TUI table column-width computation and runner-status row cell derivation
- `test-mock-executor`: Canonical test-only `MockExecutor` implementing `host.Executor` for use across packages
- `runner-mode-display`: Consistent use of `EffectiveRunnerMode()` for disk-usage mode strings and localized prune dispatch helpers

### Modified Capabilities

<!-- No existing openspec/specs/ — this is an internal refactor with no requirement-level behavior change -->

## Impact

- **Packages**: `internal/hostshell`, `internal/autostart`, `internal/diskschedule`, `internal/runner`, `internal/tui`, `internal/testutil` (new), test files in `internal/host`, `internal/autostart`, `internal/runner`, `internal/doctor`
- **Tests**: All packages above; full `go test ./...` and `go test -race ./...` required before merge
- **Issues closed**: #93, #143, #144, #145, #152, #154
- **Risk**: Low for phases 1–3 (extract helpers, no logic change); medium-low for phase 4 (mode string source change must preserve display values)

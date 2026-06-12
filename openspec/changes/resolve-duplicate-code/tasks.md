## 1. Shared shell helpers (#154, #144)

- [x] 1.1 Add `PlistEscape(s string) string` to `internal/hostshell/hostshell.go` with tests in `hostshell_test.go`
- [x] 1.2 Replace `xmlEscapePlist` in `internal/autostart/generate.go` with `hostshell.PlistEscape`; remove local function
- [x] 1.3 Replace `xmlEscapePlist` in `internal/diskschedule/diskschedule.go` with `hostshell.PlistEscape`; update `diskschedule_test.go`
- [x] 1.4 Export `LaunchdBootoutScript` from `internal/autostart/launchd.go`; update `launchd_test.go` and `autostart.go` call sites
- [x] 1.5 Refactor `diskschedule.uninstallLaunchd` to call `LaunchdBootoutScript(hostshell.PosixSingleQuote(labelBase), plistFile)`
- [x] 1.6 Remove `escapePS` from `diskschedule`; use `hostshell.PowerShellSingleQuote` in Windows task install/uninstall templates
- [x] 1.7 Add `posixScriptHeader(instance string) string` in `internal/runner/disk.go`
- [x] 1.8 Use `posixScriptHeader` in `buildDirSizesPOSIX`, `clearWorkTempPOSIX`, and `removeDirTreePOSIX`
- [x] 1.9 Extend `disk_test.go` to assert all three POSIX scripts include `set -e`
- [x] 1.10 Run `go test ./internal/hostshell/... ./internal/autostart/... ./internal/diskschedule/... ./internal/runner/...` — Closes #154, #144

## 2. TUI table rendering (#152)

- [x] 2.1 Create `internal/tui/table.go` with `computeColumnWidths` and `runnerStatusCells`
- [x] 2.2 Refactor `dashboard_view.go`: use helpers for runner list and host-metrics panel; remove `computeWidths`
- [x] 2.3 Refactor `metrics.go`: replace both inline width loops with `computeColumnWidths`
- [x] 2.4 Refactor `status.go`: use `computeColumnWidths` and `runnerStatusCells`
- [x] 2.5 Run `go test ./internal/tui/...`; verify no output drift in `status_test.go` — Closes #152

## 3. Test mock executor (#143)

- [x] 3.1 Create `internal/testutil/mock_executor.go` with `MockExecutor` implementing `host.Executor`
- [x] 3.2 Add `internal/testutil/mock_executor_test.go` covering RunFn, Responses sequence, and Calls recording
- [x] 3.3 Migrate `internal/autostart/autostart_test.go` to `testutil.MockExecutor`
- [x] 3.4 Migrate `internal/runner/disk_test.go` (`diskMockExecutor`, `sequentialMock`) to `testutil.MockExecutor`
- [x] 3.5 Migrate `internal/runner/container_test.go` and `embedutil_test.go` to `testutil.MockExecutor`
- [x] 3.6 Migrate `internal/doctor/doctor_test.go` (`diskDoctorMock`) to `testutil.MockExecutor`
- [x] 3.7 Migrate or thin-wrap `internal/host/mock_test.go`; delete duplicate type definitions
- [x] 3.8 Run `go test ./...` and `go test -race ./...` — Closes #143

## 4. Runner mode display (#145)

- [x] 4.1 Update `MeasureDiskUsage` in `disk.go` to set `entry.Mode` from `rc.EffectiveRunnerMode()` when config present
- [x] 4.2 Extract container-mode prune helper in `disk.go` for `PruneInstance` clear/cache actions
- [x] 4.3 Add or extend test covering agentic runner disk mode displays as `"container"`
- [x] 4.4 Run `go test ./internal/runner/... ./internal/config/...` — Closes #145

## 5. Verification and issue closure

- [x] 5.1 Run full suite: `go build ./...`, `go test ./...`, `go test -race ./...`, `go vet ./...`
- [ ] 5.2 Close GitHub issues #143, #144, #145, #152, #154 with PR references
- [ ] 5.3 Close meta issue #93 when all sub-issues are resolved

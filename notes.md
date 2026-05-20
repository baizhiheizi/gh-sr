# Test Improver Notes - an-lee/gh-sr

## Key Insights

- **`host.Host.conn` is typed as `Executor` interface**: Can inject `mockExecutor` for testing SSH-dependent code WITHOUT modifying any production code. Just set `h.conn = &mockExecutor{...}` after `NewHost()`.
- **`host.Host.SetConn(Executor)`** added in this run (2026-05-20) to enable mock injection; used in `internal/autostart/autostart_test.go` and `internal/runner/container_test.go`
- `mockExecutor.runFn` allows multi-call sequences to be tested (closure captures call state)
- Go proxy blocked: tests cannot run locally; CI validates all changes.

## Testing Patterns

- Table-driven tests with `t.Run(name, func(t *testing.T) { t.Parallel(); ... })`
- `t.Parallel()` in all unit tests
- `httptest.Server` for GitHub API mocking
- `mockExecutor` for SSH-dependent tests (set `h.SetConn(&mockExecutor{runFn: func(cmd string)(string,error){...}})`)
- `containerMockExecutor` for container-specific SSH tests

## Testing Backlog

- `internal/runner/container.go`: lifecycle functions; testable via `containerMockExecutor`
- `internal/autostart/autostart.go`: Detect/Install/Start/Stop/Uninstall; testable with `mockExecutor`
- ✅ `internal/host/detect.go`: now fully tested via PR #55
- ✅ `IsServiceActive`, `absRunnerDir`, `resolveAbsoluteRunnerDir`, `containerRunnerPresent`, `containerImageExists` — tested via PR (branch)

## Completed Work

- PRs #8, #11, #31, #37, #40, #55 all merged/passed
- `DetectOS`, `DetectArch`, `DetectDockerAvailable` now tested (17 cases)
- `IsServiceActive` (7 cases), `absRunnerDir` (5 cases), `ServiceBasename` (2 cases) — autostart_test.go
- `resolveAbsoluteRunnerDir` (3 cases), `containerRunnerPresent` (3 cases), `containerImageExists` (3 cases) — container_test.go
- `host.Host.SetConn()` added to enable mock injection
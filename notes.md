# Test Improver Notes - an-lee/gh-sr

## Key Insights

- **`host.Host.conn` is typed as `Executor` interface**: Can inject `mockExecutor` for testing SSH-dependent code WITHOUT modifying any production code. Just set `h.conn = &mockExecutor{...}` after `NewHost()`.
- `mockExecutor.runFn` allows multi-call sequences to be tested (closure captures call state)
- Go proxy blocked: tests cannot run locally; CI validates all changes.

## Testing Patterns

- Table-driven tests with `t.Run(name, func(t *testing.T) { t.Parallel(); ... })`
- `t.Parallel()` in all unit tests
- `httptest.Server` for GitHub API mocking
- `mockExecutor` for SSH-dependent tests (set `h.conn = &mockExecutor{runFn: func(cmd string)(string,error){...}}`)

## Testing Backlog

- `internal/runner/container.go`: lifecycle functions; testable via `mockExecutor`
- `internal/autostart/autostart.go`: Detect/Install/Start/Stop/Uninstall; testable with `mockExecutor`
- ✅ `internal/host/detect.go`: now fully tested via PR #55

## Completed Work

- PRs #8, #11, #31, #37, #40, #55 all merged/passed
- `DetectOS`, `DetectArch`, `DetectDockerAvailable` now tested (17 cases)

# Test Improver Notes - an-lee/gh-sr

## Key Insights

- **`host.Host.conn` is typed as `Executor` interface**: Can inject `mockExecutor` for testing SSH-dependent code WITHOUT modifying any production code. Just set `h.conn = &mockExecutor{...}` after `NewHost()`.
- `powerShellSingleQuoted` in `runner/native.go` and `autostart/shell.go` are identical functions (duplicated). Tests for one do NOT cover the other.
- Go proxy blocked: tests cannot run locally; CI validates all changes.

## Testing Patterns

- Table-driven tests with `t.Run(name, func(t *testing.T) { t.Parallel(); ... })`
- `t.Parallel()` in all unit tests
- `httptest.Server` for GitHub API mocking
- Mock `Executor` interface for SSH-dependent tests

## Testing Backlog

- `internal/runner/container.go`: lifecycle functions require Docker/SSH; testable with mock executor
- `internal/autostart/autostart.go`: Detect/Install/Start/Stop/Uninstall require SSH; testable with mock executor
- `internal/host/detect.go`: DetectOS/DetectArch require SSH; testable with mock executor

## Completed Work

- PRs #8, #11, #31, #37, #40 all merged
- `containerRunnerImageExtraSorted`, `applyContainerImageExtras`, `lockedWriter`, `partitionRebuildTargets` all tested

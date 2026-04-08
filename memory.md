# Test Improver Memory - an-lee/ghr

## Build/Test/Coverage Commands

- **Build**: `go build -o ghr ./cmd/ghr/`
- **Test**: `go test ./... -race -count=1` (via `make test`)
- **Vet**: `go vet ./...` (via `make vet`)
- **Coverage**: No dedicated coverage pipeline; can use `go test ./... -coverprofile=cov.out && go tool cover -html=cov.out`
- **CI**: `.github/workflows/ci.yml` — runs `go vet ./...` and `go test ./... -race -count=1`

### Known Infrastructure Issues
- `go.mod` requires `go 1.25.0` but Go proxy is blocked in the agent sandbox (Forbidden).
- Only `go 1.24.13` is available locally; `GOTOOLCHAIN=local` fails with version mismatch.
- **Tests cannot be run in this sandbox** — infrastructure limitation. Note in PRs as "Infrastructure failure".

## Testing Notes

- Test framework: standard `testing` package (no third-party assertions)
- All test packages use `t.Parallel()` for unit tests
- HTTP client mocking via `net/http/httptest`
- Integration with GitHub API mocked via custom `httptest.Server`
- Config tests use `t.TempDir()` for temp config files
- Pattern: `*_test.go` in same package (white-box testing)

## Testing Landscape

### Well-tested packages
- `internal/config`: extensive tests (validate, load, filter, resolve env)
- `internal/runner`: tests for github client, enrich status, OS mismatch
- `internal/host`: tests for SSH parsing, PowerShell encoding, metrics
- `internal/autostart`: tests for sanitize and generate
- `internal/doctor`: tests for exit code, uniqueRepos
- `internal/tui`: status_test.go present

### Untested packages
- `internal/ops`: ops.go, service.go — NO TESTS (metrics_test.go added in PR)
  - PR submitted for `sortedHostNames`

## Testing Backlog

1. **ops/metrics.go** — `sortedHostNames` ✅ PR submitted (test-assist/pure-function-tests)
2. **runner/runner.go** — `expectedGitHubRunnerOS` ✅ PR submitted (test-assist/pure-function-tests)
3. **autostart/sanitize.go** — edge cases (numbers-only, dots, etc.)
4. **ops/ops.go** — orchestration functions are integration-heavy; skip unless mock infra added

## Task Schedule (Round-Robin)

Last run: 2026-04-08 — Tasks 3 (implement tests), 7 (activity summary)
Next run should prioritize: Task 4 (maintain PRs), Task 5 (comment on issues), Task 7

## Completed Work

None merged yet.

## Open PRs

- **test-assist/pure-function-tests**: Tests for `expectedGitHubRunnerOS` and `sortedHostNames` pure functions (2026-04-08)

## Maintainer Priorities

No maintainer comments yet.

## Monthly Activity Issue

- Issue #4: "[Test Improver] Monthly Activity 2026-04" — open

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
- `internal/ops`: ops.go, metrics.go, service.go — NO TESTS
  - Highly integration-focused (SSH + GitHub API)
  - `sortedHostNames` and `CollectHostMetrics` in metrics.go have testable pure logic
  - `FilterRunners` is already tested in config package (ops just calls it)
  - Opportunity: test `sortedHostNames` function in isolation

## Testing Backlog

1. **ops/metrics.go** — `sortedHostNames` pure function (low difficulty, medium value)
2. **ops/ops.go** — orchestration functions are integration-heavy; skip unless mock infra added
3. **runner/runner.go** — `expectedGitHubRunnerOS` pure function (not tested)
4. **autostart/sanitize.go** — edge cases (numbers-only, dots, etc.)
5. **tui/status.go** — more status rendering tests

Priority: #3 (`expectedGitHubRunnerOS`) and #1 (`sortedHostNames`) are pure functions with no dependencies; easy wins.

## Task Schedule (Round-Robin)

Last run: 2026-04-07 — Tasks 1 (discover), 2 (identify opportunities), 7 (activity summary)
Next run should prioritize: Task 3 (implement tests), Task 4 (maintain PRs), Task 7

## Completed Work

None yet.

## Open PRs

None.

## Maintainer Priorities

No maintainer comments yet.

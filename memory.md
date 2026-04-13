# Test Improver Memory - an-lee/gh-sr

## Build/Test/Coverage Commands

- **Build**: `go build -o gh-sr ./cmd/gh-sr/`
- **Test**: `go test ./... -race -count=1` (via `make test`)
- **Vet**: `go vet ./...` (via `make vet`)
- **Coverage**: No dedicated coverage pipeline; can use `go test ./... -coverprofile=cov.out && go tool cover -html=cov.out`
- **CI**: `.github/workflows/ci.yml` — runs `go vet ./...` and `go test ./... -race -count=1`

### Known Infrastructure Issues
- `go.mod` requires `go 1.25.0` but Go proxy is blocked in the agent sandbox (Forbidden).
- Only `go 1.24.13` is available locally; GOTOOLCHAIN=local fails with version mismatch.
- **Tests cannot be run in this sandbox** — infrastructure limitation. Note in PRs as "Infrastructure failure".
- safeoutputs tools were unavailable in runs 2026-04-11 and 2026-04-12; now available as of 2026-04-13.

## Testing Notes

- Test framework: standard `testing` package (no third-party assertions)
- All test packages use `t.Parallel()` for unit tests
- HTTP client mocking via `net/http/httptest`
- Integration with GitHub API mocked via custom `httptest.Server`
- Config tests use `t.TempDir()` for temp config files
- Pattern: `*_test.go` in same package (white-box testing)
- Table-driven tests are the preferred pattern in this repo

## Testing Landscape

### Well-tested packages
- `internal/config`: extensive tests (validate, load, filter, resolve env, ApplyEnvFile)
- `internal/runner`: tests for github client, enrich status, OS mismatch, docker functions
- `internal/host`: tests for SSH parsing, PowerShell encoding, metrics, normalizeArch, IsLocal, paths, local connection
- `internal/autostart`: tests for sanitize (basic + edge cases added 2026-04-13) and generate
- `internal/doctor`: tests for exit code, uniqueRepos
- `internal/tui`: status_test.go present
- `internal/ops`: metrics_test.go (sortedHostNames)

### Remaining gaps
- `internal/ops/ops.go`, `service.go` — orchestration-heavy, needs mocks
- `internal/host/detect.go` — DetectOS/DetectArch require SSH; normalizeArch now covered

## Testing Backlog

1. **ops/metrics.go** — `sortedHostNames` MERGED (PR #8)
2. **runner/runner.go** — `expectedGitHubRunnerOS` MERGED (PR #8)
3. **runner/docker.go** — `shellSingleQuote`, `dockerRunnerEntryScript` MERGED (PR #11)
4. **host/detect.go** — `normalizeArch` tests — PR SUBMITTED 2026-04-13
5. **autostart/sanitize.go** — edge cases — PR SUBMITTED 2026-04-13
6. **ops/ops.go** — orchestration functions are integration-heavy; skip unless mock infra added

## Task Schedule (Round-Robin)

Last run: 2026-04-13 — Tasks 3 (implement + PR created), 7 (monthly activity updated)
Next run should prioritize: Task 4 (maintain PRs), Task 2 (identify new opportunities), Task 7

## Open PRs

- "[Test Improver] Add normalizeArch tests and expand SanitizeInstance edge cases" — submitted 2026-04-13, branch: test-assist/detect-sanitize-edge-cases

## Completed Work

- PR #8 [MERGED 2026-04-09]: Tests for `expectedGitHubRunnerOS` and `sortedHostNames` pure functions
- PR #11 [MERGED 2026-04-10]: Tests for `shellSingleQuote`, `dockerRunnerEntryScript`, `docker_pre_setup`

## Maintainer Priorities

No maintainer comments yet.

## Monthly Activity Issue

- Issue #4: "[Test Improver] Monthly Activity 2026-04" — open, updated 2026-04-13

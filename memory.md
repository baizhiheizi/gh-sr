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
- **Git push consistently fails (exit code 128)** — safeoutputs create_pull_request saves patches but cannot push branches. Issues are created as fallback with patch artifacts instead.

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
- `internal/host`: tests for SSH parsing, PowerShell encoding, metrics, normalizeArch (patch pending), IsLocal, paths, local connection
- `internal/autostart`: tests for sanitize (expanded table-driven pending in patch) and generate
- `internal/doctor`: tests for exit code, uniqueRepos
- `internal/tui`: status_test.go present
- `internal/ops`: metrics_test.go (sortedHostNames)

### Remaining gaps
- `internal/ops/ops.go`, `service.go` — orchestration-heavy, needs mocks
- `internal/host/detect.go` — DetectOS/DetectArch require SSH; normalizeArch patch pending
- `internal/agentic/agentic.go` — patch READY (issue #31, run 24512277592)

## Testing Backlog

1. **ops/metrics.go** — `sortedHostNames` MERGED (PR #8)
2. **runner/runner.go** — `expectedGitHubRunnerOS` MERGED (PR #8)
3. **runner/docker.go** — `shellSingleQuote`, `dockerRunnerEntryScript` MERGED (PR #11)
4. **host/detect.go** — `normalizeArch` tests — PATCH READY (issue #29, run 24456550513)
5. **autostart/sanitize.go** — expanded table-driven tests (15 cases) — PATCH READY (issue #29, run 24456550513)
6. **agentic/agentic.go** — `HasBlockingFailures`, `FormatRemediation`, `FormatAllRemediations` — PATCH READY (issue #31, run 24512277592)
7. **ops/ops.go** — orchestration functions are integration-heavy; skip unless mock infra added

## Task Schedule (Round-Robin)

Last run: 2026-04-16 — Tasks 2 (rescan for opportunities), 3 (agentic tests), 7 (monthly activity updated)
Next run should prioritize: Task 4 (maintain PRs), Task 5 (comment on testing issues), Task 6 (test infrastructure), Task 7

## Open Issues with Patches

- Issue #23: earlier attempt at normalizeArch/SanitizeInstance — superseded by #29
- Issue #26: earlier attempt at normalizeArch/SanitizeInstance — superseded by #29
- Issue #29: "[Test Improver] Add normalizeArch tests and expand SanitizeInstance table-driven tests" — patch in run 24456550513
- Issue #31: "[Test Improver] Add tests for agentic prereq helper functions" — patch in run 24512277592

## Completed Work

- PR #8 [MERGED]: Tests for `expectedGitHubRunnerOS` and `sortedHostNames` pure functions
- PR #11 [MERGED]: Tests for `shellSingleQuote`, `dockerRunnerEntryScript`, `docker_pre_setup`

## Maintainer Priorities

No maintainer comments yet.

## Monthly Activity Issue

- Issue #4: "[Test Improver] Monthly Activity 2026-04" — open, updated 2026-04-16

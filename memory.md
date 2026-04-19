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
- `internal/host`: tests for SSH parsing, PowerShell encoding, metrics, normalizeArch, IsLocal, paths, local connection
- `internal/autostart`: tests for sanitize (expanded table-driven), generate, and posixSingleQuote (patch in run 24629908609)
- `internal/doctor`: tests for exit code, uniqueRepos
- `internal/tui`: status_test.go present
- `internal/ops`: metrics_test.go (sortedHostNames)
- `internal/agentic`: agentic_test.go MERGED (PR #31) — HasBlockingFailures, FormatRemediation, FormatAllRemediations
- `internal/editor`: editor_test.go for Preferred/Command (patch in run 24629908609)

### Recently merged
- normalizeArch + SanitizeInstance tests: confirmed merged in PR #37 by maintainer

### Remaining gaps
- `internal/ops/ops.go`, `service.go` — orchestration-heavy, needs mocks
- `internal/host/detect.go` — DetectOS/DetectArch require SSH; normalizeArch now tested
- `internal/autostart/shell.go` + `internal/editor/editor.go` — patch pending in run 24629908609

## Testing Backlog

1. **ops/metrics.go** — `sortedHostNames` MERGED (PR #8)
2. **runner/runner.go** — `expectedGitHubRunnerOS` MERGED (PR #8)
3. **runner/docker.go** — `shellSingleQuote`, `dockerRunnerEntryScript` MERGED (PR #11)
4. **agentic/agentic.go** — `HasBlockingFailures`, `FormatRemediation`, `FormatAllRemediations` MERGED (PR #31)
5. **host/detect.go** — `normalizeArch` tests — MERGED (included in PR #37 by maintainer)
6. **autostart/sanitize.go** — expanded table-driven tests — MERGED (included in PR #37 by maintainer)
7. **autostart/shell.go** — `posixSingleQuote` (7 cases) — PATCH READY (run 24629908609)
8. **editor/editor.go** — `Preferred` + `Command` (4 tests) — PATCH READY (run 24629908609)
9. **ops/ops.go** — orchestration functions are integration-heavy; skip unless mock infra added

## Task Schedule (Round-Robin)

Last run: 2026-04-19 — Tasks 3 (posixSingleQuote+editor patch), 7 (monthly activity updated)
Next run should prioritize: Task 4 (maintain PRs), Task 5 (comment on testing issues), Task 2 (rescan for opportunities)

## Open Issues with Patches

- Issue #35: normalizeArch/SanitizeInstance tests — COMPLETED (tests now merged into main via PR #37)
- Issue #32: agentic tests fallback issue — superseded by merged PR #31

## Current Patches

- Run 24629908609: posixSingleQuote (7 cases) + editor.Preferred/Command (4 tests)
  - Files: internal/autostart/shell_test.go, internal/editor/editor_test.go
  - Branch: test-assist/posixsinglequote-editor-preferred-2026-04-19

## Completed Work

- PR #8 [MERGED]: Tests for `expectedGitHubRunnerOS` and `sortedHostNames` pure functions
- PR #11 [MERGED]: Tests for `shellSingleQuote`, `dockerRunnerEntryScript`, `docker_pre_setup`
- PR #31 [MERGED]: Tests for `HasBlockingFailures`, `FormatRemediation`, `FormatAllRemediations` in `internal/agentic`
- normalizeArch + SanitizeInstance: tests merged into main (included in PR #37, applied by maintainer)

## Maintainer Priorities

No maintainer comments yet.

## Monthly Activity Issue

- Issue #4: "[Test Improver] Monthly Activity 2026-04" — open, updated 2026-04-19

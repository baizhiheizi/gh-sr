# gh-sr Test Improver Memory

## Discovered Commands

- **Build**: `go build -o gh-sr ./cmd/gh-sr/`
- **Test**: `go test ./... -race -count=1` (also `make test`)
- **Vet**: `go vet ./...` (also `make vet`)
- **Coverage**: `go test ./... -coverprofile=cov.out && go tool cover -html=cov.out`
- **CI**: `.github/workflows/ci.yml` — runs `go vet ./...` and `go test ./... -race -count=1`

## Testing Notes

- Test framework: standard `testing` package (no third-party assertions)
- All test packages use `t.Parallel()` for unit tests
- HTTP client mocking via `net/http/httptest`
- Pattern: `*_test.go` in same package (white-box testing)
- Table-driven tests are the preferred pattern
- Go proxy blocked: `proxy.golang.org` is firewall-blocked in sandbox

## Testing Backlog (Prioritized)

1. **`internal/runner/container.go`** — `containerRunnerImageExtraSorted` tested (2026-04-24); container lifecycle (setup/start/stop/remove/rebuild) requires SSH/Docker
2. **`internal/ops/detect.go`** — `ResolveHostInfo` concurrent detection; requires SSH mock
3. **`internal/host/detect.go`** — `DetectOS`/`DetectArch` require SSH
4. **`internal/autostart/autostart.go`** — `Detect`, `Install`, `Start`, `Stop`, `Uninstall` require SSH

## Package Test Status

| Package | Status | Notes |
|---------|--------|-------|
| internal/agentic/ | Good | PR #31 merged |
| internal/config/ | Good | Extensive tests |
| internal/ops/ | Improved | ops_test.go: applyContainerImageExtras, lockedWriter (2026-04-23) |
| internal/runner/ | Improved | containerRunnerImageExtraSorted direct tests (2026-04-24) |
| internal/doctor/ | Partial | Has tests; Run (config/GitHub API paths tested) |
| internal/editor/ | Good | Preferred/Command tested |
| internal/autostart/ | Good | posixSingleQuote, sanitize tested |

## Completed Work

- PR #8 [MERGED]: `expectedGitHubRunnerOS`, `sortedHostNames`
- PR #11 [MERGED]: `shellSingleQuote`, `dockerRunnerEntryScript`, `docker_pre_setup`
- PR #31 [MERGED]: `HasBlockingFailures`, `FormatRemediation`, `FormatAllRemediations`
- PR #37 [MERGED] (maintainer): `normalizeArch`, `SanitizeInstance`
- 2026-04-24: `containerRunnerImageExtraSorted` 9 cases (PR forthcoming)

## Open Fallback Issues

- Issue #41: `posixSingleQuote` + `editor.Preferred`/`Command` patches (run 24629908609)
- Issue #35: `normalizeArch` + `SanitizeInstance` — safe to close (merged in PR #37)
- Issue #32: superseded by PR #31 — safe to close

## Infrastructure Issues

- Go proxy blocked: tests cannot run locally; CI validates
- Git push fails (exit 128): safeoutputs creates patch artifacts instead

## Monthly Activity Issue

- Issue #4: "[Test Improver] Monthly Activity 2026-04" — updated 2026-04-24

## Last Run

2026-04-24 (current run)

# gh-sr Test Improver Memory

## Discovered Commands

- **Build**: `go build -o gh-sr ./cmd/gh-sr/`
- **Test**: `go test ./... -race -count=1`
- **Vet**: `go vet ./...`
- **Coverage**: `go test ./... -coverprofile=cov.out && go tool cover -html=cov.out`
- **CI**: `.github/workflows/ci.yml` — runs `go vet ./...` and `go test ./... -race -count=1`

## Testing Notes

- Uses standard Go `testing` package with `t.Parallel()` for parallelizable tests
- Uses `httptest` for HTTP-related tests
- Table-driven test patterns preferred
- Network-blocked environment: Go proxy returns "Forbidden"

## Testing Backlog

1. **`internal/ops/ops.go`** — `runPerHostParallel` requires SSH/mock infrastructure; `partitionRebuildTargets`, `lockedWriter`, `applyContainerImageExtras` now tested (issue #42)
2. **`internal/host/detect.go`** — `DetectOS`/`DetectArch` require SSH; skip without mock SSH infrastructure
3. **`internal/autostart/autostart.go`** — `Detect`, `Install`, `Start`, `Stop`, `Uninstall` require SSH; skip without mock infrastructure

## Completed Work

- PR #8: `expectedGitHubRunnerOS` + `sortedHostNames` tests (merged)
- PR #11: `shellSingleQuote`, `dockerRunnerEntryScript`, `docker_pre_setup` tests (merged)
- PR #31: `HasBlockingFailures`, `FormatRemediation`, `FormatAllRemediations` tests (merged)
- PR #37 (maintainer): `normalizeArch`, `SanitizeInstance` tests (merged)
- Issue #42: `partitionRebuildTargets` (4 cases) + `lockedWriter` concurrency (2 tests) + `applyContainerImageExtras` (3 cases) + `uniqueOrgs` test
- Issue #41: patch for `posixSingleQuote` + `editor.Preferred`/`Command` tests (artifact in run 24629908609)
- Issue #35: closed (normalizeArch + SanitizeInstance now in main via PR #37)
- Issue #32: closed (superseded by PR #31)

## Maintainer Priorities

No specific priorities communicated yet.

## Run History Cursor

2026-04-22: Created ops_test.go + uniqueOrgs test; issue #42 created with patch

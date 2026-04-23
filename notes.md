# Test Improver Memory - an-lee/gh-sr

## Run History

### 2026-04-23 UTC - [Run](https://github.com/an-lee/gh-sr/actions/runs/24837288650)
- 🔧 Added `TestApplyContainerImageExtras` (4 cases) + `TestLockedWriter_Sequential/Concurrent` in `internal/ops/ops_test.go`
- ⚠️ Infrastructure: git push fails (no credentials); fallback issue created; patch at `/tmp/gh-aw/aw-test-assist-ops-applycontainerimageextras-lockedwriter.patch`

### 2026-04-21 UTC - [Run](https://github.com/an-lee/gh-sr/actions/runs/24724363373)
- 🔍 Identified testing landscape; 5 high-value opportunities found

### Previous runs (from memory.md)
- 2026-04-19: Last run - Tasks 3 (posixSingleQuote+editor patch), 7 (monthly activity)
- PRs #8, #11, #31 merged previously
- Patches ready in run 24629908609 for posixSingleQuote and editor tests

## Discovered Commands

- **Build**: `go build ./...` (works on self-hosted runners with Go 1.25.x)
- **Test**: `go test ./... -race -count=1` (works)
- **Vet**: `go vet ./...` (works)
- **Coverage**: No dedicated coverage pipeline; can use `go test ./... -coverprofile=cov.out`
- **CI**: `.github/workflows/ci.yml` — runs `go vet ./...` and `go test ./... -race -count=1`
- **Make**: `make test`, `make vet`, `make build`, `make bench`

## Testing Notes

- Test framework: standard `testing` package (no third-party assertions)
- All test packages use `t.Parallel()` for unit tests
- HTTP client mocking via `net/http/httptest`
- Pattern: `*_test.go` in same package (white-box testing)
- Table-driven tests are the preferred pattern

## Testing Backlog (Prioritized)

1. `internal/runner/agentic.go` - setup functions (setupAgenticDNSConfigure, setupAgenticSudoConfigure, setupRunnerTemp) - complex shell script generation; requires SSH mock
2. `internal/doctor/doctor.go` - `Run` function (665 LOC) with concurrent host checking
3. `internal/ops/detect.go` - `ResolveHostInfo` concurrent detection with mutex-protected writes
4. `internal/runner/container.go` - container lifecycle (setup, start, stop, remove, rebuild) with zero tests

## Package Test Coverage

| Package | Test Status | Notes |
|---------|-------------|-------|
| internal/agentic/ | Good | HasBlockingFailures, FormatRemediation, FormatAllRemediations tested (PR #31) |
| internal/config/ | Good | Extensive tests |
| internal/ops/ | Improved | ops_test.go: applyContainerImageExtras + lockedWriter now tested (fallback issue created); partitionRebuildTargets + sortedHostNames tested (rebuild_test.go, metrics_test.go) |
| internal/runner/ | Partial | github.go, native.go tested; container.go and agentic.go have zero tests |
| internal/doctor/ | Partial | ExitCode, uniqueRepos, uniqueHostNames, ensureDoctorHostOS, nativeInstallTargetsForHost, containerInstallTargetsForHost, containerAgenticInstallTargetsForHost, Run (config error + GitHub API) tested |
| internal/editor/ | Good | TestPreferred, TestCommand_usesPreferred in main |
| internal/autostart/ | Good | shell_test.go (posixSingleQuote), sanitize_test.go in main |

## Infrastructure Issues

- Go proxy blocked: `proxy.golang.org` is firewall-blocked in sandbox; Go 1.25.9 available but modules can't be downloaded
- Git push fails: `exit 128` — no credentials for remote push
- **Fix**: Add `network.allowed: ["defaults", "proxy.golang.org"]` to workflow frontmatter to allow module downloads

## Monthly Activity Issue

- Issue #4: "[Test Improver] Monthly Activity 2026-04" — open, last updated 2026-04-23

## Last Run

2026-04-23 (this run)

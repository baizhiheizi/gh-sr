# Test Improver Memory - an-lee/gh-sr

## Run History

### 2026-04-21 UTC - [Run](https://github.com/an-lee/gh-sr/actions/runs/24724363373)
- 🔍 Identified opportunity: Analyzed testing landscape, found 5 high-value testing opportunities
- 📊 Coverage: Go not available in sandbox, verified via git log only one commit on main

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

## Testing Notes

- Test framework: standard `testing` package (no third-party assertions)
- All test packages use `t.Parallel()` for unit tests
- HTTP client mocking via `net/http/httptest`
- Pattern: `*_test.go` in same package (white-box testing)
- Table-driven tests are the preferred pattern

## Testing Backlog (Prioritized)

1. `internal/agentic/agentic.go` - `ValidatePrereqs` and `ValidateContainerPrereqs` (highest value - gate for agentic setup, ~10 distinct shell command checks)
2. `internal/runner/agentic.go` - setup functions (setupAgenticDNSConfigure, setupAgenticSudoConfigure, setupRunnerTemp) - complex shell script generation
3. `internal/doctor/doctor.go` - `Run` function (665 LOC) with concurrent host checking
4. `internal/ops/detect.go` - `ResolveHostInfo` concurrent detection with mutex-protected writes
5. `internal/runner/container.go` - container lifecycle (setup, start, stop, remove, rebuild) with zero tests

## Package Test Coverage

| Package | Test Status | Notes |
|---------|-------------|-------|
| internal/agentic/ | Partial | HasBlockingFailures, FormatRemediation tested (PR #31); ValidatePrereqs untested |
| internal/config/ | Good | Extensive tests; validateAgenticMCPPorts, applyAgenticDefaults untested |
| internal/runner/ | Partial | github.go, native.go tested; container.go and agentic.go have zero tests |
| internal/ops/ | Minimal | detect.go and service.go have no tests |
| internal/doctor/ | Partial | ExitCode, uniqueRepos tested; Run function untested |

## Monthly Activity Issue

- Issue #4: "[Test Improver] Monthly Activity 2026-04" — open, last updated 2026-04-19

## Last Run

2026-04-21 (this run)

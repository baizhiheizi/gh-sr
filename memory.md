# Test Improver Memory - an-lee/gh-sr

## Run History

### 2026-04-26 13:XX UTC - [Run](https://github.com/an-lee/gh-sr/actions/runs/24957479847)
- 🔧 Added `Test_nativeConfigURL` (2 cases) + `Test_powerShellSingleQuoted` (8 cases) + `Test_windowsDeleteRunnerConfig_removesCredentialFiles` in `internal/runner/native_test.go`
- 🔍 Task 6: Identified SSH mock executor infrastructure opportunity — `Executor` interface already exists; `Host.conn` typed as `Executor`; can inject `mockExecutor` for testing SSH-dependent code
- ⚠️ Infrastructure: git push fails; fallback issue created with patch artifact

### 2026-04-24 13:22 UTC - [Run](https://github.com/an-lee/gh-sr/actions/runs/24891385637)
- 🔧 Added `TestContainerRunnerImageExtraSorted` (9 cases) to `internal/runner/container_test.go`

### 2026-04-23 13:XX UTC - [Run](https://github.com/an-lee/gh-sr/actions/runs/24837288650)
- 🔧 Added `TestApplyContainerImageExtras` (4 cases) + `TestLockedWriter_Sequential` + `TestLockedWriter_Concurrent` in `internal/ops/ops_test.go`

### 2026-04-22 10:36 UTC - [Run](https://github.com/an-lee/gh-sr/actions/runs/24780298651)
- 🔧 Added `TestUniqueOrgs` to `internal/doctor/doctor_test.go`; created `internal/ops/ops_test.go`

### 2026-04-19 13:12 UTC - [Run](https://github.com/an-lee/gh-sr/actions/runs/24629908609)
- 🔧 `posixSingleQuote` (7 cases) + `editor.Preferred`/`Command` (4 tests) — merged as PR #40 ✅

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
- `host.Host.conn` is typed as `Executor` interface — can inject mock for SSH-dependent tests without production code changes
- Pattern: `*_test.go` in same package (white-box testing)
- Table-driven tests are the preferred pattern
- Go proxy blocked: `proxy.golang.org` firewall-blocked; Go 1.25.9 available but modules can't be downloaded
- Git push consistently fails (exit 128) — safeoutputs saves patches as artifacts

## Testing Backlog (Prioritized)

1. **`internal/runner/container.go`** — container lifecycle (setup/start/stop/remove/rebuild) — requires SSH/Docker; high-complexity
2. **`internal/host/detect.go`** — `DetectOS`/`DetectArch` require SSH; testable with mock executor
3. **`internal/autostart/autostart.go`** — `Detect`, `Install`, `Start`, `Stop`, `Uninstall` require SSH; testable with mock executor
4. **SSH mock executor infrastructure** — `host.Executor` interface already exists; inject `mockExecutor` into `Host.conn` for testing; no production code changes needed

## Package Test Status

| Package | Status | Notes |
|---------|--------|-------|
| internal/agentic/ | Good | PR #31 merged |
| internal/config/ | Good | Extensive tests |
| internal/ops/ | Good | ops_test.go: applyContainerImageExtras, lockedWriter, partitionRebuildTargets, sortedHostNames |
| internal/runner/ | Good | github_test.go, native_test.go, container_test.go, runner_test.go all improving |
| internal/doctor/ | Good | Run (config/GitHub API paths), uniqueOrgs, uniqueHostNames, install targets tested |
| internal/editor/ | Good | Preferred/Command tested (PR #40) |
| internal/autostart/ | Good | sanitize_test.go, shell_test.go (PR #40) |
| internal/host/ | Good | normalizeArch, IsLocal, paths, SSH parsing, metrics tested |

## Completed Work

- PR #8 [MERGED]: `expectedGitHubRunnerOS` and `sortedHostNames` pure functions
- PR #11 [MERGED]: `shellSingleQuote`, `dockerRunnerEntryScript`, `docker_pre_setup`
- PR #31 [MERGED]: `HasBlockingFailures`, `FormatRemediation`, `FormatAllRemediations` in `internal/agentic`
- PR #37 [MERGED] (maintainer): `normalizeArch`, `SanitizeInstance` tests
- PR #40 [MERGED]: `posixSingleQuote` (7 cases) + `editor.Preferred`/`Command` (4 tests)

## Open Fallback Issues

- Issue #41: `posixSingleQuote` + `editor.Preferred`/`Command` — superseded by PR #40; safe to close
- Issue #35: normalizeArch/SanitizeInstance — safe to close (merged via PR #37)
- Issue #32: agentic tests fallback — superseded by PR #31; safe to close
- New issue from this run: `nativeConfigURL`, `powerShellSingleQuoted`, `windowsDeleteRunnerConfig` tests

## Infrastructure Issues

- Go proxy blocked: tests cannot run locally; CI validates
- Git push fails (exit 128): safeoutputs create_pull_request saves patches; fallback issues created

## Monthly Activity Issue

- Issue #4: "[Test Improver] Monthly Activity 2026-04" — updated 2026-04-26

## Last Run

2026-04-26 (this run)

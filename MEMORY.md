# Test Improver Memory - an-lee/gh-sr

## Run History

### 2026-05-20 05:21 UTC - [Run](https://github.com/an-lee/gh-sr/actions/runs/26142813574)
- 🔧 Created PR (branch `test-assist/autostart-helpers`): `IsServiceActive`, `absRunnerDir`, `resolveAbsoluteRunnerDir`, `containerRunnerPresent`, `containerImageExists` tests (28 cases) via mock `Executor` injection
- 🔍 Task 3: Extended mock executor pattern to `internal/autostart` and `internal/runner/container.go`; added `host.Host.SetConn()` to enable injection
- ✅ Task 7: Closed April issue #4; created May issue
- ✅ Task 6: No new infrastructure gaps — mock executor pattern is now reusable across all SSH-dependent code
- ⚠️ Go proxy blocked: tests run locally, CI validates all changes

### 2026-04-27 13:XX UTC - [Run](https://github.com/an-lee/gh-sr/actions/runs/24997203377)
- 🔧 Created PR #55: SSH mock executor infra (`mockExecutor` + `newMockHost`) + `DetectOS`/`DetectArch`/`DetectDockerAvailable` tests (17 cases) in `internal/host/`
- 🔍 Task 6: `internal/host/detect.go` had zero tests despite being a critical path; `Executor` interface enables trivial injection

### 2026-04-26 13:XX UTC - [Run]((github/redacted)://github.com/an-lee/gh-sr/actions/runs/24957479847)
- 🔧 Added `Test_nativeConfigURL` (2 cases), `Test_powerShellSingleQuoted` (8 cases), and `Test_windowsDeleteRunnerConfig_removesCredentialFiles` to `internal/runner/native_test.go`
- 🔍 Task 6: Identified SSH mock executor infrastructure opportunity — `Executor` interface already exists; `host.Host.conn` is typed as `Executor`; can inject `mockExecutor` for testing without modifying production code

### 2026-04-24 13:22 UTC - [Run](https://github.com/an-lee/gh-sr/actions/runs/24891385637)
- 🔧 Added `TestContainerRunnerImageExtraSorted` (9 cases) to `internal/runner/container_test.go`
- 📊 Coverage: `containerRunnerImageExtraSorted` (was indirect only, now ~100%)

### 2026-04-23 13:XX UTC - [Run](https://github.com/an-lee/gh-sr/actions/runs/24837288650)
- 🔧 Added `TestApplyContainerImageExtras` (4 cases) + `TestLockedWriter_Sequential` + `TestLockedWriter_Concurrent` in `internal/ops/ops_test.go`
- 📊 Coverage: `applyContainerImageExtras` (was 0%, now ~90%), `lockedWriter.Write` (was 0%, now ~90%)

### 2026-04-22 10:36 UTC - [Run](https://github.com/an-lee/gh-sr/actions/runs/24780298651)
- 🔍 Identified new opportunity: `partitionRebuildTargets`, `lockedWriter`, and `applyContainerImageExtras` in `internal/ops/ops.go` had zero tests
- 🔧 Created `internal/ops/ops_test.go`: tests for `partitionRebuildTargets` (4 cases), `lockedWriter` concurrency (2 tests), and `applyContainerImageExtras` (3 cases)
- 🔧 Added `TestUniqueOrgs` to `internal/doctor/doctor_test.go`

### 2026-04-19 13:12 UTC - [Run](https://github.com/an-lee/gh-sr/actions/runs/24629908609)
- 🔍 Identified new opportunities: `posixSingleQuote` and `editor.Preferred`/`Command` untested
- 🔧 Created patch: `posixSingleQuote` (7 cases) + `editor.Preferred`/`Command` (4 tests) — merged as PR #40 ✅

## Testing Backlog (Prioritized)

1. **`internal/runner/container.go`** — container lifecycle (setup/start/stop/remove/rebuild) — testable via `containerMockExecutor`
2. **`internal/autostart/autostart.go`** — `Detect`, `Install`, `Start`, `Stop`, `Uninstall` — testable with `mockExecutor`
3. **`internal/autostart/autostart.go`** — `remoteHome`, `writeRemoteBytes`, `powerShellSingleQuote` — pure string helpers
4. **SSH mock executor infra** — ✅ Done (PR #55 + this run) — `mockExecutor` + `newMockHost` + `host.SetConn()` available for reuse

## Discovered Commands

- **Build**: `go build -o gh-sr ./cmd/gh-sr/`
- **Test**: `go test ./... -race -count=1` (also `make test`)
- **Vet**: `go vet ./...` (also `make vet`)
- **Coverage**: `go test ./... -coverprofile=cov.out && go tool cover -html=cov.out`
- **CI**: `.github/workflows/ci.yml` — runs `go vet ./...` and `go test ./... -race -count=1`

## Package Test Status

| Package | Status |
|---------|--------|
| internal/agentic/ | Good |
| internal/config/ | Good |
| internal/ops/ | Good |
| internal/runner/ | Good |
| internal/doctor/ | Good |
| internal/editor/ | Good |
| internal/autostart/ | Good |
| internal/host/ | Good — DetectOS/DetectArch/DetectDockerAvailable tests (PR #55) |

## Open PRs

- PR (branch `test-assist/autostart-helpers`): Mock Executor tests for `IsServiceActive`, `absRunnerDir`, `resolveAbsoluteRunnerDir`, `containerRunnerPresent`, `containerImageExists` (28 cases)

## Infrastructure Issues

- Go proxy blocked: `proxy.golang.org` firewall-blocked; Go 1.25.9 available but modules can't be downloaded; CI validates all changes

## Monthly Activity Issue

- Issue #4: "[Test Improver] Monthly Activity 2026-04" — closed 2026-05-20; May issue created

## Last Run

2026-05-20 (current run)
# Test Improver Memory - an-lee/gh-sr

## Run History

### 2026-04-26 UTC - [Run](https://github.com/an-lee/gh-sr/actions/runs/24957479847)
- 🔧 Added `Test_nativeConfigURL` + `Test_powerShellSingleQuoted` + `Test_windowsDeleteRunnerConfig_removesCredentialFiles` to `internal/runner/native_test.go`
- 🔍 Identified SSH mock executor infrastructure opportunity

### 2026-04-24 UTC - [Run](https://github.com/an-lee/gh-sr/actions/runs/24891385637)
- 🔧 Added `TestContainerRunnerImageExtraSorted` (9 cases) to `internal/runner/container_test.go`

### 2026-04-23 UTC - [Run](https://github.com/an-lee/gh-sr/actions/runs/24837288650)
- 🔧 Added `TestApplyContainerImageExtras` + `TestLockedWriter_Sequential`/`Concurrent` in `internal/ops/ops_test.go`

### 2026-04-19 UTC - [Run](https://github.com/an-lee/gh-sr/actions/runs/24629908609)
- 🔧 `posixSingleQuote` + `editor.Preferred`/`Command` tests — merged as PR #40 ✅

## Testing Backlog (Prioritized)

1. **`internal/runner/container.go`** — container lifecycle (SSH/Docker required)
2. **`internal/host/detect.go`** — `DetectOS`/`DetectArch` (testable with mock executor)
3. **`internal/autostart/autostart.go`** — SSH-dependent functions (testable with mock executor)
4. **SSH mock executor infra** — `Executor` interface exists; `Host.conn` can be injected with mock

## Discovered Commands

- Build: `go build -o gh-sr ./cmd/gh-sr/`
- Test: `go test ./... -race -count=1` (also `make test`)
- Vet: `go vet ./...`
- Coverage: `go test ./... -coverprofile=cov.out`

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
| internal/host/ | Good |

## Infrastructure Issues

- Go proxy blocked: `proxy.golang.org` firewall-blocked
- Git push fails: exit 128; fallback issues created with patch artifacts

## Monthly Activity Issue

- Issue #4: "[Test Improver] Monthly Activity 2026-04" — updated 2026-04-26

## Last Run

2026-04-26 (current run)

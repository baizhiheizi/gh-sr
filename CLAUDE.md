# gh-sr

GitHub CLI extension for managing self-hosted runners.

## Verdict After Code Changes

After any code change (edit, write, refactor), explicitly state:
1. **What changed** — specific file/function modified
2. **What was verified** — tests pass, build succeeds, etc.
3. **What to watch for** — potential side effects or related areas to check

Example verdict format:
```
[VERDICT] Modified X in Y. Build: OK. Tests: OK. Watch: Z
```

## Commands

```bash
go build ./...       # Build all packages
go test ./...        # Run tests
go test -race ./... # Race-detector tests
go vet ./...         # Vet
gh run              # Use 'gh run' (not local binary) for testing
```

## Architecture

- `cmd/gh-sr/`     — CLI entry point
- `internal/`     — Core packages (agentic, autostart, config, doctor, editor, host, ops, runner, tui)
- `config/`       — Configuration files and templates
- `scripts/`      — Helper scripts
- `docs/`         — Hugo docs site

## Key Files

- `go.mod` — module definition
- `config/runner.go` — runner configuration
- `internal/runner/` — runner management logic

## Build

```bash
go build -o gh-sr ./cmd/gh-sr
./gh-sr <subcommand>
```

## Testing

Tests interact with `gh run` (GitHub CLI), not the local `gh-sr` binary. Use the actual `gh` binary for integration testing.

## Gotchas

- The project uses `gh run` for all GitHub Actions runner interactions
- `gh-sr` is a wrapper CLI, not a library
- Self-hosted runner registration uses ephemeral runners with `os.Setenv("EphemeralRunner", "true")`

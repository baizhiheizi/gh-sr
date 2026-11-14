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
```

## Architecture

- `cmd/gh-sr/`     — CLI entry point
- `internal/`     — Core packages (agentic, autostart, cache, config, doctor, editor, host, hostshell, ops, registry, runner, tui; support packages: diskschedule, dockerutil, probe, table, testutil)
- `config/`       — Sample `runners.yml` only; the configuration code lives in `internal/config/`
- `scripts/`      — Helper scripts
- `docs/`         — Hugo docs site

## Key Files

- `go.mod` — module definition
- `internal/config/config.go` — runner/pool configuration types and validation
- `internal/runner/` — runner management logic

## Build

```bash
go build -o gh-sr ./cmd/gh-sr
./gh-sr <subcommand>
```

## Testing

Tests are Go unit tests; remote host commands are faked with `testutil.MockExecutor` (implements `host.Executor`), so no `gh` or `gh-sr` binary is required. Use the real `gh` binary only for manual integration checks.

## Gotchas

- GitHub REST calls are plain `net/http` in `internal/runner/github.go`; the token is resolved from the `gh` CLI's auth (go-gh, see `internal/config/ghauth.go`) — gh-sr never shells out to `gh`
- `gh-sr` is a gh CLI extension binary (`gh sr`), not a library
- Ephemeral runners are opt-in per pool: runners.yml `ephemeral: true` becomes `--ephemeral` on `config.sh` (native mode) / the `GH_SR_RUNNER_EPHEMERAL` env var read by the container entrypoint

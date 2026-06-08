---
name: testing-notes
description: Repo-specific test patterns, mock infrastructure, gotchas
metadata:
  type: reference
---

## Mock infrastructure

`host.Executor` is the universal seam for SSH-dependent code:

```go
type Executor interface {
    Run(cmd string) (string, error)
    Upload(localPath, remotePath string) error
    Close() error
}
```

`host.Host.conn` is typed as `Executor`; `host.Host.SetConn(Executor)` (exported) lets tests inject a mock. Per-package mocks:

- `internal/host/mock_test.go` — `mockExecutor` (basic) + `newMockHost` + `matchCmd` helper
- `internal/autostart/autostart_test.go` — same pattern, local copy
- `internal/agentic/agentic_test.go` — `prereqTestExecutor` with `response map[string]string` keyed on **exact command string**; thread-safe (`sync.Mutex` + `seen` slice)

The exact-string match in `prereqTestExecutor` is a feature: it catches silent refactors of `docker exec` shell scripts. Real example: this run's first-cut iptables-inner test failed because the inner probe drops `sudo -n` (DinD inner daemon runs as root) — the mock caught it.

The mock's `sync.Mutex` makes it safe to use against parallel goroutine fan-outs (e.g. `ValidateAWFHygiene` with three `go func()` probes sharing a `sync.Mutex`).

## Conventions

- `t.Parallel()` on every subtest.
- `t.Helper()` on assertion helpers.
- Table-driven subtests with `name` as the first field.
- For `PrereqFailure` assertions: check `Name`, `Severity`, then `strings.Contains` on `Message` and `Remediation`. The remediation string is the most valuable assertion: regressions to it leave operators without a recovery path.
- Use `t.Setenv` (not `os.Setenv`) for env-var-dependent tests.
- Lift the exact-command strings the production code emits into named constants in the test file, so refactors of the command shape are caught loudly.

## Gotchas

- `gofmt -w` may break long literal strings into multi-line; re-run `go test` after `gofmt`. May also remove unused imports.
- The `prereqTestExecutor.response` map keys are **the exact command string** the helper emits. Whitespace mismatches cause `unexpected command: ...` errors.
- For non-Linux short-circuit tests, pass an unused `prereqTestExecutor{}` to `h.SetConn()` so `Close()` works; the test only exercises the early-return.
- `ValidateContainer*` inner variants drop the `sudo -n` prefix on iptables probes (DinD inner daemon runs as root). Keep separate constants for host-level and inner probe shapes.

[[repo]] [[commands]] [[backlog]]

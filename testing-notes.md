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

`host.Host.conn` is typed as `Executor`; `host.Host.SetConn(Executor)` lets tests inject a mock. Per-package mocks:

- `internal/host/mock_test.go` — `mockExecutor` (basic, with `runFn` scripting)
- `internal/autostart/autostart_test.go` — same pattern, local copy
- `internal/agentic/agentic_test.go` — `prereqTestExecutor` keyed on **exact command string**; thread-safe (`sync.Mutex`)
- `internal/hostshell/hostshell_remote_test.go` — `recordingExecutor` records every `Run` call so the test can parse the emitted shell command

The exact-string match in `prereqTestExecutor` is a feature: it catches silent refactors of `docker exec` shell scripts.

## Conventions

- `t.Parallel()` on every subtest; `t.Helper()` on assertion helpers.
- Table-driven subtests with `name` as the first field.
- For `PrereqFailure`: check `Name`, `Severity`, then `strings.Contains` on `Message` and `Remediation`.
- Use `t.Setenv` (not `os.Setenv`) for env-var-dependent tests.
- Lift exact-command strings into named test constants so refactors are caught loudly.
- For printer/output tests, prefer **suffix-anchored `strings.TrimRight(body, " ")` assertions** over exact-string equality when column widths depend on header content. Use exact strings only when the gutter / newline placement is a user-visible contract.
- For `config.RunnerConfig` test fixtures, use `Name` values that don't already contain `-` so `InstanceNames()` produces clean `<name>-<n>` keys.
- For `runner.PruneResult.Actions` printer tests, assert on the **whole** line including the "instance on host: " prefix; name the subtest by the shape (err / skipped / success / dryRun), not the function.
- For tests that assert on a shell script emitted by production code, write a small parser that walks the single-quoted runs and reverses the escape rules (`'\''` for POSIX, `''` for PowerShell) — do not `strings.Contains` for the literal path; round-trip the path through the extractor and assert equality with the original.

## Gotchas

- `gofmt -w` may break long literal strings into multi-line; re-run `go test` after `gofmt`.
- `prereqTestExecutor.response` map keys are the **exact command string** the helper emits.
- For non-Linux short-circuit tests, pass an unused `prereqTestExecutor{}` to `h.SetConn()` so `Close()` works.
- `ValidateContainer*` inner variants drop the `sudo -n` prefix on iptables probes. Keep separate constants for host-level and inner probe shapes.
- `printRow` emits `%-*s  ` for **every** cell (including the last), then `Fprintln` adds `\n`. For cells `[a, b]` with widths `[1, 1]` the output is `"a  b  \n"`.
- `PrintDiskUsageTable` is a pure printer; it does NOT sort. Sorting is `CollectDiskUsage`'s job at `internal/ops/disk.go:162`.
- `PrintDiskUsageTable` excludes err-row `TotalBytes` from the totals sum. A naive loop would leak a giant value.
- `config.IsLocalAddr("")` is **false** — only the literal string `"local"` (case-insensitive) is local. An empty `Addr` causes `host.wrapCommand` to engage on Windows and base64-encode the script. Use `Addr: "local"` in test fixtures when you want the recorded `Run` call to see the literal PowerShell/Posix script.
- `recordingExecutor` (hostshell) records every `Run` call but does NOT short-circuit `wrapCommand`'s base64 encode — `Addr: "local"` is required to see the literal emitted script.

[[repo]] [[commands]] [[backlog]]

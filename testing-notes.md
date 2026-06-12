---
name: testing-notes
description: Repo-specific test patterns, mock infrastructure, gotchas
metadata:
  type: reference
---

## Mock infrastructure

`host.Executor` is the universal seam (Run/Upload/Close). `host.Host.SetConn(Executor)` injects mocks. Per-package mocks: `internal/host/mock_test.go` (`newMockHost`), `internal/autostart` (clone), `internal/agentic` (`prereqTestExecutor` keyed on exact command string — catches silent refactors of `docker exec` scripts), `internal/hostshell` (`recordingExecutor`). `internal/testutil/mock_executor.go` is the **shared** `MockExecutor` with `RunFn`, `Responses`, `RunErr`, `UploadCalled` — default for new tests.

## Package-level factory + per-call capture (proved in `internal/ops`)

```go
// production:
var connectHostFn = ConnectHost
func runPerHostParallel(...) error {
    connect := connectHostFn  // local capture
    for _, g := range groups { go func() { h, err := connect(g.name, hcfg); ... }() }
}
```
```go
// tests (same package):
var connectHostMu sync.Mutex
func installMockConnectHost(t *testing.T, factories map[string]host.Executor) {
    connectHostMu.Lock(); prev := connectHostFn
    t.Cleanup(func() { connectHostFn = prev; connectHostMu.Unlock() })
    connectHostFn = func(name string, hcfg config.HostConfig) (*host.Host, error) {
        mock, ok := factories[name]
        if !ok { return nil, errors.New("no mock for " + name) }
        h := host.NewHost(name, hcfg); h.SetConn(mock); return h, nil
    }
}
```
Without mutex, `-race` flags unsynchronised writes. Without local capture, early-spawned goroutines see later test swaps.

## Conventions

- `t.Parallel()` on every subtest; `t.Helper()` on assertion helpers; `name` first in table-driven subtests.
- `t.Setenv` (not `os.Setenv`) for env-var tests.
- Lift exact-command strings into named test constants.
- For printer tests, prefer suffix-anchored `strings.TrimRight(body, " ")` over exact equality when column widths depend on header content.
- For shell-script assertions, write a parser that reverses the escape rules (`'\''` POSIX, `''` PowerShell) and round-trip the path through the extractor — do not `strings.Contains` the literal.

## Gotchas

- `gofmt -w` may break long literal strings; re-run `go test` after.
- `prereqTestExecutor.response` map keys are the **exact command string** emitted.
- Non-Linux short-circuit tests: pass an unused executor to `h.SetConn()` so `Close()` works.
- `ValidateContainer*` inner variants drop `sudo -n` on iptables probes — keep separate constants.
- `printRow` emits `%-*s  ` for **every** cell (including the last), then `Fprintln` adds `\n`.
- `PrintDiskUsageTable` does NOT sort; sorting is `CollectDiskUsage`'s job. Excludes err-row `TotalBytes` from totals.
- `config.IsLocalAddr("")` is **false** — only `"local"` is local. Empty `Addr` makes `host.wrapCommand` base64-encode on Windows. Use `Addr: "local"` in fixtures when you want the literal script recorded.
- **`lockedWriter`** serialises each Write call, NOT multi-Write sequences. Use `Write([]byte(completeMessage))` for "no torn writes" tests.
- **Race detector + concurrent orchestrators**: generous timeout (10s+ under `-race`) and `close(barrier)` on timeout-failure path.

[[repo]] [[commands]] [[backlog]]

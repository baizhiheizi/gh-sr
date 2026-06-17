---
name: testing-notes
description: Repo-specific test patterns, mock infrastructure, gotchas
metadata:
  type: reference
---

## Mock infrastructure

`host.Executor` is the universal seam (Run/Upload/Close). `host.Host.SetConn(Executor)` injects mocks. Per-package mocks: `internal/host/mock_test.go` (`newMockHost`), `internal/autostart` (clone), `internal/agentic` (`prereqTestExecutor` keyed on exact command string), `internal/hostshell` (`recordingExecutor`). `internal/testutil/mock_executor.go` is the **shared** `MockExecutor` with `RunFn`, `Responses`, `RunErr` — default for new tests.

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

## Real `*runner.Manager` over a mock Manager (proved in PR for `Down`)

When the test target is the orchestrator's use of a Manager method, prefer a real `*runner.Manager{GitHub: nil}` over a mock Manager. The Manager's methods issue shell commands via `h.Run`, which a `MockExecutor` answers — the test exercises the real production code path end-to-end.

`newDownMockExecutor` wires the 4 production Stop-path branches:
- `test -f $dir/svc.sh` → "no\n" (skip svc.sh path)
- `test -f $HOME/.config/systemd/user/...` → "" (no user systemd unit)
- `test -f /etc/systemd/system/...` → "" (no system systemd unit)
- `pid_file=.runner_pid` → "not running\n" (terminal state, no signal needed)

Result: `autostart.Detect` returns `KindNone` → `stopNative` takes the "no pid file" early-exit branch. All 4 `RunFn` branches are constant across the test suite; per-test customisation is unnecessary.

## Conventions

- `t.Parallel()` on every subtest; `t.Helper()` on assertion helpers.
- `t.Setenv` (not `os.Setenv`) for env-var tests.
- Lift exact-command strings into named test constants.
- For shell-script assertions, write a parser that reverses the escape rules (`'\''` POSIX, `''` PowerShell).
- **Substring-count assertion** for torn-write detection: `strings.Count(out, want)` instead of `len(out)`. Catches the failure mode of a missing mutex more reliably than total length.

## Gotchas

- `gofmt -w` may break long literal strings; re-run `go test` after.
- `prereqTestExecutor.response` map keys are the **exact command string** emitted.
- Non-Linux short-circuit tests: pass an unused executor to `h.SetConn()` so `Close()` works.
- `ValidateContainer*` inner variants drop `sudo -n` on iptables probes — keep separate constants.
- `printRow` emits `%-*s  ` for **every** cell (including the last), then `Fprintln` adds `\n`.
- `PrintDiskUsageTable` does NOT sort; sorting is `CollectDiskUsage`'s job. Excludes err-row `TotalBytes` from totals.
- `config.IsLocalAddr("")` is **false** — only `"local"` is local. Empty `Addr` makes `host.wrapCommand` base64-encode on Windows. Use `Addr: "local"` in fixtures when you want the literal script recorded.
- **`lockedWriter`** serialises each Write call, NOT multi-Write sequences.
- **Race detector + concurrent orchestrators**: 10s+ timeout under `-race`; `close(barrier)` on timeout-failure path.
- **`connectHostMu` queue limit**: mutex held for **entire** test duration. Beyond ~15-20 parallel tests using `installMockConnectHost`, queue exceeds CI's 60s. Drop `t.Parallel()` for new factory-swap tests.
- **DO NOT call `installMockConnectHost` AND then `connectHostMu.Lock()` in the same test** — deadlock.
- **Empty result ≠ nil**: `make([]T, len(...))` returns empty (non-nil) slice when no hosts match. Test `len() == 0`, not `== nil`.
- **`mgr.Stop` for container mode** is a separate code path (calls `stopContainer`). Native-mode tests on Linux only exercise the `autostart.Detect` → `stopNative` branch. Container-mode tests would need a separate mock setup that returns valid Docker state for `docker inspect` queries.

[[repo]] [[commands]] [[backlog]]

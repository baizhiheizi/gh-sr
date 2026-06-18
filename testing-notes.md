---
name: testing-notes
description: Repo-specific test patterns, mock infrastructure, gotchas
metadata:
  type: reference
---

## Mock infrastructure

`host.Executor` is the universal seam (Run/Upload/Close). `host.Host.SetConn(Executor)` injects mocks. `internal/testutil/mock_executor.go` is the **shared** `MockExecutor` with `RunFn`, `Responses`, `RunErr` — default for new tests.

## Package-level factory + per-call capture (proved in `internal/ops`)

`var connectHostFn = ConnectHost` at package level; `connect := connectHostFn` at function entry (local capture). `installMockConnectHost(t, map[string]host.Executor)` swaps the factory behind a `connectHostMu` mutex with a `t.Cleanup` restoring the previous factory. Without mutex, `-race` flags unsynchronised writes. Without local capture, early-spawned goroutines see later test swaps.

## Real `*runner.Manager` over a mock Manager (proved in PR for `Down` / `Restart`)

When the test target is the orchestrator's use of a Manager method, prefer a real `*runner.Manager{GitHub: nil}` over a mock Manager. The Manager's methods issue shell commands via `h.Run`, which a `MockExecutor` answers.

`newDownMockExecutor()` wires Stop-path branches: svc.sh probe → "no"; systemd-user/system → ""; pid_file → "not running".

`newRestartMockExecutor()` adds Start-path branches: `test -d ... run.sh` → "yes"; `nohup ./run.sh` → "started PID 12345"; `sleep 5 ...` → "ok". Disambiguate Stop vs Start by substring (`rm -f` only in Stop; `nohup` only in Start). Use `Ephemeral: true` to skip `autostart.Install`.

## Gotchas

- `t.Setenv` (not `os.Setenv`) for env-var tests.
- `config.IsLocalAddr("")` is **false** — only `"local"` is local. Empty `Addr` makes `host.wrapCommand` base64-encode on Windows.
- `lockedWriter` serialises each Write call, NOT multi-Write sequences.
- `connectHostMu` queue limit: mutex held for entire test duration. Beyond ~15-20 parallel tests using `installMockConnectHost`, queue exceeds CI's 60s. Drop `t.Parallel()` for new factory-swap tests.
- DO NOT call `installMockConnectHost` AND then `connectHostMu.Lock()` in the same test — deadlock.
- Empty result ≠ nil: `make([]T, len(...))` returns empty (non-nil) slice when no hosts match. Test `len() == 0`, not `== nil`.
- **Substring-count assertion** for torn-write detection: `strings.Count(out, want)` instead of `len(out)`.

[[repo]] [[commands]] [[backlog]]

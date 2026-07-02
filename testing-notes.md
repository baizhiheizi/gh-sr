---
name: testing-notes
description: Repo-specific test patterns, mock infrastructure, gotchas
metadata:
  type: reference
---

## Mock infrastructure

`host.Executor` is the universal seam (Run/Upload/Close). `host.Host.SetConn(Executor)` injects mocks. `internal/testutil/mock_executor.go` is the **shared** `MockExecutor` with `RunFn`, `Responses`, `RunErr`.

## Package-level factory + per-call capture (proved in `internal/ops`)

`var connectHostFn = ConnectHost` at package level; `connect := connectHostFn` at function entry. `installMockConnectHost(t, map[string]host.Executor)` swaps behind a `connectHostMu` mutex with `t.Cleanup` restoring. Without mutex, `-race` flags unsynchronised writes.

## Real `*runner.Manager` over a mock Manager

When the test target is the orchestrator's use of a Manager method, prefer a real `*runner.Manager{GitHub: ...}` over a mock Manager. The Manager's methods issue shell commands via `h.Run`, which a `MockExecutor` answers.

## Pure-function table tests (2026-07-02)

For pure helpers with branchy contracts: pass test cases as struct slice with `name`, `want`, optional assertion flags. `t.Run(tc.name, func(t *testing.T) { t.Parallel(); ... })` runs each case in parallel automatically. No mocks/SSH/exec needed. Hits 100% coverage in one focused test file. See `internal/runner/runner_format_test.go`.

## Gotchas

- `t.Setenv` (not `os.Setenv`). `t.Setenv` is incompatible with `t.Parallel()`.
- `config.IsLocalAddr("")` is **false** — only `"local"` is local.
- `connectHostMu` queue limit: held for entire test duration. Beyond ~15-20 parallel tests, queue exceeds CI's 60s. Drop `t.Parallel()` for new factory-swap tests.
- DO NOT call `installMockConnectHost` AND then `connectHostMu.Lock()` in the same test — deadlock.
- **Substring-count assertion** for torn-write detection: `strings.Count(out, want)` instead of `len(out)`.

[[repo]] [[commands]] [[backlog]]

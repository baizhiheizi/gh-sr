---
name: testing-notes
description: Repo-specific test patterns, mock infrastructure, gotchas
metadata:
  type: reference
---

## Mock infrastructure

`host.Executor` is the universal seam (Run/Upload/Close). `host.Host.SetConn(Executor)` injects mocks. `internal/testutil/mock_executor.go` is the **shared** `MockExecutor` with `RunFn`, `Responses`, `RunErr`.

## Package-level factory + per-call capture (proved in `internal/ops`)

`var connectHostFn = ConnectHost` at package level; `connect := connectHostFn` at function entry (local capture). `installMockConnectHost(t, map[string]host.Executor)` swaps the factory behind a `connectHostMu` mutex with a `t.Cleanup` restoring the previous factory. Without mutex, `-race` flags unsynchronised writes.

## Real `*runner.Manager` over a mock Manager

When the test target is the orchestrator's use of a Manager method, prefer a real `*runner.Manager{GitHub: ...}` over a mock Manager. The Manager's methods issue shell commands via `h.Run`, which a `MockExecutor` answers.

- `newDownMockExecutor()` — Stop-path: svc.sh probe → "no"; systemd-user/system → ""; pid_file → "not running".
- `newRestartMockExecutor()` — adds Start-path: `test -d ... run.sh` → "yes"; `nohup ./run.sh` → "started PID 12345"; `sleep 5 ...` → "ok". Disambiguate by substring. Use `Ephemeral: true` to skip `autostart.Install`.
- `newUpMockExecutor()` (2026-06-19) — EnsureSetup's `NativeRunnerConfigPresent` + Start's nohup launch + sleep 5. `test -d ... run.sh` matches BOTH probes. Use `nohup ./run.sh` count for unambiguous Start-launch assertion.
- `newLogsMockExecutor(logOutput)` (2026-06-20) — single-branch. Pinned by `tail -50` + `/runner.log` substring.
- `newRemoveMockExecutor()` (2026-06-21) — same shared probes as `newDownMockExecutor()`, plus: `pid_file=` → "not running", `config.sh remove` → "", `rm -rf` → "".
- `newRemovalTokenHTTPServer(t, token)` / `newRemovalTokenHTTPErrServer(t)` (2026-06-21) — httptest-backed GitHubClient fixture. Combine with `runner.NewGitHubClientWithHTTP("pat", ts.Client(), ts.URL)`.

## Gotchas

- `t.Setenv` (not `os.Setenv`). `t.Setenv` is incompatible with `t.Parallel()`.
- `config.IsLocalAddr("")` is **false** — only `"local"` is local.
- `connectHostMu` queue limit: mutex held for entire test duration. Beyond ~15-20 parallel tests, queue exceeds CI's 60s. Drop `t.Parallel()` for new factory-swap tests.
- DO NOT call `installMockConnectHost` AND then `connectHostMu.Lock()` in the same test — deadlock.
- **Substring-count assertion** for torn-write detection: `strings.Count(out, want)` instead of `len(out)`.

[[repo]] [[commands]] [[backlog]]

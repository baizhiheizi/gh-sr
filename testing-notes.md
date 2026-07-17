---
name: testing-notes
description: Repo-specific test patterns, mock infrastructure, gotchas
metadata:
  type: reference
---

- `host.Executor` is the universal seam; `host.Host.SetConn(Executor)` injects mocks. Shared mock: `internal/testutil/mock_executor.go`.
- For package-level factory swaps, use a mutex + `t.Cleanup` restore. Example: `connectHostFn = ConnectHost`, capture at function entry, and guard swaps with `connectHostMu`; otherwise `-race` catches globals.
- Prefer real `*runner.Manager{GitHub: ...}` over mock Manager when testing orchestrators; shell/API effects can be mocked below the Manager.
- Pure branchy helpers: table tests with `t.Parallel()` are cheap and high-value when no global/env seams are mutated.
- 2026-07-08 diskschedule pattern: unexported command/GOOS seams (`runtimeGOOS`, exec funcs, PowerShell funcs) make OS-specific lifecycle code testable without systemd/launchd/Windows task side effects. Reset via `t.Cleanup`; do **not** run those tests in parallel.
- 2026-07-09 `answerInstancePresence(...)` matcher: routes `RemoteBoolCheck`-appended probes for both native and container presence checks to per-instance yes/no answers in a single `RunFn`.
- For Windows-branch tests, use a non-local `Addr` (e.g. `runner@vps`) so `host.Host.wrapCommand` activates the `powershell -EncodedCommand` base64 wrapper.
- 2026-07-17 `decodeEncodedPowerShellCommand(cmd)` (disk_test.go) mirrors `host.encodePowerShellScript`'s UTF-16LE + base64 and extracts the script body. Use it to assert script content directly. Reusable for next Windows-branch tests.
- Gotchas: `t.Setenv` incompatible with `t.Parallel()`; `config.IsLocalAddr("")` false; don't double-lock `connectHostMu`; `strings.Count` for torn-write assertions.
- Full-tree `gofmt -l .` is a CI check.
- CI has no coverage profile/artifact step; keep any addition non-gating until maintainers choose a threshold policy.

[[repo]] [[commands]] [[backlog]]
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
- 2026-07-09 `answerInstancePresence(...)` matcher: routes `RemoteBoolCheck`-appended probes for both native (`test -d ... && test -f run.sh && test -f .runner`) and container (`docker inspect | grep -q '^/name$'`) presence checks to per-instance "yes"/"no" answers in a single `RunFn`. Useful whenever testing Manager-level `NeedsSetup`-style dispatchers with multiple `InstanceNames()`.
- For Windows-branch tests in `disk.go`, use a non-local `Addr` (e.g. `runner@vps`) so `host.Host.wrapCommand` activates the `powershell -EncodedCommand` base64 wrapper. Without it the wrapper silently no-ops and the test sees the raw PS script.
- Gotchas: `t.Setenv` is incompatible with `t.Parallel()`; `config.IsLocalAddr("")` is false; do not lock `connectHostMu` inside a helper that already locked it; use `strings.Count` for torn-write assertions.
- Full-tree `gofmt -l .` is a CI check. If an unrelated Go file is already non-gofmt, a gofmt-only cleanup may be needed to keep CI green.

[[repo]] [[commands]] [[backlog]]

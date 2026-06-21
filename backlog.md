---
name: backlog
description: Prioritized testing opportunities for baizhiheizi/gh-sr
metadata:
  type: project
---

## High value (next runs)

### 1. `internal/ops` user-facing orchestrators (now 54.6%)
- `runPerHostParallel`, `ResolveHostInfo`, `CollectHostMetrics`, `removeHost` all 100%; **`Remove` 94.1%**; `Down` 83.3%; `Restart` 85.7%; `Up` 77.8% (2026-06-21).
- Still at 0%: `Update`, `RebuildImage`, `CollectStatus`, `CleanupOffline`. `Logs` also at 0% (prior branch never landed).
- `Update` is the next natural target — composes `Remove + Setup + Start`; the `httptest.Server`-backed `*runner.GitHubClient` pattern is now proven.
- `CollectStatus` / `CleanupOffline` are GitHub-heavy. Same httptest pattern + new mock for the runners-list JSON.
- `Up` 22.2% gap is the container-mode branch (too Docker-coupled without a new abstraction).
- **Bug uncovered (out of scope):** `Remove` and `Update` panic on nil `w` (no `io.Discard` default). TUI callers all pass `io.Discard` so not user-visible. Suggested fix: small `writeBanner(w, ...)` helper.

### 2. `internal/diskschedule` — Detect/Install/Uninstall/Status (0%)
- `escapePS`, `parseAtTime`, `systemdQuoteArg`, `xmlEscapePlist` covered. Need `exec.Command` injection refactor.

### 3. `internal/ops/service.go` — `ServiceInstall` (30.4%), `ServiceUninstall`/`ServiceStatus` (0%)
- Same `installMockConnectHost` seam; one PR per function.

## Medium value

### 4. `internal/runner/container.go` (43.7%) — large lifecycle, real Docker work
### 5. `internal/runner/native.go` — 21 untested fns, mostly Docker exec wrappers

## Low value (defer)

### 6. `internal/tui/*` (9.7%) — UI rendering
### 7. `cmd/gh-sr/*` (0%) — tested via `internal/ops`
### 8. `internal/autostart` (36.2%) — side-effect install helpers

## Patterns proved (reusable)

- **Package-level factory var + per-call capture**: `var connectHostFn = ConnectHost` + `connect := connectHostFn` at function entry.
- **`installMockConnectHost(t, map[string]host.Executor)`**: builds `*host.Host` with `host.SetConn` pre-wired to a mock executor.
- **Real `*runner.Manager{GitHub: g}` over a mock Manager** — `g` from `httptest.NewServer` answering the GitHub API.
- **Substring-count assertion over length**: torn-write detection on concurrent output.

## Completed (do not re-do)

- 2026-06-21: Remove 0%→94.1%; ops 52.3%→54.6% (+2.3 pp). Patch at `/tmp/gh-aw/aw-test-assist-remove-orchestrator.patch` (bridge failed).
- 2026-06-20: Logs 0%→92.9%; ops 50.2%→52.3% (+2.1 pp). Patch at `/tmp/gh-aw/aw-test-assist-logs-orchestrator.patch` (bridge failed).
- 2026-06-19: Up 0%→77.8%; ops 42.8%→44.0% (+1.2 pp). Squashed via ff8d9cd.
- 2026-06-18: Restart 0%→85.7%; ops 41.9%→42.8% (+0.9 pp). Squashed via ff8d9cd.
- 2026-06-17: Down 0%→83.3%; ops 41.1%→41.9% (+0.8 pp). **PR #200 merged**.
- 2026-06-15: CollectHostMetrics 0%→100%; ops 33.4%→37.6% (+4.2 pp); **PR #189 merged**.
- 2026-06-14: ResolveHostInfo 0%→100%; ops 24.2%→33.4% (+9.2 pp); **PR #178 merged**.
- 2026-06-12: runPerHostParallel 0%→100%; ops 19.6%→24.2% (+4.6 pp); **PR #168 merged**.
- 2026-06-11: WriteRemoteBytes 0%→100%; hostshell 23.1%→100.0%; **PR #156 merged**.
- 2026-06-10: six pure helpers in ops/disk.go 0%→100%; ops 9.6%→19.6% (+10.0 pp); **PR #147 merged**.

[[repo]] [[testing-notes]] [[wip]] [[run-history]]

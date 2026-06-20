---
name: wip
description: Current test-improver work in progress
metadata:
  type: project
---

## Run #27863360393 (2026-06-20)

- **Branch:** `test-assist/logs-orchestrator`; commit `d0bca1c`
- **PR:** **NOT CREATED** — safeoutputs `create_pull_request` returned success but the bridge did not push the branch (no git credentials locally; recurring issue, same as the 2026-06-19 Up run and 2026-06-18 Restart run). Patch preserved at `/tmp/gh-aw/aw-test-assist-logs-orchestrator.patch` (~14 KB, 406 lines). Bundle at `/tmp/gh-aw/aw-test-assist-logs-orchestrator.bundle` (~4 KB). Maintainer must apply via `git am` + push, or fetch from bundle.
- **Coverage:** `internal/ops` **50.2% → 52.3%** (+2.1 pp); `Logs` **0% → 92.9%**
- **New file:** `internal/ops/logs_test.go` (362 lines, 12 contract tests)
- **Status:** build ✅, vet ✅, race ✅, full suite ✅, gofmt ✅.

**Pattern (reusable for Remove/Update):** `newLogsMockExecutor(logOutput)` — pinned by substring `tail -50` + `/runner.log` (unique signature, no other orchestrator invokes tail). Unlike Down/Restart/Up mock executors, Logs does not need `autostart.Detect` probes, so the mock is a single-branch switch — minimal surface area.

**Why Logs was the right choice over Update/Remove:** `Logs` is the only remaining 0%-covered orchestrator that does NOT require a `*runner.GitHubClient` (its call chain is `mgr.Logs` → `logsNative` → `h.Run(tail)`). `Remove`/`Update` need `m.GitHub.GetRemovalTokenScoped`, so they require either a `httptest.Server`-backed client or a mock at the `m.GitHub` field level. Tackle those in a follow-up run once the new GitHub-mock pattern is established.

**Pattern for `Remove`/`Update` follow-up:** Use `runner.NewGitHubClientWithHTTP("test", ts.Client(), ts.URL)` with an `httptest.Server` returning `{"token":"remtok"}` for the `/repos/<owner>/<repo>/actions/runners/remove-token` POST. Then `mgr := &runner.Manager{GitHub: g}` exercises `removeNative` end-to-end without panicking on the nil client.

**Next:** `internal/ops` orchestrators still at 0%: `Update` / `Remove` / `RebuildImage` / `CollectStatus` / `CleanupOffline`. `Remove` is the next natural target — it composes `mgr.Remove` + `config.RemoveRunner`, and `Remove`'s mock-executor wiring is a strict superset of `Down`'s (`pid_file → "not running"` + the autostart probes) plus a new `rm -rf <dir>` branch for `removeNativeDirectory`.

[[backlog]] [[run-history]]
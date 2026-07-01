---
name: wip
description: Current test-improver work in progress
metadata:
  type: project
---

## Run #28499581092 (2026-07-01)

- **Branch:** `test-assist/update-orchestrator`; commit `bddc280`
- **PR:** **NOT CREATED** — safeoutputs reported success but no PR landed (phantom; 3rd consecutive). Patch at `/tmp/gh-aw/aw-test-assist-update-orchestrator.patch` (~18 KB, 448 lines). Bundle at `/tmp/gh-aw/aw-test-assist-update-orchestrator.bundle` (~5.5 KB). Maintainer: `git am` + push.
- **Coverage:** `internal/ops` 92.6% → **93.6% (+1.0 pp)**; `Update` 53.8% → **100.0%** (gap fully closed — largest single-function gap in `internal/ops`).
- **New file:** `internal/ops/orchestrator_update_test.go` (385 lines, 5 contract tests + 2 helpers)
- **Status:** build ✅, vet ✅, race ✅, full suite ✅, gofmt ✅.

**Pattern (reusable):**
- `installMockConnectHost(t, map[string]host.Executor{"h1": newUpdateMockExecutor()})` + `*runner.Manager{GitHub: runner.NewGitHubClientWithHTTP(pat, ts.Client(), ts.URL)}` — the standard real-Manager-over-httptest pattern.
- `newUpdateMockExecutor` combines `newRemoveMockExecutor`'s probes (Remove phase) with `newUpMockExecutor`'s probes (Start phase) plus Setup's `test -d`+`run.sh` returning "yes" → "already installed, skipping".
- `newUpdateGitHubHTTPServer(t, tag, token)` answers `releases/latest` + `remove-token` in one server (the existing `newSetupGitHubHTTPServer` only does releases/latest+registration-token, `newRemovalTokenHTTPServer` only does remove-token).

**Lesson learned:**
- `setupNative` calls `GetLatestRunnerVersion` *unconditionally* before checking if the runner is already installed. So even a "no-op Setup" test (where `NativeRunnerConfigPresent` returns "yes") still needs a working GitHub server.
- `removeNative` calls `GetRemovalTokenScoped` before `config.sh remove`. The token is used in the deregistration call. A working server is required.

**Safeoutputs bridge status (CRITICAL):**
- 3rd consecutive phantom run: `create_pull_request`, `update_issue`, `create_issue`, and `add_comment` all reported success but the operations did not land.
- Local artifacts (patches, bundles) survive; the bridge is the failing layer.
- Maintainer must apply the patch manually via `git am`.

**Next:** `internal/diskschedule` (14.2%) for a wider package win — needs `exec.Command` injection refactor. Otherwise `internal/runner/container.go` (`removeContainer`, `rebuildContainerImage`) or `internal/autostart` (43.7%).

[[backlog]] [[run-history]]

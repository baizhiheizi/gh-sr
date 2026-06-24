---
name: wip
description: Current test-improver work in progress
metadata:
  type: project
---

## Run #28079915247 (2026-06-24)

- **Branch:** `test-assist/collect-status-orchestrator`; commit `3c7296f`
- **PR:** **NOT CREATED** — safeoutputs bridge failed (recurring). Patch at `/tmp/gh-aw/aw-test-assist-collect-status-orchestrator.patch` (~17 KB, 487 lines). Bundle at `/tmp/gh-aw/aw-test-assist-collect-status-orchestrator.bundle` (~5 KB). Maintainer: `git am` + push.
- **Coverage:** `internal/ops` 62.1% → 68.2% (+6.1 pp); `CollectStatus` 0% → 90.5%.
- **New file:** `internal/ops/collect_status_test.go` (444 lines, 9 contract tests)
- **Status:** build ✅, vet ✅, race ✅, full suite ✅, gofmt ✅.

**Pattern (reusable):**
- `newStatusNativeRunningMock()` — substring-matched kill-0 → running
- `newCollectStatusHTTPServer` / `newEmptyGitHubHTTPServer` — httptest-backed GitHubClient fixture for EnrichWithGitHubStatus
- `barrierMockExecutor` — signals on Run + blocks on barrier (proves goroutines actually run in parallel)

**Bug uncovered (out of scope):** `CollectStatus` panics on nil `mgr.GitHub` (`mgr.EnrichWithGitHubStatus` dereferences `m.GitHub`). TUI callers all pass a real GitHub client so not user-visible today, but a future programmatic caller could trip it. Suggested fix: short-circuit when `m.GitHub == nil`.

**Next:** `Setup` (56.2%) or `Update` (53.8%) are the next natural `internal/ops` targets. Or `internal/diskschedule` (14.2%) for a wider package win.

[[backlog]] [[run-history]]
---
name: wip
description: Current test-improver work in progress
metadata:
  type: project
---

## Run #27897086810 (2026-06-21)

- **Branch:** `test-assist/remove-orchestrator`; commit `3521f2b`
- **PR:** **NOT CREATED** — safeoutputs bridge failed (recurring). Patch at `/tmp/gh-aw/aw-test-assist-remove-orchestrator.patch` (~22 KB, 633 lines). Bundle at `/tmp/gh-aw/aw-test-assist-remove-orchestrator.bundle` (~5 KB). Maintainer: `git am` + push.
- **Coverage:** `internal/ops` 52.3% → 54.6% (+2.3 pp); `Remove` 0% → 94.1%; `removeHost` 0% → 100%.
- **New file:** `internal/ops/remove_test.go` (593 lines, 12 contract tests)
- **Status:** build ✅, vet ✅, race ✅, full suite ✅, gofmt ✅.

**Pattern (reusable):** httptest.Server-backed `*runner.GitHubClient` via `runner.NewGitHubClientWithHTTP("pat", ts.Client(), ts.URL)`.

**New helpers:** `newRemovalTokenHTTPServer(t, token)`, `newRemovalTokenHTTPErrServer(t)`, `newRemoveMockExecutor()`, `writeRunnersYAML(t, content)`, `runnersYAMLFor(host, runner)`.

**Known contract pin:** removing the only runner fails post-write validation. Intentional (config requires ≥1 runner).

**Bug (out of scope):** `Remove` and `Update` panic on nil `w`. TUI callers pass `io.Discard`. Suggested fix: `writeBanner(w, ...)` helper.

**Next:** `Update` (Remove+Setup+Start), `RebuildImage`, `CollectStatus`, `CleanupOffline`. Or fix the nil-writer bug.

[[backlog]] [[run-history]]

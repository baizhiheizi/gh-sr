---
name: wip
description: Current test-improver work in progress
metadata:
  type: project
---

## Run #28355199319 (2026-06-29)

- **Branch:** `test-assist/orchestrator-connect-errors`; commit `a3c3b89`
- **PR:** **NOT CREATED** — safeoutputs reported success but no PR landed (phantom; recurring bridge failure). Patch at `/tmp/gh-aw/aw-test-assist-orchestrator-connect-errors.patch` (~6.6 KB, 172 lines). Bundle at `/tmp/gh-aw/aw-test-assist-orchestrator-connect-errors.bundle` (~2.8 KB). Maintainer: `git am` + push.
- **Coverage:** `internal/ops` 90.4% → 90.9% (+0.5 pp); `Down` 83.3% → 100.0% (gap fully closed); `Restart` 85.7% → 100.0% (gap fully closed); `RebuildImage` 69.2% → 76.9% (+7.7 pp; inner callback still uncovered).
- **New file:** `internal/ops/orchestrator_connect_errors_test.go` (127 lines, 3 contract tests)
- **Status:** build ✅, vet ✅, race ✅, full suite ✅, gofmt ✅.

**Pattern (reusable):**
- `installFailingConnectHost(t, sentinel)` from `run_per_host_parallel_test.go` — installs a package-level factory swap that always returns the sentinel. Mirrors `TestSetup_ConnectError` / `TestUpdate_ConnectError` from `orchestrators_test.go`.
- Non-local `Addr` + missing `OS`/`Arch` forces `ResolveHostInfo` to probe (without probing, the host is already "resolved" and the orchestrator skips the error branch).

**Next:** `Update` (53.8%) is the largest remaining gap (Remove + Setup + Start composition). Requires httptest GitHub client + many host probes. Otherwise `internal/diskschedule` (14.2%) for a wider package win.

[[backlog]] [[run-history]]
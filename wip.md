---
name: wip
description: Current test-improver work in progress
metadata:
  type: project
---

## Run #27814296911 (2026-06-19)

- **Branch:** `test-assist/up-orchestrator`; commit `c2719e5`
- **PR:** **NOT CREATED** — safeoutputs `create_pull_request` returned success but the bridge did not push the branch (no git credentials locally either, same as Restart run). Patch preserved at `/tmp/gh-aw/aw-test-assist-up-orchestrator.patch` (~25 KB, 680 lines). Bundle at `/tmp/gh-aw/aw-test-assist-up-orchestrator.bundle` (~6 KB). Maintainer must apply via `git am` + push, or fetch from bundle.
- **Coverage:** `internal/ops` **42.8% → 44.0%** (+1.2 pp); `Up` **0% → 77.8%**
- **New file:** `internal/ops/up_test.go` (631 lines, 12 contract tests)
- **Status:** build ✅, vet ✅, race ✅, full suite ✅, gofmt ✅.

**Pattern (reusable for Update/CollectStatus):** `newUpMockExecutor()` — successor to `newDownMockExecutor()` and `newRestartMockExecutor()`. Wires BOTH EnsureSetup's `NativeRunnerConfigPresent` (`test -d DIR && test -f DIR/run.sh && test -f DIR/.runner` → "yes") and Start's nohup launch + sleep 5 stale-check. Disambiguation note: `test -d ... run.sh` substring matches BOTH EnsureSetup's probe AND `startNativeOnce`'s pre-launch probe (same underlying `NativeRunnerConfigPresent`); for N runners, count `>= N` (not `== N`) for EnsureSetup probes. Use `nohup ./run.sh` count for unambiguous Start-launch assertion.

**Restart PR follow-up (2026-06-19):** The 2026-06-18 Restart patch (`600dcda`, branch `test-assist/restart-orchestrator`) landed via squash into commit ff8d9cd (alongside the `resolveStateDirOrFallback` refactor). The Restart and Down PRs are no longer pending — both are now part of main.

**PR-creation gotcha (recurring):** When the local working copy has no git credentials, `safeoutputs create_pull_request` may return success without the PR landing. Verify via `mcp__github__list_pull_requests` (sort=created) after each call; if missing, surface to maintainer via Monthly Activity issue + preserve patch/bundle files. Hit on the 2026-06-18 Restart run AND this run (Up).

**Next:** `internal/ops` orchestrators still at 0%: `Update` / `Remove` / `RebuildImage` / `CollectStatus` / `Logs` / `CleanupOffline`. `Update` is the most natural next target — composes Remove + Setup + Start, same pattern as Restart's Stop + Start.

[[backlog]] [[run-history]]
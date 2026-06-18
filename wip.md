---
name: wip
description: Current test-improver work in progress
metadata:
  type: project
---

## Run #27743420099 (2026-06-18)

- **Branch:** `test-assist/restart-orchestrator`; commit `600dcda`
- **PR:** **NOT CREATED** — safeoutputs `create_pull_request` returned success but the bridge did not push the branch (no git credentials locally either). Patch preserved at `/tmp/gh-aw/aw-test-assist-restart-orchestrator.patch` (~25 KB, 705 lines). Maintainer must `git apply` + push.
- **Coverage:** `internal/ops` **41.9% → 42.8%** (+0.9 pp); `Restart` **0% → 85.7%**
- **New file:** `internal/ops/restart_test.go` (673 lines, 12 contract tests)
- **Status:** build ✅, vet ✅, race ✅, full suite ✅, gofmt ✅.

**Pattern (reusable for Up/Update):** `newRestartMockExecutor` is the **dual-path** successor to `newDownMockExecutor`. Both share svc.sh + autostart probes (always "no"/empty → KindNone); disambiguate Stop vs Start by substring (`rm -f` only in Stop; `nohup ./run.sh` only in Start). Use `Ephemeral: true` to skip `autostart.Install`.

**PR-creation gotcha (NEW):** When the local working copy has no git credentials, `safeoutputs create_pull_request` may return success without the PR landing. Verify via `mcp__github__list_pull_requests` (sort=created) after each call; if missing, surface to maintainer via Monthly Activity issue + preserve patch file.

**Next:** `internal/ops` orchestrators still at 0%: `Up` / `Update` / `Remove` / `CollectStatus` / `Logs` / `CleanupOffline`.

[[backlog]] [[run-history]]

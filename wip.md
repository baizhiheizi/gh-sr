---
name: wip
description: Current test-improver work in progress
metadata:
  type: project
---

## Run #27465943133 (2026-06-14 — this run)

- **Branch:** `test-assist/resolve-host-info`; commit `1831f3e`
- **PR:** draft via `create_pull_request` safeoutputs; patch at `/tmp/gh-aw/aw-test-assist-resolve-host-info.patch` (~24 KB)
- **Coverage:** `internal/ops` **24.2% → 33.4%** (+9.2 pp); `ResolveHostInfo` **0% → 100%**
- **New file:** `internal/ops/detect_test.go` (623 lines, 13 tests, parallel, race-clean)
- **Status:** build ✅, vet ✅, `go test ./... -race -count=1` ✅ (12/12), gofmt clean.

**Pinned contracts:** NeedsDetection short-circuit; local-host skip (case-insensitive); already-resolved skip; single-host happy path; partial OS-only / partial arch-only; multi-host concurrent fan-out (barrier + timeout); DetectOS / DetectArch / connect error wrapping (no partial state leak); nil writer safety; writer serialisation across goroutines (no torn tokens); host.Close() on success AND error paths; cfg.Hosts in-place mutation contract.

**Next:** `internal/ops` orchestrators (`Up`/`Down`/`Restart`/`Update`/`Remove`/`RebuildImage`) carry heavy `*runner.Manager` surface — each requires its own mock. `internal/ops/metrics.go` `CollectHostMetrics` (0%, same injection pattern as ResolveHostInfo) is the next-smallest single-PR target.

## Prior run #27415213477 (2026-06-12) — runPerHostParallel

- 0% → 100%; `internal/ops` 19.6% → 24.2% (+4.6 pp); **PR #168 merged 2026-06-12T23:00:16Z**

## Prior run #27346830049 (2026-06-11) — hostshell WriteRemoteBytes

- 0% → 100%; `internal/hostshell` 23.1% → 100.0% (+76.9 pp); **PR #156 merged 2026-06-12T02:51:50Z**

[[backlog]] [[run-history]]

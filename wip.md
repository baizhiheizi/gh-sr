---
name: wip
description: Current optimization work in progress
metadata:
  node_type: memory
  type: project
  originSessionId: 04e58c81-0a51-4f35-bd75-e67ad9ba414d
---

# Work in Progress

- PR open: `[efficiency-improver] perf(config): inline name-N construction in InstanceNames helper` (branch `efficiency/instance-names-strconv-itoa`, commit 1cc51e5). Patch at `/tmp/gh-aw/aw-efficiency-instance-names-strconv-itoa.patch`. Awaiting PR URL.
- Issue #124 open: bench regression detection in CI — benchstat comparison step still missing (bench already runs on PRs).
- Issue #125 open: Monthly Activity 2026-06 — updated this run.

## Resolved

- PR #123 (inline instance-name lookup) — MERGED 2026-06-09T03:48:43Z.
- PR #128 (perf-improver alias of Validate fix) — MERGED 2026-06-09T03:49:34Z.
- PR #131 (an-lee, disk commands) — MERGED 2026-06-09T04:52:18Z. Introduced the disk.go code audited.
- PR #136 (single du walk) — MERGED 2026-06-09 (commit 46b6278).
- PR #146 (InstanceNames Sprintf → strconv.Itoa) — MERGED 2026-06-11T04:06:49Z.

## Open

- PR (this run) `efficiency/config-find-runner-for-logs` (commit 2eb47b6): FindRunnerForLogs single-pointer + early-exit. Match 5906→790 ns/op (-86%), 297→5 allocs/op (-98%); Ambiguous 3987→150 ns/op (-96%), 66→5 allocs/op (-92%). Patch at `/tmp/gh-aw/aw-efficiency-config-find-runner-for-logs.patch`.
- Issue #124 still open: benchstat comparison on PRs (CI `bench` job already gates on `pull_request: [main]`).

---
name: wip
description: Current optimization work in progress
metadata:
  type: project
---

# Work in Progress

- Container status SSH consolidation (this run): branch `efficiency/container-status-one-shot` (commit ef6beab). 3→1 SSH round trips per container per `Manager.Status` tick. New `containerLocalStatusOneShot` folds `echo $HOME` + bootstrap-failed marker test + docker inspect into a single `h.Run` script. `parseContainerStatusInspectOutput` gained `case "failed"` arm. `TestContainerLocalStatusImageAndRevision_one_ssh_round_trip` (5 sub-cases) pins the 1-call contract. `BenchmarkContainerLocalStatusImageAndRevision` = 1,429 ns/op, 902 B/op, 6 allocs/op. **PR creation tool returned success but no PR was opened on GitHub** (3 retries; reported `incomplete` per safeoutputs policy). Commit ef6beab is preserved on the local branch and can be pushed manually.
- Issue #124 open: benchstat comparison on PRs (CI `bench` job already gates on `pull_request: [main]`). Partially satisfied: `BenchmarkExtractTrailingPercent` + `BenchmarkMetricsRow` added 2026-06-15, `BenchmarkContainerLocalStatusImageAndRevision` added this run.
- Issue #125 open: Monthly Activity 2026-06 — updated this run.

## Resolved

- PR #123 (inline instance-name lookup) — MERGED 2026-06-09T03:48:43Z.
- PR #128 (perf-improver alias of Validate fix) — MERGED 2026-06-09T03:49:34Z.
- PR #131 (an-lee, disk commands) — MERGED 2026-06-09T04:52:18Z. Introduced the disk.go code audited.
- PR #136 (single du walk) — MERGED 2026-06-09 (commit 46b6278).
- PR #146 (InstanceNames Sprintf → strconv.Itoa) — MERGED 2026-06-11T04:06:49Z.
- PR #155 (FindRunnerForLogs single-pointer + early-exit) — MERGED 2026-06-12T02:51:36Z.
- PR #167 (EnrichWithGitHubStatus inline rcByInstance) — MERGED 2026-06-12T22:59:59Z.
- PR #191 (TUI extractTrailingPercent ParseFloat) — MERGED 2026-06-15T23:22:06Z. `BenchmarkExtractTrailingPercent` 3806→452 ns/op (-88%), 36→6 allocs/op (-83%). First TUI-side bench.

---
name: wip
description: Current optimization work in progress
metadata:
  type: project
---

# Work in Progress

- **ContainerImageLayoutRevision hoist in Manager.Status** (this run, no PR): branch `efficiency/hoist-container-image-layout-revision` (commit 15a59dc). Hoists `ContainerImageLayoutRevision(m.GhSrVersion, m.containerImageExtraApt())` out of the per-instance loop, gated by `isContainer` so native runners pay zero cost. Build/test/vet/race clean. `BenchmarkManager_Status` (Count=10 agentic, container SSH mocked): 326,748 → 50,605 ns/op (-85%), 1,516,216 → 161,493 B/op (-89%), 167 → 84 allocs/op (-50%). New `BenchmarkContainerImageLayoutRevision` pins per-call cost at 30µs, 150KB, 12 allocs/op. **PR creation tool returned success but no PR was opened on GitHub** (3 retries, 2 distinct approaches; reported `incomplete` per safeoutputs policy). This is the SECOND consecutive run with this failure (see 2026-06-17 entry in completed.md). Patch + bundle artifacts preserved at `/tmp/gh-aw/aw-efficiency-hoist-container-image-layout-revision.{patch,bundle}`.
- Container status SSH consolidation (2026-06-17): branch `efficiency/container-status-one-shot` (commit ef6beab). 3→1 SSH round trips per container per `Manager.Status` tick. New `containerLocalStatusOneShot` folds `echo $HOME` + bootstrap-failed marker test + docker inspect into a single `h.Run` script. `parseContainerStatusInspectOutput` gained `case "failed"` arm. `TestContainerLocalStatusImageAndRevision_one_ssh_round_trip` (5 sub-cases) pins the 1-call contract. `BenchmarkContainerLocalStatusImageAndRevision` = 1,429 ns/op, 902 B/op, 6 allocs/op. **PR creation tool returned success but no PR was opened on GitHub** (3 retries; reported `incomplete` per safeoutputs policy). Commit ef6beab is preserved on the local branch and can be pushed manually.
- Issue #124 open: benchstat comparison on PRs (CI `bench` job already gates on `pull_request: [main]`). Partially satisfied: `BenchmarkExtractTrailingPercent` + `BenchmarkMetricsRow` added 2026-06-15, `BenchmarkContainerLocalStatusImageAndRevision` added 2026-06-17, `BenchmarkContainerImageLayoutRevision` added 2026-06-19.
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

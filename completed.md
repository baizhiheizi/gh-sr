---
name: completed
description: "Completed efficiency work (PRs, outcomes)"
metadata:
  type: project
---

# Completed Work

## 2026-06-19 — ContainerImageLayoutRevision hoist in Manager.Status (this run, no PR)

Branch: `efficiency/hoist-container-image-layout-revision` (commit 15a59dc). **PR creation tool returned success but no PR was opened on GitHub** (3 retries, 2 distinct approaches — original branch name + suffix variant; reported `incomplete` per safeoutputs policy). Second consecutive run hit by this failure (also 2026-06-17). Patch + bundle artifacts at `/tmp/gh-aw/aw-efficiency-hoist-container-image-layout-revision.{patch,bundle}` for orchestrator post-step pickup.

Independent, additive to PR #213 (already merged 2026-06-19T02:42:34Z — hoisted `mode`/`repo`/`labels`/`host`/`isContainer`). Adds the remaining loop-invariant: `expectedLayoutRev := ContainerImageLayoutRevision(m.GhSrVersion, m.containerImageExtraApt())`. Inputs are static during one `Status()` call. The function sha256-sums every embedded `gh-sr/agentic-runner` Dockerfile asset (`Dockerfile`, `entrypoint.sh`, `sudoers`, …), the gh-sr version, and the extra-apt list, then returns the first 12 hex chars. Was being recomputed once per instance on every 5-second TUI refresh tick.

**Energy evidence** (BenchmarkManager_Status, Count=10 agentic profile, container SSH mocked):
- 326,748 → 50,605 ns/op (**-85% wall-clock**)
- 1,516,216 → 161,493 B/op (**-89% transient bytes**)
- 167 → 84 allocs/op (**-50% GC pressure**)

**New pin benchmark** (`BenchmarkContainerImageLayoutRevision`, -benchtime=1000x): 30µs, 150KB, 12 allocs/op.

GSF: energy proportionality (per-tick cost now scales linearly with instances, not N × instances of sha256-of-embedded-assets), hardware efficiency (1.35 MB less transient churn per `Manager.Status` call → less DRAM refresh + GC bookkeeping energy), demand shaping (the noisiest per-tick computation in container mode runs exactly once per refresh instead of N times). Native-mode users see zero change (`isContainer` gate).

**Test status**: build clean, vet clean, format clean, 12/12 packages pass under `go test ./... -count=1` and `go test ./... -race -count=1`.

## 2026-06-17 — Container status SSH consolidation (no PR)

Branch: `efficiency/container-status-one-shot` (commit ef6beab). **PR creation tool returned success but no PR was opened on GitHub** (3 retries; reported `incomplete` per safeoutputs policy). Commit is preserved on the local branch and can be pushed manually.

New `containerLocalStatusOneShot` folds `echo $HOME` + bootstrap-failed marker test + docker inspect into a single `h.Run` script. `parseContainerStatusInspectOutput` gained `case "failed"` arm. **SSH round trips per call: 3 (Linux) / 2 (macOS) → 1 (always).** New `TestContainerLocalStatusImageAndRevision_one_ssh_round_trip` (5 sub-cases) pins the 1-call contract. New `BenchmarkContainerLocalStatusImageAndRevision` = 1,429 ns/op, 902 B/op, 6 allocs/op. GSF: energy proportionality, hardware efficiency, demand shaping.

**Negative results this run**: (1) `enrichFromScopeRunners` O(N·M)→O(N+M) map fix regressed at the 20×10 bench scale (78K→89K ns/op, +83 allocs/op for the per-scope name index — the bench fixture is too small to amortize the per-scope map header cost). (2) `strings.EqualFold` short-circuit with `gr.OS != exp` guard is only ~3 ns/call — unmeasurable in the noise floor. Both reverted; logged as learnings.

## 2026-06-15 — TUI extractTrailingPercent ParseFloat (merged as #191)

Branch: `efficiency/tui-parsefloat-percentage-parse` (commit 8ae6038). Patch: `/tmp/gh-aw/aw-efficiency-tui-parsefloat-percentage-parse.patch`.
PR title: `[efficiency-improver] perf(tui): use strconv.ParseFloat in extractTrailingPercent`. **MERGED 2026-06-15T23:22:06Z as PR #191.**

`BenchmarkExtractTrailingPercent` 3806→452 ns/op (-88%), 818→160 B/op (-80%), 36→6 allocs/op (-83%). `fmt.Sscanf` → `strconv.ParseFloat`. Hot path: per colored host-metrics cell per Bubble Tea View() call. First TUI-side bench.

## 2026-06-12 — EnrichWithGitHubStatus inline rcByInstance (merged as #167)

PR #167 MERGED 2026-06-12T22:59:59Z. `EnrichFromScopeRunners_Small` 33→28 allocs/op (-15%). Third `InstanceNames()` discard-site closed (after #146 and #155).

## 2026-06-11 — FindRunnerForLogs (merged as #155)

PR #155 MERGED 2026-06-12T02:51:36Z. `FindRunnerForLogs_Match` 5906→790 ns/op (-86%), 297→5 allocs/op (-98%). Removed dead-code map + replaced allocating InstanceNames() with allocation-free helper + single-pointer state machine.

## 2026-06-10 — InstanceNames helper (merged as #146)

PR #146 MERGED 2026-06-11T04:06:49Z. fmt.Sprintf → `name + "-" + strconv.Itoa(i)`. 21→11 allocs/op, 1239→~430 ns/op. Helper called 23+ times across codebase.

## 2026-06-09 — single du walk in dirSizesPOSIX (merged as #136)

PR #136 MERGED 2026-06-09. 4 `du -sk` → 1 `du --max-depth=1` walk. 4 SSH round trips → 1. On 50 GB remote: ~9-15s saved per instance.

## Prior (merged)

- PR #128 Validate_Large 711→411 allocs/op (-42%) — MERGED 2026-06-09
- PR #123 FilterRunners_ByName 503→1 allocs/op (502×) — MERGED 2026-06-09

---
name: completed
description: "Completed efficiency work (PRs, outcomes)"
metadata:
  type: project
---

# Completed Work

## 2026-07-06 — FormatBytesHuman inline unit suffix (-14% time) (PR tool reported success but no PR #323 created — see WIP)

Branch: `efficiency/format-bytes-human-inlined` (commit 9337d8c). All three unit-prefix branches (`internal/runner/disk.go:511`) switched from `string(AppendFloat(buf[:0], ...)) + " GiB"` to a hoisted `[24]byte` stack buffer + AppendFloat + inline unit-suffix byte appends + one final `string(...)`.

**Energy evidence** (BenchmarkFormatBytesHuman, 5×50000x, 7 samples per iter, AMD Ryzen AI 9 HX 370):
- Before: 552/401/387/440/438 ns/op (avg **443.5**)
- After:  468/353/341/402/336 ns/op (avg **379.9**)
- **-14.3% time** on the multi-sample benchmark
- Allocs/op unchanged at 8 (Go's compiler already merged `string(slice) + "unit"` into a single allocation; the win is in the conversion path, not the alloc count).
- Per-call micro-benchmark (10×200000x, single 2 GiB sample): ~79 ns/op → ~65 ns/op (**-18% per call**).

**Test status**: build/vet/format/race clean, 14/14 packages pass.

**safe-outputs caveat**: PR creation tool reported success at create time but GitHub MCP `GET /repos/.../pulls/323` returned 404. Recurring silent-failure pattern (see [[efficiency-notes]] "safe-outputs create_pull_request silent failure pattern"). Branch + patch + bundle preserved locally for manual push.

## 2026-06-26 — TUI metrics strconv.AppendFloat + stack buffer (merged via squash)

Branch: `efficiency/metrics-strconv-formatfloat-2026-06-26` (commit 6f0b887) → squashed into 2373126 `aw: upgrade & update`. `BenchmarkMetricsRow`: 20→10 allocs/op (-50%), ~1800→~1380 ns/op (-23%).

## 2026-06-24 — TUI render strings.Builder (PR #249 MERGED)

allocs 161/255/291→158/248/284 (-2-3%); B/op -5-7%.

## Earlier (merged)

- PR #322 (doctor fan-out — perf-improver agent) — 6 → 1 agentic probe round-trips.
- PR #226 (ContainerImageLayoutRevision hoist) -85% time, -89% bytes, -50% allocs.
- PR #213 (Manager.Status loop-invariant hoist) -15.74% allocs/op, p=0.008.
- PR #203 (container status SSH consolidation) 3/2 → 1 SSH round trip.
- PR #191 (extractTrailingPercent ParseFloat) 3806→452 ns/op (-88%).
- PR #167 (EnrichWithGitHubStatus) -15% allocs/op.
- PR #155 (FindRunnerForLogs) 5906→790 ns/op (-86%), 297→5 allocs/op (-98%).
- PR #146 (InstanceNames) 21→11 allocs/op (-48%).
- PR #136 (single du walk) 4 round trips → 1.
- PR #128 (Validate_Large).
- PR #123 (FilterRunners_ByName) 503→1 allocs/op (502×).

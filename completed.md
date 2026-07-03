---
name: completed
description: "Completed efficiency work (PRs, outcomes)"
metadata:
  type: project
---

# Completed Work

## 2026-07-03 — FormatBytesHuman inline unit suffix (PR created this run)

Branch: `efficiency/format-bytes-human-single-alloc` (commit 9b384c8). All 4 branches of `FormatBytesHuman` (`internal/runner/disk.go:511`) switched from `string(AppendFloat(...)) + " GiB"` to `[24]byte` stack buffer + AppendFloat + inline unit suffix bytes. Default branch uses `AppendInt` for the same single-alloc benefit.

**Energy evidence** (-benchtime=50000x -count=3, 7 samples per iter, AMD Ryzen AI 9 HX 370):
- Before: ~444 ns/op, 56 B/op, 8 allocs/op (per-call: 2 allocs, ~120 ns)
- After:  ~367 ns/op, 48 B/op, 7 allocs/op (per-call: 1 alloc,  ~52 ns)
- **-17% time, -14% bytes, -12.5% allocs (per-call 2→1 alloc)**

**Test status**: build/vet/format/race clean, 15/15 packages pass.

**Also documents** in `internal/host/metrics.go:LoadStr` why a stack-buffer version was *worse* (553 vs 385 ns/op, +44%): `strings.Builder.String()` is zero-copy, so the pre-allocated slice wins over a stack slice that must be copied on `return string(b)`.

## 2026-06-26 — TUI metrics strconv.AppendFloat + stack buffer (merged via squash)

Branch: `efficiency/metrics-strconv-formatfloat-2026-06-26` (commit 6f0b887) → squashed into 2373126 `aw: upgrade & update`. `BenchmarkMetricsRow`: 20→10 allocs/op (-50%), ~1800→~1380 ns/op (-23%).

## 2026-06-24 — TUI render strings.Builder (PR #249 MERGED)

allocs 161/255/291→158/248/284 (-2-3%); B/op -5-7%.

## Earlier (merged)

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
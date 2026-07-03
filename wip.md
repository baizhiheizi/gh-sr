---
name: wip
description: Current optimization work in progress
metadata:
  type: project
---

# Work in Progress

- **FormatBytesHuman inline unit suffix** (this run): branch `efficiency/format-bytes-human-single-alloc` (commit 9b384c8). Per-branch switch from `string(AppendFloat(...)) + " GiB"` to `[24]byte` stack buffer + AppendFloat + inline unit suffix. `BenchmarkFormatBytesHuman` (-benchtime=50000x -count=3, 7 samples per iter): avg ~444→~367 ns/op (**-17%**), 56→48 B/op (**-14%**), 8→7 allocs/op (per-call **2→1 alloc**). Build/test/vet/format clean. PR created via safe-outputs. Also documents in `internal/host/metrics.go:LoadStr` why a stack buffer regressed 80%+ there.

## Resolved

- TUI metrics `strconv.AppendFloat` + `[24]byte` stack buffer — merged via `aw: upgrade & update` squash (commit 2373126, on tree as of 2026-07-03).
- PR #249 (TUI strings.Builder) MERGED 2026-06-24T02:01:58Z.
- PR #213 (Manager.Status loop-invariant hoist) MERGED 2026-06-19T02:42:34Z.
- PR #226 (ContainerImageLayoutRevision hoist) MERGED 2026-06-19T12:05:05Z.
- PR #123 / #128 / #131 / #136 / #146 / #155 / #167 / #191 / #203 — all merged.
- Issue #124 open: benchstat comparison on PRs. 5 prereq benches in tree.
- Issue #125 (Monthly Activity 2026-06) — updated this run with FormatBytesHuman PR.
- Issue #132 open: `gh sr storage` btrfs/reflink design — HIGH per-host disk + pull energy.
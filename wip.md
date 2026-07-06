---
name: wip
description: Current optimization work in progress
metadata:
  type: project
---

# Work in Progress

- **FormatBytesHuman inline unit suffix** (this run, 2026-07-06): branch `efficiency/format-bytes-human-inlined` (commit 9337d8c). Switched the three unit-prefix branches (GiB/MiB/KiB) from `string(AppendFloat(buf[:0], ...)) + " GiB"` to a single `[24]byte` stack buffer + AppendFloat + inline unit-suffix bytes + one final `string(...)`. `BenchmarkFormatBytesHuman` (5×50000x, AMD Ryzen AI 9 HX 370): avg **443.5 → 379.9 ns/op (-14.3%)**, B/op and allocs/op unchanged (Go's compiler already merged the string+concat into one alloc; the win is on the conversion path). Build/vet/format/test clean (14/14 packages). **safe-outputs `create_pull_request` reported success but GitHub MCP confirmed no PR #323 (404)** — recurring silent-failure pattern noted in [[efficiency-notes]]. Patch at `/tmp/gh-aw/aw-efficiency-format-bytes-human-inlined.patch` and bundle at `/tmp/gh-aw/aw-efficiency-format-bytes-human-inlined.bundle`.

## Resolved

- TUI metrics `strconv.AppendFloat` + `[24]byte` stack buffer — merged via `aw: upgrade & update` squash (commit 2373126, on tree as of 2026-07-03).
- PR #249 (TUI strings.Builder) MERGED 2026-06-24T02:01:58Z.
- PR #213 (Manager.Status loop-invariant hoist) MERGED 2026-06-19T02:42:34Z.
- PR #226 (ContainerImageLayoutRevision hoist) MERGED 2026-06-19T12:05:05Z.
- PR #123 / #128 / #131 / #136 / #146 / #155 / #167 / #191 / #203 — all merged.
- Issue #124 open: benchstat comparison on PRs. 5 prereq benches in tree.
- Issue #125 (Monthly Activity 2026-06) — being closed this run; opening Issue #XXX for 2026-07.
- Issue #132 open: `gh sr storage` btrfs/reflink design — HIGH per-host disk + pull energy.

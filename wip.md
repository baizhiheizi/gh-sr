# Work in Progress

- **PR pending (this run, 2026-07-11)**: branch `efficiency/benchstat-formatnumber-write-direct` (commit 46ed29d, amended from f875693 to include `BenchmarkWriteNumber`). Title: `[efficiency-improver] perf(benchstat): write FormatNumber digits directly into the builder`. RenderMarkdown: 47→2 allocs/op (-95.7%); FullPipeline: 145→100 allocs/op (-31%). Patch at `/tmp/gh-aw/aw-efficiency-benchstat-formatnumber-write-direct.patch`, bundle at `/tmp/gh-aw/aw-efficiency-benchstat-formatnumber-write-direct.bundle`. `safe-outputs create_pull_request` reported success — treating as the delayed-squash pattern from prior runs.

## Open issues

- Issue #324 (monthly activity — being updated this run).
- Issue #132: btrfs/reflink design — HIGH per-host disk + pull energy; still open since 2026-06-15. Last engagement: 2026-06-15 (me) + 2026-06-09 (Repo Assist). No new human comments since my last engagement → re-engagement rule says skip until human re-engages.

## Closed issues / merged PRs since 2026-07-07

- PR #345 MERGED 2026-07-09 — `RenderMarkdown` `fmt.Fprintf`→builder (squash-merge).
- Issue #124 CLOSED 2026-07-08 — benchstat on PRs (5 prereq benches in tree; PR #333 closed it).
- PR #323 MERGED 2026-07-07 — `FormatBytesHuman` inline unit suffix (squash-merge 14d1edb).

## Resolved

- **FormatBytesHuman inline unit suffix** MERGED 2026-07-07 as **PR #323** via squash (branch 9337d8c → tree 14d1edb). `BenchmarkFormatBytesHuman` (5×50000x, AMD Ryzen AI 9 HX 370): avg **443.5 → 379.9 ns/op (-14.3%)**; per-call micro-bench ~79 → ~65 ns/op (**-18%**).
- TUI metrics `strconv.AppendFloat` + `[24]byte` stack buffer — merged via squash 2373126.
- PR #345 / #249 / #213 / #226 / #123 / #128 / #131 / #136 / #146 / #155 / #167 / #191 / #203 — all merged.

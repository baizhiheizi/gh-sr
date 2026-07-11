# Completed Work

## 2026-07-11 — scripts/benchstat writeNumber direct-to-builder (-95.7% allocs/op) — **DRAFT PR pending**

Branch `efficiency/benchstat-formatnumber-write-direct` (46ed29d, amended from f875693). `FormatNumber(f)` returned a string coerced from a stack `[32]byte` slice; `sb.WriteString` copied those bytes back into the builder — one alloc per call. `writeNumber(sb, f)` appends digits via `sb.Write(strconv.AppendFloat(b[:0], …, 64))` and skips the string coercion.

| Bench | Before (PR #345) | After | Δ |
|---|---|---|---|
| `BenchmarkRenderMarkdown` | 7,746 ns · 2,800 B · **47 allocs/op** | 4,928 ns · 2,560 B · **2 allocs/op** | **−36.4% / −8.6% / −95.7%** |
| `BenchmarkFullPipeline` | ~37,000 ns · 25,570 B · 145 allocs | ~30,600 ns · 25,368 B · **100 allocs** | −17.3% / −0.8% / **−31.0%** |
| `BenchmarkWriteNumber` (new) | n/a | 545 ns · 64 B · 1 allocs | new sentinel for `bench`/`bench-compare` |

`FormatNumber` lowercased to `writeNumber` (no external callers). Added `TestWriteNumber` + `TestWriteNumber_zeroAllocsPerCall` regression guard. 14/14 existing + 2 new tests pass; 17/17 packages OK.

## 2026-07-09 — scripts/benchstat RenderMarkdown (-79.2% allocs/op) — **MERGED PR #345**

| Bench | Before | After | Δ |
|---|---|---|---|
| `BenchmarkRenderMarkdown` | 12,877 ns · 6,205 B · **226 allocs/op** | 6,248 ns · 2,800 B · **47 allocs/op** | **−51.5% / −54.9% / −79.2%** |
| `BenchmarkFullPipeline` | 42,309 ns · 29,045 B · 324 allocs | 31,427 ns · 25,637 B · 145 allocs | **−25.7% / −11.7% / −55.2%** |

## 2026-07-07 — FormatBytesHuman inline unit suffix (-14.3%) — **MERGED PR #323**

PR #323 (squash-merge 14d1edb). `BenchmarkFormatBytesHuman`: 443.5 → 379.9 ns/op.

## Earlier merged (run-history.md for dates)

PR #322 doctor fan-out · #226 ContainerImageLayoutRevision · #213 Manager.Status · #203 container SSH (3→1) · #191 extractTrailingPercent · #167 EnrichWithGitHubStatus · #155 FindRunnerForLogs · #146 InstanceNames · #136 dirSizesPOSIX · #128 Validate_Large · #123 FilterRunners · #249 TUI render · TUI metrics (squash 2373126).

---
name: repo-assist-state
description: Repo Assist persistent state — in-flight work, backlog, verified knowledge
metadata:
  type: project
---

# Repo Assist state — 2026-07-07 (run 28896353244)

## Last run (28896353244) — Tasks 8, 2, 4
- **Task 8** — `repo-assist/perf-parsefourint64s-no-fields-alloc-2026-07-07` @ `1d71bbf`. disk.go +61/−7 + new disk_parse_bench_test.go +17. `BenchmarkParseFourInt64s`: 90.5→52.8 ns/op (-42%), 64→0 B/op (-100%), 1→0 allocs/op. PR reported success.
- **Task 2** — Comment on #330 (FormatBytesHuman table-driven refactor analysis).
- **Task 4** — `repo-assist/eng-makefile-coverage-target-2026-07-07` @ `7b4e8fa`. Makefile +16/−1. Adds `make coverage` + `make coverage-html`. PR reported success.
- **Task 11** — Updated #309.

## In-flight / backlog
- **#124 / #307** — bench-compare piece. `2c76716` on `repo-assist/eng-bench-compare-2026-07-01`. Phantom PR. Recoverable from GitHub Actions artifact of run `28545571496` until late September 2026.
- **#132** (gh sr storage) — on hold.
- **#208** parent — duplicate-code detector; recent findings closed/commented on (#325/#326/#327/#330 from 2026-07-06/07).
- **#309** — current Monthly Activity.
- **#330** (FormatBytesHuman) — Repo Assist commented 2026-07-07 with sub-scope offer; awaiting maintainer call.

## Notes
- **phantom-PR pattern:** 16+ occurrences. Recovery via GitHub Actions artifact of run `28545571496` (90-day retention).
- **mid-run reverts** (re-confirmed this run): NOT observed. Both new branches survived commit + bridge calls cleanly.

## Verified knowledge
- **Quoting:** `strconv.Quote` for docker CLI args; `hostshell.PosixSingleQuote` for shell snippets.
- **Protected:** `go.mod`, `go.sum`, `CHANGELOG.md`, `.github/workflows/ci.yml`, `.github/workflows/bench-compare.yml`.
- **Render:** `renderRow`/`renderHighlightedRow` share `renderRowWith`; `table.RenderPlain` = strings.Builder + appendRowPlain, 1-alloc/cell.
- **autostart:** `ListInstalled(h)` linux/darwin/windows/unsupported dispatch; `parseInstanceLines(out)` preserves order.
- **host.Host.RunShell:** wraps via wrapCommand (windows+non-local base64; else no-op).
- **tui:** `hostMetricsHeaders`, `buildHostMetricsRows`, `hostMetricsColorize` shared by PrintHostMetricsTable + FormatHostMetrics + viewHostMetrics.
- **FormatBytesHuman (898e101):** 4× fmt.Sprintf → AppendFloat + [16]byte + FormatInt. -40% ns/op, -50% B/op, -40% allocs.
- **tui.formatPercent + formatUsedTotal (e0c9de9):** AppendFloat + [24]byte / [48]byte. `BenchmarkFormatHostMetrics` -10% ns/op, -2% B/op.
- **scripts/benchstat/main.go (2c76716):** //go:build ignore, stdlib only. Thresholds: ns/op 10/30%, B/op 15/50%, allocs/op 10/25%.
- **bench-compare.yml (2c76716):** pull_request:[main], downloads main bench-*, posts PR comment via peter-evans/create-or-update-comment@v4 edit-mode:replace. PROTECTED FILE.
- **host.LoadStr (977e1c5, branch only):** AppendFloat + [40]byte. -9.5% ns/op, -33% B/op. Maintainer reverted working tree; PR not opened.
- **editor.Preferred + Open:** Windows notepad default + exec error chain wraps `exec.ErrNotFound` (NOT fs.ErrNotExist).
- **Makefile targets after 7b4e8fa:** all build test bench coverage coverage-html vet fmt tidy ci check clean install uninstall. `make ci` mirrors ci.yml (vet + fmt + test); `make coverage` adds -coverprofile + project total; `make coverage-html` writes annotated HTML.
- **parseFourInt64s (1d71bbf, this run):** manual ASCII scan instead of strings.Fields + make([]int64, 4). [4]string + [4]int64 stack buffers; LastIndexByte('\n') + ASCII-only trim. BenchmarkParseFourInt64s 90.5→52.8 ns/op (-42%), 64→0 B/op (-100%), 1→0 allocs/op.
- **agentic fanout (#325):** `containerAgenticFanoutCheckCommand` lines 760-805 inlines 6 shell bodies (duplicated from `*CheckCommand` helpers at 945-1005). `containerAgenticFanoutSpecs` lines 815-909 repeats metadata (duplicated from per-check wrappers at 530-688). `agentic_fanout_test.go` snapshots both the assembled command string and the spec map — load-bearing coupling for any unification refactor.
- **Coverage (this run, project total = 58.0%):** highs `internal/autostart` 94.7%, `internal/ops` 93.6%, `internal/editor` 92.3%, `internal/agentic` 89.6%. Lows `internal/diskschedule` 14.2%, `internal/tui` 15.0%, `internal/runner` 57.6%, `internal/host` 59.6%.

---
name: repo-assist-state
description: Repo Assist persistent state — in-flight work, backlog, verified knowledge (latest: 2026-07-09 #29047663570)
metadata:
  type: project
---

# Repo Assist state — 2026-07-09 (run 29047663570)

## Last run — Tasks 5, 8, 3
- **Task 5** — `repo-assist/improve-tui-runner-status-table-columns-2026-07-09` @ `1a1fb8c`. Resolves #346. Runner-status headers + colorize lambda dedup. Net +29/−32.
- **Task 8** — `repo-assist/perf-shared-fmtfloat-helper-2026-07-09` @ `d126f1a`. Resolves #347. New `internal/strfmt.FmtFloat`. Benchmarks unchanged (alloc counts and ns/op within noise).
- **Task 3** — Fallback (no bug/help-wanted/good-first-issue).
- **Task 11** — Updated #309. Body rebuilt; Run History trimmed (older 2026-07-04 / 2026-07-03 / 2026-07-01 collapsed to pointer) to fit the 10 KB tool-input limit.

## In-flight / backlog
- **#307 / #124** — bench-compare piece. Closes via PR #333 (merged 2026-07-08).
- **#132** (gh sr storage) — on hold.
- **#208** parent — duplicate-code detector; sub-issues from 2026-07-06/07 all closed; **#346 + #347 from 2026-07-09 PRs awaiting review**; #341 from 2026-07-08 still pending scope pick.
- **#309** — current Monthly Activity.
- **#341** — pending maintainer scope choice (A/B/C).
- **Open PRs awaiting review:** #328, #323, #338, #339, #340, #342, #343, #344, #345, plus my new #346 + #347 PRs.

## Notes
- phantom-PR pattern: 16+ occurrences historically; now resolved.
- mid-run reverts: not observed.
- issue body 10 KB cap: trim Run History to keep #309 updatable.

## Verified knowledge (cumulative)
- **Quoting:** `strconv.Quote` for docker CLI args; `hostshell.PosixSingleQuote` for shell snippets; `hostshell.PowerShellSingleQuote` for PowerShell.
- **Protected:** `go.mod`, `go.sum`, `CHANGELOG.md`, `.github/workflows/ci.yml`, `.github/workflows/bench-compare.yml`.
- **Render:** `renderRow`/`renderHighlightedRow` share `renderRowWith`; `table.RenderPlain` = strings.Builder + appendRowPlain, 1-alloc/cell.
- **autostart:** `ListInstalled(h)` linux/darwin/windows/unsupported dispatch; `parseInstanceLines(out)` preserves order.
- **host.Host.RunShell:** wraps via wrapCommand (windows+non-local base64; else no-op).
- **tui:** `hostMetricsHeaders`, `buildHostMetricsRows`, `hostMetricsColorize` shared by PrintHostMetricsTable + FormatHostMetrics + viewHostMetrics.
- **tui runner-status (1a1fb8c):** `runnerStatusHeaders` + `runnerStatusColorize` in `table.go` next to `runnerStatusCells`. 9 columns: INSTANCE, HOST, REPO, MODE, IMAGE, BUILD, LOCAL, GITHUB, LABELS. Column indices 5/6/7 map to BUILD/LOCAL/GITHUB.
- **FormatBytesHuman (898e101):** 4× fmt.Sprintf → AppendFloat + [16]byte + FormatInt. -40% ns/op, -50% B/op, -40% allocs.
- **tui.formatPercent + formatUsedTotal (e0c9de9):** AppendFloat + [24]byte / [48]byte.
- **tui.extractTrailingPercent (965dc50):** manual byte scan, no string allocations. 252→75 ns/op (-75%), 160→0 B/op, 6→0 allocs.
- **scripts/benchstat/main.go (2c76716):** //go:build ignore, stdlib only. Thresholds: ns/op 10/30%, B/op 15/50%, allocs/op 10/25%.
- **bench-compare.yml (2c76716):** pull_request:[main], downloads main bench-*, posts PR comment via peter-evans/create-or-update-comment@v4 edit-mode:replace. PROTECTED FILE.
- **host.LoadStr (977e1c5, branch only):** AppendFloat + [40]byte. -9.5% ns/op, -33% B/op. Maintainer reverted working tree; PR not opened.
- **editor.Preferred + Open:** Windows notepad default + exec error chain wraps `exec.ErrNotFound` (NOT fs.ErrNotExist).
- **Makefile targets after 7b4e8fa:** all build test bench coverage coverage-html vet fmt tidy ci check clean install uninstall. `make ci` mirrors ci.yml (vet + fmt + test); `make coverage` adds -coverprofile + project total; `make coverage-html` writes annotated HTML.
- **parseFourInt64s (1d71bbf):** manual ASCII scan, [4]string + [4]int64 stack buffers, LastIndexByte('\n'). 90.5→52.8 ns/op (-42%), 64→0 B/op, 1→0 allocs.
- **agentic fanout (#325 → PR #334, merged 2026-07-08):** `containerCheckDefs()` single source of truth. Net −42 lines.
- **agentic fanout test contract:** `agentic_fanout_test.go` snapshots both the assembled command and the spec map.
- **doctor refactor (8c71476 → PR #335, merged 2026-07-08):** `checkRunnerScope` + `printAPIFailures`. GitHub-API block ~40→~17 lines.
- **diskschedule seam (PR #336, merged 2026-07-08):** 7 seam vars + `resetDiskScheduleSeams`. Coverage 14.2% → ~88.2% on orchestration surface.
- **diskschedule probe duplication (#341, open):** two shapes (existence + state), 4 sites. `diskschedule.go:83`+`:162` raw `'%s'`; `autostart.go:107` + `active_check.go:38` `hostshell.PowerShellSingleQuote(name)`. `serviceBase = "ghsr-disk-prune"` safe today only because hard-coded constant.
- **startContainer one-shot (#342, draft PR):** chains marker `rm -f` + `docker update --restart=` + `docker start`. Saves 2 SSH round-trips per instance per `gh sr up`.
- **runner NeedsSetup/RebuildImage tests (#343, draft PR):** 14 new tests. `internal/runner` 57.6%→59.6%.
- **.gitignore after `3019889`:** added `coverage.out`, `coverage.html`, `*.test`, `.vscode/`, `.idea/`, `*.swp`, `*.swo`, `.DS_Store` to existing list.
- **internal/strfmt (d126f1a):** `FmtFloat(dst, v, prec) []byte` one-line wrapper around `strconv.AppendFloat(_, _, 'f', prec, 64)`. Owns the 24-byte upper-bound documentation. Migrated callers: `internal/tui/metrics.go` (formatPercent, formatUsedTotal — 4 call sites), `internal/runner/disk.go` (FormatBytesHuman — 3 switch arms). `scripts/benchstat` cannot import (//go:build ignore, stdlib-only).
- **AllocsPerRun test contract:** `testing.AllocsPerRun` panics with "AllocsPerRun called during parallel test" when combined with `t.Parallel()`. Make alloc-asserting tests sequential.
- **Coverage (latest, project total ≈ 58.0%):** highs `internal/autostart` 94.7%, `internal/ops` 93.6%, `internal/editor` 92.3%, `internal/agentic` 89.6%, `internal/hostshell` 89.7%, **new `internal/strfmt` 100%**. Lows `internal/diskschedule` 14.2%, `internal/tui` 17.4% (was 15.0%, +2.4 pp from PR #338), `internal/runner` 59.6% (was 57.6%, +2.0 pp from PR #343), `internal/host` 59.6%.
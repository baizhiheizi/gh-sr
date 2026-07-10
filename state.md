---
name: repo-assist-state
description: Repo Assist persistent state — in-flight work, backlog, verified knowledge (latest: 2026-07-10 #29102861577)
metadata:
  type: project
---

# Repo Assist state — 2026-07-10 (run 29102861577)

## Last run — Tasks 5, 9, 2
- **Task 5** — `repo-assist/improve-update-filter-dedup-2026-07-10` @ `4fee61e`. `internal/tui/dashboard.go` +17/−28 (net −11). Extracts `updateFilterList(key, choices, setter)` from `updateFilterHost` + `updateFilterRepo`. PR success.
- **Task 9** — `repo-assist/test-extract-trailing-percent-edge-cases-2026-07-10` @ `7364f83`. `internal/tui/metrics_test.go` +61. 16-case `TestExtractTrailingPercent_edgeCases`. `internal/tui` 18.4% → 18.6%. PR success.
- **Task 2** — Fallback (no candidate; all 10 open issues comment-by-Repo-Assist-awaiting-human or agentic-bot bookkeeping).
- **Task 11** — Updated #309. Body under 10 KB.

## Backlog
- **#132** (gh sr storage design) — on hold.
- **#208** parent duplicate-code — sub-issues closed; awaiting #346/#347 review + #341 scope pick.
- **#341** — PowerShell probe dedup, awaiting maintainer pick A/B/C.
- **#309** — current Monthly Activity.
- **Open PRs awaiting review:** #328, #323, #338, #339, #340, #342, #343, #344, #345, plus #346 + #347 from 2026-07-09, plus this run's Task 5 + Task 9 PRs.

## Verified contracts (cumulative)
- **Protected:** `go.mod`, `go.sum`, `CHANGELOG.md`, `.github/workflows/ci.yml`, `.github/workflows/bench-compare.yml`.
- **Quoting:** `strconv.Quote` for docker args; `hostshell.PosixSingleQuote` / `PowerShellSingleQuote` for shell snippets.
- **tui:** `runnerStatusHeaders`+`runnerStatusColorize` (1a1fb8c). `updateFilterList` (4fee61e) shares `esc/j/k/enter` loop between `updateFilterHost`/`updateFilterRepo`. `extractTrailingPercent` (965dc50+7364f83): manual byte scan, zero-alloc; **implicit leading-whitespace tolerance** (" 3.2%" → 3.2, walk-back stops at non-digit/non-dot).
- **Render:** `renderRow`/`renderHighlightedRow` share `renderRowWith`; `table.RenderPlain` = strings.Builder + appendRowPlain, 1-alloc/cell.
- **autostart:** `ListInstalled(h)` linux/darwin/windows/unsupported; `parseInstanceLines(out)` preserves order.
- **host.Host.RunShell:** wraps via wrapCommand (windows+non-local base64; else no-op).
- **FormatBytesHuman (898e101):** 4× fmt.Sprintf → AppendFloat + [16]byte + FormatInt. -40% ns/op, -50% B/op.
- **scripts/benchstat (2c76716):** //go:build ignore, stdlib only. Thresholds ns/op 10/30%, B/op 15/50%, allocs/op 10/25%.
- **bench-compare.yml (2c76716):** pull_request:[main], peter-evans/create-or-update-comment@v4 edit-mode:replace. PROTECTED.
- **host.LoadStr (977e1c5, branch only):** AppendFloat + [40]byte. -9.5% ns/op, -33% B/op. Maintainer reverted; PR not opened.
- **editor.Preferred + Open:** Windows notepad default + exec error chain wraps `exec.ErrNotFound` (NOT fs.ErrNotExist).
- **Makefile (7b4e8fa):** all build test bench coverage coverage-html vet fmt tidy ci check clean install uninstall. `make ci` mirrors ci.yml.
- **parseFourInt64s (1d71bbf):** manual ASCII scan, [4]string+[4]int64 stack buffers. 90.5→52.8 ns/op (-42%), 0 B/op.
- **agentic fanout (#334):** `containerCheckDefs()` single source of truth. Net −42 lines.
- **doctor refactor (#335):** `checkRunnerScope` + `printAPIFailures`. GitHub-API block ~40→~17 lines.
- **diskschedule seam (#336):** 7 seam vars + `resetDiskScheduleSeams`. Coverage 14.2% → ~88.2%.
- **startContainer one-shot (#342):** chains `rm -f` + `docker update --restart=` + `docker start`. Saves 2 SSH round-trips per instance per `gh sr up`.
- **setupContainer/needsSetupContainer one-shot (#350):** `containersPresentOneShot(h, names)` from one docker-ps. Saves N-1 SSH round-trips.
- **runner NeedsSetup/RebuildImage tests (#343):** 14 new tests.
- **.gitignore (`3019889`):** added coverage.out/html, *.test, .vscode/, .idea/, *.swp, *.swo, .DS_Store.
- **internal/strfmt (d126f1a):** `FmtFloat(dst, v, prec) []byte`. Migrated callers: tui (4 sites), runner/disk.go (3 arms). scripts/benchstat cannot import.
- **AllocsPerRun contract:** panics with "AllocsPerRun called during parallel test" when combined with `t.Parallel()`.
- **Coverage (latest ≈ 58% total):** highs autostart 94.7%, ops 93.6%, editor 92.3%, agentic 89.5%, hostshell 89.7%, **strfmt 100%**, testutil 88.2%, table 87.5%, benchstat 88.1%, diskschedule 88.2%, config 83.9%. Lows **tui 18.6%** (+1.2pp across #338/#340/#346/#7364f83), runner 60.8%, **hostshell/ps 60%** (new; Exec/CombinedOutput hard cross-platform), host 65.8%, doctor 69.5%.

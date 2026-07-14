---
name: repo-assist-state
description: Repo Assist persistent state — in-flight work, backlog, verified knowledge (latest: 2026-07-14 #29318676288)
metadata:
  type: project
---

# Repo Assist state — 2026-07-14 (run 29318676288)

## Last run — Tasks 5, 10, 3
- **Task 5** — `repo-assist/improve-benchstat-formatdelta-and-buffer-2026-07-14` @ `73ba243`. `scripts/benchstat/benchstat.go` +23/−14, `benchstat_test.go` −17. Closes #360, partial fix for #359. Removes dead `FormatDelta`, replaces body with `appendDelta` helper, extracts local `appendFloat` analogue of `internal/strfmt.FmtFloat`, normalizes `writeNumber`'s `[32]byte` → `[24]byte`. Net −8 LOC. PR success.
- **Task 10** — `repo-assist/test-doctor-has-container-agentic-runners-2026-07-14` @ `edc1bed`. `internal/doctor/doctor_test.go` +44. `TestHasContainerAgenticRunners` (5 sub-cases) pins the host-agnostic predicate gating `checkContainerAgenticInnerHygiene`. `internal/doctor` 69.5% → **70.7%**. PR success.
- **Task 3** — Fallback (no `bug`/`help-wanted`/`good-first-issue` open).
- **Task 11** — Updated #309. Body under 10 KB.

## Backlog
- **#132** (gh sr storage design) — on hold.
- **#208** parent duplicate-code — sub-issues closed; no scope pick on outstanding items.
- **#309** — current Monthly Activity.
- **Open PRs awaiting review (this run):** benchstat refactor (#360 partial), doctor hasContainerAgenticRunners test.
- **Open PRs from previous runs:** PR #362 (test-improver, draft, `[agentic-threat-detected]` label — needs maintainer decision before any follow-up).
- **Carry-over merged this week:** #355 (LoadStr), #354 (bench-save), #353 (posixRunnerDirVar), #352 (extractTrailingPercent edge cases), #351 (updateFilter dedup), #349 (strfmt shared FmtFloat), #348 (runner-status headers+colorize), #344 (.gitignore).

## Verified contracts (cumulative)
- **Protected:** `go.mod`, `go.sum`, `CHANGELOG.md`, `.github/workflows/ci.yml`, `.github/workflows/bench-compare.yml`.
- **Quoting:** `strconv.Quote` for docker args; `hostshell.PosixSingleQuote` / `PowerShellSingleQuote` for shell snippets.
- **tui:** `runnerStatusHeaders`+`runnerStatusColorize` (1a1fb8c, PR #346). `updateFilterList` (4fee61e, PR #351) shares `esc/j/k/enter` loop between `updateFilterHost`/`updateFilterRepo`. `extractTrailingPercent` (965dc50+7364f83, PR #340+#352): manual byte scan, zero-alloc; **implicit leading-whitespace tolerance** (" 3.2%" → 3.2, walk-back stops at non-digit/non-dot).
- **Render:** `renderRow`/`renderHighlightedRow` share `renderRowWith`; `table.RenderPlain` = strings.Builder + appendRowPlain, 1-alloc/cell.
- **autostart:** `ListInstalled(h)` linux/darwin/windows/unsupported; `parseInstanceLines(out)` preserves order.
- **host.Host.RunShell:** wraps via wrapCommand (windows+non-local base64; else no-op).
- **FormatBytesHuman (898e101):** 4× fmt.Sprintf → AppendFloat + [16]byte + FormatInt. -40% ns/op, -50% B/op.
- **scripts/benchstat (73ba243, this run):** dropped dead `FormatDelta`; `appendDelta(sb, d)` wraps `formatDeltaTo` with a 24-byte stack buffer; local `appendFloat(dst, v, prec)` mirrors `internal/strfmt.FmtFloat`; `writeNumber` normalized to `[24]byte`. Net −8 LOC. `BenchmarkRenderMarkdown` unchanged within noise.
- **scripts/benchstat (2c76716):** //go:build ignore, stdlib only. Thresholds ns/op 10/30%, B/op 15/50%, allocs/op 10/25%.
- **bench-compare.yml (2c76716):** pull_request:[main], peter-evans/create-or-update-comment@v4 edit-mode:replace. PROTECTED.
- **host.LoadStr (b43ab41):** stack `[40]byte` + `strfmt.FmtFloat` + 2 spaces. Median 457→361 ns (-26%), 24→16 B (-33%). Replaces the earlier reverted 977e1c5 attempt.
- **editor.Preferred + Open:** Windows notepad default + exec error chain wraps `exec.ErrNotFound` (NOT fs.ErrNotExist).
- **Makefile (7b4e8fa + 90c5e94):** all build test bench coverage coverage-html vet fmt tidy ci check clean install uninstall bench-save. `make ci` mirrors ci.yml.
- **parseFourInt64s (1d71bbf):** manual ASCII scan, [4]string+[4]int64 stack buffers. 90.5→52.8 ns/op (-42%), 0 B/op.
- **agentic fanout (#334):** `containerCheckDefs()` single source of truth. Net −42 lines.
- **doctor refactor (#335):** `checkRunnerScope` + `printAPIFailures`. GitHub-API block ~40→~17 lines.
- **diskschedule seam (#336):** 7 seam vars + `resetDiskScheduleSeams`. Coverage 14.2% → ~88.2%.
- **startContainer one-shot (#342):** chains `rm -f` + `docker update --restart=` + `docker start`. Saves 2 SSH round-trips per instance per `gh sr up`.
- **setupContainer/needsSetupContainer one-shot (#350):** `containersPresentOneShot(h, names)` from one docker-ps. Saves N-1 SSH round-trips.
- **runner NeedsSetup/RebuildImage tests (#343):** 14 new tests.
- **.gitignore (`3019889`, PR #344):** added coverage.out/html, *.test, .vscode/, .idea/, *.swp, *.swo, .DS_Store.
- **internal/strfmt (d126f1a, PR #347+#349):** `FmtFloat(dst, v, prec) []byte`. Migrated callers: tui (4 sites), runner/disk.go (3 arms), **now also host.LoadStr** (b43ab41). scripts/benchstat cannot import — has local `appendFloat` (this run).
- **posixRunnerDirVar (98e085c, PR #353):** shared `posixInstanceEscaper` + sized strings.Builder. 836→33 ns (-96%), 6887→64 B (-99%), 10→1 allocs.
- **hasContainerAgenticRunners (edc1bed, this run):** host-agnostic predicate; returns true iff any runner has `Profile: agentic` (which transitively implies `IsContainerMode() == true`). Documented contract: caller `installTargetsForHost` does the per-host filter. 0% → 100% coverage.
- **AllocsPerRun contract:** panics with "AllocsPerRun called during parallel test" when combined with `t.Parallel()`.
- **Coverage (latest ≈ 58% total):** highs autostart 94.7%, ops 93.6%, editor 92.3%, agentic 89.5%, hostshell 89.7%, **strfmt 100%**, testutil 88.2%, table 87.5%, benchstat 88.1%, diskschedule 88.2%, config 83.9%, **doctor 70.7%** (this run, +1.2pp). Lows **tui 18.6%**, runner 60.8%, **hostshell/ps 60%** (Exec/CombinedOutput hard cross-platform), host 65.8%.
---
name: repo-assist-state
description: Repo Assist persistent state — in-flight work, backlog, verified knowledge (latest: 2026-07-15 #29402596947)
metadata:
  type: project
---

# Repo Assist state — 2026-07-15 (run 29402596947)

## Last run — Tasks 3, 2, 4
- **Task 3** — Fallback to Task 2: 0 open issues labelled `bug`, `help wanted`, or `good first issue`.
- **Task 2** — Commented on #359/#360 (PR #363 disposition) and #369/#370 (one-round-trip-safe consolidation boundary). Cursor advanced through #370.
- **Task 4** — Fallback to Task 5: dependency/CI/runtime/build audit found no distinct low-risk investment. Implemented #370 on `repo-assist/improve-linux-instance-probe-2026-07-15` @ `46ab07d`; draft PR temporary ID `#aw_probe370` accepted for creation.
- **Task 11** — Rebuilt #309 in the required format with all 8 live PRs, all unacknowledged Repo Assist comments, #305 close action, #373 decision, and this run.

## Tracking
- **Issue-comment cursor:** #370; next scan should consider #373, then reset to oldest.
- **Comments made:** #132 (2026-06-09), #208 (2026-07-08), #359/#360/#369/#370 (2026-07-15), #368 (2026-07-15 previous run). Re-engage only after new human activity.
- **Fix attempts:** #370 → branch `repo-assist/improve-linux-instance-probe-2026-07-15`, commit `46ab07d`, PR temp `#aw_probe370`; closes #370, partial #369.
- **Maintainer checkbox state:** no checked items observed in #309 this run.

## Backlog
- **#132** (`gh sr storage`) — on hold pending loop-mount persistence choice.
- **#208** parent duplicate-code — no active child scope requested.
- **#369** cross-package systemd probe duplication — defer unless a pure shell-fragment/parser API preserves one SSH round-trip.
- **#373** protected `.git-blame-ignore-revs`/README patch — awaiting human decision/application.
- **#309** — Monthly Activity.
- **Open Repo Assist PRs:** #363, #364, #366, #367, #372, plus `#aw_probe370` from this run.
- **Other open PRs needing review:** #362, #365, #371.
- **Carry-over merged:** #355/#354/#353/#352/#351/#349/#348/#344.

## Verified contracts (abridged)
- **Protected:** `go.mod`, `go.sum`, `CHANGELOG.md`, `.github/workflows/{ci,bench-compare}.yml`.
- **Quoting:** `strconv.Quote` for docker args; `hostshell.PosixSingleQuote`/`PowerShellSingleQuote` for shell snippets.
- **tui:** `runnerStatusHeaders`+`runnerStatusColorize` (PR #346). `updateFilterList` (PR #351) shares `esc/j/k/enter` loop. `extractTrailingPercent` (PR #340+#352): manual byte scan, zero-alloc, **implicit leading-whitespace tolerance**.
- **Render:** `renderRow`/`renderHighlightedRow` share `renderRowWith`; `table.RenderPlain` = strings.Builder + appendRowPlain, 1-alloc/cell.
- **autostart:** `ListInstalled(h)` linux/darwin/windows/unsupported; `parseInstanceLines(out)` preserves order.
- **host.Host.RunShell:** wraps via wrapCommand (windows+non-local base64; else no-op).
- **FormatBytesHuman (898e101):** 4× fmt.Sprintf → AppendFloat + [16]byte + FormatInt. -40% ns/op, -50% B/op.
- **scripts/benchstat (73ba243):** dropped dead `FormatDelta`; `appendDelta(sb, d)` wraps `formatDeltaTo` with 24-byte stack buffer; local `appendFloat(dst, v, prec)` mirrors `internal/strfmt.FmtFloat`; `writeNumber` normalized to `[24]byte`. Net −8 LOC. Benchstat is `//go:build ignore`, stdlib only (cannot import internal/).
- **scripts/benchstat (2c76716):** //go:build ignore, stdlib only. Thresholds ns/op 10/30%, B/op 15/50%, allocs/op 10/25%.
- **bench-compare.yml (2c76716):** pull_request:[main], peter-evans/create-or-update-comment@v4 edit-mode:replace.
- **host.LoadStr (b43ab41):** stack `[40]byte` + `strfmt.FmtFloat` + 2 spaces. Median 457→361 ns (-26%), 24→16 B (-33%).
- **editor.Preferred + Open:** Windows notepad default + exec error chain wraps `exec.ErrNotFound` (NOT fs.ErrNotExist).
- **Makefile (7b4e8fa + 90c5e94):** build test bench coverage coverage-html vet fmt tidy ci check clean install uninstall bench-save.
- **parseFourInt64s (1d71bbf):** manual ASCII scan, [4]string+[4]int64 stack buffers. 90.5→52.8 ns/op (-42%), 0 B/op.
- **agentic fanout (#334):** `containerCheckDefs()` single source of truth. Net −42 lines.
- **doctor refactor (#335):** `checkRunnerScope` + `printAPIFailures`. GitHub-API block ~40→~17 lines.
- **diskschedule seam (#336):** 7 seam vars + `resetDiskScheduleSeams`. Coverage 14.2% → ~88.2%.
- **startContainer one-shot (#342):** chains `rm -f` + `docker update --restart=` + `docker start`. Saves 2 SSH round-trips per instance per `gh sr up`.
- **setupContainer/needsSetupContainer one-shot (#350):** `containersPresentOneShot(h, names)` from one docker-ps. Saves N-1 SSH round-trips.
- **runner NeedsSetup/RebuildImage tests (#343):** 14 new tests.
- **.gitignore (`3019889`, PR #344):** coverage.out/html, *.test, .vscode/, .idea/, *.swp, *.swo, .DS_Store.
- **internal/strfmt (d126f1a, PR #347+#349):** `FmtFloat(dst, v, prec) []byte`. Migrated callers: tui (4 sites), runner/disk.go (3 arms), **host.LoadStr** (b43ab41).
- **posixRunnerDirVar (98e085c, PR #353):** shared `posixInstanceEscaper` + sized strings.Builder. 836→33 ns (-96%), 6887→64 B (-99%), 10→1 allocs.
- **hasContainerAgenticRunners (edc1bed):** host-agnostic predicate; `Profile: agentic` transitively implies `IsContainerMode() == true`. Caller `installTargetsForHost` does per-host filter. 0% → 100%.
- **checkNative + checkLinuxSudo (1713ef4, this run):** OS-dispatch + linux-sudo probes. 16.7%/64.3% → 100%/100%.
- **ensureDoctorHostOS + checkNativeRunnerInstall (88eec32):** per-host OS detection short-circuit + per-instance native install probe. 50.0%/71.4% → 100%/100%.
- **linuxInstanceProbe (46ab07d, this run):** shared one-SSH Linux S/U/Y/D probe for `linuxSvcAndAutostartProbe` + `orphanLinuxPlanProbe`; `includeDir` controls D only; parser uses `TrimSpace`; runner `$HOME` must remain expandable (instance sanitized). Closes #370, partial #369.
- **AllocsPerRun contract:** panics with "AllocsPerRun called during parallel test" when combined with `t.Parallel()`.
- **Coverage (latest ≈ 60% total):** highs autostart 94.7%, ops 93.6%, editor 92.3%, agentic 89.5%, hostshell 89.7%, **strfmt 100%**, testutil 88.2%, table 87.5%, benchstat 88.1%, diskschedule 88.2%, config 83.9%, **doctor 73.9%** (this run combined +4.4pp). Lows **tui 18.9%**, runner 62.8%, **hostshell/ps 60%** (Exec/CombinedOutput hard cross-platform), host 65.8%.
---
name: repo-assist-state
description: Repo Assist persistent state — in-flight work, backlog, verified knowledge (latest: 2026-07-14 #29340778400)
metadata:
  type: project
---

# Repo Assist state — 2026-07-14 (run 29340778400)

## Last run — Tasks 9, 10, 3
- **Task 9** — `repo-assist/test-doctor-check-native-and-sudo-2026-07-14` @ `1713ef4`. +292. 12 tests for `checkNative` (8) + `checkLinuxSudo` (4). 16.7%/64.3% → 100%/100%. PR success.
- **Task 10** — `repo-assist/test-doctor-host-os-and-native-install-2026-07-14` @ `88eec32`. +199. 8 tests for `ensureDoctorHostOS` (3) + `checkNativeRunnerInstall` (5). 50.0%/71.4% → 100%/100%. PR success.
- **Task 3** — Fallback.
- **Task 11** — Updated #309.

## Backlog
- **#132** (gh sr storage) — on hold.
- **#208** parent duplicate-code — no scope pick.
- **#309** — Monthly Activity.
- **Open PRs:** this run's two doctor-test PRs; PR #362 (test-improver draft), #364 (hasContainerAgenticRunners), #363 (benchstat FormatDelta).
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
- **ensureDoctorHostOS + checkNativeRunnerInstall (88eec32, this run):** per-host OS detection short-circuit + per-instance native install probe. 50.0%/71.4% → 100%/100%.
- **AllocsPerRun contract:** panics with "AllocsPerRun called during parallel test" when combined with `t.Parallel()`.
- **Coverage (latest ≈ 60% total):** highs autostart 94.7%, ops 93.6%, editor 92.3%, agentic 89.5%, hostshell 89.7%, **strfmt 100%**, testutil 88.2%, table 87.5%, benchstat 88.1%, diskschedule 88.2%, config 83.9%, **doctor 73.9%** (this run combined +4.4pp). Lows **tui 18.9%**, runner 62.8%, **hostshell/ps 60%** (Exec/CombinedOutput hard cross-platform), host 65.8%.
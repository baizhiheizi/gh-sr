---
name: repo-assist-state
description: Repo Assist persistent state — in-flight work, backlog, verified knowledge (latest: 2026-07-26 #30218669368)
metadata:
  type: project
---

# Repo Assist state — 2026-07-26 (run 30218669368)

## Last run — Tasks 4, 9, 10

- **Task 9** — `internal/tui/dashboard_test.go` +301 committed as `1a192ed` on `repo-assist/test-tui-dashboard-helpers-2026-07-26`. **`create_pull_request` reported success (PR #391)**. Coverage `internal/tui` 18.9% → 23.8%.
- **Task 4** — `make ci`/`ci.yml` `go mod tidy` drift gate prototyped, verified locally (14/14 tests pass, `make ci` passes), then **reverted in the working tree before any commit** (no PR opened). Matches the established pre-push revert pattern from PRs #363/#365 — speculative CI gates with no concrete drift event are low signal. Two local commits and the Makefile change were dropped; branch deleted.
- **Task 10** — surveyed the 10 open issues; no actionable next step beyond Task 9. #132/#373 are on hold behind maintainer decisions; #148/#214 are AW-managed; #305/#306/#309/#318/#324 are sibling-automation monthly summaries; #384 sub-issues are 3/3 complete.
- **Task 11** — Updated #309 to record PR #391, the pre-push revert on the tidy-check prototype, and the no-CI-gate-without-drift-event signal.

## Maintainer feedback budget (close calls for the rest of July)

- **PR #363 (closed 2026-07-15 10:08:55):** dead-`FormatDelta` removal is fine; reject any companion helper around `strconv.AppendFloat`.
- **PR #365 (closed 2026-07-15 10:08:51):** micro-optimization theater; tightened bar on benchstat/tui micro-optimizations.
- **2026-07-26 (this run, no PR):** speculative `go mod tidy` CI gate reverted pre-push. Interpretation: don't add CI gates that produce zero diff in a clean repo; drive Task 4 from a concrete drift event instead.

## Tracking

- **Issue-comment cursor:** still at #370 (last engaged in #29402596947). Next scan: #373 first, then reset to oldest.
- **Comments made:** #132 (2026-06-09), #208 (2026-07-08), #359/#360/#369/#370 (2026-07-15), #368 (2026-07-15 prior run). Re-engage only after new human activity.
- **Local branches preserved (not pushed):** none.
- **Open Repo Assist PRs:** #391 (tui dashboard helpers coverage, this run). PR #363 and PR #365 closed without merging earlier in July.

## Backlog

- **#132** storage — on hold pending loop-mount persistence choice.
- **#208** duplicate-code — no active child scope requested.
- **#369** cross-package systemd probe duplication — defer unless pure shell-fragment/parser API preserves one SSH round-trip.
- **#373** protected `.git-blame-ignore-revs`/README patch — awaiting human decision/application.
- **#309** Monthly Activity.
- **Carry-over merged:** #390/#389/#388/#387/#382/#381/#380/#379/#375/#374/#372/#371/#367/#366/#364/#363/#362/#361/#358/#357/#355/#354/#353/#352/#351/#350/#349/#348/#345/#344/#343/#342/#340/#339/#338/#336/#335/#334.

## Verified contracts (abridged, recent)

- **Protected:** `go.mod`, `go.sum`, `CHANGELOG.md`, `.github/workflows/{ci,bench-compare}.yml`.
- **Quoting:** `strconv.Quote` for docker args; `hostshell.PosixSingleQuote`/`PowerShellSingleQuote` for shell snippets.
- **scripts/benchstat (73ba243 / 8da99f8):** dead `FormatDelta` removed; `appendDelta`/`appendFloat` 24-byte stack buffer; benchstat is `//go:build ignore`, stdlib only.
- **host.LoadStr (b43ab41):** stack `[40]byte` + `strfmt.FmtFloat` + 2 spaces. Median 457→361 ns (-26%), 24→16 B (-33%).
- **posixRunnerDirVar (98e085c, PR #353):** 836→33 ns, 6887→64 B, 10→1 allocs.
- **setupContainer/needsSetupContainer one-shot (#350):** `containersPresentOneShot(h, names)` from one docker-ps. Saves N-1 SSH round-trips.
- **linuxInstanceProbe (46ab07d, #374):** shared one-SSH Linux S/U/Y/D probe; `includeDir` controls D; `TrimSpace` parser; runner `$HOME` must remain expandable.
- **AllocsPerRun contract:** panics with "AllocsPerRun called during parallel test" when combined with `t.Parallel()`.
- **sortedRepoNames edge case:** config validation already rejects a runner with neither Org nor Repo (config.go:557), so the empty-string contribution to the picker is unreachable at runtime. Test pins the contract.
- **Coverage:** highs autostart 94.7%, ops 93.6%, strfmt 100%, doctor 77.7%, tui 23.8% (after #391); lows tui 23.8%, hostshell/ps 60% (Exec/CombinedOutput hard cross-platform), host 66.2%, runner 71.6%.

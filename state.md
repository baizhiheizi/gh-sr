---
name: repo-assist-state
description: Repo Assist persistent state — in-flight work, backlog, verified knowledge (latest: 2026-07-27 #30303841146)
metadata:
  type: project
---

# Repo Assist state — 2026-07-27 (run 30303841146)

## Last run — Tasks 2, 4, 3

- **Task 2** — commented on #384: detector workflow expired, 3/3 sub-issues closed (`not_planned`), suggested close. Flag-construction dedupe from #385 already tracked inline in `internal/runner/native.go`.
- **Task 4** — no actionable candidate. Surveyed native.go/container.go for residual strictly-N→1 SSH folds; remaining `h.Run` calls are already chained, on divergent paths, or on cold paths. No-CI-gate-without-drift-event + no-micro-optimization signals upheld.
- **Task 3** — no fixable issues. None of the 10 open issues carry `bug`/`help wanted`/`good first issue` labels. Task 3 falls back to Task 2.
- **Task 11** — Updated #309: added new run entry, refreshed Suggested Actions (removed PR #391 closed 2026-07-26; #208 closed completed 2026-07-22; #368/#369/#370 closed not_planned; added new comment on #384).

## Previous run (2026-07-26) — Tasks 4, 9, 10

- **Task 9** — `internal/tui/dashboard_test.go` +301 at `1a192ed` on `repo-assist/test-tui-dashboard-helpers-2026-07-26`. PR #391 (closed 2026-07-26 22:47:54; superseded by maintainer-side test coverage on the same surface). Coverage `internal/tui` 18.9% → 23.8%.
- **Task 4** — `make ci`/`ci.yml` `go mod tidy` drift gate prototyped, verified locally, **reverted in working tree before any commit** (no PR). Matches the pre-push revert pattern from PRs #363/#365. Branch deleted.
- **Task 10** — surveyed 10 open issues; no actionable next step beyond Task 9. Monthly Activity issues (#305/#306/#309/#318/#324) are sibling-automation. #132/#373 on hold behind maintainer decisions.
- **Task 11** — Updated #309 to record PR #391, the pre-push revert, and the no-CI-gate-without-drift-event signal.

## Maintainer close signals

- **PR #363 (closed 2026-07-15 10:08:55):** dead-`FormatDelta` removal fine; reject companion `appendFloat` wrapper around `strconv.AppendFloat`.
- **PR #365 (closed 2026-07-15 10:08:51):** micro-optimization theater; tightened bar on benchstat/tui micro-optimizations.
- **2026-07-26:** speculative `go mod tidy` CI gate reverted pre-push. Drive Task 4 from a concrete drift event, not a clean-tidy guarantee.

## Tracking

- **Issue-comment cursor:** #384 (this run). Next scan: #373 (top of open list, repo-assist's own parked issue).
- **Comments made:** #132 (2026-06-09), #208 (2026-07-08), #359/#360/#369/#370 (2026-07-15), #368 (2026-07-15 prior run), #384 (2026-07-27). Re-engage only after new human activity.
- **Open Repo Assist PRs:** none. PR #391 and #363/#365 all closed.

## Backlog

- **#132** storage — on hold pending loop-mount persistence choice.
- **#208** closed completed 2026-07-22.
- **#368/#369/#370** closed not_planned 2026-07-15; follow-up handled by PRs #372/#374/#379.
- **#373** protected `.git-blame-ignore-revs`/README patch — awaiting human decision.
- **#384** duplicate-code group parent — workflow expired, 3/3 sub-issues closed; suggested close in 2026-07-27 comment.
- **#309** Monthly Activity.
- **Carry-over merged:** #390/#389/#388/#387/#382/#381/#380/#379/#375/#374/#372/#371/#367/#366/#364/#363/#362/#361/#358/#357/#355/#354/#353/#352/#351/#350/#349/#348/#345/#344/#343/#342/#340/#339/#338/#336/#335/#334.

## Verified contracts

- **Protected:** `go.mod`, `go.sum`, `CHANGELOG.md`, `.github/workflows/{ci,bench-compare}.yml`.
- **Quoting:** `strconv.Quote` for docker args; `hostshell.PosixSingleQuote`/`PowerShellSingleQuote` for shell snippets.
- **scripts/benchstat (73ba243/8da99f8):** dead `FormatDelta` removed; `appendDelta`/`appendFloat` 24-byte stack buffer; benchstat is `//go:build ignore`, stdlib only.
- **host.LoadStr (b43ab41):** stack `[40]byte` + `strfmt.FmtFloat` + 2 spaces. Median 457→361 ns (-26%), 24→16 B (-33%).
- **posixRunnerDirVar (98e085c, #353):** 836→33 ns, 6887→64 B, 10→1 allocs.
- **setupContainer/needsSetupContainer one-shot (#350):** `containersPresentOneShot(h, names)` from one docker-ps. Saves N-1 SSH round-trips.
- **linuxInstanceProbe (46ab07d, #374):** shared one-SSH Linux S/U/Y/D probe; `includeDir` controls D; `TrimSpace` parser; runner `$HOME` must remain expandable.
- **AllocsPerRun contract:** panics with "AllocsPerRun called during parallel test" when combined with `t.Parallel()`.
- **sortedRepoNames edge case:** config validation already rejects a runner with neither Org nor Repo (config.go:557), so the empty-string contribution to the picker is unreachable at runtime. Test pins the contract.
- **Coverage:** highs autostart 94.7%, ops 93.6%, strfmt 100%, doctor 77.7%, tui 23.8% (after #391); lows tui 23.8%, hostshell/ps 60% (Exec/CombinedOutput hard cross-platform), host 66.2%, runner 71.6%.

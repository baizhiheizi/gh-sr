---
name: repo-assist-state
description: Repo Assist persistent state — in-flight work, backlog, verified knowledge (latest: 2026-07-29 #30487911560)
metadata:
  type: project
---

# Repo Assist state — 2026-07-29 (run 30487911560)

## Last run — Tasks 2, 1, 5

- **Task 1** — fell back to Task 2: all open issues already labelled (per pre-step).
- **Task 2** — no new comment: no human activity after #132/#384 comments; #148/#214 are automation logs, #373 parked, sibling monthly summaries skipped, fresh duplicate-code sub-issues #392/#393/#394 under #384 will expire Jul 30 and are noise not value.
- **Task 5** — three candidates surfaced (svc.sh helper #392 x5 sites, windowsRunnerScript #393 x11 sites, Severity enum #394 x5 sites). Deferred: none reduces round-trips, and maintainer close signal on PR #365 prioritizes user-visible round-trip reduction over pure code dedup.
- **Task 11** — #309 rewritten in exact required format and updated with this run.

## Maintainer close signals

- PR #363: dead `FormatDelta` removal fine; reject wrapper indirection around `strconv.AppendFloat`.
- PR #365: reject allocation-focused micro-optimization theater; prioritize user-visible round-trip reduction.
- 2026-07-26: speculative `go mod tidy` CI gate reverted before commit; require a concrete drift event.

## Tracking

- **Issue-comment cursor:** #373. Resume oldest-open scan from #373, then #384 and wrap to #132.
- **Comments made:** #132 (2026-06-09), #208 (2026-07-08, now closed), #359/#360/#368/#369/#370 (2026-07-15, now closed), #384 (2026-07-27). Re-engage only after new human activity.
- **Open Repo Assist PRs:** none.

## Backlog

- **#132** storage — on hold pending loop-mount persistence choice.
- **#305** duplicate Test Improver monthly summary; maintainer action to close remains in #309.
- **#373** protected `.git-blame-ignore-revs`/README patch — awaiting human decision.
- **#384** detector expired; reopened with 3 new sub-issues #392/#393/#394 (expires Jul 30); suggested close on 2026-07-27 stands.
- **#392/#393/#394** auto-generated duplicate-code sub-issues, expire Jul 30 2026 — not worth engagement; if any candidate graduates to a measurable improvement, file a fresh issue rather than reopening these.
- **Performance candidate:** batch disk usage listing/measurement per host only after maintainer approval; benchmark or assert remote-call reduction.
- **#309** Monthly Activity.

## Verified contracts

- **Protected:** `go.mod`, `go.sum`, `CHANGELOG.md`, `.github/workflows/{ci,bench-compare}.yml`.
- **Quoting:** `strconv.Quote` for docker args; `hostshell.PosixSingleQuote`/`PowerShellSingleQuote` for shell snippets.
- **Round-trip bar:** favor strict N→1 production remote-call folds with tests; reject allocation-only edits without user-visible impact.
- **AllocsPerRun contract:** panics with "AllocsPerRun called during parallel test" when combined with `t.Parallel()`.
- **Coverage:** tui 23.8%, hostshell/ps 60%, host 66.2%, runner 71.6% as last measured.
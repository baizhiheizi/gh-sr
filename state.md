---
name: repo-assist-state
description: Repo Assist persistent state — in-flight work, backlog, verified knowledge (latest: 2026-07-30 #30579864305)
metadata:
  type: project
---

# Repo Assist state — 2026-07-30 (run 30579864305)

## Last run — Tasks 2, 10, 5

- **Task 2** — no new comment: no human activity after #132/#384 comments; #148/#214 are automation logs, #373 parked, sibling monthly summaries skipped, duplicate-code sub-issues #392/#393/#394 auto-closed today (Jul 30 10:27 PM UTC) — engagement would have been noise.
- **Task 10** — re-verified the disk-usage N+1 SSH batching candidate at `internal/ops/disk.go:102-144`. Aligns with maintainer's PR #365 close signal (user-visible round-trip reduction). Memory guard requires maintainer approval before opening a PR; kept as pending **Define goal** item in #309.
- **Task 5** — three duplicate-code refactor candidates (#392 svc.sh, #393 windowsRunnerScript, #394 Severity) re-reviewed; remain deferred (DRY-only, no measurable user-visible benefit). No new low-risk improvement identified this run.
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
- **#384** detector expired; sub-issues #392/#393/#394 auto-closed on Jul 30 2026; parent ready to close per #309.
- **Performance candidate:** batch disk usage listing/measurement per host only after maintainer approval; benchmark or assert remote-call reduction. File a fresh issue (rather than reusing #384's sub-issues) once maintainer signals appetite.
- **#309** Monthly Activity.

## Verified contracts

- **Protected:** `go.mod`, `go.sum`, `CHANGELOG.md`, `.github/workflows/{ci,bench-compare}.yml`.
- **Quoting:** `strconv.Quote` for docker args; `hostshell.PosixSingleQuote`/`PowerShellSingleQuote` for shell snippets.
- **Round-trip bar:** favor strict N→1 production remote-call folds with tests; reject allocation-only edits without user-visible impact.
- **AllocsPerRun contract:** panics with "AllocsPerRun called during parallel test" when combined with `t.Parallel()`.
- **Coverage:** tui 23.8%, hostshell/ps 60%, host 66.2%, runner 71.6% as last measured.
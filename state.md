---
name: repo-assist-state
description: Repo Assist persistent state — in-flight work, backlog, verified knowledge (latest: 2026-08-01 #30716573580)
metadata:
  type: project
---

# Repo Assist state — 2026-08-01 (run 30716573580)

## Last run — Tasks 9, 3, 2

- **Task 9** — added seven focused/table-driven tests for `doctor.checkContainerRunnerInstall` in `internal/doctor/doctor_test.go`. Covered no-target, bootstrap-failed, missing/error/stopped container, failed inner dockerd/registration, healthy output, severity counters, remediation text, and terminal-probe short-circuiting. Coverage: function 0% → 100%; `internal/doctor` 77.7% → 85.3%.
- **Task 3** — fell back to Task 2: no open issue carried `bug`, `help wanted`, or `good first issue`; no confident issue fix was available.
- **Task 2** — scanned #373, #384, #132, and the remaining automation/monthly issues. No new human activity followed Repo Assist's comments; no duplicate comment posted.
- **Task 11** — safeoutputs accepted creation of August summary `#aw_aug26` and closure of July summary #309. August summary forward-references draft PR `#aw_doc_tst`.

## In-flight output

- Branch `repo-assist/test-doctor-container-install-2026-08-01`, commit `78c16b5`.
- Draft PR transaction is to use temporary ID `#aw_doc_tst`; verify the resulting PR number and state next run before doing follow-up work.
- Verification passed: focused doctor tests, `go build ./...`, `go vet ./...`, full gofmt check, `go test -race ./...`, and `git diff --check`.

## Maintainer close signals

- PR #363: dead `FormatDelta` removal fine; reject wrapper indirection around `strconv.AppendFloat`.
- PR #365: reject allocation-focused micro-optimization theater; prioritize user-visible round-trip reduction.
- 2026-07-26: speculative `go mod tidy` CI gate reverted before commit; require a concrete drift event.

## Tracking

- **Issue-comment cursor:** #148. The 2026-08-01 scan reached the end (#373/#384), wrapped through #132, and found no new human activity.
- **Comments made:** #132 (2026-06-09), #208 (2026-07-08, closed), #359/#360/#368/#369/#370 (2026-07-15, closed), #384 (2026-07-27). Re-engage only after new human activity.
- **Open Repo Assist PRs:** safeoutput transaction `#aw_doc_tst` pending application; verify live state next run.

## Backlog

- **#132** storage — on hold pending loop-mount persistence choice.
- **#305** duplicate Test Improver monthly summary; maintainer action to close remains in August summary.
- **#373** protected `.git-blame-ignore-revs`/README patch — awaiting human decision.
- **#384** detector expired and all six sub-issues (#383/#385/#386/#392/#393/#394) are closed; parent ready to close.
- **Performance candidate:** batch disk usage listing/measurement per host only after maintainer approval; benchmark or assert remote-call reduction. File a fresh issue once maintainer signals appetite.
- **Testing candidate:** `doctor.checkContainerAgenticInnerHygiene` remains 0% covered; act only if tests protect meaningful user-facing diagnostics without brittle probe over-mocking.
- **Monthly Activity:** August summary transaction `#aw_aug26`; July #309 closure accepted by safeoutputs.

## Verified contracts

- **Protected:** `go.mod`, `go.sum`, `CHANGELOG.md`, `.github/workflows/{ci,bench-compare}.yml`.
- **Quoting:** `strconv.Quote` for docker args; `hostshell.PosixSingleQuote`/`PowerShellSingleQuote` for shell snippets.
- **Round-trip bar:** favor strict N→1 production remote-call folds with tests; reject allocation-only edits without user-visible impact.
- **AllocsPerRun contract:** panics with "AllocsPerRun called during parallel test" when combined with `t.Parallel()`.
- **Coverage after 2026-08-01 tests:** doctor 85.3%, tui 23.8%, hostshell/ps 60%, host 66.2%, runner 71.4%.

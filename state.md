---
name: repo-assist-state
description: Repo Assist persistent state — in-flight work, backlog, verified knowledge (latest: 2026-08-07 #31135682079)
metadata:
  type: project
---

# Repo Assist state — 2026-08-07 (run 31135682079)

## Last run — Tasks 3, 10, 5

- **Task 3** — no open issue carried `bug`, `help wanted`, or `good first issue`. No confident fix available; recorded fallback. The remaining open issues are design proposals (#132), parked automation (#373), and an expired detector group (#384).
- **Task 10** — folded the N+1 SSH pattern in `gh sr disk usage` and `gh sr doctor` into a single batched script per host. New `runner.MeasureDiskUsageBatch` + `runner.dirSizesBatch` (POSIX + Windows). `internal/ops/disk.go:CollectDiskUsage` and `internal/doctor/doctor.go:checkRunnerDiskUsage` refactored to use the batched API. Per-host SSH round-trips collapse from 1 + N to 2.
- **Task 5** — three focused tests for `pruneInnerDockerCache` (probe-down, happy path, probe-error). Function coverage 0% → 100%; runner 73.4% → 73.7%.
- **Task 11** — safeoutputs accepted the August summary update at #396 and the draft PR transaction `#aw_diskperf`.

## In-flight output

- Branch `repo-assist/perf-disk-usage-batch-ssh-2026-08-07`.
- Commits `d2e92c6` (perf) + `e6eae34` (test).
- Draft PR transaction is to use temporary ID `#aw_diskperf`; verify the resulting PR number next run before doing follow-up work.
- Verification passed: `go build ./...`, `go vet ./...`, full `gofmt -l .`, `go test -race ./...`, and `git diff --check`.

## Maintainer close signals

- PR #363: dead `FormatDelta` removal fine; reject wrapper indirection around `strconv.AppendFloat`.
- PR #365: reject allocation-focused micro-optimization theater; prioritize user-visible round-trip reduction.
- 2026-07-26: speculative `go mod tidy` CI gate reverted before commit; require a concrete drift event.
- 2026-08-07: merged PR family (#358/#361/#372/#379/#380) confirms appetite for SSH round-trip folds. The disk-usage N+1 candidate was the next in the same family.

## Tracking

- **Issue-comment cursor:** #148. Last comment posted before the cursor: #384 (2026-07-27). No new human activity after prior Repo Assist comments on any open issue.
- **Comments made:** #132 (2026-06-09), #208 (2026-07-08, closed), #359/#360/#368/#369/#370 (2026-07-15, closed), #384 (2026-07-27). Re-engage only after new human activity.
- **Open Repo Assist PRs:** safeoutput transactions `#aw_doc_tst` (prior run, merged as #397) and `#aw_hygtest` (merged as #398); new transaction `#aw_diskperf` pending application — verify live state next run.

## Backlog

- **#132** storage — on hold pending loop-mount persistence choice.
- **#305** duplicate Test Improver monthly summary; maintainer action to close remains in August summary.
- **#373** protected `.git-blame-ignore-revs`/README patch — awaiting human decision.
- **#384** detector expired and all six sub-issues (#383/#385/#386/#392/#393/#394) are closed; parent ready to close.
- **Performance candidate (next):** the per-instance `disk prune` mutation calls still pay one SSH per prune action on container-mode runners; the inner-cache prune probe + work/temp clear could fold into one batched script. File a fresh issue once maintainer signals appetite, or push a follow-up to #aw_diskperf if the pattern is small enough.
- **Testing candidate:** `pruneInnerDockerCache` now 100%; remaining low-coverage runner entry points (container/native lifecycle) are on Test Improver's established backlog.
- **Monthly Activity:** August summary issue #396 updated with this run's history; forward-references draft PR `#aw_diskperf`.

## Verified contracts

- **Protected:** `go.mod`, `go.sum`, `CHANGELOG.md`, `.github/workflows/{ci,bench-compare}.yml`.
- **Quoting:** `strconv.Quote` for docker args; `hostshell.PosixSingleQuote`/`PowerShellSingleQuote` for shell snippets.
- **Round-trip bar:** favor strict N→1 production remote-call folds with tests; reject allocation-only edits without user-visible impact.
- **AllocsPerRun contract:** panics with "AllocsPerRun called during parallel test" when combined with `t.Parallel()`.
- **Batch disk-usage contract:** `MeasureDiskUsageBatch` returns one entry per input instance; unsafe names short-circuit with Err (no SSH round-trip); host error propagates to every safe entry; missing instance in host output is an explicit error (no silent drop).
- **Coverage after 2026-08-07 tests:** runner 73.4% → 73.7%, doctor 85.3% → 90.9% (was already 90.9, no change this run), tui 23.8%, hostshell/ps 60%, host 66.2%, ops 93.7%.

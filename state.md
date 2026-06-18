---
name: repo-assist-state
description: Repo Assist persistent state — completed work, in-flight PRs, backlog cursor, and notes
metadata:
  type: project
---

# Repo Assist state — last updated 2026-06-18

## Last run (2026-06-18 ~18:30 UTC, run 27772439940)

- Selected tasks: 5 (Coding Improvements), 3 (Issue Fix), 4 (Engineering Investments).
- **Task 5 (Coding Improvements)** — Archived the completed `resolve-duplicate-code` openspec change. Branch `repo-assist/archive-resolve-duplicate-code-2026-06-18`, commit `e3dbf82`. Moved `openspec/changes/resolve-duplicate-code/` → `openspec/changes/archive/2026-06-18-resolve-duplicate-code/` (matches the `2026-06-14-org-runners-support` archive pattern: preserves `.openspec.yaml`, `proposal.md`, `design.md`, `tasks.md`, and `specs/`). Synced the 4 delta specs (`shared-shell-helpers`, `tui-table-rendering`, `test-mock-executor`, `runner-mode-display`) into `openspec/specs/` so the implemented capabilities appear alongside the active `org-runner-*` specs. All four implementation phases are complete and the referenced GitHub issues (#143, #144, #145, #152, #154) plus meta issue #93 are closed by `an-lee` on 2026-06-12. Patch at `/tmp/gh-aw/aw-repo-assist-archive-resolve-duplicate-code-2026-06-18.{patch,bundle}` (~14 KB / ~5 KB). 12 files changed (8 renames + 4 spec adds). Build / vet / `go test ./...` clean (12/12 packages pass). `create_pull_request` returned success.
- **Task 3 (Issue Fix)** — No `bug` / `help wanted` / `good first issue` issues open → fallback to Task 2. Nothing new to comment on (already commented on #209 and #210 last run; both have subsequent human/bot activity but no new human ask). Skipped per anti-spam.
- **Task 4 (Engineering Investments)** — Reviewed candidates: dependency updates (deps all current; no clear outdated path), CI gaps (gofmt check on hold per memory — `#160/#161` manual apply by maintainer), build system. gofmt drift observed in 5 files (`internal/config/config.go`, `internal/diskschedule/diskschedule.go`, `internal/host/detect_test.go`, `internal/ops/rebuild_test.go`, `internal/tui/dashboard.go`) — all whitespace-alignment cosmetic, deliberately not bundled with the archive PR (CI doesn't enforce gofmt yet; would be a separate, larger conversation). No actionable engineering investment landed this run → fallback to Task 5.
- **Task 11 (Activity)** — `add_comment` on #100 carrying this run's record.

## In-flight work

- **PR (this run, in-flight)** — `repo-assist/archive-resolve-duplicate-code-2026-06-18`, commit `e3dbf82`. Archives the completed `resolve-duplicate-code` openspec change + syncs 4 delta specs to `openspec/specs/`. Patch at `/tmp/gh-aw/aw-repo-assist-archive-resolve-duplicate-code-2026-06-18.{patch,bundle}` (~14 KB / ~5 KB).
- **PR #215 (in-flight, prior run, repo-assist)** — `[repo-assist] refactor(runner): collapse 3 docker create env arg helpers via dockerCreateEnvLineIf`. Branch `repo-assist/refactor-docker-create-env-line-if-2026-06-18-39cbf01faaeb602a`, commit `0192354f`. Closes #209. Awaiting maintainer review.
- **PR #216 (in-flight, this cycle, not repo-assist)** — `[test-improver] test(ops): cover Restart orchestrator (0% → 85.7%)`. Branch `test-assist/restart-orchestrator-f0fb98c85c3bb518`, commit `02440374`. 10 race-clean tests; +0.9 pp ops coverage. Awaiting review.
- **PR #212 (in-flight, this cycle, not repo-assist)** — `[efficiency-improver] perf(runner): hoist loop-invariant work in Manager.Status`. 197→166 allocs/op (-15.74%, p=0.008). Awaiting review.
- **PR #213 (in-flight, this cycle, not repo-assist)** — `[efficiency-improver] perf(runner): hoist loop-invariant work in Manager.Status`. Duplicate of #212 (same workflow run, two pushes).
- **PR #211 (MERGED)** — `repo-assist/refactor-container-config-accessors-2026-06-17`, commit `98b8a43`. `positiveIntOrDefault` helper + 3 accessor rewrites. Merged 2026-06-18 as PR #211.
- **PR #206 (MERGED)** — `repo-assist/perf-metrics-splitseq-2026-06-17`. Merged 2026-06-17.
- **PR #202 (MERGED)** — `repo-assist/perf-container-parse-status-2026-06-17`. Merged 2026-06-17.
- **PR #200 (MERGED)** — `test-assist/down-orchestrator-bcb888384bc03fdb`. Merged 2026-06-17.
- **PR #199 (MERGED)** — `repo-assist/perf-diskschedule-parse-at-time-e8806c619afb0865`. Merged 2026-06-17.

## Backlog / next high-value task

- **#210 (`resolveRunnerImage` block — 3 sites, 2 files)** — needs a maintainer signal on the helper signature (3-return `resolveRunnerImage` vs. 2-return `resolveRunnerImageTag`). On hold.
- **#132 (gh sr storage — btrfs loop + reflink seed)** — human-authored design. v1 scaffold is the natural next deliverable; **on hold** pending maintainer signal on the loop-mount persistence approach.
- **#190 / #201 (docs PR placeholders)** — manual `CHANGELOG.md` apply by maintainer.
- **#160 / #161 (gofmt CI check)** — manual `ci.yml` apply by maintainer. NOTE this run observed gofmt drift in 5 files (whitespace alignment in `internal/config/config.go`, `internal/diskschedule/diskschedule.go`, `internal/host/detect_test.go`, `internal/ops/rebuild_test.go`, `internal/tui/dashboard.go`); deliberately not bundled with the archive PR (CI doesn't enforce gofmt yet).
- **Safe-outputs MCP bridge** — `update_issue` for #100 has been failing 10+ consecutive runs at the body-persistence layer (14KB > 10KB MCP limit). `add_comment` is the durable fallback. PR creation (`create_pull_request`) eventually pushes on its own (intermittent; not persistent). This run's `create_pull_request` for the archive PR returned success with patch/bundle artifacts.

## Backlog cursor for Task 2 (Issue Comment)

- 12 open items (1 human = #132, 11 bot-generated workflow trackers / docs-PR placeholders / duplicate-code findings). Duplicate-code findings #207/#208/#209/#210 still in scope as a cluster; #209 PR #215 in-flight, #210 awaiting maintainer signal. No new human activity since last run on the bot-generated trackers.

## Completed work (PRs MERGED + current drafts)

- **PR (this run, in-flight)** — `[repo-assist] chore(openspec): archive resolve-duplicate-code change`. Branch `repo-assist/archive-resolve-duplicate-code-2026-06-18`, commit `e3dbf82`. Move + 4 spec syncs. Patch at `/tmp/gh-aw/aw-repo-assist-archive-resolve-duplicate-code-2026-06-18.{patch,bundle}` (~14 KB / ~5 KB).
- **PR #215 (in-flight, prior run, repo-assist)** — `dockerCreateEnvLineIf` helper for #209. Branch `repo-assist/refactor-docker-create-env-line-if-2026-06-18-39cbf01faaeb602a`, commit `0192354f`.
- **PR #211 (MERGED)** — `positiveIntOrDefault` helper (closes #207). Commit `98b8a43`.
- **PR #206 (MERGED)** — `strings.SplitSeq` in parseUnixMetrics + splitNonEmptyLines. ~21% faster.
- **PR #202 (MERGED)** — `strings.Cut` chain in parseContainerStatusInspectOutput. ~2.7x faster, 0 allocs/op.
- **#199 (MERGED)** — perf diskschedule parseAtTime. ~22x faster, 0 allocs/op.
- **#200 (MERGED)** — test-assist/down-orchestrator. 0% → 83.3%.
- **#198** — `fix(runner): cap container bootstrap retries`. Merged 2026-06-17.
- **#197** — `fix(autostart): stop native units looping`. Merged 2026-06-17.
- **#194** — `test(diskschedule): table-test parseAtTime`. Merged 2026-06-14.
- **#193** — `docs: changelog entries`. Merged 2026-06-14.
- **#192** — `refactor(tui): extract renderMenuItems`. Merged 2026-06-14.
- **#191** — `perf(tui): strconv.ParseFloat`. Merged 2026-06-16.
- **#189** — `test: cover CollectHostMetrics`. Merged 2026-06-15.
- **#187** — `feat(org-runners): docs`. Merged.
- **#184** — `[repo-assist] refactor(autostart): extract runActiveCheck helper` (Closes #174). Merged 2026-06-14.
- **#183** — `[repo-assist] refactor(ops): extract resolveAndFilter preamble helper` (Closes #175). Merged 2026-06-14.
- **#180** — `[repo-assist] refactor(autostart): unify launchd domain list via launchdDomainList helper` (Closes #176). Merged 2026-06-14.
- **#178** — `test(ops): cover ResolveHostInfo via mock-host injection`. Merged.
- **#172** — `[repo-assist] refactor(tui): extract renderHeader / renderRow helpers` (Closes #166). Merged 2026-06-14.
- **Earlier merges (2026-06-12/13 batch):** #159, #158, #157, #156, #155, #149, #147, #146, #142, #141, #139, #136, #131, #128, #127, #126, #113, #114, #110, #118, #99.

## Activity / PR history (compressed)

- 2026-06-18 (run 27772439940, ~18:30 UTC): Archived `resolve-duplicate-code` openspec change + synced 4 delta specs. Tasks 5/4/3. #100 add_comment (this run).
- 2026-06-18 (run 27754636694): `dockerCreateEnvLineIf` helper for #209; #215 in-flight.
- 2026-06-17 (3 runs): refactor container config accessors → PR #211 (merged); perf metrics SplitSeq; perf container parse → PR #202.
- 2026-06-16/14/13/12: PRs #194 (test diskschedule), #180/#183/#184 (refactors #176/#175/#174), #172 (#166), #171 (#165), #169.
- Earlier: PRs #157/#158/#159/#160/#161 (gofmt CI placeholder)/#168 — all merged or addressed.

## Notes for next run

- **#100 body refresh.** If the next `update_issue` for #100 succeeds, the body should add the new `Review PR` line for the #209 refactor, drop nothing else (no merged/closed items this run). The exact intended body is captured in this run's `add_comment` on #100 (`aw_bp1uEBW8`). NOTE: the intended body exceeds the 10KB MCP limit — `update_issue` will error; the comment is the durable record.
- **PR (this run) manual push.** Patch at `/tmp/gh-aw/aw-repo-assist-archive-resolve-duplicate-code-2026-06-18.patch` (13920 bytes / 241 lines); bundle at `/tmp/gh-aw/aw-repo-assist-archive-resolve-duplicate-code-2026-06-18.bundle`. Branch `repo-assist/archive-resolve-duplicate-code-2026-06-18`, commit `e3dbf82`.
- **#210 / #132 design signals still pending.** Maintainer is in selective mode.
- **gofmt drift observed.** 5 files (`internal/{config/config,diskschedule/diskschedule,host/detect_test,ops/rebuild_test,tui/dashboard}.go`). CI doesn't enforce gofmt yet (per `#160/#161`); not bundled into the archive PR.
- **Safe-outputs bridge:** intermittent on `create_pull_request`, persistent on `update_issue` (must use `add_comment` as fallback; the 14KB MCP limit is a hard failure mode). Continue to verify body updates with a follow-up read; do not assume `success` = applied.
- **Posture:** revert rate is 0/16+ over the life of the workflow. Recent merges (#180, #183, #184, #189, #191, #198, #197, #202, #206, #211) all landed cleanly.
- **Next high-confidence action if a new `bug` / `help wanted` / `good first issue` issue opens:** investigate root cause and implement a minimal fix on a `repo-assist/fix-issue-N-<desc>` branch. None currently open.
- **Hot path inventory still has headroom:** `internal/tui/dashboard.go:810` (`wrapLines`) uses `strings.Split(s, "\n")` but is called only on scroll/config-format events, not per tick. `internal/autostart/cleanup.go:57,80,94` and `internal/agentic/agentic.go:159,860` use `strings.Split` for one-shot CLI paths (off the hot path). `internal/doctor/doctor.go:506,604` use `strings.Split` for one-shot remediation text rendering (off the hot path). The hot-path strings.Split cleanup is complete; remaining call sites are deliberate one-shot paths.
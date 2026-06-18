---
name: repo-assist-state
description: Repo Assist persistent state — completed work, in-flight PRs, backlog cursor, and notes
metadata:
  type: project
---

# Repo Assist state — last updated 2026-06-18 ~19:30 UTC

## Last run (2026-06-18 ~19:30 UTC, run 27789197909)

- Selected tasks: 2 (Issue Comment), 3 (Issue Fix), 4 (Engineering Investments).
- **Task 2 (Issue Comment)** — Posted comments on #219 and #218.
  - #219: noted that PR #215 (`dockerCreateEnvLineIf`) already addresses the same `*DockerCreateArg` pattern that #219's detector flagged; recommend closing as superseded rather than re-running the refactor.
  - #218: signalled intent to take on the `resolveStateDirOrFallback` extraction as Task 3 below.
- **Task 3 (Issue Fix)** — Implemented `resolveStateDirOrFallback` helper for #218. New helper owns the "resolve or fall back to literal `$HOME/...`" invariant. Three best-effort sites (`clearContainerBootstrapMarkers`, `ContainerBootstrapFailed`, `removeContainer`) collapse to one-liners; the error-returning site (`createContainerInstance`) keeps using `resolveAbsoluteRunnerDir`. Branch `repo-assist/fix-issue-218-state-dir-fallback-2026-06-18`, commit `8eee26b`. New `TestResolveStateDirOrFallback` (3 subtests: linux resolve succeeds → absolute; linux resolve fails → `$HOME` literal; windows path → as-is). Build / vet / `go test ./... -race -count=1` clean (12/12 packages). Patch at `/tmp/gh-aw/aw-repo-assist-fix-issue-218-state-dir-fallback-2026-06-18.{patch,bundle}` (~6 KB / ~2 KB). `create_pull_request` returned success — bridge expected to push as PR #221.
- **Task 4 (Engineering Investments)** — `go list -m -u` shows mostly patch-version bumps (`charm.land/bubbletea/v2 v2.0.2 → v2.0.7`, `dario.cat/mergo v1.0.1 → v1.0.2`, `cpuguy83/go-md2man/v2 v2.0.6 → v2.0.7`, `cli/safeexec v1.0.0 → v1.0.1`); no major-version upgrades or security fixes warrant a bundled PR. Dependabot already covers these. No actionable engineering investment landed this run.
- **Task 11 (Activity)** — `add_comment` on #100 carrying this run's record (`aw_WNlFnaTh`).

## In-flight work

- **PR (this run, in-flight)** — `repo-assist/fix-issue-218-state-dir-fallback-2026-06-18`, commit `8eee26b`. `resolveStateDirOrFallback` helper for #218. Patch at `/tmp/gh-aw/aw-repo-assist-fix-issue-218-state-dir-fallback-2026-06-18.{patch,bundle}` (~6 KB / ~2 KB).
- **PR #220 (in-flight, prior run, repo-assist)** — `[repo-assist] chore(openspec): archive resolve-duplicate-code change`. Branch `repo-assist/archive-resolve-duplicate-code-2026-06-18-10aef83371bfc176`, commit `b1c0bf0`. Pure openspec housekeeping.
- **PR #216 (in-flight, this cycle, not repo-assist)** — `[test-improver] test(ops): cover Restart orchestrator (0% → 85.7%)`. Branch `test-assist/restart-orchestrator-f0fb98c85c3bb518`, commit `02440374`.
- **PR #215 (in-flight, prior run, repo-assist)** — `[repo-assist] refactor(runner): collapse 3 docker create env arg helpers via dockerCreateEnvLineIf`. Branch `repo-assist/refactor-docker-create-env-line-if-2026-06-18-39cbf01faaeb602a`, commit `0192354f`. Closes #209.
- **PR #213 (in-flight, this cycle, not repo-assist)** — `[efficiency-improver] perf(runner): hoist loop-invariant work in Manager.Status`. 197→166 allocs/op (-15.74%, p=0.008). Duplicate of #212 (same workflow run, two pushes).
- **PR #212 (in-flight, this cycle, not repo-assist)** — `[efficiency-improver] perf(runner): hoist loop-invariant work in Manager.Status`. Same as #213.
- **PR #211 (MERGED)** — `repo-assist/refactor-container-config-accessors-2026-06-17`, commit `98b8a43`. `positiveIntOrDefault` helper + 3 accessor rewrites. Merged 2026-06-18.
- **PR #206 (MERGED)** — `repo-assist/perf-metrics-splitseq-2026-06-17`. Merged 2026-06-17.
- **PR #202 (MERGED)** — `repo-assist/perf-container-parse-status-2026-06-17`. Merged 2026-06-17.
- **PR #200 (MERGED)** — `test-assist/down-orchestrator-bcb888384bc03fdb`. Merged 2026-06-17.
- **PR #199 (MERGED)** — `repo-assist/perf-diskschedule-parse-at-time-e8806c619afb0865`. Merged 2026-06-17.

## Backlog / next high-value task

- **#210 (`resolveRunnerImage` block — 3 sites, 2 files)** — needs a maintainer signal on the helper signature (3-return `resolveRunnerImage` vs. 2-return `resolveRunnerImageTag`). On hold.
- **#217 (Container Status Shell Script duplication)** — `containerLocalStatusFromDocker` is dead code in production (callers all go through `containerLocalStatusImageAndRevision` → `containerLocalStatusOneShot`). Detector's recommendation is to delete the function or extract a shared `buildContainerStatusScript` helper. Lower priority; leave for `@copilot` unless maintainer prefers Repo Assist take it.
- **#132 (gh sr storage — btrfs loop + reflink seed)** — human-authored design. v1 scaffold is the natural next deliverable; **on hold** pending maintainer signal on the loop-mount persistence approach.
- **#190 / #201 (docs PR placeholders)** — manual `CHANGELOG.md` apply by maintainer.
- **#160 / #161 (gofmt CI check)** — manual `ci.yml` apply by maintainer. NOTE this run observed gofmt drift in 5 files (whitespace alignment in `internal/config/config.go`, `internal/diskschedule/diskschedule.go`, `internal/host/detect_test.go`, `internal/ops/rebuild_test.go`, `internal/tui/dashboard.go`); deliberately not bundled with the archive PR (CI doesn't enforce gofmt yet).
- **Safe-outputs MCP bridge** — `update_issue` for #100 has been failing 10+ consecutive runs at the body-persistence layer (14KB > 10KB MCP limit). `add_comment` is the durable fallback. PR creation (`create_pull_request`) eventually pushes on its own (intermittent; not persistent). This run's `create_pull_request` for the #218 fix returned success with patch/bundle artifacts.

## Backlog cursor for Task 2 (Issue Comment)

- 15 open items (1 human = #132, 14 bot-generated workflow trackers / docs-PR placeholders / duplicate-code findings). Duplicate-code cluster: #207/#208/#209/#210 + #217/#218/#219 — #209 covered by PR #215 in flight; #210 awaiting maintainer signal; #218 fixed this run; #219 supersedes by #215; #217 to delegate. No new human activity since last run on the bot-generated trackers.

## Completed work (PRs MERGED + current drafts)

- **PR (this run, in-flight)** — `[repo-assist] refactor(runner): extract resolveStateDirOrFallback helper (Closes #218)`. Branch `repo-assist/fix-issue-218-state-dir-fallback-2026-06-18`, commit `8eee26b`. 3 best-effort sites collapse behind one helper. Patch at `/tmp/gh-aw/aw-repo-assist-fix-issue-218-state-dir-fallback-2026-06-18.{patch,bundle}` (~6 KB / ~2 KB).
- **PR #220 (in-flight, prior run, repo-assist)** — `[repo-assist] chore(openspec): archive resolve-duplicate-code change`. Branch `repo-assist/archive-resolve-duplicate-code-2026-06-18-10aef83371bfc176`, commit `b1c0bf0`.
- **PR #215 (in-flight, prior run, repo-assist)** — `dockerCreateEnvLineIf` helper for #209. Branch `repo-assist/refactor-docker-create-env-line-if-2026-06-18-39cbf01faaeb602a`, commit `0192354f`.
- **PR #211 (MERGED)** — `positiveIntOrDefault` helper (closes #207). Commit `98b8a43`.
- **PR #206 (MERGED)** — `strings.SplitSeq` in parseUnixMetrics + splitNonEmptyLines. ~21% faster.
- **PR #202 (MERGED)** — `strings.Cut` chain in parseContainerStatusInspectOutput. ~2.7x faster, 0 allocs/op.
- **PR #200 (MERGED)** — `test-assist/down-orchestrator`. 0% → 83.3%.
- **PR #199 (MERGED)** — perf diskschedule parseAtTime. ~22x faster, 0 allocs/op.
- **PR #198** — `fix(runner): cap container bootstrap retries`. Merged 2026-06-17.
- **PR #197** — `fix(autostart): stop native units looping`. Merged 2026-06-17.
- **PR #194** — `test(diskschedule): table-test parseAtTime`. Merged 2026-06-14.
- **PR #193** — `docs: changelog entries`. Merged 2026-06-14.
- **PR #192** — `refactor(tui): extract renderMenuItems`. Merged 2026-06-14.
- **PR #191** — `perf(tui): strconv.ParseFloat`. Merged 2026-06-16.
- **PR #189** — `test: cover CollectHostMetrics`. Merged 2026-06-15.
- **PR #187** — `feat(org-runners): docs`. Merged.
- **PR #184** — `[repo-assist] refactor(autostart): extract runActiveCheck helper` (Closes #174). Merged 2026-06-14.
- **PR #183** — `[repo-assist] refactor(ops): extract resolveAndFilter preamble helper` (Closes #175). Merged 2026-06-14.
- **PR #180** — `[repo-assist] refactor(autostart): unify launchd domain list via launchdDomainList helper` (Closes #176). Merged 2026-06-14.
- **PR #178** — `test(ops): cover ResolveHostInfo via mock-host injection`. Merged.
- **PR #172** — `[repo-assist] refactor(tui): extract renderHeader / renderRow helpers` (Closes #166). Merged 2026-06-14.
- **Earlier merges (2026-06-12/13 batch):** #159, #158, #157, #156, #155, #149, #147, #146, #142, #141, #139, #136, #131, #128, #127, #126, #113, #114, #110, #118, #99.

## Activity / PR history (compressed)

- 2026-06-18 (run 27789197909, ~19:30 UTC): Implemented `resolveStateDirOrFallback` helper for #218. Tasks 2/3/4. #100 add_comment (`aw_WNlFnaTh`).
- 2026-06-18 (run 27772439940, ~18:30 UTC): Archived `resolve-duplicate-code` openspec change + synced 4 delta specs. Tasks 5/4/3. #100 add_comment (`aw_bp1uEBW8`).
- 2026-06-18 (run 27754636694): `dockerCreateEnvLineIf` helper for #209; #215 in-flight.
- 2026-06-17 (3 runs): refactor container config accessors → PR #211 (merged); perf metrics SplitSeq; perf container parse → PR #202.
- 2026-06-16/14/13/12: PRs #194 (test diskschedule), #180/#183/#184 (refactors #176/#175/#174), #172 (#166), #171 (#165), #169.
- Earlier: PRs #157/#158/#159/#160/#161 (gofmt CI placeholder)/#168 — all merged or addressed.

## Notes for next run

- **#100 body refresh.** If the next `update_issue` for #100 succeeds, the body should add the new `Review PR` line for the #218 fix, drop nothing else (no merged/closed items this run). The exact intended body is captured in this run's `add_comment` on #100 (`aw_WNlFnaTh`). NOTE: the intended body exceeds the 10KB MCP limit — `update_issue` will error; the comment is the durable record.
- **PR (this run) manual push.** Patch at `/tmp/gh-aw/aw-repo-assist-fix-issue-218-state-dir-fallback-2026-06-18.patch` (6382 bytes / 160 lines); bundle at `/tmp/gh-aw/aw-repo-assist-fix-issue-218-state-dir-fallback-2026-06-18.bundle` (2047 bytes). Branch `repo-assist/fix-issue-218-state-dir-fallback-2026-06-18`, commit `8eee26b`. 2 files changed, 74 insertions(+), 14 deletions(-).
- **#210 / #132 design signals still pending.** Maintainer is in selective mode.
- **#215 + #218 conflict check.** Both modify `internal/runner/container.go`. Different functions (PR #215: `mtuDockerCreateArg` + 2 siblings via `dockerCreateEnvLineIf`; this run's PR: `clearContainerBootstrapMarkers`/`ContainerBootstrapFailed`/`removeContainer` via `resolveStateDirOrFallback`). No textual conflict expected when both land; review for accidental overlapping changes.
- **gofmt drift observed.** 5 files (`internal/{config/config,diskschedule/diskschedule,host/detect_test,ops/rebuild_test,tui/dashboard}.go`). CI doesn't enforce gofmt yet (per `#160/#161`); not bundled into the archive PR.
- **Safe-outputs bridge:** intermittent on `create_pull_request`, persistent on `update_issue` (must use `add_comment` as fallback; the 14KB MCP limit is a hard failure mode). Continue to verify body updates with a follow-up read; do not assume `success` = applied.
- **Posture:** revert rate is 0/16+ over the life of the workflow. Recent merges (#180, #183, #184, #189, #191, #198, #197, #202, #206, #211) all landed cleanly.
- **Next high-confidence action if a new `bug` / `help wanted` / `good first issue` issue opens:** investigate root cause and implement a minimal fix on a `repo-assist/fix-issue-N-<desc>` branch. None currently open.
- **Hot path inventory still has headroom:** `internal/tui/dashboard.go:810` (`wrapLines`) uses `strings.Split(s, "\n")` but is called only on scroll/config-format events, not per tick. `internal/autostart/cleanup.go:57,80,94` and `internal/agentic/agentic.go:159,860` use `strings.Split` for one-shot CLI paths (off the hot path). `internal/doctor/doctor.go:506,604` use `strings.Split` for one-shot remediation text rendering (off the hot path). The hot-path strings.Split cleanup is complete; remaining call sites are deliberate one-shot paths.
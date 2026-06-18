---
name: repo-assist-state
description: Repo Assist persistent state — completed work, in-flight PRs, backlog cursor, and notes
metadata:
  type: project
---

# Repo Assist state — last updated 2026-06-18

## Last run (2026-06-18 14:25 UTC, run 27754636694)

- Selected tasks: 5 (Coding Improvements), 2 (Comment), 1 (Labelling).
- **Task 5 / 2 / 1 (consolidated on #209)** — Created draft PR (branch `repo-assist/refactor-docker-create-env-line-if-2026-06-18`, commit `64a31e9`). Closes #209. New free-function `dockerCreateEnvLineIf(name string, value int, emit bool) string` in `internal/runner/container.go` collapses `mtuDockerCreateArg` (with `[576, 1500)` range), `dockerdStartTimeoutDockerCreateArg` (with `seconds > 0`), and `bootstrapMaxRetriesDockerCreateArg` (with `maxRetries > 0`). Each prior validity check was the exact boolean negation of the new emit predicate, so behaviour is preserved bit-for-bit. New `TestDockerCreateEnvLineIf` (6 subtests) + negative-input case for `TestDockerdStartTimeoutDockerCreateArg` + `maxRetries=0` case for `TestBootstrapMaxRetriesDockerCreateArg`. All 12 packages pass under `-race -count=1`; gofmt + vet clean. Patch at `/tmp/gh-aw/aw-repo-assist-refactor-docker-create-env-line-if-2026-06-18.{patch,bundle}` (6447 bytes / 145 lines). Scope deliberately excludes #210 (`resolveRunnerImage` block, 3 sites, 2 files) — needs a maintainer signal on the helper signature (3-return vs 2-return; the `environment.go` site omits the `arch :=` line, so the three call sites aren't shape-identical).
- **Task 2 (Comment)** — Commented on #209 ([aw_Hyx12zyv](https://github.com/baizhiheizi/gh-sr/issues/209#issuecomment-aw_Hyx12zyv)) with rationale, scope decision, and why #210 is deferred. Commented on #210 ([aw_mTTV0fjw](https://github.com/baizhiheizi/gh-sr/issues/210#issuecomment-aw_mTTV0fjw)) asking for a maintainer call on the helper signature.
- **Task 1 (Labelling)** — fallback to Task 2 (0 unlabelled issues).
- **Task 11 (Activity)** — `add_comment` on #100 (temporary_id `aw_bp1uEBW8`) carrying this run's record. `update_issue` for the body continues to fail (body is past 14KB; MCP limit 10KB). Comment is the durable record per the established fallback.

## In-flight work

- **PR (this run, in-flight)** — `repo-assist/refactor-docker-create-env-line-if-2026-06-18`, commit `64a31e9`. Closes #209. `dockerCreateEnvLineIf` helper + 3 accessor rewrites. Patch at `/tmp/gh-aw/aw-repo-assist-refactor-docker-create-env-line-if-2026-06-18.{patch,bundle}` (6447 bytes / 145 lines).
- **PR #212 (in-flight, this cycle, not repo-assist)** — `[efficiency-improver] perf(runner): hoist loop-invariant work in Manager.Status` (branch `efficiency/manager-status-hoist-labels-join-75cd20d33694b5c4`, commit `c27db0d`). 197→166 allocs/op (-15.74%, p=0.008). Awaiting review.
- **PR #213 (in-flight, this cycle, not repo-assist)** — `[efficiency-improver] perf(runner): hoist loop-invariant work in Manager.Status` (branch `efficiency/manager-status-hoist-labels-join-33a02534ff41057a`, commit `317a129`). Duplicate of #212 (same workflow run, two pushes).
- **PR #211 (in-flight, prior run, MERGED)** — `repo-assist/refactor-container-config-accessors-2026-06-17`, commit `98b8a43`. `positiveIntOrDefault` helper + 3 accessor rewrites. Merged 2026-06-18 as PR #211. Was started as PR #207 in run 27718044819; the bridge eventually pushed it as #211.
- **PR #206 (in-flight, prior run, MERGED)** — `repo-assist/perf-metrics-splitseq-2026-06-17`. Merged 2026-06-17.
- **PR #202 (in-flight, prior run, MERGED)** — `repo-assist/perf-container-parse-status-2026-06-17`. Merged 2026-06-17.
- **PR #200 (in-flight, prior run, MERGED)** — `test-assist/down-orchestrator-bcb888384bc03fdb`. Merged 2026-06-17.
- **PR #199 (in-flight, prior run, MERGED)** — `repo-assist/perf-diskschedule-parse-at-time-e8806c619afb0865`. Merged 2026-06-17.

## Backlog / next high-value task

- **#210 (`resolveRunnerImage` block — 3 sites, 2 files)** — needs a maintainer signal on the helper signature (3-return `resolveRunnerImage` vs. 2-return `resolveRunnerImageTag`). On hold.
- **#132 (gh sr storage — btrfs loop + reflink seed)** — human-authored design. v1 scaffold is the natural next deliverable; **on hold** pending maintainer signal on the loop-mount persistence approach.
- **#190 / #201 (docs PR placeholders)** — manual `CHANGELOG.md` apply by maintainer.
- **#160 / #161 (gofmt CI check)** — manual `ci.yml` apply by maintainer.
- **Safe-outputs MCP bridge** — `update_issue` for #100 has been failing 10+ consecutive runs at the body-persistence layer (14KB > 10KB MCP limit). `add_comment` is the durable fallback. PR creation (`create_pull_request`) eventually pushes on its own (intermittent; not persistent). This run's `create_pull_request` for #209 returned success with patch/bundle artifacts; bridge expected to push as the next available number.

## Backlog cursor for Task 2 (Issue Comment)

- 12 open items (1 human = #132, 11 bot-generated workflow trackers / docs-PR placeholders / duplicate-code findings). Duplicate-code findings #207/#208/#209/#210 are now in scope as a cluster; #209 engaged this run, #210 awaiting maintainer signal.

## Completed work (PRs MERGED + current drafts)

- **PR (this run, in-flight)** — `[repo-assist] refactor(runner): collapse 3 docker create env arg helpers via dockerCreateEnvLineIf`. Branch `repo-assist/refactor-docker-create-env-line-if-2026-06-18`, commit `64a31e9`. Closes #209. New helper + 3 accessor rewrites.
- **PR #211 (in-flight, prior run, MERGED)** — `[repo-assist] refactor(runner): collapse 3 container config accessors via positiveIntOrDefault`. Branch `repo-assist/refactor-container-config-accessors-2026-06-17`, commit `98b8a43`. Closes #207. Bug fix (positivity-check drift) + 8 new subtests.
- **PR #206 (in-flight, prior run, MERGED)** — `[repo-assist] perf(host,runner): strings.SplitSeq in parseUnixMetrics and splitNonEmptyLines`. ~21% faster on both functions; -78% and -35% memory respectively.
- **PR #202 (in-flight, prior run, MERGED)** — `[repo-assist] perf(runner): strings.Cut chain in parseContainerStatusInspectOutput`. ~2.7x faster, 0 allocs/op.
- **#199 (in-flight, MERGED)** — `repo-assist/perf-diskschedule-parse-at-time-e8806c619afb0865`. ~22x faster, 0 allocs/op.
- **#200 (in-flight, MERGED)** — `test-assist/down-orchestrator-bcb888384bc03fdb`. Down 0% → 83.3%.
- **#198** — `fix(runner): cap container bootstrap retries when inner dockerd fails`. Merged 2026-06-17.
- **#197** — `fix(autostart): stop native units looping on missing runner dirs`. Merged 2026-06-17.
- **#194** — `test(diskschedule): table-test parseAtTime and systemdQuoteArg`. Merged 2026-06-14.
- **#193** — `docs: changelog entries for org-runner docs and gh-aw Node.js LTS`. Merged 2026-06-14.
- **#192** — `refactor(tui): extract renderMenuItems + newAltView helpers`. Merged 2026-06-14.
- **#191** — `perf(tui): use strconv.ParseFloat in extractTrailingPercent`. Merged 2026-06-16.
- **#189** — `test: cover CollectHostMetrics (0% -> 100%)`. Merged 2026-06-15.
- **#187** — `feat(org-runners): document and improve org-level runner support`. Merged.
- **#184** — `[repo-assist] refactor(autostart): extract runActiveCheck helper` (Closes #174). Merged 2026-06-14.
- **#183** — `[repo-assist] refactor(ops): extract resolveAndFilter preamble helper` (Closes #175). Merged 2026-06-14.
- **#180** — `[repo-assist] refactor(autostart): unify launchd domain list via launchdDomainList helper` (Closes #176). Merged 2026-06-14.
- **#178** — `test(ops): cover ResolveHostInfo via mock-host injection`. Merged.
- **#172** — `[repo-assist] refactor(tui): extract renderHeader / renderRow helpers` (Closes #166). Merged 2026-06-14.
- Earlier merges: #159, #158, #157, #156, #155, #149, #147, #146, #142, #141, #139, #136, #131, #128, #127, #126, #113, #114, #110, #118, #127, #99.

## Activity / PR history (compressed)

- 2026-06-18 (run 27754636694, 14:25 UTC): Created PR (this run, in-flight) — `dockerCreateEnvLineIf` helper collapses 3 docker create env arg helpers. Tasks 5/2/1 consolidated on #209. Commented on #209 (`aw_Hyx12zyv`) + #210 (`aw_mTTV0fjw`, design-signal ask). Activity comment on #100 (`aw_bp1uEBW8`).
- 2026-06-17 (run 27718044819, 21:53 UTC): Created PR #207 (refactor container config accessors, in-flight). Tasks 4/2/3 consolidated on #207. Commented on #207 (`aw_HT4dqN0O`). Activity comment on #100 (`aw_m4COO3Y3`). PR #211 (the work) eventually merged 2026-06-18.
- 2026-06-17 (run 27703792995, 17:13 UTC): Created PR (this run) — perf metrics SplitSeq. Tasks 6/4 no-op, Task 8 created perf PR. Activity comment on #100 (`aw_toOsZ2sB`); `update_issue` errored on 14KB body > 10KB MCP limit.
- 2026-06-17 (run 27685313566, 12:13 UTC): Created PR #202 (perf container parse, in-flight). Tasks 8/9 bundled. Tasks 10/2 no-op. Activity comment on #100 (`aw_z7qR5rYZ`); `update_issue` for body refresh returned `success` but did not apply.
- 2026-06-16 (run 27597907360, 06:12 UTC): Created PR #194 (test diskschedule table-test). Bridge eventually pushed. 12.8% → 14.2%.
- 2026-06-16 (run 27577906493, 00:53 UTC): No-op run. Activity comment on #100 (`aw_qTaFu2JT`).
- 2026-06-15 (run 27526398615, 05:43 UTC): No-op run. Maintenance mode.
- 2026-06-14 (run 27475684135, 04:39 UTC): Implemented #174 fix. Bridge eventually pushed as PR #184.
- 2026-06-14 (run 27467295298, 03:48 UTC): Implemented #175 fix. Bridge eventually pushed as PR #183.
- 2026-06-14 (run 27460481325, 03:30 UTC): Implemented #176 fix. Bridge eventually pushed as PR #180.
- 2026-06-13 (run 27452358684, 01:30 UTC): Implemented #166 fix. Bridge eventually pushed as PR #172.
- 2026-06-12 (run 27436420474, 19:07 UTC): Implemented #165 fix. Bridge eventually pushed as PR #171.
- 2026-06-12 (run 27418691649, 13:34 UTC): Created PR #169. Landed.
- Earlier: Created PRs #157, #159, #160/#161, #158, #169, #172, #180, #183, #184, #194. All merged or addressed.

## Notes for next run

- **#100 body refresh.** If the next `update_issue` for #100 succeeds, the body should add the new `Review PR` line for the #209 refactor, drop nothing else (no merged/closed items this run). The exact intended body is captured in this run's `add_comment` on #100 (`aw_bp1uEBW8`). NOTE: the intended body exceeds the 10KB MCP limit — `update_issue` will error; the comment is the durable record.
- **PR (this run) manual push.** If the bridge did not push it, the patch is at `/tmp/gh-aw/aw-repo-assist-refactor-docker-create-env-line-if-2026-06-18.patch` (6447 bytes / 145 lines) and the branch `repo-assist/refactor-docker-create-env-line-if-2026-06-18` is on the local checkout with commit `64a31e9`. Mirror the patch via `git am` and push to open the PR.
- **#210 / #132 design signals still pending.** Maintainer is in selective mode; both refactors require a maintainer signal on helper signatures (#210) and loop-mount persistence (#132) before implementation.
- **Safe-outputs bridge:** intermittent on `create_pull_request` (eventually pushes on its own), persistent on `update_issue` (must use `add_comment` as fallback; the 14KB MCP limit is a hard failure mode). Continue to verify body updates with a follow-up read; do not assume `success` = applied.
- **Posture:** revert rate is 0/15+ over the life of the workflow. Recent merges (#180, #183, #184, #189, #191, #198, #197, #202, #206, #211) all landed cleanly.
- **Next high-confidence action if a new `bug` / `help wanted` / `good first issue` issue opens:** investigate root cause and implement a minimal fix on a `repo-assist/fix-issue-N-<desc>` branch. None currently open.
- **Hot path inventory still has headroom:** `internal/tui/dashboard.go:810` (`wrapLines`) uses `strings.Split(s, "\n")` but is called only on scroll/config-format events, not per tick. `internal/autostart/cleanup.go:57,80,94` and `internal/agentic/agentic.go:159,860` use `strings.Split` for one-shot CLI paths (off the hot path). `internal/doctor/doctor.go:506,604` use `strings.Split` for one-shot remediation text rendering (off the hot path). The hot-path strings.Split cleanup is complete; remaining call sites are deliberate one-shot paths.
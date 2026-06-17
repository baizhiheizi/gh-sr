---
name: repo-assist-state
description: Repo Assist persistent state — completed work, in-flight PRs, backlog cursor, and notes
metadata:
  type: project
---

# Repo Assist state — last updated 2026-06-17

## Last run (2026-06-17 12:13 UTC, run 27685313566)

- Selected tasks: 8 (Performance), 10 (Take Forward), 9 (Testing).
- **Task 8 / 9** (Performance + Testing bundled) — created draft PR locally on branch `repo-assist/perf-container-parse-status-2026-06-17`, commit `de5949f`. `parseContainerStatusInspectOutput` in `internal/runner/container.go` rewritten with a 3-call `strings.Cut` chain (was `strings.Split` + padding loop). Hot path: runs once per host per `Manager.Status()` — called on every 5s TUI refresh tick. Benchmark: 222 → 81 ns/op (~2.7x faster), 320 → 0 B/op, 5 → 0 allocs/op. Pure perf refactor — behavior preserved by 8 existing `TestParseContainerStatusInspectOutput` cases. `safe-outputs create_pull_request` returned success with patch/bundle artifacts; bridge expected to push as PR #202.
- **Task 10** (Take Forward) — no-op. #132 v1 scaffold on hold pending maintainer signal. Backlog all merged/closed. Revert rate stays at 0/15 over the life of the workflow.
- **Task 11** (Activity) — `update_issue` for #100 returned `success` with the refreshed body but did not apply (same pattern as the prior 6+ runs; `updated_at` still 2026-06-17T08:20:02Z). Fell back to `add_comment` (temporary_id `aw_z7qR5rYZ`) carrying the new Run History entry and the intended Suggested Actions refresh list.

## In-flight work

- **PR #202 (in-flight, this run)** — `repo-assist/perf-container-parse-status-2026-06-17`, commit `de5949f`. `parseContainerStatusInspectOutput` Cut chain. Patch at `/tmp/gh-aw/aw-repo-assist-perf-container-parse-status-2026-06-17.patch`.
- **PR #199 (in-flight, prior run, pushed)** — `repo-assist/perf-diskschedule-parse-at-time-e8806c619afb0865`, commit `5349344`. Open in PR list, awaiting review.
- **PR #200 (Test Improver, pushed)** — `test-assist/down-orchestrator-bcb888384bc03fdb`. Open in PR list, awaiting review.
- **PR #201 (doc-updater, blocked)** — docs PR placeholder, `CHANGELOG.md` protected. Manual apply required.

## Backlog / next high-value task

- **#132 (gh sr storage — btrfs loop + reflink seed)** — human-authored design. v1 scaffold is the natural next deliverable; **on hold** pending maintainer signal on the loop-mount persistence approach.
- **#190 / #201 (docs PR placeholders)** — manual `CHANGELOG.md` apply by maintainer.
- **#160 / #161 (gofmt CI check)** — manual `ci.yml` apply by maintainer.
- **Safe-outputs MCP bridge** — `update_issue` for #100 has been failing 7+ consecutive runs at the body-persistence layer. `add_comment` is the durable fallback. PR creation (`create_pull_request`) eventually pushes on its own (intermittent; not persistent). If a new task requires `update_issue` to land, plan a follow-up run or a patch-comment artifact.

## Backlog cursor for Task 2 (Issue Comment)

- 8 open items (1 human = #132, 7 bot-generated workflow trackers / docs-PR placeholders). Cursor parked on #132. Re-engage when the maintainer or a new human comment appears.

## Completed work (PRs MERGED + current drafts)

- **PR #202 (this run, in-flight)** — `[repo-assist] perf(runner): strings.Cut chain in parseContainerStatusInspectOutput`. Branch `repo-assist/perf-container-parse-status-2026-06-17`, commit `de5949f`. ~2.7x faster, 0 allocs/op.
- **#199 (in-flight)** — `repo-assist/perf-diskschedule-parse-at-time-e8806c619afb0865`. ~22x faster, 0 allocs/op.
- **#200 (in-flight, Test Improver)** — `test-assist/down-orchestrator-bcb888384bc03fdb`. Down 0% → 83.3%.
- **#201 (in-flight, doc-updater)** — docs PR placeholder for 2026-06-17 features.
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

- **#100 body refresh.** If the next `update_issue` for #100 succeeds, the body should drop the stale `Review PR: parseAtTime + systemdQuoteArg` line (merged as #194), drop the `Close issue #148` line, drop the `#194 missing` mention in the merge list, add the `Review PR #199`, `Review PR #200`, `Apply patch manually #201` lines, add `#194` to the merged list, and prepend the 2026-06-17 12:13 UTC Run History entry. The exact intended body is captured in this run's `add_comment` on #100 (`aw_z7qR5rYZ`).
- **PR #202 manual push.** If the bridge did not push it, the patch is at `/tmp/gh-aw/aw-repo-assist-perf-container-parse-status-2026-06-17.patch` and the branch `repo-assist/perf-container-parse-status-2026-06-17` is on the local checkout with commit `de5949f`. Mirror the patch via `git am` and push to open a PR.
- **#132 design signal still pending.** Maintainer is in selective mode; v1 scaffold requires a maintainer signal on loop-mount persistence.
- **Safe-outputs bridge:** intermittent on `create_pull_request` (eventually pushes on its own), persistent on `update_issue` (must use `add_comment` as fallback). Continue to verify body updates with a follow-up read; do not assume `success` = applied.
- **Posture:** revert rate is 0/15 over the life of the workflow. Recent merges (#180, #183, #184, #189, #191) all landed cleanly.
- **Next high-confidence action if a new `bug` / `help wanted` / `good first issue` issue opens:** investigate root cause and implement a minimal fix on a `repo-assist/fix-issue-N-<desc>` branch. None currently open.
- **Hot path inventory still has headroom:** `internal/runner/disk.go:148` and `internal/host/metrics.go:191` both use `strings.Split(s, "\n")` for line iteration in the per-host output path. A `strings.SplitSeq` migration would defer allocation, but is deferred until the use case for per-line streaming (vs. random access) is clearly present.

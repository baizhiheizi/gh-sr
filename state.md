---
name: repo-assist-state
description: Repo Assist persistent state — completed work, in-flight PRs, backlog cursor, and notes
metadata:
  type: project
---

# Repo Assist state — last updated 2026-06-14

## Last run (2026-06-14 03:30 UTC, run 27460481325)

- Selected tasks: 3 (Issue Fix), 9 (Testing), 4 (Engineering Investments).
- **Task 3** (Issue Fix) — implemented #176 fix locally: extracted `launchdDomainList() string` returning the pre-quoted shell word list `"gui/$UID" "user/$UID"`; migrated 4 call sites (3 in `internal/autostart/launchd.go`, 1 in `internal/autostart/autostart.go::Stop` KindLaunchd arm) to splice the helper via `fmt.Sprintf`. New `TestLaunchdDomainList_IsSingleSourceOfTruth` (counts the canonical for-loop line in each generated script) and `TestLaunchdDomainList_ReturnsPreQuotedTokens` (locks the contract). Commit `5048410` on branch `repo-assist/fix-176-launchd-domain-list`. Build/vet/gofmt/`go test -race ./... -count=1` all clean across 12 packages.
- **Task 9** (Testing) — addressed as part of Task 3: 2 new subtests lock the helper's contract.
- **Task 4** (Engineering Investments) — no-op. Dependencies current (Go 1.25.9, latest charmbracelet/cli/cobra). CI config minimal and clean; no actionable gaps.
- **Task 6** (Maintain Repo Assist PRs) — verified PR #172 (the #166 fix) is open; not stale.
- **Task 11** (Activity) — `create_pull_request` for #176 returned `success` twice with patch/bundle artifacts but the branch never pushed to origin (6th consecutive occurrence of the persistence issue). `update_issue` on #100 returned `success` but the body refresh did not apply. `add_comment` on #100 returned `success` with temporary_id `aw_O5wV5Myg` but the comment may or may not have applied (response cached at 2 comments, may appear in next poll).

## In-flight work

- **1 local commit, no open PR of mine this run.** Local: `5048410` on `repo-assist/fix-176-launchd-domain-list`.
- **Safe-outputs bridge persistence issue confirmed a 6th time.** All three write tools (`create_pull_request`, `update_issue`, `add_comment`) returned `success` at the MCP HTTP layer but the writes did not appear on the repo. Patch + bundle artifacts at `/tmp/gh-aw/aw-repo-assist-fix-176-launchd-domain-list.{patch,bundle}` — maintainer can apply with `git am < /tmp/gh-aw/aw-repo-assist-fix-176-launchd-domain-list.patch` from this checkout.

## Backlog / next high-value task

- **#132 (gh sr storage — btrfs loop + reflink seed)** — human-authored design. v1 scaffold (subcommand skeleton in `cmd/gh-sr/`, `internal/storage/` package, status detection) is the natural next deliverable; **on hold** pending maintainer signal on the loop-mount persistence approach.
- **#175** (ops orchestrator preamble, 8+ sites) — duplicate-code detector found a higher-severity version of the same pattern that #159 partially addressed. Would need a `runOrchestrated(w, cfg, mgr, filterHost, filterRepo, nameArgs, applyExtras, perRunner)` helper in `internal/ops/ops.go` per the detector's recommendation. Larger scope than #176.
- **#174** (IsServiceActive vs Status duplication, high severity) — 8 shell-command blocks (4 kinds × 2 functions) duplicate the `is-active` shell strings. Detector recommends extracting `activeCheck(h, kind, base, label, taskName) (string, error)` or `Kind.IsActive(...)` method. ~1 hour of mechanical work.
- **#176** — implemented locally (commit `5048410`) but safe-outputs bridge didn't push the PR. If bridge gets fixed, push this first; otherwise treat as next-fix-up work.

## Backlog cursor for Task 2 (Issue Comment)

- 16 open issues (1 mine, 1 test-improver, 1 perf-improver, 1 efficiency-improver, 1 test-assist, 4 aw, 6 agentic-workflows/duplicate-code). Cursor parked on #132 (only human-authored). Re-engage when the maintainer or a new human comment appears.

## Completed work (PRs MERGED + current drafts)

- **#172** (mine, draft) — `[repo-assist] refactor(tui): extract renderHeader / renderRow helpers` (Closes #166). Open since 2026-06-13T01:36:34Z. Awaiting maintainer review.
- **#171** (mine, draft→merged) — `[repo-assist] refactor(autostart): extract resolveAutostartTarget preamble helper` (Closes #165). MERGED 2026-06-13T07:00:42Z as commit 904dd60.
- **#169** (mine, draft→merged) — `[repo-assist] refactor(hostshell): add LinuxElevatePreludeSoft and adopt it in runner/disk.go` (Closes #163). MERGED.
- **#168** (test-improver draft→merged) — `[test-improver] test(ops): cover runPerHostParallel with mock-host injection`.
- **#167** (efficiency-improver draft→merged) — `[efficiency-improver] perf(runner): inline rcByInstance name construction in EnrichWithGitHubStatus`.
- **#159** (an-lee) — `refactor: resolve duplicate-code issues with shared helpers` (Closes #143/144/145/152/154). Merged 2026-06-12 05:35:02Z.
- **#158** — `[docs] Update documentation for features from 2026-06-11`. Merged.
- **#157** (mine) — `[repo-assist] refactor(config): extract loadYAMLRoot helper in mutate.go` (Closes #153). Merged.
- **#156** — `[test-improver] test(hostshell): cover WriteRemoteBytes across POSIX and Windows branches`. Merged.
- **#155** — `[efficiency-improver] perf(config): inline single-pointer scan + early-exit in FindRunnerForLogs`. Merged.
- **#149** (mine) — `[repo-assist] test(diskschedule): cover escapePS PowerShell single-quote escape`. Merged.
- **#147** — `[test-improver] test(ops): cover disk-printer helpers and per-host instance map`. Merged.
- **#146** — `[efficiency-improver] perf(config): inline name-N construction in InstanceNames helper`. Merged.
- **#142** (mine) — `[repo-assist] fix(runner): unify GitHub registration URL helper` (Closes #122). Merged.
- **#141** (mine) — `[repo-assist] fix(runner): error on unsupported host OS instead of silently using POSIX` (Closes #135). Merged.
- **#139** (mine) — `[repo-assist] refactor(runner): extract containerEscalation + passwordlessSudo helpers` (Closes #134). Merged.
- **#136** — `[efficiency-improver] perf(runner): single du walk in dirSizesPOSIX (4 SSH round trips → 1)`. Merged.
- **#131** — an-lee: `gh sr disk usage|prune|schedule` — adds `internal/diskschedule/` and the `disk` subcommand family. Merged.

## Activity / PR history (compressed)

- 2026-06-14 (run 27460481325, 03:30 UTC): Implemented #176 fix locally (commit 5048410 on `repo-assist/fix-176-launchd-domain-list`); build/vet/gofmt/test-race all clean. safe-outputs bridge persistence issue (6th consecutive): create_pull_request + update_issue + add_comment all returned success but did not apply to GitHub. Patch + bundle saved.
- 2026-06-13 (run 27452358684, 01:30 UTC): Implemented #166 fix locally (commit 9295b2d on `repo-assist/fix-166-tui-table-render-helper`); build/vet/gofmt/test-race all clean. safe-outputs bridge persistence issue (5th consecutive): create_pull_request + update_issue both returned success but did not apply to GitHub. Patch + bundle artifacts saved. (Note: the bridge did push #166 as PR #172 a few hours later — bridge is intermittent.)
- 2026-06-12 (run 27436420474, 19:07 UTC): Implemented #165 fix locally (commit 03d587f on `repo-assist/fix-165-autostart-preamble-helper`); build/vet/gofmt/test-race all clean. safe-outputs bridge persistence issue (4th consecutive): patch + bundle saved.
- 2026-06-12 (run 27418691649, 13:34 UTC): Created PR #169 (LinuxElevatePreludeSoft for #163). Landed despite bridge persistence noise.
- 2026-06-12 (run 27402673134, 08:03 UTC): Created gofmt fix + CI check PR (dde5c83). Blocked by workflow file permission; issue #160/#161 filed.
- 2026-06-12 (run 27388413942, 02:21 UTC): Verified #157 clean, refreshed #100 (body 9.5 KB → 8.4 KB compressed), no new PRs.
- 2026-06-11 (run 27371229238, 19:25 UTC): Created PR #157 (loadYAMLRoot helper for #153).
- 2026-06-10 (run 27279843892): Commented on #142 (stale flag); created TestEscapePS PR (landed as #149).
- 2026-06-09 (run 27228879760): Created PR (draft) for #134 (disk-helpers extraction).
- 2026-06-09 (run 27191010253): Substantive comment on #132; verified all queued PRs merged.
- 2026-06-09 (run 27177519565): Held the line on 7 PRs awaiting review.
- 2026-06-08 and earlier: Created PRs #127, #118, #114, #113, #110, #102, #99 (all merged); held the line on multiple no-action runs.

## Notes for next run

- **safe-outputs MCP bridge persistence issue (6th occurrence):** every `create_pull_request`/`update_issue`/`add_comment` call this run returned `success` at the HTTP layer but the writes did not appear on GitHub. **Pattern confirmed across 3 different safe-output tools in a single run.** Patch + bundle artifacts are saved to `/tmp/gh-aw/aw-*.{patch,bundle}`. The maintainer can apply with: `git am < /tmp/gh-aw/aw-repo-assist-fix-176-launchd-domain-list.patch` (run from this checkout). **Configuration diagnosis needed:** the issue may be in `safe-outputs.create-pull-request` (not configured for non-protected pushes), in `update-issue: target: '*'` (not set so update targets the triggering issue, which doesn't exist for scheduled runs), or in the MCP server itself. **Recommend:** if this persists, file an issue to the agentics repo requesting diagnosis.
- **Posture:** revert rate is 0/15 over the life of the workflow. Production refactors paired with bot-flagged issues are demonstrably safe.
- **#132 design signal still pending.** Maintainer is in selective mode; v1 scaffold requires a maintainer signal on loop-mount persistence.
- **Branch name pattern:** Maintainer should be aware that `repo-assist/*-XXXXXXXX` (with SHA suffix) is the gh-aw orchestrator's standard; not something the workflow can suppress.
- **Next high-confidence action if safe-outputs is fixed:** push local `repo-assist/fix-176-launchd-domain-list` (commit `5048410`) to origin + open PR. Work is already complete and tested.

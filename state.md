---
name: repo-assist-state
description: Repo Assist persistent state — completed work, in-flight PRs, backlog cursor, and notes
metadata:
  type: project
---

# Repo Assist state — last updated 2026-06-13

## Last run (2026-06-13 01:30 UTC, run 27452358684)

- Selected tasks: 2 (Comment), 8 (Performance), 10 (Take Repo Forward).
- **Task 2** (Comment) — no-op: cursor parked on #132 (human-authored design); no new human activity.
- **Task 8** (Performance) — no-op: no fresh high-value alloc hotspots identified in newly-merged code (PRs #167 / #168 / #169 / #171 are all refactors or test additions, not new alloc paths). The high-value efficiency targets (FilterRunners, Validate, FindRunnerForLogs, EnrichWithGitHubStatus, dirSizesPOSIX, InstanceNames) have all been addressed.
- **Task 10** (Take Repo Forward) — implemented #166 fix locally: extracted `renderHeader`, `renderRow`, `renderHighlightedRow` helpers in `internal/tui/table.go`; migrated `PrintStatusTable`, `PrintHostMetricsTable`, `viewMain` (cursor highlight preserved), `viewHostMetrics`; removed unused `lipgloss` import from `dashboard_view.go`; added 4 subtests in `table_test.go`. Commit `9295b2d` on branch `repo-assist/fix-166-tui-table-render-helper`. Build/vet/gofmt/test-race all clean across 12 packages.
- **Task 11** (Activity) — `update_issue` on #100 and `create_pull_request` for #166 both returned `success` at the MCP bridge but did not apply to GitHub (5th consecutive occurrence of the persistence issue).

## In-flight work

- **1 local commit, no open PR of mine this run.** Local: `9295b2d` on `repo-assist/fix-166-tui-table-render-helper`.
- **Safe-outputs bridge persistence issue confirmed a 5th time.** `create_pull_request` and `update_issue` both returned `success` at the MCP HTTP layer but the writes did not appear on the repo. Patch + bundle artifacts at `/tmp/gh-aw/aw-repo-assist-fix-166-tui-table-render-helper.{patch,bundle}` — maintainer can apply with `git am < /tmp/gh-aw/aw-repo-assist-fix-166-tui-table-render-helper.patch` from this checkout.

## Backlog / next high-value task

- **#132 (gh sr storage — btrfs loop + reflink seed)** — human-authored design. v1 scaffold (subcommand skeleton in `cmd/gh-sr/`, `internal/storage/` package, status detection) is the natural next deliverable; **on hold** pending maintainer signal on the loop-mount persistence approach.
- **#124** — benchstat comparison on PRs (MEDIUM impact, infrastructure). Touches `.github/workflows/ci.yml` — needs manual apply or workflow-permission change first.
- **#166** — implemented locally (commit `9295b2d`) but safe-outputs bridge didn't push the PR. If bridge gets fixed, push this first; otherwise treat as next-fix-up work.

## Backlog cursor for Task 2 (Issue Comment)

- 11 open issues (1 mine, 1 test-improver, 1 perf-improver, 1 efficiency-improver, 1 aw, 6 agentic-workflows). Cursor parked on #132 (only human-authored). Re-engage when the maintainer or a new human comment appears.

## Completed work (PRs MERGED + current drafts)

- **#171** (mine, draft→merged) — `[repo-assist] refactor(autostart): extract resolveAutostartTarget preamble helper` (Closes #165). MERGED 2026-06-13T07:00:42Z as commit 904dd60. The previous run's local commit `03d587f` was applied by maintainer (or the bridge worked in that run).
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

- 2026-06-13 (run 27452358684, 01:30 UTC): Implemented #166 fix locally (commit 9295b2d on `repo-assist/fix-166-tui-table-render-helper`); build/vet/gofmt/test-race all clean. safe-outputs bridge persistence issue (5th consecutive): create_pull_request + update_issue both returned success but did not apply to GitHub. Patch + bundle artifacts saved.
- 2026-06-12 (run 27436420474, 19:07 UTC): Implemented #165 fix locally (commit 03d587f on `repo-assist/fix-165-autostart-preamble-helper`); build/vet/gofmt/test-race all clean. safe-outputs bridge persistence issue (4th consecutive): patch + bundle saved. (Note: that branch + commit were later MERGED on 2026-06-13T07:00:42Z as PR #171, so the bridge clearly worked once between runs — likely a transient race condition rather than a permanent failure.)
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

- **safe-outputs MCP bridge persistence issue (5th occurrence):** every `create_pull_request`/`add_comment`/`update_issue` call returned `success` at the HTTP layer but the writes did not appear on GitHub. This is now a stable, reproducible pattern, but **PR #171 did get pushed and merged** between runs, so the issue may be intermittent — perhaps related to MCP session lifecycle or repository state caching. Patch + bundle artifacts are saved to `/tmp/gh-aw/aw-*.{patch,bundle}`. The maintainer can apply with: `git am < /tmp/gh-aw/aw-repo-assist-fix-166-tui-table-render-helper.patch` (run from this checkout). **Configuration diagnosis needed:** the issue may be in `safe-outputs.create-pull-request` (not configured for non-protected pushes), in `update-issue: target: '*'` (not set so update targets the triggering issue, which doesn't exist for scheduled runs), or in the MCP server itself. **Recommend:** if this persists in the next run, file an issue to the agentics repo requesting diagnosis.
- **Posture:** revert rate is 0/15 over the life of the workflow. Production refactors paired with bot-flagged issues are demonstrably safe.
- **#132 design signal still pending.** Maintainer is in selective mode; v1 scaffold requires a maintainer signal on loop-mount persistence.
- **Branch name pattern:** Maintainer should be aware that `repo-assist/*-XXXXXXXX` (with SHA suffix) is the gh-aw orchestrator's standard; not something the workflow can suppress.
- **#100 size trend:** if it grows back past 9 KB, compress more aggressively by removing the 3-day-old run entries.
- **Next high-confidence action if safe-outputs is fixed:** push local `repo-assist/fix-166-tui-table-render-helper` (commit `9295b2d`) to origin + open PR. Work is already complete and tested.

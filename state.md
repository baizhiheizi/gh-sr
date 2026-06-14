---
name: repo-assist-state
description: Repo Assist persistent state — completed work, in-flight PRs, backlog cursor, and notes
metadata:
  type: project
---

# Repo Assist state — last updated 2026-06-14

## Last run (2026-06-14 03:48 UTC, run 27467295298)

- Selected tasks: 2 (Comment), 3 (Issue Fix), 6 (Maintain Repo Assist PRs).
- **Task 2** (Comment) — no-op: cursor parked on #132; new candidates are agentic-automated-analysis or workflow-tracker issues (#174/#175/#176/#177/#179), not human-originated, so left alone.
- **Task 3** (Issue Fix) — implemented #175 fix locally: extracted `resolveAndFilter(w, cfg, filterHost, filterRepo, nameArgs) ([]config.RunnerConfig, error)` helper in `internal/ops/ops.go`; migrated 12 call sites (Setup, Up, Down, Restart, RebuildImage, Update, Remove, CollectStatus, CollectDiskUsage, PruneDisk, ServiceInstall, ServiceUninstall, ServiceStatus). `applyContainerImageExtras` stays at the call site (rationale: only 4 sites have it, and a `bool` parameter would force it to broaden scope or add noise). `Logs` deliberately not migrated (single-runner flow with `FindRunnerForLogs`). Commit `94979db` on branch `repo-assist/fix-175-orchestrator-preamble-helper`. Build/vet/gofmt/test-race all clean across 12 packages. 5 new sub-tests in `TestResolveAndFilter`.
- **Task 6** (Maintain PRs) — flagged PR #181 as a duplicate of #180 via a comment on #181 (same title, same body, same diff, opened 16s apart from run 27460481325).
- **Task 11** (Activity) — updated #100 with this run's entry, removed the now-stale "Apply patch manually #176" line, promoted it to "Review PR #180/#181 (duplicate)". Added #175 patch-and-bundle line.
- **safe-outputs bridge:** `create_pull_request` for #175 returned `success` with patch/bundle artifacts but did not push to origin (7th consecutive occurrence of the persistence issue). The bridge DID push earlier this run (PRs #180/#181). Patch + bundle saved at `/tmp/gh-aw/aw-repo-assist-fix-175-orchestrator-preamble-helper.{patch,bundle}`. Maintainer can apply with `git am < /tmp/gh-aw/aw-repo-assist-fix-175-orchestrator-preamble-helper.patch`.

## In-flight work

- **1 local commit, no new open PR of mine on origin this run.** Local: `94979db` on `repo-assist/fix-175-orchestrator-preamble-helper`.
- **safe-outputs bridge persistence issue** — confirmed 7th occurrence this run. Intermittent (other workflows' PRs in the same window did push: #178, #180, #181). Patch + bundle artifacts are saved; maintainer can apply with `git am`.

## Backlog / next high-value task

- **#132 (gh sr storage — btrfs loop + reflink seed)** — human-authored design. v1 scaffold (subcommand skeleton in `cmd/gh-sr/`, `internal/storage/` package, status detection) is the natural next deliverable; **on hold** pending maintainer signal on the loop-mount persistence approach.
- **#174** — Severity High; ~120 lines duplicated. Natural follow-up to #171. Not yet implemented.
- **#175** — implemented locally (commit `94979db`) but safe-outputs bridge didn't push the PR. If bridge gets fixed, push this first; otherwise treat as next-fix-up work.
- **#124** — benchstat comparison on PRs (MEDIUM impact, infrastructure). Touches `.github/workflows/ci.yml` — needs manual apply or workflow-permission change first.

## Backlog cursor for Task 2 (Issue Comment)

- 17 open issues (0 unlabelled; 4 of mine + 1 test-improver + 1 aw-tracker). Cursor parked on #132 (only human-authored). Re-engage when the maintainer or a new human comment appears.

## Completed work (PRs MERGED + current drafts)

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

- 2026-06-14 (run 27467295298, 03:48 UTC): Implemented #175 fix locally (commit 94979db on `repo-assist/fix-175-orchestrator-preamble-helper`); build/vet/gofmt/test-race all clean. safe-outputs bridge persistence issue (7th consecutive): create_pull_request + update_issue both returned success; #175 PR did not push; #100 update applied. Commented on PR #181 flagging it as duplicate of #180.
- 2026-06-14 (run 27460481325, 03:30 UTC): Implemented #176 fix locally; build/vet/gofmt/test-race all clean. safe-outputs bridge did not push initially, but eventually produced PRs #180 and #181 (duplicate from parallel-run race).
- 2026-06-13 (run 27452358684, 01:30 UTC): Implemented #166 fix locally (commit 9295b2d); build/vet/gofmt/test-race all clean. safe-outputs bridge persistence issue (5th consecutive): patch + bundle saved. (Later: PR #172 did push and is now open.)
- 2026-06-12 (run 27436420474, 19:07 UTC): Implemented #165 fix locally (commit 03d587f on `repo-assist/fix-165-autostart-preamble-helper`); build/vet/gofmt/test-race all clean. (Later: PR #171 pushed and merged.)
- 2026-06-12 (run 27418691649, 13:34 UTC): Created PR #169 (LinuxElevatePreludeSoft for #163).
- 2026-06-12 (run 27402673134, 08:03 UTC): Created gofmt fix + CI check PR (dde5c83). Blocked by workflow file permission; issue #160/#161 filed.
- 2026-06-12 (run 27388413942, 02:21 UTC): Verified #157 clean, refreshed #100 (compressed), no new PRs.
- 2026-06-11 (run 27371229238, 19:25 UTC): Created PR #157 (loadYAMLRoot helper for #153).
- 2026-06-10 (run 27279843892): Commented on #142 (stale flag); created TestEscapePS PR (landed as #149).
- 2026-06-09 (run 27228879760): Created PR (draft) for #134 (disk-helpers extraction).
- 2026-06-09 (run 27191010253): Substantive comment on #132; verified all queued PRs merged.
- 2026-06-09 (run 27177519565): Held the line on 7 PRs awaiting review.
- 2026-06-08 and earlier: Created PRs #127, #118, #114, #113, #110, #102, #99 (all merged); held the line on multiple no-action runs.

## Notes for next run

- **safe-outputs MCP bridge persistence issue (7th occurrence):** `create_pull_request` for #175 returned `success` with patch/bundle artifacts but the PR did not appear on origin and the branch did not push. The bridge DID push PR #172 between prior runs, and PRs #180/#181 earlier this same run window. **Pattern: intermittent, not consistent.** Patch + bundle artifacts at `/tmp/gh-aw/aw-repo-assist-fix-175-orchestrator-preamble-helper.{patch,bundle}` — maintainer can apply with `git am < /tmp/gh-aw/aw-repo-assist-fix-175-orchestrator-preamble-helper.patch` from this checkout.
- **PR #181 is a duplicate of #180** — same title, same body, same diff, opened 16 s apart from the same workflow run (27460481325). This is the first time I've seen a duplicate-PR race in the orchestrator; recommended action is to close #181.
- **Posture:** revert rate is 0/15 over the life of the workflow. Production refactors paired with bot-flagged issues are demonstrably safe.
- **#132 design signal still pending.** Maintainer is in selective mode; v1 scaffold requires a maintainer signal on loop-mount persistence.
- **Branch name pattern:** Maintainer should be aware that `repo-assist/*-XXXXXXXX` (with SHA suffix) is the gh-aw orchestrator's standard; not something the workflow can suppress.
- **#100 size trend:** now ~7.2 KB compressed; if it grows past 9 KB, compress older run entries further.
- **Next high-confidence action if safe-outputs is fixed:** push local `repo-assist/fix-175-orchestrator-preamble-helper` (commit `94979db`) to origin + open PR. Work is already complete and tested.
- **#174 (Autostart active-check duplication)** is Severity High and would mirror the pattern of #171 and #175. Natural next fix once #175 lands.
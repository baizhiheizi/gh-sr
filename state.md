---
name: repo-assist-state
description: Repo Assist persistent state — completed work, in-flight PRs, backlog cursor, and notes
metadata:
  type: project
---

# Repo Assist state — last updated 2026-06-20 ~16:30 UTC

## Last run (2026-06-20 ~16:30 UTC, run 27882401019)

- Selected tasks: 2 (Issue Comment), 3 (Issue Fix), 5 (Coding Improvements).
- **Task 5** — branch `repo-assist/fix-issue-238-doctor-filter-collect-helpers`, commit `6f1e7af`: collapsed `uniqueRepos` / `uniqueOrgs` / `uniqueHostNames` (3 near-identical 10-13-line helpers) into `uniqueStringsBy(runners, key func(config.RunnerConfig) string) []string`; collapsed `nativeInstallTargetsForHost` / `containerInstallTargetsForHost` / `containerAgenticInstallTargetsForHost` (3 near-identical 11-13-line helpers) into `installTargetsForHost(runners, hostName, predicate func(*config.RunnerConfig) bool) [][2]string`. Empty-key skip now uniform across all three `uniqueStringsBy` call sites (fixes the latent asymmetry — `uniqueHostNames` previously did not skip empty hosts). Agentic predicate simplifies from `IsContainerMode() && IsAgentic()` to just `IsAgentic()`. All 6 call sites in `doctor.go` updated. Production: ~95 lines of duplicated bodies → ~30 lines of intent. Tests: `TestUniqueStringsBy` (3 sub-tests) replaces 2 prior tests; `TestInstallTargetsForHost` (5 sub-tests) replaces 3 prior tests. `Closes #238`.
- **Task 2** — no-op. Cursor parked on #132 (no new human activity since 2026-06-09).
- **Task 3** — no-op. No `bug` / `help wanted` / `good first issue` issues open. The new #236 / #237 / #238 are duplicate-code findings (code-quality), not bug-class.
- **Task 11** — `update_issue` on #100 succeeded (full body refresh). Bridge-pending line for #230 was replaced with PR #235 link; new bridge-pending line for #238 added.
- **Posted comment** on #238 (`aw_PVpC5wC4`) summarising the refactor.
- **Verified:** `go build ./...` OK, `go vet ./...` OK, `go test ./...` OK (12/12 packages), `go test -race ./...` OK, `gofmt -l internal/doctor/` clean.

## In-flight work

- **Branch `repo-assist/fix-issue-238-doctor-filter-collect-helpers`** — 1 commit (refactor + tests). Bridge pending.
- **PR #235 (DRAFT 2026-06-20)** — `[repo-assist] refactor(runner): collapse stale-registration probe` (closes #230). Bridge latency ~4h.
- **PR #234 (DRAFT 2026-06-20)** — `[repo-assist] refactor(runner): collapse image-build preamble` (closes #228).
- **PR #233 (DRAFT 2026-06-20)** — `[test-improver] test(ops): cover Logs orchestrator`.
- **Issue #225 (FALLBACK REVIEW — dep-update)** — patch downloadable from run 27822014346.

## Backlog / next high-value task

- **#236 (duplicate-code)** — `printAgenticFailures(w, hostName, prefix, failures)` helper in `internal/doctor/doctor.go` consolidating 2× 18-line failure-iteration blocks. ~30 min. ~36 → ~18 lines.
- **#237 (duplicate-code)** — `printHostBanner(w, verb, hostName, addr string)` helper in `internal/ops/` consolidating 5+1 inline local-vs-SSH banner blocks. ~15 min. ~24 → ~6+1 lines.
- **#132 (gh sr storage)** — on hold pending maintainer signal on loop-mount persistence.

## Backlog cursor for Task 2 (Issue Comment)

- 15 open issues (1 human = #132, 14 bot-generated including #225, #230, #236, #237, #238, plus monthly rollups #85/#100/#109/#124/#125/#214/#148). 0 unlabelled. #228 closed via PR #234.

## Completed work (recent PRs)

- **PR #235, #234, #233 (DRAFTS 2026-06-20)** — see "In-flight work" above.
- **PR #232 (MERGED 2026-06-20)** — disk.go dispatcher refactor + ops service orchestrator tests.
- **PR #226 (MERGED)** — perf: hoist ContainerImageLayoutRevision out of Status() loop.
- **PR #224 (MERGED)** — test(ops): Up orchestrator coverage 0% → 77.8%.
- **PR #223 (MERGED)** — delete dead containerLocalStatusFromDocker (closes #217).
- **PR #222 (MERGED)** — gofmt clean drift in 5 files.
- **PR #221 (MERGED)** — extract resolveStateDirOrFallback helper (closes #218).
- **PR #220 (MERGED)** — archive resolve-duplicate-code change.
- **PR #215 (MERGED)** — collapse 3 docker create env arg helpers (closes #209).
- **Earlier (compressed):** #211, #206, #202, #200, #199, #198, #197, #194, #193, #192, #191, #189, #187, #184, #183, #180, #178, #172; 2026-06-12/13 batch: #159, #158, #157, #156, #155, #149, #147, #146, #142, #141, #139, #136, #131, #128, #127, #126, #113, #114, #110, #118, #99.

## Activity / PR history (compressed)

- 2026-06-20 (run 27882401019, ~16:30 UTC): Tasks 2/3/5 — doctor.go filter-collect refactor (closes #238). 2 helpers replace 6.
- 2026-06-20 (run 27874659199, ~11:30 UTC): Tasks 4/2/3 — stale-registration probe refactor (closes #230). PR #235.
- 2026-06-20 (run 27867528048, ~09:30 UTC): Tasks 2/3/8 — image-build preamble refactor (closes #228). PR #234.
- 2026-06-19 (run 27846567496, ~22:30 UTC): Tasks 5/3/9 — disk.go dispatcher refactor (closes #229) + ops service tests. PR #232.

## Notes for next run

- **PR creation bridge latency: 30 min – 4h observed.** Pattern: `create_pull_request` returns `success` with patch/bundle artifacts; bridge pushes branch + opens PR afterwards. Confirmed for #232 (merged 2026-06-20), #235, #234, #183, #180, #172, #171.
- **#236 / #237 now the next-highest-priority deferred duplicate-code cleanups.** Both are small (~15-30 min) and low-risk.
- **#225 (dep-update fallback).** Patch downloadable from run 27822014346.
- **#100 body refresh.** `update_issue` succeeded this run. Recovery stable.
- **Posture:** revert rate is 0/18+ over the life of the workflow. Recent merges (#211, #215, #220, #221, #222, #223, #224, #226, #232) all landed cleanly.
- **Next high-confidence action if a new `bug` / `help wanted` / `good first issue` issue opens:** investigate and implement a minimal fix. None currently open.
- **Repo state:** 15 open issues (1 human = #132, 14 bot-generated). 0 unlabelled. 3 open Repo Assist PRs (#233/#234/#235) + this run's branch in bridge-pending state.

## Verified knowledge

- `safeoutputs` create_pull_request creates a **fallback review issue** when the patch modifies protected files (`go.mod`, `go.sum`, `CHANGELOG.md`, `.github/workflows/ci.yml`); branch IS pushed to origin but PR doesn't open.
- `safeoutputs` create_pull_request also sometimes returns `success` with patch/bundle artifacts when the agent has no git credentials. Patch at `/tmp/gh-aw/aw-*.patch` and bundle at `/tmp/gh-aw/aw-*.bundle` are the durable artifacts; the bridge typically pushes and opens the PR afterwards.
- `git log origin/repo-assist/<branch> -p -- go.mod` shows the branch's go.mod history — useful for verifying whether a dep-update was actually committed.
- `internal/ops/connectHostFn` is a package-level seam that tests swap via `installMockConnectHost` + `connectHostMu` for race-detector-clean parallel tests.
- `runOnHostOS[T any]` in `internal/runner/hostos.go` is the canonical host-OS dispatcher. Any new switch on `h.OS` in this package is a duplicate-code smell.
- `GitHubClient.GetLatestRunnerVersion()` uses `sync.Once` and caches the version+error. Repeat calls within one process are free — refactors that extract the version→arch→imageTag preamble don't lose the cache hit.
- `containerImageExists(h, imageTag)` swallows `h.Run` errors and returns `(false, nil)`. The `"checking image: %w"` wrapping in `buildRunnerImageIfMissing` is forward-compat but currently unreachable; `"building container runner image: %w"` IS reachable.
- `IsContainerMode()` returns `true` for both `RunnerMode: container` and `Profile: agentic` (via `EffectiveRunnerMode`). Therefore `IsContainerMode() && IsAgentic()` ≡ `IsAgentic()`. Used this simplification in the #238 agentic predicate.
- `uniqueStringsBy(runners, key)` is the canonical dedupe+sort helper in `internal/doctor/doctor.go` (added this run). Empty keys are skipped uniformly.
- `installTargetsForHost(runners, hostName, predicate)` is the canonical host-filter+instance-flatten helper in `internal/doctor/doctor.go` (added this run). Predicate signature `func(*config.RunnerConfig) bool` so callers can use any pointer-receiver method.
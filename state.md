---
name: repo-assist-state
description: Repo Assist persistent state — completed work, in-flight PRs, backlog cursor, and notes
metadata:
  type: project
---

# Repo Assist state — last updated 2026-06-21 ~16:30 UTC

## Last run (2026-06-21 ~16:30 UTC, run 27916163915)

- Selected tasks: 5 (Coding Improvements), 10 (Take Repo Forward), 4 (Engineering Investments).
- **Task 5** — branch `repo-assist/improve-issue-236-agentic-failures-helper`, commit `0fc1eb7`: added `printAgenticFailures(w io.Writer, hostName string, r *Result, defaultSev, prefix string, failures []agentic.PrereqFailure)` in `internal/doctor/doctor.go`. Helper owns severity classification (`SeverityError → sevFail + r.Fail++`, `SeverityWarning → sevWarn + r.Warn++`, anything else uses `defaultSev` and increments the matching counter), `printLine` rendering, multi-line remediation splitting, and DocRef rendering. Replaced 2× 18-line inline blocks in `checkContainerHostPrereqs` (`defaultSev=sevFail`, prefix `"container: "`) and `checkContainerAgenticInnerHygiene` (`defaultSev=sevWarn`, prefix `"container(agent): "`). `internal/doctor/doctor.go` 35-line net reduction; both call sites collapse to a single line. 7 new sub-tests (`SeverityError`, `SeverityWarning`, `DefaultSevFail`, `DefaultSevWarn`, `RemediationAndDocRef`, `EmptySlice`, `MixedSeverities`). Internal/doctor coverage 67.0% → 68.3%. Closes #236. Bridge-pending PR.
- **Task 10** — `add_comment` on #243 (`aw_zZSRJbZ7`): flagged #243 as a re-detection of the same 5 sites already addressed by PR #240 (closes #237). Detector re-fired against `775c939` today and surfaced an identical pattern with the same locations; suggested closing as superseded.
- **Task 4** — no-op. All valuable engineering investments are already in flight as fallback-review issues #241 (gofmt CI check) and #225 (deps bump) — both blocked by the protected-files list. No additional non-protected work identified this run.
- **Task 11** — `update_issue` on #100 succeeded (10 KB limit; truncated the oldest 2026-06-19 run entry to fit the new entry). Refreshed Suggested Actions: promoted #237 → PR #240 (open, draft), added PR #242 (test-improver Remove orchestrator), added bridge-pending entry for #236, added "Apply patch manually" lines for #225 and #241 (replace bridge-pending entries), added "Close issue" entry for #243 (re-detection), kept "Define goal" for #132. Future Work: removed #236 (now done); refreshed the next-duplicate-code cleanup note to point at the open PRs.
- **Verified:** `go build ./...` OK, `go vet ./...` OK, `go test ./... -race -count=1` OK (12/12 packages), `go test -race ./internal/doctor/...` OK (7 new sub-tests + all existing tests), `gofmt -l .` clean.

## In-flight work

- **Branch `repo-assist/improve-issue-236-agentic-failures-helper`** — 1 commit (refactor + 7 sub-tests). Bridge pending. Patch at `/tmp/gh-aw/aw-repo-assist-improve-issue-236-agentic-failures-helper.patch` (10 176 bytes).
- **PR #240 (DRAFT 2026-06-21)** — `[repo-assist] refactor(ops): collapse local-vs-SSH banner via writeHostBanner` (closes #237). Bridge-pending branch landed as PR.
- **PR #239 (DRAFT 2026-06-20)** — `[repo-assist] refactor(doctor): collapse uniqueX + installTargetsXForHost via generic helpers` (closes #238).
- **PR #235 (DRAFT 2026-06-20)** — `[repo-assist] refactor(runner): collapse stale-registration probe` (closes #230).
- **PR #234 (DRAFT 2026-06-20)** — `[repo-assist] refactor(runner): collapse image-build preamble` (closes #228).
- **PR #242 (DRAFT 2026-06-21)** — `[test-improver] test(ops): cover Remove orchestrator (0% → 94.1%)`.
- **PR #233 (DRAFT 2026-06-20)** — `[test-improver] test(ops): cover Logs orchestrator`.
- **Issue #241 (fallback-review 2026-06-21)** — `[repo-assist] ci: add gofmt drift check to PR pipeline`. Patch downloadable from run 27894682067.
- **Issue #225 (fallback-review 2026-06-19)** — `[repo-assist] deps: bump bubbletea v2.0.7, x/crypto v0.53.0, x/term v0.44.0`. Patch downloadable from run 27822014346.
- **Issue #243 (2026-06-21)** — re-detection of the same 5 sites already addressed by PR #240 / #237. `add_comment` posted (`aw_zZSRJbZ7`); recommended "Close issue".

## Backlog / next high-value task

- The remaining duplicate-code findings against `775c939` are all addressed by open PRs (#228 → #234, #230 → #235, #236 → bridge-pending, #237 → #240, #238 → #239). The detector is firing periodically against the same commit; future re-detections should be evaluated against in-flight refactors before starting new work.
- **#132 (gh sr storage)** — on hold pending maintainer signal on loop-mount persistence.

## Backlog cursor for Task 2 (Issue Comment)

- 16 open issues (1 human = #132, 15 bot-generated including #225, #230, #236, #237, #238, #241, #243, plus monthly rollups #85/#100/#109/#124/#125/#214/#148/#208). 0 unlabelled.

## Completed work (recent PRs)

- **Branch `repo-assist/improve-issue-236-agentic-failures-helper`** — printAgenticFailures refactor (closes #236). Bridge-pending.
- **PR #240 (DRAFT 2026-06-21)** — writeHostBanner refactor (closes #237). 1 helper + 5 call sites.
- **PR #239 (DRAFT 2026-06-20)** — doctor filter-collect refactor (closes #238).
- **PR #235 (DRAFT 2026-06-20)** — stale-registration probe refactor (closes #230).
- **PR #234 (DRAFT 2026-06-20)** — image-build preamble refactor (closes #228).
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

- 2026-06-21 (run 27916163915, ~16:30 UTC): Tasks 5/10/4 — printAgenticFailures refactor (closes #236) + comment on #243 (re-detection of #237 pattern).
- 2026-06-21 (run 27894682067, ~14:30 UTC): Tasks 5/4/2 — banner helper (closes #237) + gofmt CI check + comment on #237.
- 2026-06-20 (run 27882401019, ~16:30 UTC): Tasks 2/3/5 — doctor.go filter-collect refactor (closes #238). 2 helpers replace 6.
- 2026-06-20 (run 27874659199, ~11:30 UTC): Tasks 4/2/3 — stale-registration probe refactor (closes #230). PR #235.
- 2026-06-20 (run 27867528048, ~09:30 UTC): Tasks 2/3/8 — image-build preamble refactor (closes #228). PR #234.
- 2026-06-19 (run 27846567496, ~22:30 UTC): Tasks 5/3/9 — disk.go dispatcher refactor (closes #229) + ops service tests. PR #232.

## Notes for next run

- **PR creation bridge latency: 30 min – 5h observed.** Pattern: `create_pull_request` returns `success` with patch/bundle artifacts; bridge pushes branch + opens PR afterwards. Confirmed for #232 (merged 2026-06-20), #240 (open draft), #239, #235, #234, #233, #183, #180, #172, #171.
- **#241 (gofmt CI check)** — fallback-review issue, maintainer needs to apply the patch from run 27894682067 manually.
- **#243 (re-detection)** — same 5 sites as #237. Recommended "Close issue" in the suggested-actions list.
- **Posture:** revert rate is 0/18+ over the life of the workflow. Recent merges (#211, #215, #220, #221, #222, #223, #224, #226, #232) all landed cleanly.
- **Next high-confidence action if a new `bug` / `help wanted` / `good first issue` issue opens:** investigate and implement a minimal fix. None currently open.
- **Repo state:** 16 open issues (1 human = #132, 15 bot-generated). 0 unlabelled. 6 open Repo Assist PRs (#233/#234/#235/#239/#240/#242) + 1 test-improver PR + 1 bridge-pending branch (#236) + 2 fallback-review issues (#225, #241).

## Verified knowledge

- `safeoutputs` create_pull_request creates a **fallback review issue** when the patch modifies protected files (`go.mod`, `go.sum`, `CHANGELOG.md`, `.github/workflows/ci.yml`); branch IS pushed to origin but PR doesn't open.
- `safeoutputs` create_pull_request also sometimes returns `success` with patch/bundle artifacts when the agent has no git credentials. Patch at `/tmp/gh-aw/aw-*.patch` and bundle at `/tmp/gh-aw/aw-*.bundle` are the durable artifacts; the bridge typically pushes and opens the PR afterwards.
- `git log origin/repo-assist/<branch> -p -- go.mod` shows the branch's go.mod history — useful for verifying whether a dep-update was actually committed.
- `internal/ops/connectHostFn` is a package-level seam that tests swap via `installMockConnectHost` + `connectHostMu` for race-detector-clean parallel tests.
- `runOnHostOS[T any]` in `internal/runner/hostos.go` is the canonical host-OS dispatcher. Any new switch on `h.OS` in this package is a duplicate-code smell.
- `GitHubClient.GetLatestRunnerVersion()` uses `sync.Once` and caches the version+error. Repeat calls within one process are free — refactors that extract the version→arch→imageTag preamble don't lose the cache hit.
- `containerImageExists(h, imageTag)` swallows `h.Run` errors and returns `(false, nil)`. The `"checking image: %w"` wrapping in `buildRunnerImageIfMissing` is forward-compat but currently unreachable; `"building container runner image: %w"` IS reachable.
- `IsContainerMode()` returns `true` for both `RunnerMode: container` and `Profile: agentic` (via `EffectiveRunnerMode`). Therefore `IsContainerMode() && IsAgentic()` ≡ `IsAgentic()`. Used this simplification in the #238 agentic predicate.
- `uniqueStringsBy(runners, key)` is the canonical dedupe+sort helper in `internal/doctor/doctor.go`. Empty keys are skipped uniformly.
- `installTargetsForHost(runners, hostName, predicate)` is the canonical host-filter+instance-flatten helper in `internal/doctor/doctor.go`. Predicate signature `func(*config.RunnerConfig) bool` so callers can use any pointer-receiver method.
- `writeHostBanner(w, prefix, addr)` is the canonical local-vs-SSH banner helper in `internal/ops/ops.go`. `prefix` is everything up to the suffix; the helper handles the `IsLocalAddr` toggle between `(local)` and `(<addr>)`. Callers compose the prefix with whatever verb/object they need.
- `printAgenticFailures(w, hostName, r, defaultSev, prefix, failures)` is the canonical agentic-failure renderer in `internal/doctor/doctor.go` (added this run). `defaultSev` (sevFail or sevWarn) governs the severity for any failure whose Severity is neither `SeverityError` nor `SeverityWarning`. Both doctor check sites (`checkContainerHostPrereqs` and `checkContainerAgenticInnerHygiene`) now use it.
- `.github/workflows/ci.yml` is on the protected-files list — modifications route through the fallback-review issue path; bridge push may not open the PR automatically.
- `update_issue` body limit is 10 KB. Long Run History entries must be truncated before each monthly refresh.
- The duplicate-code detector fires periodically against `main` (currently `775c939`). Each new finding should be cross-checked against in-flight PRs before starting work — #243 today re-surfaced the same 5 sites #237 covers; recommended "Close issue".
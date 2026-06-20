---
name: repo-assist-state
description: Repo Assist persistent state — completed work, in-flight PRs, backlog cursor, and notes
metadata:
  type: project
---

# Repo Assist state — last updated 2026-06-20 ~09:30 UTC

## Last run (2026-06-20 ~09:30 UTC, run 27867528048)

- Selected tasks: 2 (Issue Comment), 3 (Issue Fix), 8 (Performance Improvements).
- **Task 8 (Performance Improvements)** — branch `repo-assist/improve-image-preamble-2026-06-20`, commit `fb264cb`: extracted `(m *Manager) resolveRunnerImageInputs(h)` (version→arch→imageTag preamble) and `(m *Manager) buildRunnerImageIfMissing(h, imageTag, version, arch) (built bool, err error)` (exists-check + build pair). `setupContainer` uses both with log granularity preserved via `built` bool. `rebuildContainerImage` uses the resolver only (keeps intentional `docker rmi -f` + unconditional build). `ContainerEnvironment.Provision` uses both; body drops from 14 lines of structural noise to 4 lines of intent. 5 new tests in `internal/runner/container_test.go` + small `releaseHandler` fixture. Error wrapping preserved verbatim. `Closes #228`.
- **Task 2 (Issue Comment)** — no-op. Cursor parked on #132 (no new human activity since 2026-06-09); no engagement candidates in the bot-generated duplicates cluster.
- **Task 3 (Issue Fix)** — no-op. No issues labelled `bug` / `help wanted` / `good first issue` open.
- **Task 11 (Activity)** — `update_issue` on #100 succeeded (full body refresh this run).
- **Posted comment** on #228 (`aw_BKhhnNCs`) linking to the PR with rationale, test status, and helper design notes.
- **safe-outputs `create_pull_request` behavior** — returned `success` with patch/bundle artifacts at `/tmp/gh-aw/aw-repo-assist-improve-image-preamble-2026-06-20.{patch,bundle}` (16004 bytes patch, 4367 bytes bundle). Branch was NOT pushed to origin. Same pattern as the prior 4+ runs.
- **Verified:** `go build ./...` OK, `go vet ./...` OK, `go test ./...` OK (12/12 packages), `go test -race ./internal/runner/...` OK, `gofmt -l internal/runner/` clean.

## In-flight work

- **Branch `repo-assist/improve-image-preamble-2026-06-20`** — 1 commit (refactor + tests). Patch at `/tmp/gh-aw/aw-repo-assist-improve-image-preamble-2026-06-20.patch`. Bridge pending.
- **Issue #225 (FALLBACK REVIEW — dep-update)** — `[repo-assist] deps: bump bubbletea v2.0.7, x/crypto v0.53.0, x/term v0.44.0`. Patch downloadable from run 27822014346.

## Backlog / next high-value task

- **#230 (stale-registration probe Win/POSIX)** — `staleRegistrationScript(h, instanceName)` helper co-locating `staleRegistrationMsg` with both call sites in `internal/runner/native.go`. ~45-min refactor; now the highest-priority deferred item since #228 was closed this run. Next-run candidate.
- **#132 (gh sr storage)** — on hold pending maintainer signal on loop-mount persistence.

## Backlog cursor for Task 2 (Issue Comment)

- 13 open issues (1 human = #132, 12 bot-generated including #225, #228, #229, #230, plus the Monthly Activity rollups #85/#100/#109/#124/#125/#214/#148). 0 unlabelled. #218 closed by maintainer earlier; #209 closed via PR #215; #217 closed via PR #223; #210 closed as not_planned (superseded by #228). #228 closed this run via PR draft. Duplicate-code cluster: #208 (parent) + #210 (closed) + #219 (closed) + #228 (closed) + #229 (closed) + #230.

## Completed work (PRs MERGED + current drafts)

- **PR #232 (MERGED 2026-06-20)** — `[repo-assist] refactor(runner): route disk.go dispatchers through runOnHostOS + test(ops): cover service orchestrators`. Bridge recovered from run 27846567496.
- **PR #226 (MERGED 2026-06-19 12:05 UTC)** — `[efficiency-improver] perf(runner): hoist ContainerImageLayoutRevision out of Status() per-instance loop`.
- **PR #224 (MERGED 2026-06-19 10:21 UTC)** — `[test-improver] test(ops): cover Up orchestrator (0% → 77.8%)`.
- **PR #223 (MERGED 2026-06-19 10:20 UTC)** — `[repo-assist] refactor(runner): delete dead containerLocalStatusFromDocker (closes #217)`.
- **PR #222 (MERGED 2026-06-19 10:20 UTC)** — `[repo-assist] style: gofmt clean drift in 5 files`.
- **PR #221 (MERGED 2026-06-19 02:44 UTC)** — `[repo-assist] refactor(runner): extract resolveStateDirOrFallback helper (Closes #218)`.
- **PR #220 (MERGED 2026-06-19 02:42 UTC)** — `[repo-assist] chore(openspec): archive resolve-duplicate-code change`.
- **PR #215 (MERGED 2026-06-19 02:43 UTC)** — `[repo-assist] refactor(runner): collapse 3 docker create env arg helpers via dockerCreateEnvLineIf` (Closes #209).
- **PR #213 / #212 (CLOSED, efficiency-improver)** — `Manager.Status` loop-invariant hoist. PR #213 closed as duplicate of #212.
- **PR #211 (MERGED 2026-06-18)** — `positiveIntOrDefault` helper (closes #207).
- **PR #206 / #202 / #200 / #199 (MERGED 2026-06-17/18)** — perf + test batch.
- **Earlier merges:** #198, #197, #194, #193, #192, #191, #189, #187, #184, #183, #180, #178, #172; 2026-06-12/13 batch: #159, #158, #157, #156, #155, #149, #147, #146, #142, #141, #139, #136, #131, #128, #127, #126, #113, #114, #110, #118, #99.

## Activity / PR history (compressed)

- 2026-06-20 (run 27867528048, ~09:30 UTC): Tasks 2/3/8 — image-build preamble refactor (closes #228). 5 new tests in `internal/runner/container_test.go`. `update_issue` on #100 succeeded (full body refresh). `add_comment` on #228 (aw_BKhhnNCs).
- 2026-06-19 (run 27846567496, ~22:30 UTC): Tasks 5/3/9 — disk.go dispatcher refactor (closes #229) + 11 new ops service orchestrator tests. ops coverage 44.0% → 50.2%. `update_issue` on #100 succeeded. safe-outputs `create_pull_request` returned success with patch/bundle only (bridge-pending).
- 2026-06-19 (run 27835554189, ~12:10 UTC): Tasks 4/2/1 no-op; #225 dep-update fallback. #100 add_comment (`aw_run27835554`).
- 2026-06-19 (run 27822014346, ~11:13 UTC): dep update PR attempt REJECTED by safeoutputs bridge; created fallback issue #225.
- 2026-06-19 (run 27807621567, ~02:50 UTC): gofmt cleanup PR (#222, merged); dead-code deletion PR (#223, merged); #219 supersession comment. PRs #224 also landed.

## Notes for next run

- **PR creation bridge is intermittent.** Last 5 runs returned `success` with patch/bundle artifacts only. Bridge recovery observed for past runs (#183, #180, #172, #171) AND just-confirmed: run 27846567496's branch `repo-assist/improve-runonhostos-disk-2026-06-19` eventually surfaced as PR #232 (merged 2026-06-20). Pattern: patch + bundle at `/tmp/gh-aw/` are the durable artifacts; the bridge typically pushes the branch and opens the PR afterwards (sometimes within minutes, sometimes days).
- **#228 closed this run.** Draft PR has the same bridge-pending pattern as #232's predecessor. If the bridge is healthy, expect PR `repo-assist/improve-image-preamble-2026-06-20` to surface within 24-72h.
- **#230 now the highest-priority deferred duplicate-code cleanup.** `staleRegistrationScript(h, instanceName)` helper + named `staleRegistrationWarmup` const. ~45 min. The recent `runOnHostOS` refactor (PR #232 / #231) and the `resolveRunnerImageInputs` refactor (this run) demonstrate a stable pattern: extract a small OS-aware helper to co-locate duplicated boilerplate.
- **#225 (dep-update fallback).** Patch downloadable from run 27822014346 (`gh run download 27822014346 -n agent`). Branch `origin/repo-assist/deps-update-2026-06-19-e8db2354d6a0e4fc` is at stale tip (PR #224).
- **#100 body refresh.** `update_issue` on #100 succeeded this run. Previous run (27846567496) also succeeded. Recovery stable.
- **Posture:** revert rate is 0/18+ over the life of the workflow. Recent merges (#211, #215, #220, #221, #222, #223, #224, #226, #232) all landed cleanly. This run's branch (image-preamble refactor) is the first bridge-pending artifact for the current week.
- **Next high-confidence action if a new `bug` / `help wanted` / `good first issue` issue opens:** investigate and implement a minimal fix. None currently open.
- **Repo state:** 13 open issues (1 human = #132, 12 bot-generated including #225 dep-update fallback, #230 duplicate-code, monthly rollups). 0 unlabelled. 0 open Repo Assist PRs (this run's branch in bridge-pending state).

## Verified knowledge

- `safeoutputs` create_pull_request creates a **fallback review issue** when the patch modifies protected files (`go.mod`, `go.sum`, `CHANGELOG.md`, `.github/workflows/ci.yml`, etc.) — same handler as for `push_to_pull_request_branch` rejection. The branch IS pushed to origin (visible as `origin/repo-assist/<desc>-<sha>`); the PR just doesn't open. Issue body contains manual-apply instructions.
- `safeoutputs` create_pull_request **also** sometimes returns `success` with patch/bundle artifacts when the agent has no git credentials (all runs in this environment). The patch at `/tmp/gh-aw/aw-*.patch` and bundle at `/tmp/gh-aw/aw-*.bundle` are the durable artifacts; the bridge typically pushes and opens the PR on its own afterwards. **CONFIRMED via run 27846567496 → PR #232 (merged 2026-06-20).** Bridge recovery is reliable but slow (24-72h observed).
- `git log origin/repo-assist/<branch> -p -- go.mod` shows the branch's go.mod history — useful for verifying whether a dep-update was actually committed or only patched.
- `internal/ops/connectHostFn` is a package-level seam that tests swap via `installMockConnectHost` + `connectHostMu` for race-detector-clean parallel tests.
- `autostart.Detect` on Linux uses `os.Stat` of `$HOME/.config/systemd/user/<base>.timer`, so it's coupled to the test runner's filesystem. Tests that need a specific `Kind` either mock the executor's `systemctl --user is-active` (when KindSystemdUser is detected) or rely on the absence of the timer file (KindNone — the default in test envs).
- `runOnHostOS[T any]` in `internal/runner/hostos.go` is the canonical host-OS dispatcher (Windows/POSIX/default-error). Any new switch on `h.OS` in this package is a duplicate-code smell.
- `GitHubClient.GetLatestRunnerVersion()` uses `sync.Once` and caches the version+error in `g.latestVersion` / `g.latestVersionErr`. Repeat calls within one process are free — refactors that extract the version→arch→imageTag preamble into a helper don't lose the cache hit.
- `containerImageExists(h, imageTag)` returns `(bool, error)` but the implementation **swallows** `h.Run` errors and returns `(false, nil)`. The `"checking image: %w"` error wrapping in the new `buildRunnerImageIfMissing` helper is forward-compat (preserves the historical error message) but currently unreachable via the helper's normal call path. The `"building container runner image: %w"` wrapping IS reachable (when the build itself fails).
- `releaseHandler(tag string)` is a small httptest fixture in `internal/runner/container_test.go` (added this run) that centralizes the runner-release JSON response. Reusable for future tests of the image-build path.

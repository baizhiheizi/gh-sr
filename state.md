---
name: repo-assist-state
description: Repo Assist persistent state — completed work, in-flight PRs, backlog cursor, and notes
metadata:
  type: project
---

# Repo Assist state — last updated 2026-06-19 ~22:30 UTC

## Last run (2026-06-19 ~22:30 UTC, run 27846567496)

- Selected tasks: 5 (Coding Improvements), 3 (Issue Fix), 9 (Testing Improvements).
- **Task 5 (Coding Improvements)** — branch `repo-assist/improve-runonhostos-disk-2026-06-19`, commit `22c7600`: replaced 3 hand-rolled switch dispatchers in `internal/runner/disk.go` (`dirSizes`, `clearWorkTemp`, `removeDirTree`) with the existing `runOnHostOS` generic helper. Added a small `dirSizesResult` struct for the 4-int64 result. Public function signatures unchanged. `Closes #229`.
- **Task 9 (Testing Improvements)** — commit `31508ee`: added 11 new tests in `internal/ops/` for `ServiceInstall` (5), `ServiceUninstall` (3), `ServiceStatus` (3). Plus `TestDiskDispatchers_unsupportedHostOS` regression guard in `internal/runner/disk_test.go`. `internal/ops` coverage 44.0% → 50.2% (+6.2 pp).
- **Task 3 (Issue Fix)** — no-op. No issues labelled `bug` / `help wanted` / `good first issue` open. Fallback to Task 2 also no-op: cursor parked on #132 (no new human activity since 2026-06-09), no engagement candidates with new comments.
- **Task 11 (Activity)** — `update_issue` on #100 (this run; bridge recovered — body refresh succeeded for the first time since run 27835554189).
- **safe-outputs `create_pull_request` behavior** — returned `success` with patch/bundle artifacts at `/tmp/gh-aw/aw-repo-assist-improve-runonhostos-disk-2026-06-19.{patch,bundle}` (23434 bytes patch, 7227 bytes bundle). Branch was NOT pushed to origin (verified via `git ls-remote`). Same pattern as #225 dep-update fallback. Bridge recovery observed in past runs (PR #183/#180/#172/#171 pushed after a delay). Patch + bundle are the durable artifacts.
- **Verified:** `go build ./...` OK, `go vet ./...` OK, `go test ./...` OK (12/12 packages), `go test -race ./...` OK, `gofmt -l internal/runner/ internal/ops/` clean.

## In-flight work

- **Branch `repo-assist/improve-runonhostos-disk-2026-06-19`** — 2 commits (refactor + tests). Patch at `/tmp/gh-aw/aw-repo-assist-improve-runonhostos-disk-2026-06-19.patch`. Bridge pending.
- **Issue #225 (FALLBACK REVIEW — dep-update)** — `[repo-assist] deps: bump bubbletea v2.0.7, x/crypto v0.53.0, x/term v0.44.0`. Branch `origin/repo-assist/deps-update-2026-06-19-e8db2354d6a0e4fc` at stale tip; patch downloadable from run 27822014346.

## Backlog / next high-value task

- **#228 (image-build preamble triplicated)** — high-severity duplicate of closed #210. `resolveRunnerImageInputs` + `buildRunnerImageIfMissing` helpers on `*Manager`. 1-hour refactor; deferred to next run with confirmation. The previously-pending question on #210's helper signature (3-return vs 2-return) was resolved by closure — #210 closed as not_planned on 2026-06-19 (superseded by #228).
- **#230 (stale-registration probe Win/POSIX)** — `staleRegistrationScript(h, instanceName)` helper co-locating `staleRegistrationMsg` with both call sites in `internal/runner/native.go`. ~45-min refactor; deferred.
- **#132 (gh sr storage)** — on hold pending maintainer signal on loop-mount persistence.

## Backlog cursor for Task 2 (Issue Comment)

- 14 open issues (1 human = #132, 13 bot-generated including #225, #228, #229, #230, plus the Monthly Activity rollups #85/#100/#109/#124/#125). 0 unlabelled. #218 closed by maintainer earlier today; #209 closed via PR #215; #217 closed via PR #223; #210 closed as not_planned (superseded by #228). Duplicate-code cluster: #208 (parent) + #210 (closed) + #219 (closed) + #228 + #229 + #230.

## Completed work (PRs MERGED + current drafts)

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

- 2026-06-19 (run 27846567496, ~22:30 UTC): Tasks 5/3/9 — disk.go dispatcher refactor (closes #229) + 11 new ops service orchestrator tests. ops coverage 44.0% → 50.2%. `update_issue` on #100 succeeded. safe-outputs `create_pull_request` returned success with patch/bundle only (bridge-pending).
- 2026-06-19 (run 27835554189, ~12:10 UTC): Tasks 4/2/1 no-op; #225 dep-update fallback. #100 add_comment (`aw_run27835554`).
- 2026-06-19 (run 27822014346, ~11:13 UTC): dep update PR attempt REJECTED by safeoutputs bridge; created fallback issue #225.
- 2026-06-19 (run 27807621567, ~02:50 UTC): gofmt cleanup PR (#222, merged); dead-code deletion PR (#223, merged); #219 supersession comment. PRs #224 also landed.
- 2026-06-18 (run 27789197909, ~19:30 UTC): `resolveStateDirOrFallback` for #218 → PR #221 (merged).
- 2026-06-18 (run 27772439940, ~18:30 UTC): openspec archive → PR #220 (merged).
- 2026-06-18 (run 27754636694): `dockerCreateEnvLineIf` for #209 → PR #215 (merged).

## Notes for next run

- **PR creation bridge is intermittent.** All four runs this week returned `success` with patch/bundle artifacts only; the patch path and bundle file at `/tmp/gh-aw/` are the durable artifacts. Bridge recovery observed for past runs (#183, #180, #172, #171). For protected-file changes (go.mod, go.sum, CHANGELOG.md, .github/workflows/ci.yml), the bridge intentionally opens a fallback review issue instead.
- **#225 (dep-update fallback).** Patch downloadable from run 27822014346 (`gh run download 27822014346 -n agent`). Branch `origin/repo-assist/deps-update-2026-06-19-e8db2354d6a0e4fc` is at stale tip (PR #224).
- **#100 body refresh.** `update_issue` on #100 succeeded this run (run 27846567496). Previous runs (27835554189, 27822014346, 27807621567) had to use `add_comment` as fallback. Recovery reason unknown — body size now ~5.6 KB (under the 14 KB MCP limit).
- **#228 (image-build preamble) deferred.** Higher-severity than #229 (which this run closed). The previously-blocking question on helper signature (3-return vs 2-return) was resolved by closing #210 as not_planned; #228 has a clear two-helper approach (`resolveRunnerImageInputs` + `buildRunnerImageIfMissing`). Next-run candidate.
- **#210 closed as not_planned (superseded by #228).** Maintained signal: the helper signature discussion can move forward when #228 is addressed.
- **Posture:** revert rate is 0/18+ over the life of the workflow. Recent merges (#211, #215, #220, #221, #222, #223, #224, #226) all landed cleanly. This run's branch (disk.go refactor + ops tests) is the first bridge-pending artifact for the current week.
- **Next high-confidence action if a new `bug` / `help wanted` / `good first issue` issue opens:** investigate and implement a minimal fix. None currently open.
- **Repo state:** 14 open issues (1 human = #132, 13 bot-generated including #225 dep-update fallback, #228/#229/#230 duplicate-code). 0 unlabelled. 0 open Repo Assist PRs (branch in bridge-pending state).

## Verified knowledge

- `safeoutputs` create_pull_request creates a **fallback review issue** when the patch modifies protected files (`go.mod`, `go.sum`, `CHANGELOG.md`, `.github/workflows/ci.yml`, etc.) — same handler as for `push_to_pull_request_branch` rejection. The branch IS pushed to origin (visible as `origin/repo-assist/<desc>-<sha>`); the PR just doesn't open. Issue body contains manual-apply instructions.
- `safeoutputs` create_pull_request **also** sometimes returns `success` with patch/bundle artifacts when the agent has no git credentials (all runs in this environment). The patch at `/tmp/gh-aw/aw-*.patch` and bundle at `/tmp/gh-aw/aw-*.bundle` are the durable artifacts; the bridge typically pushes and opens the PR on its own afterwards (observed for past runs).
- `git log origin/repo-assist/<branch> -p -- go.mod` shows the branch's go.mod history — useful for verifying whether a dep-update was actually committed or only patched.
- `internal/ops/connectHostFn` is a package-level seam that tests swap via `installMockConnectHost` + `connectHostMu` for race-detector-clean parallel tests.
- `autostart.Detect` on Linux uses `os.Stat` of `$HOME/.config/systemd/user/<base>.timer`, so it's coupled to the test runner's filesystem. Tests that need a specific `Kind` either mock the executor's `systemctl --user is-active` (when KindSystemdUser is detected) or rely on the absence of the timer file (KindNone — the default in test envs).
- `runOnHostOS[T any]` in `internal/runner/hostos.go` is the canonical host-OS dispatcher (Windows/POSIX/default-error). Any new switch on `h.OS` in this package is a duplicate-code smell.
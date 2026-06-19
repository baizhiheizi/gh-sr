---
name: repo-assist-state
description: Repo Assist persistent state — completed work, in-flight PRs, backlog cursor, and notes
metadata:
  type: project
---

# Repo Assist state — last updated 2026-06-19 ~12:10 UTC

## Last run (2026-06-19 ~12:10 UTC, run 27835554189)

- Selected tasks: 4 (Engineering Investments), 2 (Issue Comment), 1 (Issue Labelling).
- **Task 4 (Engineering Investments)** — Dep-update attempt from prior run (run 27822014346) was **rejected by safeoutputs bridge** because patch modified protected files (`go.mod`/`go.sum`). Bridge created fallback **review issue #225** with patch + manual-apply instructions. Verified `origin/repo-assist/deps-update-2026-06-19-e8db2354d6a0e4fc` exists on remote but is parked at prior-run tip (`6756e86` = PR #224); the dep-update diff is in the patch (downloadable from run 27822014346 via `gh run download`), NOT committed to the branch. No other high-value engineering investment actionable; gofmt already clean (PR #222 merged); CI on latest actions versions.
- **Task 2 (Issue Comment)** — No new engagement opportunities. #132 / #210 still awaiting maintainer signal. All open issues accounted for.
- **Task 1 (Issue Labelling)** — 0 unlabelled issues (audit confirmed). Fallback to Task 2 — no-op.
- **Task 11 (Activity)** — `add_comment` on #100 (`aw_run27835554`) carrying this run's record.

## In-flight work

- **Issue #225 (FALLBACK REVIEW — created by safeoutputs bridge for blocked dep-update PR)** — `[repo-assist] deps: bump bubbletea v2.0.7, x/crypto v0.53.0, x/term v0.44.0`. Branch `origin/repo-assist/deps-update-2026-06-19-e8db2354d6a0e4fc` exists but at stale tip. Patch downloadable from run 27822014346. Same pattern as #160/#161/#190/#201 — manual apply by maintainer.
- **PR #226 (MERGED 2026-06-19 12:05 UTC)** — `[efficiency-improver] perf(runner): hoist ContainerImageLayoutRevision out of Status() per-instance loop`.
- **PR #224 (MERGED 2026-06-19 10:21 UTC)** — `[test-improver] test(ops): cover Up orchestrator (0% → 77.8%)`.
- **PR #223 (MERGED 2026-06-19 10:20 UTC)** — `[repo-assist] refactor(runner): delete dead containerLocalStatusFromDocker (closes #217)`.
- **PR #222 (MERGED 2026-06-19 10:20 UTC)** — `[repo-assist] style: gofmt clean drift in 5 files`.

## Backlog / next high-value task

- **#225 (dep-update fallback)** — added to manual-apply list. Branch needs fast-forward + patch re-applied, OR wait for repo-assist to re-attempt in future run.
- **#210 (`resolveRunnerImage` block — 3 sites, 2 files)** — needs maintainer signal on the helper signature (3-return `resolveRunnerImage` vs. 2-return `resolveRunnerImageTag`). On hold.
- **#132 (gh sr storage — btrfs loop + reflink seed)** — human-authored design. On hold pending maintainer signal on loop-mount persistence.
- **#160 / #161 (gofmt CI check)** — manual `ci.yml` apply by maintainer. **The gofmt-drift portion landed via PR #222** (merged 2026-06-19T10:20:28Z), so only the CI check itself remains.
- **#190 / #201 (docs PR placeholders)** — manual `CHANGELOG.md` apply by maintainer.

## Backlog cursor for Task 2 (Issue Comment)

- 12 open issues (1 human = #132, 11 bot-generated including #225). #218 closed by maintainer earlier today; #209 closed via PR #215; #217 closed via PR #223. Duplicate-code cluster: #208 (parent) + #210 + #219 (superseded). No new human activity since prior runs. Unlabelled issues = 0.

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

- 2026-06-19 (run 27835554189, ~12:10 UTC): Tasks 4/2/1 no-op (deferred to #225 fallback review, no new engagement, 0 unlabelled). #100 add_comment (`aw_run27835554`).
- 2026-06-19 (run 27822014346, ~11:13 UTC): dep update PR attempt (bubbletea/x/crypto/x/term) — REJECTED by safeoutputs bridge (protected go.mod/go.sum); created fallback issue #225. #100 add_comment (`aw_run27822014`).
- 2026-06-19 (run 27807621567, ~02:50 UTC): gofmt cleanup PR (5 files, draft); dead-code deletion PR (closes #217, draft); #219 supersession confirmation comment. Tasks 10/5/3. PRs #222 + #223 landed 2026-06-19 10:20 UTC; PR #224 (test-improver Up orchestrator) landed 2026-06-19 10:21 UTC.
- 2026-06-18 (run 27789197909, ~19:30 UTC): `resolveStateDirOrFallback` for #218 → PR #221 (merged 2026-06-19).
- 2026-06-18 (run 27772439940, ~18:30 UTC): openspec archive → PR #220 (merged 2026-06-19).
- 2026-06-18 (run 27754636694): `dockerCreateEnvLineIf` for #209 → PR #215 (merged 2026-06-19).
- 2026-06-17 (3 runs): refactor container config accessors → PR #211 (merged); perf metrics SplitSeq → PR #206; perf container parse → PR #202.

## Notes for next run

- **#225 (dep-update fallback).** Patch downloadable from run 27822014346 (`gh run download 27822014346 -n agent`). Branch `origin/repo-assist/deps-update-2026-06-19-e8db2354d6a0e4fc` is at stale tip (PR #224) — needs fast-forward to current main + rebase, OR the maintainer can re-run repo-assist to regenerate the patch.
- **#100 body refresh.** Same persistence issue persists (14KB MCP limit). `add_comment` is the durable fallback (this run: `aw_run27835554`).
- **#210 / #132 design signals still pending.** Maintainer in selective mode.
- **gofmt drift now clean.** PR #222 landed; only the CI check (PR #160/#161) remains for maintainer manual apply.
- **Posture:** revert rate is 0/18+ over the life of the workflow. Recent merges (#211, #215, #220, #221, #222, #223, #224, #226) all landed cleanly.
- **Next high-confidence action if a new `bug` / `help wanted` / `good first issue` issue opens:** investigate and implement a minimal fix. None currently open.
- **Hot path strings.Split cleanup: complete.** All remaining call sites are deliberate one-shot paths.
- **Repo state:** 12 open issues (1 human = #132, 11 bot-generated including #225 dep-update fallback). 0 unlabelled. 0 open Repo Assist PRs.

## Verified knowledge

- `safeoutputs` create_pull_request creates a **fallback review issue** when the patch modifies protected files (`go.mod`, `go.sum`, `CHANGELOG.md`, `.github/workflows/ci.yml`, etc.) — same handler as for `push_to_pull_request_branch` rejection. The branch IS pushed to origin (visible as `origin/repo-assist/<desc>-<sha>`); the PR just doesn't open. Issue body contains manual-apply instructions.
- `git log origin/repo-assist/<branch> -p -- go.mod` shows the branch's go.mod history — useful for verifying whether a dep-update was actually committed or only patched.

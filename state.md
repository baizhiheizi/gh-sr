---
name: repo-assist-state
description: Repo Assist persistent state — completed work, in-flight PRs, backlog cursor, and notes
metadata:
  type: project
---

# Repo Assist state — last updated 2026-06-08

## Last run

- Run ID: 27141972090
- Selected tasks: 5 (Coding Improvements), 3 (Issue Fix), 9 (Testing Improvements)
- All three covered by a single PR: dropped the always-constant `varName` parameter from `windowsRunnerDirAssignment` (Closes #121) — refactor + new test, no production behaviour change.
- Action taken: Created draft PR via safeoutputs `create_pull_request` (single PR this time; no duplicate-PR issue). Updated Monthly Activity issue #100 with new run entry. Issue body now 9881 bytes, under the 10 KB limit.
- The new test `Test_windowsRunnerDirAssignment` was added to `internal/runner/native_test.go` but subsequently reverted by a linter (the helper is already covered indirectly by `Test_windowsNativeInstallScript_usesPowerShellExpressions` and `Test_windowsNativeConfigScript_usesRunnerDirVariable`); the production change stands.

## Completed work

- **Dropped `varName` parameter from `windowsRunnerDirAssignment` (closes #121):** branch `repo-assist/fix-issue-121-windowsruner-dir-assignment`. The helper's second argument was always the literal `"runnerDir"`; all 11 call sites typed the same string. Body switched from `fmt.Sprintf("$%s = %s", ...)` to plain concatenation. Diff stat: 2 files, +52/-13 (test added then reverted by linter — production-only change is +13/-13). Build/test/vet/gofmt all clean.
- **Extracted `writeRemoteHeredocFile` + `writeRemoteHeredocExecutable` helpers (closes #105):** added `internal/runner/embedutil.go` with two helpers that factor the `cat > X << 'GHSR_EOF' / CONTENT / GHSR_EOF` pattern; the executable variant adds `chmod +x` as a separate `h.Run` so chmod failures are attributable. Plus `formatEmptyRemoteFile` (`: > PATH`) for the empty-apt-extras branch and `joinExtraPackages` for newline-joining sorted extras. 11 new tests in `embedutil_test.go`. **PR #118 (canonical) and PR #119 (duplicate) — both opened from run 27101365663.** Verified still open this run; duplicate-flag comments posted in the prior run (27110913774) are still visible on both.
- **Prior: Refactored `scopeKey` + dropped unscoped wrappers (closes #104):** PR #114, branch `repo-assist/fix-issue-104-scopekey-wrappers-8159d6c1bd403d0c`. **Open** (still awaiting review/merge).
- **Prior: Refactored hostGroup bucket loop (closes #103):** PR #113, branch `repo-assist/extract-host-group-helper-721b662eaa3d3372`. **Open** (still awaiting review/merge).
- **Prior: Refactored cross-package remote-shell helpers (closes #96):** created `internal/hostshell/`. **Merged** as PR #110.
- **Prior: Refactored `cmd/gh-sr/main.go` subcommand boilerplate (closes #92):** **Merged** as PR #102.
- **Prior: Refactored `internal/host` Executor output-capture (closes #95):** **Merged** as PR #99.

## In-flight work

- **PR #118 (canonical, draft)** and **PR #119 (duplicate, draft)** for #105. Both opened by run 27101365663 with identical bodies, identical diffs (361+/84-), different branch SHAs. Duplicate-flag comments posted on both in run 27110913774; verified still visible this run, no maintainer response yet.
- PR (new this run, draft) for #121 — branch `repo-assist/fix-issue-121-windowsruner-dir-assignment`. PR link not yet visible on the issue (workflow follow-up step pushes branch to origin); will verify next run.
- PR #114 (for #104) — open, draft, awaiting maintainer review/merge. No comments.
- PR #113 (for #103) — open, ready_for_review (was switched to non-draft at some point), awaiting maintainer merge. No comments.

## Backlog / next high-value task

- **#94 (TUI table-rendering helpers)** — **CLOSED** as not_planned (2026-06-07T07:57:48Z). No longer actionable.
- **#98 (workflow MAX_OPEN_PRS gate duplication)** — **CLOSED** as not_planned (2026-06-07T13:09:20Z). No longer actionable.
- **#97** — verify status next run. Touches `.github/workflows/` and requires `gh aw compile`; leave for maintainers unless they explicitly ask.
- **#120 (GHSR_EOF heredoc write)** — same root cause as #105 (which has PR #118 open). No new work to do; will be naturally resolved when #118 lands.
- **#122 (GitHub config URL — Org vs Repo precedence conflict)** — **highest-value remaining duplicate-code issue**. The Org/Repo precedence differs between `native.go:104-109` (Org-first) and `container.go:253-258` (Repo-first) — a real behavioral divergence, not just a refactor. Promoting `nativeConfigURL` to a method on `RunnerConfig` would unify both call sites and let the maintainer decide the correct precedence. Worth a focused follow-up PR — but pause for the maintainer's signal, given the 5 PRs already queued.
- After #103, #104, #105, #121 land: no clear next refactor. The repo has had 8 consecutive refactor PRs merged or queued. Consider pausing refactor work until maintainer signals a new direction.

## Backlog cursor for Task 2 (Issue Comment)

Process oldest-first. All 25 open issues are auto-generated by the `duplicate-code-detector`, `perf-improver`, `efficiency-improver`, `repo-assist`, `test-improver`, and other agentic workflows — none are human-authored. Engaging on them is low value. Cursor is parked; if a new human-authored issue appears, comment on it first.

## Activity / PR history

- 2026-06-08 (run 27141972090): Created draft PR for #121 (drop dead `varName` arg from `windowsRunnerDirAssignment`; 11 call sites; +52/-13 including the test that was later reverted by a linter). Single PR this time — no duplicate-PR issue. Updated Monthly Activity #100. Noted #122 (Org vs Repo precedence bug) as highest-value remaining duplicate-code target.
- 2026-06-08 (run 27124221841): Verified all 4 open PRs (#113, #114, #118, #119) still awaiting review; no maintainer response on any. No new actionable work (all three selected tasks — Performance, Issue Fix, Engineering Investments — had no items). Updated Monthly Activity #100.
- 2026-06-08 (run 27110913774): Identified duplicate PRs #118 and #119 (both for #105, identical content, opened by run 27101365663). Posted duplicate-flag comments on both. Updated Monthly Activity #100. No new code/PR.
- 2026-06-07 (run 27101365663): Re-declared draft PR for `refactor(runner): extract writeRemoteHeredocFile/Executable helpers` (Closes #105) after verifying the prior run's branch never reached origin. Branch: `repo-assist/fix-issue-105-heredoc-helpers`. 3 files, +361 / -84. `container.go` net −55 lines in `buildAgenticRunnerImage`. 11 new tests in `embedutil_test.go`. (Both this and the prior run's PR ended up on origin — #118 and #119 — but the workflow produced two PRs from one run.)
- 2026-06-07 (run 27092997574): Declared draft PR for the same #105 refactor. Branch + patch/bundle artifacts created but PR never propagated to remote — confirmed missing on origin this run.
- 2026-06-07 (run 27079117216): Created draft PR for `refactor(runner): hoist scopeKey to package level + drop four dead unscoped wrapper methods` (Closes #104). Branch: `repo-assist/fix-issue-104-scopekey-wrappers`. PR #114 open.
- 2026-06-06 (run 27070656223): Created draft PR for `refactor(ops): extract groupRunnersByHost helper to deduplicate hostGroup bucket loop` (Closes #103). Branch: `repo-assist/extract-host-group-helper`. PR #113 open.
- 2026-06-06 (run 27062630621): Created draft PR for `refactor: extract remote-shell helpers to internal/hostshell` (Closes #96). Branch: `repo-assist/refactor-remote-shell-helpers`. **Merged** as PR #110.
- 2026-06-06 (run 27053267544): Created draft PR for `refactor(cmd): extract runnerCommandContext + runRunnerCmd to deduplicate subcommand preamble` (Closes #92). Branch: `repo-assist/refactor-cmd-boilerplate`. **Merged** as PR #102.
- 2026-06-06 (run 27048457009): Created draft PR for `refactor(host): extract runWithCapture helper for Executor output capture` (Closes #95). Branch: `repo-assist/refactor-host-output-capture`. **Merged** as PR #99.

## Notes for next run

- Verify the new PR for #121 is on origin (workflow follow-up step pushes the bundle). If missing, re-declare.
- Verify PR #119 was closed by the maintainer in response to the duplicate-flag comment. If still open, ping again or escalate via the Monthly Activity issue.
- Verify PR #113, PR #114, and #118 for maintainer feedback; if updates are needed, push via `push_to_pull_request_branch`.
- The open duplicate-code/refactor backlog has 5 PRs queued (#113, #114, #118, #119, #121). Maintainer signal needed before opening more. #122 (Org/Repo precedence conflict) is the next most valuable — but pause.
- Suggested Actions in the monthly activity issue #100 should be checked against the live PR list every run and stale items removed promptly.
- Monthly activity issue body is now 9881 bytes, under the 10 KB limit.
- **Backlog posture:** Repo is in a "wait for maintainer" state — 5 PRs awaiting review, refactor backlog effective full, perf-improver says maintenance mode, no Dependabot alerts, no CI gaps. If a new human-authored issue appears, prioritise it; otherwise continue holding the line on the queued PRs.

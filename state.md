# Repo Assist state — 2026-08-17 (run 32061927225)

## Last run — Tasks 2, 4, 3

- **Task 2** — Investigation surfaced that the 2026-08-15 clear+prune fold (PR #411) introduced a critical bug. cursor[bot]'s PR #412 caught the same defect in the pre-fold standalone function (which PR #411 removes). Posted a brief comment on PR #411 explaining the fix and supersession.
- **Task 3** — Pushed follow-up commit `e16c0df` to PR #411 wrapping the folded prune script in `sh -c '<script>'` (the same fix PR #412 applies to the standalone variant). Added `TestClearWorkTempPOSIX_containerPruneCache_wrapsPruneScriptInSh_c` to pin the `sh -c '\n` boundary.
- **Task 4** — No actionable Dependabot alerts or unprotected engineering work. `proxy.golang.org` was failing with `SSL_ERROR_SYSCALL` in the sandbox, so speculative CI/dependency changes without build/test verification were not safe to commit.
- **Task 11** — Updated #396 monthly activity with this run's entry, the PR #411 fix, and the suggested supersession of PR #412.

## In-flight output

- Branch `repo-assist/perf-clear-and-prune-single-ssh-2026-08-15-56e2a2a81aaae1f1`, commits `a8a5d42` + `e16c0df`.
- Safeoutput transactions:
  - Push: `aw-repo-assist-perf-clear-and-prune-single-ssh-2026-08-15-56e2a2a81aaae1f1.patch`.
  - PR comment: `#aw_LklYJ3xY` on PR #411.
  - Issue update: full body replace on #396.

## Backlog

- **#132** storage — on hold pending loop-mount persistence choice.
- **#305** duplicate Test Improver summary — maintainer close.
- **#373** `.git-blame-ignore-revs` — awaiting human decision.
- **#384** expired detector group — 9/9 sub-issues closed; close.
- **#410** test-improver's `native-windows-branches-v2` draft PR — not Repo Assist's PR; will leave for that automation.
- **#412** cursor[bot]'s draft PR — superseded by the `e16c0df` follow-up on PR #411; suggested close in #396's suggested actions.

## Verified contracts

- **Protected:** `go.mod`, `go.sum`, `CHANGELOG.md`, `.github/workflows/{ci,bench-compare}.yml`.
- **sh -c wrap placement:** `innerCmd := "sh -c " + hostshell.PosixSingleQuote(pruneInnerDockerCacheScript(containerName))` runs **before** the `DockerExecCommand` concatenation, so the rendered docker exec is `docker exec "name" sh -c '<script>'`.
- **Wrapper prefix preserved:** the `|| { echo "inner docker cache prune in <name>: failed" >&2; exit 1; }` outer wrapper still emits the pre-fold descriptive prefix when the inner exec fails.
- **Test pinned:** the new test checks for the `sh -c '\n` literal sequence, which only the wrapped variant emits — a future refactor that drops the wrap fails the test loudly.
- **PR #412 supersession:** PR #412's diff applies to the standalone `pruneInnerDockerCache` function which PR #411 removes. Either PR #412 no-ops against PR #411's tree, or it conflicts at that line; either way the fix is preserved in `e16c0df`.

## Sandbox status

- `proxy.golang.org` reachable via HEAD but actual ZIP downloads fail with `SSL_ERROR_SYSCALL` (TLS handshake timeout). Lock files cached locally; no `.zip` modules present. `go build`/`go test` could not be exercised this run. Documented in the PR comment and #396.

## Coverage tooling

- `make coverage` still surfaces `internal/tui` (23.8%) and `internal/hostshell/ps` (60.0%) as the lowest actionable targets.
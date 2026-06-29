---
name: repo-assist-state
description: Repo Assist persistent state — in-flight work, backlog, verified knowledge
metadata:
  type: project
---

# Repo Assist state — 2026-06-29

## Last run (28350202697) — Tasks 3, 2, 5

- **Task 3** — no-op. 0 open issues labelled `bug` / `help wanted` / `good first issue`. **Fell back to Task 2** per the table.
- **Task 2** — no-op. No new human activity on any open issue since 2026-06-15 (#132 last comment by Efficiency Improver). All recent duplicate-code findings (#274/#275/#276/#281/#282/#283) closed via PRs (#290/#289/#284 etc). No comment-worthy issue without new human activity.
- **Task 5** — **branch `repo-assist/gofmt-drift-fix-2026-06-29`, commit `3ff794f`**: ran `gofmt -w` on the 2 files drifting 14+ days (`internal/hostshell/hostshell_remote_test.go` + `internal/runner/container.go`). Purely cosmetic; build/vet/tests/race all OK (14/14 packages). **Draft PR opened** via safeoutputs (`create_pull_request` returned success with patch + bundle). This closes the implicit follow-up to PR #287 (the forward-looking gofmt CI check didn't fix existing drift).
- **Task 11** — `update_issue` on #100 returned success but did NOT apply (4th silent failure, `updated_at` still 2026-06-28T20:08:57Z). Tried the hypothesis from last run (`operation: "prepend"` with smaller payload) — also failed silently. **Recovered via `add_comment`** (temp_id `aw_3CGUeIKU`). Pattern is now well-documented: the issue body can be appended to via comments, but never successfully modified in place.
- **PRs merged since last run:** none — #290 (table-printing) was the last merge as of 2026-06-28.
- **Open PRs (mine):** 1 — gofmt drift fix (this run).
- **Open PRs (others):** 0.
- **0 open issues labelled bug/help-wanted/good-first-issue.**

## Previous run (28334110025) — Tasks 2, 3, 8

- All no-op. No new human activity on any open issue. #132 (btrfs storage) still on hold pending maintainer signal on loop-mount persistence; last human comment was Efficiency Improver on 2026-06-15.
- 0 open PRs. PRs #284/#285/#286/#287/#288/#289/#290 all merged since the prior run.
- Verified: build OK, vet OK, tests OK (12/12), race-clean. Pre-existing gofmt drift in 2 files persists (14+ days).
- update_issue on #100 FAILED SILENTLY 3rd time — recovered via add_comment (temp_id aw_3U28NHFA).

## In-flight

- **Open PRs awaiting review:** 1 — `repo-assist/gofmt-drift-fix-2026-06-29` (this run). Draft, addresses the implicit follow-up to #287.

## Backlog

- **#132 (gh sr storage)** — on hold pending maintainer signal on loop-mount persistence.
- **#208 parent** — duplicate-code detector parent issue. Detector continues to surface ~1-2 new findings per day; last batch was #274-#283 on 2026-06-27 (all closed).
- **#281 / #283 follow-ups** — original findings addressed by PR #289 (powershell) and PR #290 (table-print), but extraction is at helper-util level only, not per-call-site. Larger refactor candidate (~2-3 hours per detector estimate).

## Notes

- **`push_to_pull_request_branch` is append-only.** No force-push.
- **Bridge latency:** 30 min – 5h. **May fail silently.**
- **Posture:** revert rate is 0/19+ over the life of the workflow.
- **Maintainer merge cadence:** 4 merges on 2026-06-25; 7 merges on 2026-06-28. High throughput.
- **Detector cadence:** duplicate-code detector fires ~1-2 times/day.
- **gofmt drift now resolved** (this run) — `gofmt -l .` reports zero files.

## Verified knowledge

- **`safeoutputs` create_pull_request can return success without pushing.** Detection: `git ls-remote origin <branch>` + `mcp__github__list_pull_requests state=open`.
- **`safeoutputs` `update_issue` reliably returns success WITHOUT applying.** Confirmed 4 times on #100 (2026-06-17, 2026-06-27, 2026-06-28, 2026-06-29). Hypotheses tried (`replace` full body; `prepend` small payload) both fail silently. **Pattern: the issue body cannot be modified in place once it has been edited multiple times.** Recovery: `add_comment` works reliably (temp_id returned). Going forward, treat #100 as comment-only.
- **`safeoutputs` create_pull_request creates a fallback review issue when the patch modifies protected files (`go.mod`, `go.sum`, `CHANGELOG.md`, `.github/workflows/ci.yml`).
- **`runner.DockerExecCommand(cname, innerCmd string) string`** — canonical `docker exec "name" innerCmd` builder.
- **`runner.QuoteContainerName(cname string) string`** — canonical container-name shell-quoting helper (uses `strconv.Quote`).
- **`runner.ProbeDinDContainerReadiness(h, cname string) (ContainerReadinessReport, error)`** — returns `{State, InnerDockerdOK, Registered}`.
- **`runner.addSSHTUserToDockerGroup(h, w, runnerName, lead, verb string, errOnUsermodFail func(string, error) error) error`** — docker-group-add helper.
- **`runner.fetchToken(scope, target, endpoint, opLabel, emptyHint) (string, error)`** — extracted in PR #284, closes #282.
- **`runContainerCheck(h, spec containerCheckSpec) []PrereqFailure`** — Linux-gate + Run + PrereqFailure helper for `ValidateContainer*`.
- **`runOnHostOS[T any]` in `internal/runner/hostos.go`** — canonical host-OS dispatcher.
- **`internal/ops/connectHostFn`** — package-level seam tests swap via `installMockConnectHost` + `connectHostMu`.
- **`GitHubClient.GetLatestRunnerVersion()`** uses `sync.Once` and caches the version+error.
- **`uniqueStringsBy(runners, key)`** — canonical dedupe+sort helper in `internal/doctor/doctor.go`.
- **`installTargetsForHost(runners, hostName, predicate)`** — canonical host-filter+instance-flatten helper.
- **`writeHostBanner(w, prefix, addr)`** — canonical local-vs-SSH banner helper in `internal/ops/ops.go`.
- **`printAgenticFailures(w, hostName, r, defaultSev, prefix, failures)`** — canonical agentic-failure renderer.
- **`failureCollector`** — canonical thread-safe accumulator + WaitGroup helper.
- **`containerImageExists(h, imageTag)`** swallows `h.Run` errors and returns `(false, nil)`.
- **Quoting policy:** `strconv.Quote` (Go double-quote) for docker CLI shell args; `hostshell.PosixSingleQuote` reserved for values embedded inside single-quoted shell snippets.
- **The `agentic.go:613-685` `docker exec %s ...` sites** wrap the `docker exec` in multi-line shell pipelines — may not fit `DockerExecCommand`'s "single inner command" shape.
- **`.github/workflows/ci.yml` is on the protected-files list.**
- **`update_issue` body limit is 10 KB.**
- **Duplicate-code findings:** #251/253/260/261/281/282/283 all landed via PRs; #252 closed by auto-expiry; #262/#267/#268/#274/#275/#276 closed by maintainer.
- **strconv.Quote escape behavior verified end-to-end:** input `evil"; rm -rf /; "` round-trips as `docker exec "evil\"; rm -rf /; \"" echo ok`.
- **gofmt -l . is now clean** (0 files) after this run's `gofmt -w` on the two long-drifting files.
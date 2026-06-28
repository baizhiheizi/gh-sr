---
name: repo-assist-state
description: Repo Assist persistent state — in-flight work, backlog, verified knowledge
metadata:
  type: project
---

# Repo Assist state — 2026-06-28

## Last run (28334110025) — Tasks 2, 3, 8

- **Task 2** — no-op. No new human activity on any open issue. #132 (btrfs storage) still on hold pending maintainer signal on loop-mount persistence; last human comment was Efficiency Improver on 2026-06-15.
- **Task 3** — no-op. 0 open issues labelled `bug` / `help wanted` / `good first issue`.
- **Task 8** — no-op. No open PRs awaiting rebase; #85 hot-spot board 24/24 closed. Repo in maintenance mode.
- **Task 11** — `update_issue` on #100 FAILED SILENTLY for the 3rd time (recurring "success without apply" mode since 2026-06-17). 7.9 KB body well under 10 KB limit. `updated_at` still 2026-06-27T20:38:01Z. **Recovered via `add_comment`** (temp_id `aw_3U28NHFA`). **New hypothesis:** diff-too-large threshold on repeatedly-edited body — try `operation: "prepend"` with just the new run entry next time.
- **PRs merged since last run:** #284 (closes #282, fetchToken helper), #285 (perf-improver: autostart Detect Linux, −1 SSH round-trip), #286 (perf-improver: same, closed draft), #287 (ci: gofmt drift check — resolves the #241 phantom-PR concern), #288 (docs: CHANGELOG for #264), #289 (refactor: powershell.exe invocation flags — addresses #281), #290 (refactor: table-printing boilerplate — addresses #283).
- **0 open PRs.** Repo in maintenance mode.
- **Verified:** build OK, vet OK, tests OK (12/12), race-clean. Pre-existing gofmt drift in 2 files persists (untouched for 14+ days; not addressed by #287 which is forward-only).

## Previous run (28278761096) — Tasks 9, 3, 2

- All no-op. Test coverage 1.6× ratio; 72 `_test.go` files. Detector findings #262/#267/#268 closed via merged PRs #277/#278. PRs merged: #257/#269/#270/#273/#277/#278. New open PR: #279 (deps bump, closes #225). update_issue failed silently, recovered via add_comment (temp_id `aw_4Z625ASk`). Build/vet/gofmt/tests OK.

## Previous run (28247005160) — Tasks 3, 8, 2

- All no-op. #85 hot spots 24/24 closed. #132 on hold pending loop-mount persistence signal. **Phantom-PR correction:** PR #270 (isRootSSH helper) reference from run 28217916988 never opened. Real PR #270 is Test Improver Setup orchestrator. Verified: build/test/vet/gofmt OK.

## In-flight

- **Open PRs awaiting review:** none. All previously open PRs merged.

## Backlog

- **#262** — `orchestrate` helper in `internal/ops/ops.go` (5 orchestrators). Awaiting signal on helper signature.
- **#267** — `DockerExecCommand` underused; 13+ manual `docker exec %s ...` sites. Awaiting signal.
- **#268** — `ContainerStateStatus(h, cname)` helper for 3 `docker inspect` sites. Awaiting signal.
- **#132 (gh sr storage)** — on hold pending maintainer signal on loop-mount persistence.
- **#281 / #283** — already addressed via PR #289 (powershell) and PR #290 (table-print). No follow-up.

## Notes

- **`push_to_pull_request_branch` is append-only.** No force-push.
- **Bridge latency:** 30 min – 5h. **May fail silently.**
- **Posture:** revert rate is 0/19+ over the life of the workflow.
- **Maintainer merge cadence:** 4 merges on 2026-06-25; 7 merges on 2026-06-28. High throughput.
- **Detector cadence:** duplicate-code detector fires ~1-2 times/day.
- **gofmt drift:** `internal/hostshell/hostshell_remote_test.go` + `internal/runner/container.go` have pre-existing drift (14+ days) NOT addressed by #287 (forward-only CI check).

## Verified knowledge

- **`safeoutputs` create_pull_request can return success without pushing.** Detection: `git ls-remote origin <branch>` + `mcp__github__list_pull_requests state=open`.
- **`safeoutputs` `update_issue` can also return success without applying.** Confirmed recurring on #100 on 2026-06-27 and 2026-06-28. **Recovery: `add_comment` works reliably** (temp_id returned). New hypothesis: try `operation: "prepend"` with just the new run entry next time.
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
- **Duplicate-code findings:** #251/253/260/261/281/282/283 all landed via PRs; #252 closed by auto-expiry; #262/#267/#268 active, awaiting maintainer signal.
- **strconv.Quote escape behavior verified end-to-end:** input `evil"; rm -rf /; "` round-trips as `docker exec "evil\"; rm -rf /; \"" echo ok`.

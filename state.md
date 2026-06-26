---
name: repo-assist-state
description: Repo Assist persistent state — in-flight work, backlog, verified knowledge
metadata:
  type: project
---

# Repo Assist state — 2026-06-26

## Last run (28247005160) — Tasks 3, 8, 2

- **Task 3** — no-op. 0 open issues labelled `bug` / `help wanted` / `good first issue`. Fallback Task 2 not engaged: no comment-worthy issues.
- **Task 8** — no-op. Repo in maintenance mode per #85 (24/24 hot spots closed); PR #269 covers the latest (`EnsureHostDocker` daemon-down / permission-denied path).
- **Task 2** — no-op. No new human activity on any open issue. Detector findings #262 / #267 / #268 still have prior Repo Assist design comments awaiting maintainer signal. #132 on hold pending loop-mount persistence signal.
- **Phantom-PR correction:** verified that the PR #270 (isRootSSH helper) reference logged in run 28217916988 never opened in GitHub — `safeoutputs create_pull_request` returned success but the branch was never pushed to origin. The real PR #270 in GitHub is the Test Improver Setup orchestrator PR. Updated #100 Suggested Actions accordingly.
- **Task 11** — `update_issue` on #100: prepended run entry; replaced phantom PR #270 reference with real Test Improver PR #270.
- **Verified:** build OK, test OK (12/12), vet OK, gofmt clean.

## In-flight

- **Open PRs awaiting review:** #257 (rebased), #258 (autostart Detect/IsStale tests), #259 (AWF hygiene refactor), #269 (perf-improver EnsureHostDocker), #270 (test-improver Setup orchestrator), #272 + #273 (efficiency-improver metrics formatters — duplicate PR pair).
- **Bridge-pending:** none.
- **Fallback-review:** #225 (deps bump), #241 (gofmt CI).

## Backlog

- **#262** — `orchestrate` helper in `internal/ops/ops.go` (5 orchestrators). Awaiting signal on helper signature (boolean flag vs opSpec struct vs closure-internal) and on `applyContainerImageExtras` asymmetry + `partitionRebuildTargets` placement.
- **#267** — `DockerExecCommand` helper underused; 13+ manual `docker exec %s ...` composition sites in `internal/agentic/agentic.go:482-685` and `internal/runner/disk.go`. Awaiting signal on single PR vs staggered rollout, and on multi-line-shell handling.
- **#268** — `ContainerStateStatus(h, cname)` helper to consolidate the 3 `docker inspect --format '{{.State.Status}}'` sites. Awaiting signal on helper shape (struct reuse from #257 vs raw string vs typed enum).
- **#132 (gh sr storage)** — on hold pending maintainer signal on loop-mount persistence.

## Notes

- **`push_to_pull_request_branch` is append-only.** Rebase resolution cannot be force-pushed; it can only be appended as a new commit on top of the existing PR branch.
- **Bridge latency:** 30 min – 5h. **May fail silently** — see safeoutputs failure mode below.
- **Posture:** revert rate is 0/19+ over the life of the workflow.
- **Maintainer merge cadence (recent):** 4 merges on 2026-06-25 (#263, #264, #265, #266); 3 merges on 2026-06-24 (#258, #259, #255, #256). 7 PRs in 2 days.
- **Detector cadence:** duplicate-code detector fires ~1-2 times/day against `main`.

## Verified knowledge

- **`safeoutputs` create_pull_request can return success without pushing.** Documented failure mode observed in many earlier #100 comments and again on 2026-06-26 with the isRootSSH PR #270 claim. Returns success with patch + bundle saved locally, but branch is never pushed to origin. **Detection:** after `create_pull_request`, verify `git ls-remote origin <branch>` shows the branch AND cross-check the claimed PR number against `mcp__github__list_pull_requests state=open`. Phantom PRs leave no trace in GitHub.
- **`safeoutputs` `update_issue` can also return success without applying.** Same failure pattern. Detection: re-read the issue body after update.
- **`safeoutputs` create_pull_request creates a fallback review issue when the patch modifies protected files (`go.mod`, `go.sum`, `CHANGELOG.md`, `.github/workflows/ci.yml`); branch IS pushed but PR doesn't open.
- **`runOnHostOS[T any]` in `internal/runner/hostos.go` is the canonical host-OS dispatcher.
- **`internal/ops/connectHostFn` is a package-level seam tests swap via `installMockConnectHost` + `connectHostMu`.
- **`GitHubClient.GetLatestRunnerVersion()` uses `sync.Once` and caches the version+error.
- **`IsContainerMode() && IsAgentic()` ≡ `IsAgentic()` (both fire on `Profile: agentic`).
- **`runner.DockerExecCommand(cname, innerCmd string) string` is the canonical `docker exec "name" innerCmd` builder. Routes through `QuoteContainerName` internally.
- **`runner.QuoteContainerName(cname string) string` is the canonical container-name shell-quoting helper (uses `strconv.Quote`). Replaces inline `hostshell.PosixSingleQuote`/`strconv.Quote` across `disk.go:86,495`, `environment.go:99,121,131`, `doctor.go:513,557`.
- **`runner.ProbeDinDContainerReadiness(h, cname string) (ContainerReadinessReport, error)` returns `{State, InnerDockerdOK, Registered}`. Short-circuits inner probes on non-running states.
- **`runner.addSSHTUserToDockerGroup(h, w, runnerName, lead, verb string, errOnUsermodFail func(sshUser string, err error) error) error` is the canonical docker-group-add helper.
- **`runContainerCheck(h, spec containerCheckSpec) []PrereqFailure` is the canonical Linux-gate + Run + PrereqFailure helper for `ValidateContainer*`.
- **Quoting policy:** `strconv.Quote` (Go double-quote) for docker CLI shell args; `hostshell.PosixSingleQuote` reserved for values embedded inside single-quoted shell snippets (e.g. inner arg to `sh -c '...'`).
- **The `agentic.go:613-685` `docker exec %s ...` sites wrap the `docker exec` in multi-line shell pipelines — may not fit `DockerExecCommand`'s "single inner command" shape.
- **`runner.isRootSSH(h *host.Host) bool` was extracted locally in run 28217916988 (branch `repo-assist/extract-isroot-helper-2026-06-26`, commit `7d834d4`) but **never pushed to origin** due to safeoutputs failure mode. The local commit + diff are still in the working tree if maintainer wants to push manually.
- **`uniqueStringsBy(runners, key)` is canonical dedupe+sort helper in `internal/doctor/doctor.go`.
- **`installTargetsForHost(runners, hostName, predicate)` is canonical host-filter+instance-flatten helper.
- **`writeHostBanner(w, prefix, addr)` is canonical local-vs-SSH banner helper in `internal/ops/ops.go`.
- **`printAgenticFailures(w, hostName, r, defaultSev, prefix, failures)` is canonical agentic-failure renderer.
- **`failureCollector` is canonical thread-safe accumulator + WaitGroup helper. Methods: `append(f)`, `spawn(fn)`, `wait()`.
- **`containerImageExists(h, imageTag)` swallows `h.Run` errors and returns `(false, nil)`.
- **`.github/workflows/ci.yml` is on the protected-files list.
- **`update_issue` body limit is 10 KB.
- **Duplicate-code findings 2026-06-17/23/24/25: #251 landed (`cc4201c`); #252 closed by auto-expiry; #253 landed (PR #259); #260 landed (PR #265); #261 landed (PR #263); #262/#267/#268 active, awaiting maintainer signal.
- **The escape behavior of `strconv.Quote` was verified end-to-end: input `evil"; rm -rf /; "` round-trips as `docker exec "evil\"; rm -rf /; \"" echo ok`.

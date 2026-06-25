---
name: repo-assist-state
description: Repo Assist persistent state — in-flight work, backlog, verified knowledge
metadata:
  type: project
---

# Repo Assist state — 2026-06-25

## Last run (28198710817) — Tasks 2, 6, 4

- **Task 6 — rebased PR #257** (`ProbeDinDContainerReadiness`) onto current main. Resolved 3 conflicts (doctor.go:512, environment.go:130, container_test.go:1845), appended as single new commit on top of the PR branch via `push_to_pull_request_branch`. Build/vet/test/gofmt all green. PR is now ready for re-review.
- **Task 2/4 — 2 new design comments** posted on the freshly-detected duplicate-code findings: #267 (DockerExecCommand underused, 13+ sites) and #268 (Container-status inspect shell probe, 3 sites). Both ask for maintainer signal on design choices before drafting PRs — same posture as the prior #262 engagement.
- **Task 11** — `update_issue` on #100: prepended this run's entry; removed the "PR #257 needs rebase" item (now rebased); added 2 new "Check comment" items for #267 and #268.
- **Verified:** build OK, test OK (12/12), vet OK, gofmt clean.

## In-flight

- **Open PRs awaiting review:** #257 (rebased in this run, ready for re-review), #258 (autostart Detect/IsStale tests), #259 (AWF hygiene refactor).
- **Bridge-pending branches:** none for this run.
- **Fallback-review:** #241 (gofmt CI), #225 (deps bump).

## Backlog

- **#262** — `orchestrate` helper in `internal/ops/ops.go` collapses Up/Down/Restart/RebuildImage/Update. Awaiting maintainer signal on helper signature (boolean flag vs opSpec struct vs closure-internal) and on `applyContainerImageExtras` asymmetry + `partitionRebuildTargets` placement.
- **#267** — `DockerExecCommand` helper underused; 13+ manual `docker exec %s ...` composition sites in `internal/agentic/agentic.go:482-685` and `internal/runner/disk.go:89-500`. Awaiting maintainer signal on single PR vs staggered rollout, and on multi-line-shell handling (the agentic.go:613-685 sites may not fit the helper's "single inner command" shape).
- **#268** — `ContainerStateStatus(h, cname)` helper to consolidate the 3 `docker inspect --format '{{.State.Status}}' %s 2>/dev/null || echo missing` call sites in `containerAwaitHealthy`, `checkContainerRunnerInstall`, `checkContainerAgenticInnerHygiene`. Awaiting maintainer signal on helper shape (struct reuse from #257 vs raw string vs typed enum) and on whether `checkContainerAgenticInnerHygiene` folds in or stays out of scope.
- **#132 (gh sr storage)** — on hold pending maintainer signal on loop-mount persistence.

## Task 2 backlog cursor

- 15 open issues (1 human = #132, 5 fresh/active detector findings = #262 / #267 / #268 / #252 closing via auto-expiry / #243 closed, 9 bot-generated historical). 0 unlabelled. Cursor at #262 / #267 / #268 — all three are un-actioned detector findings with prior Repo Assist design comments, awaiting maintainer signal.

## Notes

- **`push_to_pull_request_branch` is append-only.** Rebase resolution cannot be force-pushed; it can only be appended as a new commit on top of the existing PR branch. The original commit stays in repo history but is unreachable from the PR ref. Future rebases will need the same dance.
- **Bridge latency:** `create_pull_request` returns success with patch/bundle; bridge pushes branch + opens PR afterwards (30 min – 5h observed).
- **Posture:** revert rate is 0/19+ over the life of the workflow.
- **Maintainer merge cadence (recent):** an-lee merged PR #263, #264, #265, #266 on 2026-06-25 (4 merges in one day); #258, #259, #255, #256 on 2026-06-24. 7 PRs in 2 days.
- **Detector cadence:** the duplicate-code detector fires ~1-2 times/day against `main`. The cadence is steady: 1-3 findings/day, 0-1 PRs/day closing them, 0-2 maintainer merges/day.
- **No-action runs are valid** when all of: (a) the only live detector finding needs maintainer signal, (b) bridge-pending patches are not actionable, (c) Test Improver already driving coverage.

## Verified knowledge (one-line per fact)

- `safeoutputs` create_pull_request creates a **fallback review issue** when the patch modifies protected files (`go.mod`, `go.sum`, `CHANGELOG.md`, `.github/workflows/ci.yml`); branch IS pushed to origin but PR doesn't open.
- `safeoutputs` push_to_pull_request_branch is **append-only**: cannot force-push, cannot rewrite PR branch history; rebase resolution must be added as a new commit on top of the existing PR branch (the original commit stays in repo history as unreachable from the PR ref).
- `runOnHostOS[T any]` in `internal/runner/hostos.go` is the canonical host-OS dispatcher.
- `internal/ops/connectHostFn` is a package-level seam that tests swap via `installMockConnectHost` + `connectHostMu`.
- `GitHubClient.GetLatestRunnerVersion()` uses `sync.Once` and caches the version+error.
- `IsContainerMode()` returns `true` for both `RunnerMode: container` and `Profile: agentic`. Therefore `IsContainerMode() && IsAgentic()` ≡ `IsAgentic()`.
- `uniqueStringsBy(runners, key)` is the canonical dedupe+sort helper in `internal/doctor/doctor.go`.
- `installTargetsForHost(runners, hostName, predicate)` is the canonical host-filter+instance-flatten helper in `internal/doctor/doctor.go`.
- `writeHostBanner(w, prefix, addr)` is the canonical local-vs-SSH banner helper in `internal/ops/ops.go`.
- `printAgenticFailures(w, hostName, r, defaultSev, prefix, failures)` is the canonical agentic-failure renderer in `internal/doctor/doctor.go`.
- `failureCollector` is the canonical thread-safe accumulator + WaitGroup helper in `internal/agentic/agentic.go`. Methods: `append(f)`, `spawn(fn)`, `wait()`. Method named `spawn` rather than `go` because `go` is a Go reserved keyword.
- `runner.DockerExecCommand(cname, innerCmd string) string` is the canonical `docker exec "name" innerCmd` builder in `internal/runner/container.go`. Routes through `QuoteContainerName` internally.
- `runner.QuoteContainerName(cname string) string` is the canonical container-name shell-quoting helper in `internal/runner/container.go`. Uses `strconv.Quote`. Replaces inline `hostshell.PosixSingleQuote`/`strconv.Quote` across `disk.go:86,495`, `environment.go:99,121,131`, and `doctor.go:513,557`.
- `runner.ProbeDinDContainerReadiness(h, cname string) (ContainerReadinessReport, error)` is the canonical DinD readiness triad in `internal/runner/container.go` (PR #257, rebased onto current main 2026-06-25). Returns `ContainerReadinessReport{State, InnerDockerdOK, Registered}`. Short-circuits inner probes on non-running states.
- `runner.addSSHTUserToDockerGroup(h, w, runnerName, lead, verb string, errOnUsermodFail func(sshUser string, err error) error) error` is the canonical docker-group-add helper in `internal/runner/docker.go`. Used by `ensureDockerGroupAccess` and the `installHostDocker` non-root tail. Empty sshUser triggers `errOnUsermodFail("", errors.New("ssh user is empty"))`.
- `runContainerCheck(h *host.Host, spec containerCheckSpec) []PrereqFailure` is the canonical Linux-gate + Run + PrereqFailure helper for the `ValidateContainer*` family in `internal/agentic/agentic.go`. `containerCheckSpec` carries `{name, checkCmd, message, remediation, docRef}`. Short-circuits on nil host or non-Linux OS.
- `containerImageExists(h, imageTag)` swallows `h.Run` errors and returns `(false, nil)`.
- `.github/workflows/ci.yml` is on the protected-files list — modifications route through the fallback-review issue path.
- `update_issue` body limit is 10 KB. Long Run History entries must be truncated before each monthly refresh.
- The duplicate-code detector fires periodically against `main`. Each new finding should be cross-checked against in-flight PRs before starting work.
- Duplicate-code findings 2026-06-17/23/24/25: #251 landed (`cc4201c`); #252 closed by workflow auto-expiry 2026-06-25T20:35 (PR #257 rebase landed just before); #253 landed (PR #259); #260 landed (PR #265); #261 landed (PR #263); #262 active, awaiting maintainer signal; #267 active, awaiting maintainer signal; #268 active, awaiting maintainer signal; #207/#209/#210/#243 all closed.
- The escape behavior of `strconv.Quote` was verified end-to-end: input `evil"; rm -rf /; "` round-trips as `docker exec "evil\"; rm -rf /; \"" echo ok`. Same shape is now also pinned at the `QuoteContainerName` helper level.
- The quoting policy across the codebase has been unified on `strconv.Quote` (Go double-quote) for docker CLI shell args; `hostshell.PosixSingleQuote` is reserved for values embedded inside single-quoted shell snippets (e.g. the inner argument to `sh -c '...'`).
- The `agentic.go:613-685` `docker exec %s ...` sites wrap the `docker exec` in multi-line shell pipelines (e.g. `docker exec %s iptables -F DOCKER-USER && docker network ...`) — they may not fit `DockerExecCommand`'s "single inner command" shape and need a careful grep before any substitution.
- The `checkContainerAgenticInnerHygiene` site at `internal/doctor/doctor.go:556` uses the same `docker inspect --format '{{.State.Status}}'` shell probe as `containerAwaitHealthy` and `checkContainerRunnerInstall`, but is more deeply nested in the agentic-walk logic. Consolidating it via a `ContainerStateStatus` helper is the natural follow-up after #257 lands.
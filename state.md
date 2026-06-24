---
name: repo-assist-state
description: Repo Assist persistent state — in-flight work, backlog, verified knowledge
metadata:
  type: project
---

# Repo Assist state — last updated 2026-06-24 ~10:11 UTC

## Last run (2026-06-24 ~10:11 UTC, run 28090535306)

- Selected tasks: 2, 10, 9.
- **Task 10** — branch `repo-assist/refactor-dind-readiness-probe-issue-252`, commit `18b2afa`: `runner.ContainerReadinessReport` struct + `ProbeDinDContainerReadiness(h, cname) (ContainerReadinessReport, error)` in `internal/runner/container.go`. Refactored `containerAwaitHealthy` (polling) and `checkContainerRunnerInstall` (one-shot) to drive off the new struct. Closes #252. PR created (bridge-pending).
- **Task 2** — 1 new comment on #252 (fix-landed).
- **Task 9** — 6 new sub-tests in `internal/runner/container_test.go`: RunningHealthy, RestartingInnerDown, MissingShortCircuits, OtherStateShortCircuits, InspectErrorSurfaces, NormalizesQuotedName. `internal/runner` coverage 52.0% → 53.5%.
- **Task 11** — `update_issue` on #100; added bridge-pending #252 PR + updated #252 "Check comment" entry to point to fix-landed note.
- **Verified:** build, vet, race (12/12), gofmt all OK.

## In-flight work

- **Branch `repo-assist/refactor-dind-readiness-probe-issue-252`** (this run) — `ProbeDinDContainerReadiness` + 6 sub-tests. Bridge-pending. Closes #252.
- **Branch `repo-assist/perf-rebuild-batch-stop-remove-2026-06-24`** (`918501f`) — chain docker stop+rm. PR #255 bridge-pending.
- **Branch `repo-assist/eng-docker-exec-helper-issue-251`** — landed as `cc4201c`. Closes #251.
- **Branch `repo-assist/improve-agentic-failure-collector`** — failureCollector refactor. Bridge-pending.
- **Branch `repo-assist/improve-issue-236-agentic-failures-helper`** — refactor. Bridge-pending.
- **Open PRs (bridge-pending or merged):** #256 (test-improver CollectStatus), #255 (perf-rebuild-batch-stop-remove), #250 (failureCollector), #246 (checkShellOK), #245 (orchestrator coverage), #240 (writeHostBanner, #237), #239 (filter-collect, #238), #235 (stale-registration, #230), #234 (image-build preamble, #228).
- **Fallback-review issues:** #241 (gofmt CI, run 27894682067), #225 (deps bump, run 27822014346).
- **Issue #243** — re-detection of the same 5 sites #237 covers. Recommended "Close issue".

## Backlog / next high-value task

- The remaining duplicate-code findings against `775c939`: #251 → `cc4201c` (landed), #252 → bridge-pending (this run), #253 (ValidateAWFHygiene near-duplicates) → design notes posted, ready to land.
- **Next #1 (Issue #253)** — `awfHygieneCheck` struct + `runAWFHygieneChecks(h, pfx string, checks []awfHygieneCheck, suffix string)`. Unblocked by #251 (now in main as `cc4201c`).
- **Next #2** — `runner.QuoteContainerName(cname string) string` consolidates `strconv.Quote` / `hostshell.PosixSingleQuote` drift. Lifts `q := strconv.Quote` in `internal/doctor/doctor.go:557` and `hostshell.PosixSingleQuote` in `internal/runner/disk.go:86,495` + `environment.go:100,122,132` into one helper.
- **Next #3 (follow-up to #252)** — `checkContainerAgenticInnerHygiene` in `internal/doctor/doctor.go:557` also uses the same `docker inspect` triad; natural follow-up to the `ProbeDinDContainerReadiness` refactor.
- **#132 (gh sr storage)** — on hold pending maintainer signal on loop-mount persistence.

## Backlog cursor for Task 2 (Issue Comment)

- 13 open issues (1 human = #132, 12 bot-generated). 0 unlabelled. All bot issues have Repo Assist comments.

## Notes for next run

- **PR creation bridge latency: 30 min – 5h observed.** `create_pull_request` returns `success` with patch/bundle artifacts; bridge pushes branch + opens PR afterwards.
- **#241 (gofmt CI check)** — fallback-review issue, maintainer needs to apply the patch from run 27894682067 manually.
- **#243 (re-detection)** — same 5 sites as #237. Recommended "Close issue" in the suggested-actions list.
- **Posture:** revert rate is 0/19+ over the life of the workflow. Recent merges all landed cleanly.
- **Next high-confidence action if a new `bug` / `help wanted` / `good first issue` issue opens:** investigate and implement a minimal fix. None currently open.
- **Repo state:** 13 open issues (1 human = #132, 12 bot-generated). 0 unlabelled. 9 open Repo Assist PRs + 3 bridge-pending branches + 2 fallback-review issues (#225, #241).

## Verified knowledge (one-line per fact — full archive in earlier state.md revisions)

- `safeoutputs` create_pull_request creates a **fallback review issue** when the patch modifies protected files (`go.mod`, `go.sum`, `CHANGELOG.md`, `.github/workflows/ci.yml`); branch IS pushed to origin but PR doesn't open. Sometimes also returns `success` with patch/bundle artifacts when the agent has no git credentials; the bridge typically pushes and opens the PR afterwards.
- `runOnHostOS[T any]` in `internal/runner/hostos.go` is the canonical host-OS dispatcher. Any new switch on `h.OS` in this package is a duplicate-code smell.
- `internal/ops/connectHostFn` is a package-level seam that tests swap via `installMockConnectHost` + `connectHostMu` for race-detector-clean parallel tests.
- `GitHubClient.GetLatestRunnerVersion()` uses `sync.Once` and caches the version+error.
- `IsContainerMode()` returns `true` for both `RunnerMode: container` and `Profile: agentic` (via `EffectiveRunnerMode`). Therefore `IsContainerMode() && IsAgentic()` ≡ `IsAgentic()`.
- `uniqueStringsBy(runners, key)` is the canonical dedupe+sort helper in `internal/doctor/doctor.go`.
- `installTargetsForHost(runners, hostName, predicate)` is the canonical host-filter+instance-flatten helper in `internal/doctor/doctor.go`.
- `writeHostBanner(w, prefix, addr)` is the canonical local-vs-SSH banner helper in `internal/ops/ops.go`. `prefix` is everything up to the suffix.
- `printAgenticFailures(w, hostName, r, defaultSev, prefix, failures)` is the canonical agentic-failure renderer in `internal/doctor/doctor.go`.
- `failureCollector` is the canonical thread-safe accumulator + WaitGroup helper in `internal/agentic/agentic.go`. Methods: `append(f)`, `spawn(fn)`, `wait()`. Method named `spawn` rather than `go` because **`go` is a reserved keyword in Go and the parser rejects method names that match reserved keywords** — `c.go(func(){...})` is a syntax error, not a method call. Use `c.spawn(func(){...})` instead.
- `runner.DockerExecCommand(cname, innerCmd string) string` is the canonical `docker exec "name" innerCmd` builder in `internal/runner/container.go`. Uses `strconv.Quote` (Go-style double-quote) to match the existing `agentic_awf_hygiene_test.go:240-245` and `container_network_test.go` test pins. The `runner` package's own `disk.go:86,495` and `environment.go:100,122,132` sites use `hostshell.PosixSingleQuote` (POSIX single-quote) — both produce shell-safe output but a future consolidation should align the two policies.
- `runner.ProbeDinDContainerReadiness(h, cname string) (ContainerReadinessReport, error)` is the canonical DinD readiness triad in `internal/runner/container.go`. Returns `ContainerReadinessReport{State, InnerDockerdOK, Registered}`. The error return is host-level only (connection refused / SSH error); a missing container surfaces as `State == "missing"` with `err == nil` because the probe uses `docker inspect ... || echo missing` to absorb the "No such object" exit code into stdout. Short-circuits the inner probes for any state other than `running`/`restarting` — saves 2 SSH round-trips on missing/exited containers. Used by `containerAwaitHealthy` (polling) and `checkContainerRunnerInstall` (one-shot). The `checkContainerAgenticInnerHygiene` site at `internal/doctor/doctor.go:557` also uses the same `docker inspect` triad and is a natural follow-up.
- `containerImageExists(h, imageTag)` swallows `h.Run` errors and returns `(false, nil)`. The `"checking image: %w"` wrapping in `buildRunnerImageIfMissing` is forward-compat but currently unreachable; `"building container runner image: %w"` IS reachable.
- `.github/workflows/ci.yml` is on the protected-files list — modifications route through the fallback-review issue path; bridge push may not open the PR automatically.
- `update_issue` body limit is 10 KB. Long Run History entries must be truncated before each monthly refresh.
- The duplicate-code detector fires periodically against `main`. Each new finding should be cross-checked against in-flight PRs before starting work — #243 today re-surfaced the same 5 sites #237 covers; recommended "Close issue".
- The duplicate-code detector created 3 new high-quality findings on 2026-06-23: #251 (docker exec quoting, 9 sites), #252 (DinD container readiness probe, 2 sites), #253 (ValidateAWFHygiene/Inner near-duplicates, 75 lines each). #251 landed as `cc4201c`; #252 bridge-pending this run; #253 design notes posted.
- The escape behavior of `strconv.Quote` was verified end-to-end: input `evil"; rm -rf /; "` round-trips as `docker exec "evil\"; rm -rf /; \"" echo ok` — the inner double-quotes are escaped, so a malicious container name cannot break out of the shell-quoted name segment. The 6 `TestProbeDinDContainerReadiness_*` sub-tests this run pin this behavior at the `ProbeDinDContainerReadiness` boundary.
- The 6 `container*CheckCommand` helpers in `internal/agentic/agentic.go` all have direct tests via `TestContainer{InnerNetwork,AWF,InnerResolv,MTU,AWFServiceRouting,NodeNPM}CheckCommand`. The `TestContainerNodeNPMCheckCommand` added in run 28055272739 also asserts the double-quote wrapping, acting as a regression guard for the DockerExecCommand refactor.
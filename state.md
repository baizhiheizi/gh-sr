---
name: repo-assist-state
description: Repo Assist persistent state — in-flight work, backlog, verified knowledge
metadata:
  type: project
---

# Repo Assist state — 2026-06-25

## Last run (28147522118) — Tasks 5, 2, 3

- **Task 3** — branch `repo-assist/refactor-validatecontainer-issue-260-2026-06-25`, commit `cca95ec`: extracted `runContainerCheck(h, spec)` helper + `containerCheckSpec` struct in `internal/agentic/agentic.go`. All six `ValidateContainer*` wrappers collapse from ~22 lines each to a single call. Closes #260. PR bridge-pending.
- **Task 5** — branch `repo-assist/improve-quote-container-name-2026-06-25`, commit `e7dcbb7`: extracted `runner.QuoteContainerName(cname)` helper using `strconv.Quote`. Replaces 5 inline quoting call sites across `disk.go` / `environment.go` / `doctor.go`. `DockerExecCommand` now routes through `QuoteContainerName` internally. Drops unused `hostshell`/`strconv` imports. PR bridge-pending.
- **Task 2** — 1 new comment on #262. Posted design notes on the `orchestrate` helper signature (3 options) and asked for maintainer signal on `applyContainerImageExtras` asymmetry + `RebuildImage`'s `partitionRebuildTargets` placement.
- **Task 11** — `update_issue` on #100: prepended this run, removed completed #260 follow-up from Future Work, added new bridge-pending PR review items, added #262 "Check comment" item.
- **Verified:** build, vet, race (12/12), gofmt OK. `internal/agentic` coverage 82.7% (unchanged); `internal/runner` coverage unchanged.

## In-flight

- **Bridge-pending branches:** `refactor-validatecontainer-issue-260-2026-06-25`, `improve-quote-container-name-2026-06-25` (this run); `refactor-docker-group-add-issue-261-2026-06-24` (opened as PR #263, prior run).
- **Open PRs awaiting review:** #259 (AWF hygiene, closes #253), #258 (autostart tests), #257 (DinD readiness, closes #252).
- **Fallback-review:** #241 (gofmt CI), #225 (deps bump).
- **Issue #243** — re-detection of #237's 5 sites. Recommended "Close issue".

## Backlog

- **#262** — `orchestrate` helper in `internal/ops/ops.go` collapses Up/Down/Restart/RebuildImage/Update. Awaiting maintainer signal on helper signature.
- **#132 (gh sr storage)** — on hold pending maintainer signal on loop-mount persistence.

## Task 2 backlog cursor

- 16 open issues (1 human = #132, 2 fresh detector findings = #262 commented / #260 closed, 12 bot-generated historical). 0 unlabelled. Cursor at #262; next run will check #260/#261 close state and look for new findings.

## Notes

- **Bridge latency:** `create_pull_request` returns success with patch/bundle; bridge pushes branch + opens PR afterwards (30 min – 5h observed).
- **Posture:** revert rate is 0/19+ over the life of the workflow.
- **Maintainer merge cadence (recent):** an-lee merged PR #255 on 2026-06-24 ~11:18 UTC, PR #256 shortly after.
- **6 PRs landed in last 7 days:** #256, #255, #240, #239, #235, #234.

## Verified knowledge (one-line per fact)

- `safeoutputs` create_pull_request creates a **fallback review issue** when the patch modifies protected files (`go.mod`, `go.sum`, `CHANGELOG.md`, `.github/workflows/ci.yml`); branch IS pushed to origin but PR doesn't open.
- `runOnHostOS[T any]` in `internal/runner/hostos.go` is the canonical host-OS dispatcher.
- `internal/ops/connectHostFn` is a package-level seam that tests swap via `installMockConnectHost` + `connectHostMu`.
- `GitHubClient.GetLatestRunnerVersion()` uses `sync.Once` and caches the version+error.
- `IsContainerMode()` returns `true` for both `RunnerMode: container` and `Profile: agentic`. Therefore `IsContainerMode() && IsAgentic()` ≡ `IsAgentic()`.
- `uniqueStringsBy(runners, key)` is the canonical dedupe+sort helper in `internal/doctor/doctor.go`.
- `installTargetsForHost(runners, hostName, predicate)` is the canonical host-filter+instance-flatten helper in `internal/doctor/doctor.go`.
- `writeHostBanner(w, prefix, addr)` is the canonical local-vs-SSH banner helper in `internal/ops/ops.go`.
- `printAgenticFailures(w, hostName, r, defaultSev, prefix, failures)` is the canonical agentic-failure renderer in `internal/doctor/doctor.go`.
- `failureCollector` is the canonical thread-safe accumulator + WaitGroup helper in `internal/agentic/agentic.go`. Methods: `append(f)`, `spawn(fn)`, `wait()`. Method named `spawn` rather than `go` because `go` is a Go reserved keyword.
- `runner.DockerExecCommand(cname, innerCmd string) string` is the canonical `docker exec "name" innerCmd` builder in `internal/runner/container.go`. Routes through `QuoteContainerName` internally (as of 2026-06-25).
- `runner.QuoteContainerName(cname string) string` is the canonical container-name shell-quoting helper in `internal/runner/container.go`. Uses `strconv.Quote`. Replaces inline `hostshell.PosixSingleQuote`/`strconv.Quote` across `disk.go:86,495`, `environment.go:100,122,132`, and `doctor.go:513,557` (consolidated 2026-06-25).
- `runner.ProbeDinDContainerReadiness(h, cname string) (ContainerReadinessReport, error)` is the canonical DinD readiness triad in `internal/runner/container.go`. Returns `ContainerReadinessReport{State, InnerDockerdOK, Registered}`.
- `runner.addSSHTUserToDockerGroup(h, w, runnerName, lead, verb string, errOnUsermodFail func(sshUser string, err error) error) error` is the canonical docker-group-add helper in `internal/runner/docker.go`. Used by `ensureDockerGroupAccess` and the `installHostDocker` non-root tail. Empty sshUser triggers `errOnUsermodFail("", errors.New("ssh user is empty"))`.
- `runContainerCheck(h *host.Host, spec containerCheckSpec) []PrereqFailure` is the canonical Linux-gate + Run + PrereqFailure helper for the `ValidateContainer*` family in `internal/agentic/agentic.go` (added 2026-06-25 for #260). `containerCheckSpec` carries `{name, checkCmd, message, remediation, docRef}`. Short-circuits on nil host or non-Linux OS; emits a single `SeverityWarning` `PrereqFailure` when `spec.checkCmd` errors. `ValidateContainerMTU` keeps its `hostEgressMTU` numeric guard at the wrapper level.
- `containerImageExists(h, imageTag)` swallows `h.Run` errors and returns `(false, nil)`.
- `.github/workflows/ci.yml` is on the protected-files list — modifications route through the fallback-review issue path.
- `update_issue` body limit is 10 KB. Long Run History entries must be truncated before each monthly refresh.
- The duplicate-code detector fires periodically against `main`. Each new finding should be cross-checked against in-flight PRs before starting work.
- Duplicate-code findings 2026-06-23/24/25: #251 landed (`cc4201c`); #252 bridge-pending as PR #257; #253 bridge-pending as PR #259; #261 bridge-pending as PR #263; #260 bridge-pending (this run).
- The escape behavior of `strconv.Quote` was verified end-to-end: input `evil"; rm -rf /; "` round-trips as `docker exec "evil\"; rm -rf /; \"" echo ok`. Same shape is now also pinned at the `QuoteContainerName` helper level.
- The quoting policy across the codebase has been unified on `strconv.Quote` (Go double-quote) for docker CLI shell args; `hostshell.PosixSingleQuote` is reserved for values embedded inside single-quoted shell snippets (e.g. the inner argument to `sh -c '...'`).

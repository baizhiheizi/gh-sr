---
name: repo-assist-state
description: Repo Assist persistent state — in-flight work, backlog, verified knowledge
metadata:
  type: project
---

# Repo Assist state — 2026-06-25

## Last run (28161948236) — Tasks 4, 9, 3

- **No-action run.** Task 3 fell back to Task 2: no comment-worthy issues (the most actionable candidate #262 has prior Repo Assist design notes still awaiting maintainer response). Task 4 (Engineering Investments): no clearly beneficial change - the most actionable candidates (deps bump #225, gofmt CI #241) are bridge-pending patches, and the only live detector finding #262 needs maintainer signal on the helper signature first. Task 9 (Testing Improvements): test coverage already strong (ops 68.2%, runner 54.3%, agentic 81.0%).
- **Task 11** — `update_issue` on #100: prepended no-action run entry; removed #243 (already closed 2026-06-23), #265/#266 bridge-pending items (both now merged); added #257 (now `dirty` after subsequent merges).
- **Verified:** build OK, test OK (12/12), gofmt clean.

## In-flight

- **Bridge-pending branches:** none for this run.
- **Open PRs awaiting review:** #257 (DinD readiness probe, **dirty** after #263/#264/#265/#266 landed; needs rebase), #258 (autostart Detect/IsStale tests), #259 (AWF hygiene refactor).
- **Fallback-review:** #241 (gofmt CI), #225 (deps bump).

## Backlog

- **#262** — `orchestrate` helper in `internal/ops/ops.go` collapses Up/Down/Restart/RebuildImage/Update. Awaiting maintainer signal on helper signature (boolean flag vs opSpec struct vs closure-internal) and on `applyContainerImageExtras` asymmetry + `partitionRebuildTargets` placement.
- **#132 (gh sr storage)** — on hold pending maintainer signal on loop-mount persistence.

## Task 2 backlog cursor

- 13 open issues (1 human = #132, 3 fresh detector findings = #262 active / #252 closing via #257 / #243 closed, 9 bot-generated historical including auto-managed monthly-activity/detector/no-op). 0 unlabelled. Cursor at #262; next run will check for new detector findings.

## Notes

- **Bridge latency:** `create_pull_request` returns success with patch/bundle; bridge pushes branch + opens PR afterwards (30 min – 5h observed).
- **Posture:** revert rate is 0/19+ over the life of the workflow.
- **Maintainer merge cadence (recent):** an-lee merged PR #255 on 2026-06-24 ~11:18 UTC, PR #256 shortly after. Then #263, #264, #265, #266 on 2026-06-25 (4 merges in one day).
- **7 PRs landed in last 2 days:** #263, #264, #265, #266 (2026-06-25), #259, #258, #255 (2026-06-24).
- **No-action runs are valid** when all of: (a) the only live detector finding needs maintainer signal, (b) bridge-pending patches are not actionable, (c) Test Improver already driving coverage. "Do nothing rather than add low-value output" is the right verdict.

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
- `runner.QuoteContainerName(cname string) string` is the canonical container-name shell-quoting helper in `internal/runner/container.go`. Uses `strconv.Quote`. Replaces inline `hostshell.PosixSingleQuote`/`strconv.Quote` across `disk.go:86,495`, `environment.go:99,121,131`, and `doctor.go:513,557` (consolidated 2026-06-25).
- `runner.ProbeDinDContainerReadiness(h, cname string) (ContainerReadinessReport, error)` is the canonical DinD readiness triad in `internal/runner/container.go` (PR #257, not yet merged on main). Returns `ContainerReadinessReport{State, InnerDockerdOK, Registered}`.
- `runner.addSSHTUserToDockerGroup(h, w, runnerName, lead, verb string, errOnUsermodFail func(sshUser string, err error) error) error` is the canonical docker-group-add helper in `internal/runner/docker.go`. Used by `ensureDockerGroupAccess` and the `installHostDocker` non-root tail. Empty sshUser triggers `errOnUsermodFail("", errors.New("ssh user is empty"))`.
- `runContainerCheck(h *host.Host, spec containerCheckSpec) []PrereqFailure` is the canonical Linux-gate + Run + PrereqFailure helper for the `ValidateContainer*` family in `internal/agentic/agentic.go` (added 2026-06-25 for #260, PR #265 merged). `containerCheckSpec` carries `{name, checkCmd, message, remediation, docRef}`. Short-circuits on nil host or non-Linux OS; emits a single `SeverityWarning` `PrereqFailure` when `spec.checkCmd` errors. `ValidateContainerMTU` keeps its `hostEgressMTU` numeric guard at the wrapper level.
- `containerImageExists(h, imageTag)` swallows `h.Run` errors and returns `(false, nil)`.
- `.github/workflows/ci.yml` is on the protected-files list — modifications route through the fallback-review issue path.
- `update_issue` body limit is 10 KB. Long Run History entries must be truncated before each monthly refresh.
- The duplicate-code detector fires periodically against `main`. Each new finding should be cross-checked against in-flight PRs before starting work.
- Duplicate-code findings 2026-06-17/24/25: #251 landed (`cc4201c`); #252 PR #257 dirty, needs rebase; #253 bridge-pending as PR #259; #261 bridge-pending as PR #263 (merged); #260 PR #265 (merged); #262 active, awaiting maintainer signal; #207/#209/#210/#243 all closed.
- The escape behavior of `strconv.Quote` was verified end-to-end: input `evil"; rm -rf /; "` round-trips as `docker exec "evil\"; rm -rf /; \"" echo ok`. Same shape is now also pinned at the `QuoteContainerName` helper level.
- The quoting policy across the codebase has been unified on `strconv.Quote` (Go double-quote) for docker CLI shell args; `hostshell.PosixSingleQuote` is reserved for values embedded inside single-quoted shell snippets (e.g. the inner argument to `sh -c '...'`).

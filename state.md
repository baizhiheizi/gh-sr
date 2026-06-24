---
name: repo-assist-state
description: Repo Assist persistent state — in-flight work, backlog, verified knowledge
metadata:
  type: project
---

# Repo Assist state — 2026-06-24 ~17:00 UTC

## Last run (28127022038) — Tasks 2, 10, 3

- **Task 10/3** — branch `repo-assist/refactor-docker-group-add-issue-261-2026-06-24`, commit `ed87ae1`: extracted `addSSHTUserToDockerGroup(h, w, runnerName, lead, verb, errOnUsermodFail)` helper in `internal/runner/docker.go`. `ensureDockerGroupAccess` and `installHostDocker` non-root tail both collapse to use the new helper. 4 new sub-tests. Closes #261. PR bridge-pending.
- **Task 2** — no new comments. Candidates either on hold or covered by prior context.
- **Task 11** — `update_issue` on #100: prepended this run, removed merged PR #255, promoted prior bridge-pending branches to PR numbers (#258, #259).
- **Verified:** build, vet, race (12/12), gofmt OK. `internal/runner` coverage 53.5% (unchanged).

## In-flight

- **Branch `repo-assist/refactor-docker-group-add-issue-261-2026-06-24`** (`ed87ae1`) — bridge-pending, closes #261.
- **Open PRs awaiting review:** #259 (AWF hygiene, closes #253), #258 (autostart tests), #257 (DinD readiness, closes #252). All bridge-pushed from earlier runs today.
- **Fallback-review:** #241 (gofmt CI), #225 (deps bump).
- **Issue #243** — re-detection of #237's 5 sites. Recommended "Close issue".

## Backlog

- **#260** — `runContainerCheck` collapses 6 `ValidateContainer*` Linux-gate + Run + PrereqFailure sites in `internal/agentic/agentic.go` (~72→~30 lines). Same shape as #253.
- **#262** — `orchestrate(w, cfg, mgr, filterHost, filterRepo, nameArgs, verb, applyExtras, op)` in `internal/ops/ops.go` collapses Up/Down/Restart/RebuildImage/Update template. Needs `applyContainerImageExtras` asymmetry analysis first.
- **`runner.QuoteContainerName(cname string) string`** — consolidates `strconv.Quote` / `hostshell.PosixSingleQuote` drift in `doctor.go:503,548`, `disk.go:86,495`, `environment.go:100,122,132`.
- **#132 (gh sr storage)** — on hold pending maintainer signal on loop-mount persistence.

## Task 2 backlog cursor

- 16 open issues (1 human = #132, 3 fresh duplicate-code detectors = #260/#261/#262, 12 bot-generated historical). 0 unlabelled. All bot-generated issues have prior Repo Assist context.

## Notes

- **Bridge latency:** `create_pull_request` returns success with patch/bundle; bridge pushes branch + opens PR afterwards (30 min – 5h observed). Verify with `list_pull_requests` next run.
- **Posture:** revert rate is 0/19+ over the life of the workflow. Recent merges all landed cleanly.
- **Maintainer merge cadence (recent):** an-lee merged PR #255 on 2026-06-24 ~11:18 UTC, PR #256 shortly after.
- **6 PRs landed in last 7 days:** #256 (test-improver CollectStatus), #255 (perf-rebuild-batch-stop-remove), #240 (writeHostBanner, #237), #239 (filter-collect, #238), #235 (stale-registration, #230), #234 (image-build preamble, #228).
- **No `bug` / `help wanted` / `good first issue` issues currently open.** Next high-confidence action is one of the backlog items above.

## Verified knowledge (one-line per fact)

- `safeoutputs` create_pull_request creates a **fallback review issue** when the patch modifies protected files (`go.mod`, `go.sum`, `CHANGELOG.md`, `.github/workflows/ci.yml`); branch IS pushed to origin but PR doesn't open.
- `runOnHostOS[T any]` in `internal/runner/hostos.go` is the canonical host-OS dispatcher.
- `internal/ops/connectHostFn` is a package-level seam that tests swap via `installMockConnectHost` + `connectHostMu` for race-detector-clean parallel tests.
- `GitHubClient.GetLatestRunnerVersion()` uses `sync.Once` and caches the version+error.
- `IsContainerMode()` returns `true` for both `RunnerMode: container` and `Profile: agentic`. Therefore `IsContainerMode() && IsAgentic()` ≡ `IsAgentic()`.
- `uniqueStringsBy(runners, key)` is the canonical dedupe+sort helper in `internal/doctor/doctor.go`.
- `installTargetsForHost(runners, hostName, predicate)` is the canonical host-filter+instance-flatten helper in `internal/doctor/doctor.go`.
- `writeHostBanner(w, prefix, addr)` is the canonical local-vs-SSH banner helper in `internal/ops/ops.go`. `prefix` is everything up to the suffix.
- `printAgenticFailures(w, hostName, r, defaultSev, prefix, failures)` is the canonical agentic-failure renderer in `internal/doctor/doctor.go`.
- `failureCollector` is the canonical thread-safe accumulator + WaitGroup helper in `internal/agentic/agentic.go`. Methods: `append(f)`, `spawn(fn)`, `wait()`. Method named `spawn` rather than `go` because `go` is a Go reserved keyword.
- `runner.DockerExecCommand(cname, innerCmd string) string` is the canonical `docker exec "name" innerCmd` builder in `internal/runner/container.go`. Uses `strconv.Quote`.
- `runner.ProbeDinDContainerReadiness(h, cname string) (ContainerReadinessReport, error)` is the canonical DinD readiness triad in `internal/runner/container.go`. Returns `ContainerReadinessReport{State, InnerDockerdOK, Registered}`. Short-circuits the inner probes for any state other than `running`/`restarting`.
- `runner.addSSHTUserToDockerGroup(h, w, runnerName, lead, verb string, errOnUsermodFail func(sshUser string, err error) error) error` is the canonical docker-group-add helper in `internal/runner/docker.go`. Used by `ensureDockerGroupAccess` (lead="  Added ", verb="to") and the `installHostDocker` non-root tail (lead="  Docker installed and ", verb="added to"). `errOnUsermodFail` callback receives the sshUser and underlying error; retry path passes `permissionDeniedError(h)` while fresh-install wraps with `fmt.Errorf("adding %s to docker group: %w", sshUser, err)`. Empty sshUser triggers `errOnUsermodFail("", errors.New("ssh user is empty"))`; the fresh-install silent-skip is preserved by the caller checking `h.SSHUser() == ""` before invoking the helper.
- `containerImageExists(h, imageTag)` swallows `h.Run` errors and returns `(false, nil)`.
- `.github/workflows/ci.yml` is on the protected-files list — modifications route through the fallback-review issue path.
- `update_issue` body limit is 10 KB. Long Run History entries must be truncated before each monthly refresh.
- The duplicate-code detector fires periodically against `main`. Each new finding should be cross-checked against in-flight PRs before starting work.
- Duplicate-code findings 2026-06-23/24: #251 landed (`cc4201c`); #252 bridge-pending as PR #257; #253 bridge-pending as PR #259; #261 bridge-pending (this run).
- The escape behavior of `strconv.Quote` was verified end-to-end: input `evil"; rm -rf /; "` round-trips as `docker exec "evil\"; rm -rf /; \"" echo ok`.
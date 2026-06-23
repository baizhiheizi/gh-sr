---
name: repo-assist-state
description: Repo Assist persistent state — completed work, in-flight PRs, backlog cursor, and notes
metadata:
  type: project
---

# Repo Assist state — last updated 2026-06-23 ~20:38 UTC

## Last run (2026-06-23 ~20:38 UTC, run 28055272739)

- Selected tasks: 4 (Engineering Investments), 2 (Issue Comments), 9 (Testing Improvements).
- **Task 4** — branch `repo-assist/eng-docker-exec-helper-issue-251` (2 commits `f9be85b` + `8c05ba1`): added `runner.DockerExecCommand(cname, innerCmd)` helper. Replaced 9 inline `docker exec` quoting blocks across `agentic.go` (1 + 6) and `doctor.go` (1; the `inspect` site left alone). 4 helper sub-tests + 1 `TestContainerNodeNPMCheckCommand` regression guard. Closes #251. Bridge-pending.
- **Task 2** — 3 new comments: #251 (fix-landed note); #252 (design notes for `ProbeDinDContainerReadiness` — struct return shape vs 3 booleans, polling vs one-shot caller policy, calls out a `runner.QuoteContainerName` helper that would unblock the refactor); #253 (design notes for `awfHygieneCheck` struct + `runAWFHygieneChecks` runner — nameSuffix as a runner parameter, RemediationTemplate via `fmt.Sprintf`).
- **Task 9** — `TestContainerNodeNPMCheckCommand` added to `internal/agentic/container_network_test.go` (matches the style of the other 5 `container*CheckCommand` helper tests). Also asserts the double-quote wrapping of the container name (regression guard for the DockerExecCommand refactor). Internal/agentic coverage 83.9%.
- **Task 11** — `update_issue` on #100 (8.0 KB body). Added the new run to the top of Run History. Promoted the bridge-pending branch as a Suggested Action. Added 3 new "Check comment" entries (#251, #252, #253).
- **Verified:** `go build ./...` OK, `go vet ./...` OK, `go test ./... -race -count=1` OK (12/12 packages), `gofmt -l .` clean.

## In-flight work

- **Branch `repo-assist/eng-docker-exec-helper-issue-251`** — 2 commits: DockerExecCommand refactor + 4 helper sub-tests + TestContainerNodeNPMCheckCommand. Bridge-pending. Closes #251. Patch at `/tmp/gh-aw/aw-repo-assist-eng-docker-exec-helper-issue-251.patch` (14 468 bytes).
- **Branch `repo-assist/improve-agentic-failure-collector`** — failureCollector refactor + 5 sub-tests. Bridge pending.
- **Branch `repo-assist/improve-issue-236-agentic-failures-helper`** — refactor + 7 sub-tests. Bridge pending.
- **Open PRs (bridge-pending or merged):** #250 (failureCollector), #246 (checkShellOK), #245 (orchestrator coverage), #240 (writeHostBanner, closes #237), #239 (filter-collect, closes #238), #235 (stale-registration, closes #230), #234 (image-build preamble, closes #228).
- **Fallback-review issues:** #241 (gofmt CI check, run 27894682067), #225 (deps bump, run 27822014346).
- **Issue #243** — re-detection of the same 5 sites #237 covers. Recommended "Close issue".

## Backlog / next high-value task

- The remaining duplicate-code findings against `775c939` are all addressed by open PRs (#228 → #234, #230 → #235, #236 → bridge-pending, #237 → #240, #238 → #239, #251 → bridge-pending this run).
- **Next high-confidence refactor #1 (Issue #252)** — `runner.ProbeDinDContainerReadiness(h, cname) (...)` or `runner.ContainerReadinessReport` struct. Design notes posted on #252 this run.
- **Next high-confidence refactor #2 (Issue #253)** — `awfHygieneCheck` struct + `runAWFHygieneChecks(h, pfx string, checks []awfHygieneCheck, suffix string)`. Design notes posted on #253 this run.
- **Next high-confidence refactor #3** — `runner.QuoteContainerName(cname string) string` helper that would consolidate the `strconv.Quote` / `hostshell.PosixSingleQuote` drift. Flagged in #100 Future Work this run.
- **#132 (gh sr storage)** — on hold pending maintainer signal on loop-mount persistence.

## Backlog cursor for Task 2 (Issue Comment)

- 15 open issues (1 human = #132, 14 bot-generated). 0 unlabelled. All bot issues now have Repo Assist comments.

## Completed work (compressed)

- **PR #250 (DRAFT 2026-06-23)** — failureCollector refactor. 1 helper + 3 call sites.
- **PR #246 (DRAFT 2026-06-23)** — checkShellOK refactor. 1 helper + 2 call sites.
- **PR #245 (DRAFT 2026-06-22)** — CleanupOffline + Setup/Update/RebuildImage orchestrator coverage.
- **PR #240 (DRAFT 2026-06-21)** — writeHostBanner refactor (closes #237). 1 helper + 5 call sites.
- **PR #239 (DRAFT 2026-06-20)** — doctor filter-collect refactor (closes #238).
- **PR #235 (DRAFT 2026-06-20)** — stale-registration probe refactor (closes #230).
- **PR #234 (DRAFT 2026-06-20)** — image-build preamble refactor (closes #228).
- **PR #232 (MERGED 2026-06-20)** — disk.go dispatcher refactor + ops service orchestrator tests.
- **PR #226 (MERGED)** — perf: hoist ContainerImageLayoutRevision out of Status() loop.
- **Earlier merges (compressed):** #224, #223, #222, #221, #220, #215, #211, #206, #202, #200, #199, #198, #197, #194, #193, #192, #191, #189, #187, #184, #183, #180, #178, #172; 2026-06-12/13 batch: #159, #158, #157, #156, #155, #149, #147, #146, #142, #141, #139, #136, #131, #128, #127, #126, #113, #114, #110, #118, #99.

## Activity / PR history (compressed)

- 2026-06-23 (runs 28055272739 + 28037507600, 20:38 + 15:59 UTC): Tasks 4/2/9 (DockerExecCommand, closes #251) + Tasks 3/5/1 (failureCollector refactor).
- 2026-06-23 (run 28018815293, ~10:27 UTC): Tasks 8/2/5 — checkShellOK refactor (PR #246) + TUI strings.Builder held back.
- 2026-06-22 (run 27932993839, ~07:30 UTC): Tasks 9/3/2 — ops orchestrator coverage tests (PR #245).
- 2026-06-21 (run 27916163915, ~16:30 UTC): Tasks 5/10/4 — printAgenticFailures refactor (closes #236) + comment on #243.
- 2026-06-21 (run 27894682067, ~14:30 UTC): Tasks 5/4/2 — banner helper (closes #237) + gofmt CI check.
- 2026-06-20 (3 runs): image-build preamble (PR #234), stale-registration probe (PR #235), doctor.go filter-collect (PR #239).
- 2026-06-19 (run 27846567496, ~22:30 UTC): Tasks 5/3/9 — disk.go dispatcher refactor (closes #229) + ops service tests. PR #232.

## Notes for next run

- **PR creation bridge latency: 30 min – 5h observed.** `create_pull_request` returns `success` with patch/bundle artifacts; bridge pushes branch + opens PR afterwards. Confirmed for #232 (merged), #245, #246, #240, #239, #235, #234, #233, #250.
- **#241 (gofmt CI check)** — fallback-review issue, maintainer needs to apply the patch from run 27894682067 manually.
- **#243 (re-detection)** — same 5 sites as #237. Recommended "Close issue" in the suggested-actions list.
- **Posture:** revert rate is 0/19+ over the life of the workflow. Recent merges (#211, #215, #220, #221, #222, #223, #224, #226, #232) all landed cleanly.
- **Next high-confidence action if a new `bug` / `help wanted` / `good first issue` issue opens:** investigate and implement a minimal fix. None currently open.
- **Repo state:** 15 open issues (1 human = #132, 14 bot-generated). 0 unlabelled. 10 open Repo Assist PRs + 3 bridge-pending branches + 2 fallback-review issues (#225, #241).

## Verified knowledge

- `safeoutputs` create_pull_request creates a **fallback review issue** when the patch modifies protected files (`go.mod`, `go.sum`, `CHANGELOG.md`, `.github/workflows/ci.yml`); branch IS pushed to origin but PR doesn't open. Sometimes also returns `success` with patch/bundle artifacts when the agent has no git credentials (patch at `/tmp/gh-aw/aw-*.patch`, bundle at `/tmp/gh-aw/aw-*.bundle`); the bridge typically pushes and opens the PR afterwards.
- `git log origin/repo-assist/<branch> -p -- go.mod` shows the branch's go.mod history — useful for verifying whether a dep-update was actually committed.
- `internal/ops/connectHostFn` is a package-level seam that tests swap via `installMockConnectHost` + `connectHostMu` for race-detector-clean parallel tests.
- `runOnHostOS[T any]` in `internal/runner/hostos.go` is the canonical host-OS dispatcher. Any new switch on `h.OS` in this package is a duplicate-code smell.
- `GitHubClient.GetLatestRunnerVersion()` uses `sync.Once` and caches the version+error. Repeat calls within one process are free.
- `containerImageExists(h, imageTag)` swallows `h.Run` errors and returns `(false, nil)`. The `"checking image: %w"` wrapping in `buildRunnerImageIfMissing` is forward-compat but currently unreachable; `"building container runner image: %w"` IS reachable.
- `IsContainerMode()` returns `true` for both `RunnerMode: container` and `Profile: agentic` (via `EffectiveRunnerMode`). Therefore `IsContainerMode() && IsAgentic()` ≡ `IsAgentic()`.
- `uniqueStringsBy(runners, key)` is the canonical dedupe+sort helper in `internal/doctor/doctor.go`. Empty keys are skipped uniformly.
- `installTargetsForHost(runners, hostName, predicate)` is the canonical host-filter+instance-flatten helper in `internal/doctor/doctor.go`.
- `writeHostBanner(w, prefix, addr)` is the canonical local-vs-SSH banner helper in `internal/ops/ops.go`. `prefix` is everything up to the suffix.
- `printAgenticFailures(w, hostName, r, defaultSev, prefix, failures)` is the canonical agentic-failure renderer in `internal/doctor/doctor.go`.
- `failureCollector` is the canonical thread-safe accumulator + WaitGroup helper in `internal/agentic/agentic.go`. Methods: `append(f)`, `spawn(fn)`, `wait()`. Method named `spawn` rather than `go` because **`go` is a reserved keyword in Go and the parser rejects method names that match reserved keywords** — `c.go(func(){...})` is a syntax error, not a method call. Use `c.spawn(func(){...})` instead.
- `runner.DockerExecCommand(cname, innerCmd string) string` is the canonical `docker exec "name" innerCmd` builder in `internal/runner/container.go`. Uses `strconv.Quote` (Go-style double-quote) to match the format the existing `agentic_awf_hygiene_test.go:240-245` and `container_network_test.go` test pins assert. The `runner` package's own `disk.go:86,495` and `environment.go:100,122,132` sites use `hostshell.PosixSingleQuote` (POSIX single-quote) — both produce shell-safe output but a future consolidation should align the two policies.
- `.github/workflows/ci.yml` is on the protected-files list — modifications route through the fallback-review issue path; bridge push may not open the PR automatically.
- `update_issue` body limit is 10 KB. Long Run History entries must be truncated before each monthly refresh.
- The duplicate-code detector fires periodically against `main` (currently `775c939`). Each new finding should be cross-checked against in-flight PRs before starting work — #243 today re-surfaced the same 5 sites #237 covers; recommended "Close issue".
- The duplicate-code detector created 3 new high-quality findings on 2026-06-23: #251 (docker exec quoting, 9 sites), #252 (DinD container readiness probe, 2 sites), #253 (ValidateAWFHygiene/Inner near-duplicates, 75 lines each). #251 was implemented this run; #252 + #253 design notes posted this run.
- The escape behavior of `strconv.Quote` was verified end-to-end this run: input `evil"; rm -rf /; "` round-trips as `docker exec "evil\"; rm -rf /; \"" echo ok` — the inner double-quotes are escaped, so a malicious container name cannot break out of the shell-quoted name segment.
- The 6 `container*CheckCommand` helpers in `internal/agentic/agentic.go` all have direct tests via `TestContainer{InnerNetwork,AWF,InnerResolv,MTU,AWFServiceRouting,NodeNPM}CheckCommand`. The new `TestContainerNodeNPMCheckCommand` also asserts the double-quote wrapping, acting as a regression guard for the DockerExecCommand refactor.

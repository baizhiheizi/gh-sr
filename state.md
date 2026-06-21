---
name: repo-assist-state
description: Repo Assist persistent state — completed work, in-flight PRs, backlog cursor, and notes
metadata:
  type: project
---

# Repo Assist state — last updated 2026-06-21 ~14:30 UTC

## Last run (2026-06-21 ~14:30 UTC, run 27894682067)

- Selected tasks: 5 (Coding Improvements), 4 (Engineering Investments), 2 (Issue Investigation and Comment).
- **Task 5** — branch `repo-assist/improve-issue-237-host-banner-helper`, commit `595f436`: added `writeHostBanner(w io.Writer, prefix, addr string)` in `internal/ops/ops.go`; replaced 5 inline `if config.IsLocalAddr(hcfg.Addr) { … (local) … } else { … (<addr>) … }` blocks across `Setup` / `Remove` (ops.go) and `ServiceInstall` / `ServiceUninstall` / `ServiceCleanup` (service.go). The `ServiceCleanup` `name` vs `rc.Host` drift is preserved verbatim — `writeHostBanner` owns only the suffix; prefix composition stays at each call site because the verb/object shapes differ (`Setting up on`, `Removing %s from`, `Autostart for %s on`, `Removing autostart for %s on`, `Checking orphan runners on`). 24 lines of duplicate bodies → 1 helper + 5 single-line call sites. `TestWriteHostBanner` (5 sub-tests: local, remote, case-insensitive `Local`, verb+args prefix, empty prefix). Closes #237.
- **Task 4** — branch `repo-assist/eng-gofmt-ci-check-2026-06-21`, commit `093689d`: inserted a `Format` step between `Vet` and `Test` in `.github/workflows/ci.yml` running `gofmt -l .` and failing CI on non-empty output. Repo currently gofmt-clean (lands a passing pipeline); future drift caught at PR time, eliminating dedicated gofmt-only cleanup PRs (the most recent was #222). Closes the missing half of the deferred #160/#161 CI-check follow-up.
- **Task 2** — `add_comment` on #237 (`aw_Qm9oKc2o`) summarising the `writeHostBanner` refactor.
- **Task 11** — `update_issue` on #100 succeeded (full body refresh). Added 2 bridge-pending entries (#237 banner refactor, gofmt CI check); promoted #238 bridge-pending line to PR #239 review line; retired #160/#161 (superseded); refreshed Future Work to keep #236 as the next deferred duplicate-code cleanup.
- **Verified:** `go build ./...` OK, `go vet ./...` OK, `go test ./...` OK (12/12 packages), `go test -race ./internal/ops/...` OK, `gofmt -l internal/ops/` clean, `gofmt -l .` clean.

## In-flight work

- **Branch `repo-assist/improve-issue-237-host-banner-helper`** — 1 commit (refactor + 5 sub-tests). Bridge pending.
- **Branch `repo-assist/eng-gofmt-ci-check-2026-06-21`** — 1 commit (CI workflow change). Bridge pending. Modifies `.github/workflows/ci.yml` which is on the protected-files list — the bridge push may be rejected and require manual PR creation per the `#160/#161` pattern.
- **PR #239 (DRAFT 2026-06-20)** — `[repo-assist] refactor(doctor): collapse uniqueX + installTargetsXForHost via generic helpers` (closes #238).
- **PR #235 (DRAFT 2026-06-20)** — `[repo-assist] refactor(runner): collapse stale-registration probe` (closes #230).
- **PR #234 (DRAFT 2026-06-20)** — `[repo-assist] refactor(runner): collapse image-build preamble` (closes #228).
- **PR #233 (DRAFT 2026-06-20)** — `[test-improver] test(ops): cover Logs orchestrator`.
- **Issue #225 (FALLBACK REVIEW — dep-update)** — patch downloadable from run 27822014346.

## Backlog / next high-value task

- **#236 (duplicate-code)** — `printAgenticFailures(w, hostName, prefix, failures)` helper in `internal/doctor/doctor.go` consolidating 2× 18-line failure-iteration blocks. ~30 min. ~36 → ~18 lines. Now the next-highest-priority deferred duplicate-code cleanup (was #236/#237 last run; #237 just landed).
- **#132 (gh sr storage)** — on hold pending maintainer signal on loop-mount persistence.

## Backlog cursor for Task 2 (Issue Comment)

- 15 open issues (1 human = #132, 14 bot-generated including #225, #230, #236, #237, #238, plus monthly rollups #85/#100/#109/#124/#125/#214/#148). 0 unlabelled. #228 closed via PR #234.

## Completed work (recent PRs)

- **PR #239 (DRAFT 2026-06-20)** — doctor filter-collect refactor (closes #238). Bridge latency ~4h.
- **PR #235, #234, #233 (DRAFTS 2026-06-20)** — see "In-flight work" above.
- **PR #232 (MERGED 2026-06-20)** — disk.go dispatcher refactor + ops service orchestrator tests.
- **PR #226 (MERGED)** — perf: hoist ContainerImageLayoutRevision out of Status() loop.
- **PR #224 (MERGED)** — test(ops): Up orchestrator coverage 0% → 77.8%.
- **PR #223 (MERGED)** — delete dead containerLocalStatusFromDocker (closes #217).
- **PR #222 (MERGED)** — gofmt clean drift in 5 files.
- **PR #221 (MERGED)** — extract resolveStateDirOrFallback helper (closes #218).
- **PR #220 (MERGED)** — archive resolve-duplicate-code change.
- **PR #215 (MERGED)** — collapse 3 docker create env arg helpers (closes #209).
- **Earlier (compressed):** #211, #206, #202, #200, #199, #198, #197, #194, #193, #192, #191, #189, #187, #184, #183, #180, #178, #172; 2026-06-12/13 batch: #159, #158, #157, #156, #155, #149, #147, #146, #142, #141, #139, #136, #131, #128, #127, #126, #113, #114, #110, #118, #99.

## Activity / PR history (compressed)

- 2026-06-21 (run 27894682067, ~14:30 UTC): Tasks 5/4/2 — banner helper (closes #237) + gofmt CI check + comment on #237.
- 2026-06-20 (run 27882401019, ~16:30 UTC): Tasks 2/3/5 — doctor.go filter-collect refactor (closes #238). 2 helpers replace 6.
- 2026-06-20 (run 27874659199, ~11:30 UTC): Tasks 4/2/3 — stale-registration probe refactor (closes #230). PR #235.
- 2026-06-20 (run 27867528048, ~09:30 UTC): Tasks 2/3/8 — image-build preamble refactor (closes #228). PR #234.
- 2026-06-19 (run 27846567496, ~22:30 UTC): Tasks 5/3/9 — disk.go dispatcher refactor (closes #229) + ops service tests. PR #232.

## Notes for next run

- **PR creation bridge latency: 30 min – 4h observed.** Pattern: `create_pull_request` returns `success` with patch/bundle artifacts; bridge pushes branch + opens PR afterwards. Confirmed for #232 (merged 2026-06-20), #239, #235, #234, #233, #183, #180, #172, #171.
- **#236 now the next-highest-priority deferred duplicate-code cleanup.** Small (~30 min) and low-risk.
- **#225 (dep-update fallback).** Patch downloadable from run 27822014346.
- **#100 body refresh.** `update_issue` succeeded this run (10 KB limit reached; future updates may need to truncate Run History).
- **Posture:** revert rate is 0/18+ over the life of the workflow. Recent merges (#211, #215, #220, #221, #222, #223, #224, #226, #232) all landed cleanly.
- **#160/#161 (gofmt CI check) bridge pending** — this run shipped a `repo-assist/eng-gofmt-ci-check-2026-06-21` branch (single commit `093689d`). Modifies `.github/workflows/ci.yml` (protected file). Bridge may need the manual-apply fallback. If the bridge succeeds the original #160/#161 issues can be closed as superseded.
- **Next high-confidence action if a new `bug` / `help wanted` / `good first issue` issue opens:** investigate and implement a minimal fix. None currently open.
- **Repo state:** 15 open issues (1 human = #132, 14 bot-generated). 0 unlabelled. 4 open Repo Assist PRs (#233/#234/#235/#239) + 2 bridge-pending branches (#237 banner refactor, gofmt CI check).

## Verified knowledge

- `safeoutputs` create_pull_request creates a **fallback review issue** when the patch modifies protected files (`go.mod`, `go.sum`, `CHANGELOG.md`, `.github/workflows/ci.yml`); branch IS pushed to origin but PR doesn't open.
- `safeoutputs` create_pull_request also sometimes returns `success` with patch/bundle artifacts when the agent has no git credentials. Patch at `/tmp/gh-aw/aw-*.patch` and bundle at `/tmp/gh-aw/aw-*.bundle` are the durable artifacts; the bridge typically pushes and opens the PR afterwards.
- `git log origin/repo-assist/<branch> -p -- go.mod` shows the branch's go.mod history — useful for verifying whether a dep-update was actually committed.
- `internal/ops/connectHostFn` is a package-level seam that tests swap via `installMockConnectHost` + `connectHostMu` for race-detector-clean parallel tests.
- `runOnHostOS[T any]` in `internal/runner/hostos.go` is the canonical host-OS dispatcher. Any new switch on `h.OS` in this package is a duplicate-code smell.
- `GitHubClient.GetLatestRunnerVersion()` uses `sync.Once` and caches the version+error. Repeat calls within one process are free — refactors that extract the version→arch→imageTag preamble don't lose the cache hit.
- `containerImageExists(h, imageTag)` swallows `h.Run` errors and returns `(false, nil)`. The `"checking image: %w"` wrapping in `buildRunnerImageIfMissing` is forward-compat but currently unreachable; `"building container runner image: %w"` IS reachable.
- `IsContainerMode()` returns `true` for both `RunnerMode: container` and `Profile: agentic` (via `EffectiveRunnerMode`). Therefore `IsContainerMode() && IsAgentic()` ≡ `IsAgentic()`. Used this simplification in the #238 agentic predicate.
- `uniqueStringsBy(runners, key)` is the canonical dedupe+sort helper in `internal/doctor/doctor.go`. Empty keys are skipped uniformly.
- `installTargetsForHost(runners, hostName, predicate)` is the canonical host-filter+instance-flatten helper in `internal/doctor/doctor.go`. Predicate signature `func(*config.RunnerConfig) bool` so callers can use any pointer-receiver method.
- `writeHostBanner(w, prefix, addr)` is the canonical local-vs-SSH banner helper in `internal/ops/ops.go` (added this run). `prefix` is everything up to the suffix; the helper handles the `IsLocalAddr` toggle between `(local)` and `(<addr>)`. Callers compose the prefix with whatever verb/object they need.
- `.github/workflows/ci.yml` is on the protected-files list — modifications route through the fallback-review issue path; bridge push may not open the PR automatically.
- `update_issue` body limit is 10 KB. Long Run History entries must be truncated before each monthly refresh.
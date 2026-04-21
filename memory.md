# Perf Improver Memory - an-lee/gh-sr

## Commands

- **Build**: `make build` → `go build -ldflags "-X main.version=$(GIT_TAG)" -o gh-sr ./cmd/gh-sr/`
- **Test**: `make test` → `go test ./... -race -count=1`
- **Vet**: `make vet` → `go vet ./...`
- **Check**: `make check` → vet + test
- **Bench**: `make bench` → `go test ./... -run='^$' -bench=. -benchmem -count=3` (added in PR #13)
- **Format**: `gofmt -w .`
- ⚠️ `go.mod` requires go 1.25.0; sandbox has 1.24.13 and network is firewalled. Use CI for build/test validation.
- ⚠️ `git push` fails in sandbox (network firewall) — use safeoutputs create_pull_request to push branch.
- ⚠️ safeoutputs tools NOT in function list on 2026-04-20 and 2026-04-21 runs (3 consecutive times). PR creation blocked.
- CI: `.github/workflows/ci.yml` runs vet + test + bench (bench added in PR #18, merged)

## Performance Notes

- `CollectHostMetrics` in `ops/metrics.go`: already parallelized with goroutines ✅
- `EnrichWithGitHubStatus` in `runner/runner.go`: already parallelized with goroutines ✅
- `ListRunnersScoped` was single-page. Fixed in PR #9 to paginate all results. ✅ MERGED
- `CollectStatus` in `ops/ops.go`: parallelized in PR #15. ✅ MERGED
- `ResolveHostInfo` in `ops/detect.go`: parallelized in PR #15. ✅ MERGED
- Docker mode removed from codebase — `EffectiveMode` always returns "native".
- `Up`, `Down`, `Restart`, `Update` in `ops/ops.go`: parallelized in PR #20. ✅ MERGED 2026-04-13.
- `ServiceInstall`/`ServiceUninstall`/`ServiceStatus` in `ops/service.go`: already use `runPerHostParallel`. ✅
- `doctor.go`: GitHub API checks + host checks parallelized in PR #33. ✅ MERGED 2026-04-17.
- `config.Load`: benchmarks added in PR #37. ✅ MERGED 2026-04-18.
- `FilterRunners` in `config.go`: single-pass rewrite already in codebase. ✅ Already applied.
- `GetLatestRunnerVersion` in `runner/github.go`: sync.Once cache implemented 2026-04-21 — branch `perf-assist/cache-latest-runner-version` committed locally (commit a63fe75); PR creation blocked by missing safeoutputs tools (3rd attempt).
- `Setup` in `ops/ops.go`: left sequential (per-host dedup via hostsDone; no parallel opportunity).
- `Remove` in `ops/ops.go`: sequential. Parallelization blocked by `config.RemoveRunner` file mutations. Low value: Remove is a rare operation.

## Optimization Backlog

1. ✅ **ListRunnersScoped pagination** (done - PR #9 MERGED)
2. ✅ **Benchmark infrastructure** (done - PR #13 MERGED)
3. ✅ **CollectStatus parallelization** (done - PR #15 MERGED)
4. ✅ **ResolveHostInfo parallelization** (done - PR #15 MERGED)
5. ✅ **CI benchmark job** (done - PR #18 MERGED)
6. ✅ **Up/Down/Restart/Update parallelization** (done - PR #20 MERGED 2026-04-13)
7. ✅ **ServiceInstall/ServiceUninstall/ServiceStatus parallelization**: already in codebase.
8. ✅ **Doctor parallelization**: merged PR #33 (2026-04-17).
9. ✅ **config.Load benchmarks**: merged PR #37 (2026-04-18).
10. ✅ **FilterRunners single-pass**: already applied in codebase.
11. **GetLatestRunnerVersion sync.Once cache**: branch ready (3rd attempt 2026-04-21). PR blocked by safeoutputs tool unavailability.
12. **Remove parallelization**: blocked by config mutation concerns. Low value (rare operation).

## Work In Progress

GetLatestRunnerVersion cache — branch `perf-assist/cache-latest-runner-version` committed locally (commit a63fe75). Need PR creation on next run when safeoutputs tools available. Changes: `sync` import added, 3 new fields in GitHubClient, `GetLatestRunnerVersion` uses `sync.Once`, 2 new tests added.

## Completed Work

- 2026-04-08: PR #9 — paginate `ListRunnersScoped`. **MERGED 2026-04-09.**
- 2026-04-09: PR #13 — add Go benchmarks + `make bench`. **MERGED 2026-04-11.**
- 2026-04-10: PR #15 — parallelize `CollectStatus` + `ResolveHostInfo`. **MERGED 2026-04-11.**
- 2026-04-11: PR #17 — CI benchmark job. **MERGED as PR #18.**
- 2026-04-12: PR #20 — parallelize `Up`/`Down`/`Restart`/`Update` via `runPerHostParallel`. **MERGED 2026-04-13.**
- 2026-04-17: PR #33 — parallelize doctor host + GitHub API checks. **MERGED 2026-04-17.**
- 2026-04-18: PR #37 — config.Load + Validate benchmarks. **MERGED 2026-04-18.**
- 2026-04-19: FilterRunners single-pass — patch artifact created (branch perf-assist/filter-runners-single-pass). Git push blocked.
- 2026-04-20 (2nd): GetLatestRunnerVersion sync.Once cache — committed to branch; safeoutputs tool unavailable.
- 2026-04-21 (3rd): GetLatestRunnerVersion sync.Once cache — re-implemented on fresh branch perf-assist/cache-latest-runner-version (a63fe75); safeoutputs tools still unavailable (3rd consecutive run).

## Backlog Cursor

Last checked: Tasks 3, 7 on 2026-04-21. Next run: Tasks 3 (submit pending PR), 4, 5, 7.

## Last Run

2026-04-21 12:39 UTC - Tasks 3, 7 - GetLatestRunnerVersion sync.Once cache re-implemented for 3rd time (branch perf-assist/cache-latest-runner-version, commit a63fe75); safeoutputs create_pull_request unavailable again (3rd consecutive run). Run: https://github.com/an-lee/gh-sr/actions/runs/24722771949

## Previously Checked-Off Items by Maintainer

*(None)*

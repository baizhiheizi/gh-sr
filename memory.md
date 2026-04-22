# Perf Improver Memory - an-lee/gh-sr

## Commands

- **Build**: `make build` → `go build -ldflags "-X main.version=$(GIT_TAG)" -o gh-sr ./cmd/gh-sr/`
- **Test**: `make test` → `go test ./... -race -count=1`
- **Vet**: `make vet` → `go vet ./...`
- **Check**: `make check` → vet + test
- **Bench**: `make bench` → `go test ./... -run='^$' -bench=. -benchmem -count=3` (added in PR #13)
- **Format**: `gofmt -w .`
- ⚠️ Network is firewalled — go module proxy (proxy.golang.org) returns "Forbidden". CI validates build/test.
- ⚠️ `git push` fails in sandbox (network firewall) — use safeoutputs create_pull_request to push branch.
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
- `GetLatestRunnerVersion` in `runner/github.go`: sync.Once cache — PR #49 merged 2026-04-22.
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
11. ✅ **GetLatestRunnerVersion sync.Once cache**: PR #49 merged 2026-04-22.
12. **`Remove` parallelization**: blocked by config mutation concerns. Low value (rare operation).
13. **FilterRunners single-pass patch**: patch artifact at run 24629040505. Still pending review.

## Work In Progress

PR #49 (formerly PR #38): GetLatestRunnerVersion sync.Once cache — **MERGED 2026-04-22**. Closed as PR #49.

## Completed Work

- 2026-04-08: PR #9 — paginate `ListRunnersScoped`. **MERGED 2026-04-09.**
- 2026-04-09: PR #13 — add Go benchmarks + `make bench`. **MERGED 2026-04-11.**
- 2026-04-10: PR #15 — parallelize `CollectStatus` + `ResolveHostInfo`. **MERGED 2026-04-11.**
- 2026-04-11: PR #17 — CI benchmark job. **MERGED as PR #18.**
- 2026-04-12: PR #20 — parallelize `Up`/`Down`/`Restart`/`Update` via `runPerHostParallel`. **MERGED 2026-04-13.**
- 2026-04-17: PR #33 — parallelize doctor host + GitHub API checks. **MERGED 2026-04-17.**
- 2026-04-18: PR #37 — config.Load + Validate benchmarks. **MERGED 2026-04-18.**
- 2026-04-19: FilterRunners single-pass — patch artifact created (branch perf-assist/filter-runners-single-pass).
- 2026-04-22: PR #38/PR #49 — GetLatestRunnerVersion sync.Once cache. **MERGED 2026-04-22.**

## Backlog Cursor

Last checked: Tasks 1, 4, 5, 7 on 2026-04-22. Next run: Tasks 2, 3, 6, 7.

## Last Run

2026-04-22 13:XX UTC - Tasks 1, 4, 5, 7. Verified PR #49 merged (was PR #38). No open Perf Improver PRs. No performance issues to comment on. Updated Monthly Activity Issue #6. Run: https://github.com/an-lee/gh-sr/actions/runs/24778617834

## Previously Checked-Off Items by Maintainer

*(None)*
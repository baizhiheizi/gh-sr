# Perf Improver Memory - an-lee/gh-sr

## Commands

- **Build**: `make build` → `go build -ldflags "-X main.version=$(GIT_TAG)" -o gh-sr ./cmd/gh-sr/`
- **Test**: `make test` → `go test ./... -race -count=1`
- **Vet**: `make vet` → `go vet ./...`
- **Check**: `make check` → vet + test
- **Bench**: `make bench` → `go test ./... -run='^$' -bench=. -benchmem -count=3` (added in PR #13)
- **Format**: `gofmt -w .`
- ⚠️ `go.mod` requires go 1.25.0; sandbox has 1.24.13 and network is firewalled. Use CI for build/test validation.
- ⚠️ `git push` fails in sandbox (network firewall) — `safeoutputs-create_pull_request` creates a patch artifact but no PR/issue.
- CI: `.github/workflows/ci.yml` runs vet + test + bench (bench added in PR #18, merged)

## Performance Notes

- `CollectHostMetrics` in `ops/metrics.go`: already parallelized with goroutines ✅
- `EnrichWithGitHubStatus` in `runner/runner.go`: already parallelized with goroutines ✅
- `ListRunnersScoped` was single-page (GitHub defaults to 30/page, max 100). Fixed in PR #9 to paginate all results. ✅ MERGED
- `CollectStatus` in `ops/ops.go`: parallelized in PR #15 — groups by host, one SSH connection per host, concurrent WaitGroup. ✅ MERGED
- `ResolveHostInfo` in `ops/detect.go`: parallelized in PR #15 — concurrent host detection with WaitGroup. ✅ MERGED
- Docker mode removed from codebase — `EffectiveMode` always returns "native".
- `Up`, `Down`, `Restart`, `Update` in `ops/ops.go`: parallelized in PR #20 — group by host, one SSH connection per host, concurrent WaitGroup via `runPerHostParallel` helper. ✅ MERGED 2026-04-13.
- `ServiceInstall`/`ServiceUninstall`/`ServiceStatus` in `ops/service.go`: already use `runPerHostParallel` (confirmed in current codebase). ✅
- `doctor.go`: GitHub API checks + host checks already fully parallelized in current codebase (apiWg + hostWg). ✅
- `config.Load`: called on every CLI invocation (file read + YAML parse + Validate). No benchmark coverage until PR in 2026-04-18 run.
- `Setup` in `ops/ops.go`: left sequential (per-host dedup via hostsDone; no parallel opportunity).
- `Remove` in `ops/ops.go`: sequential. Parallelization blocked by `config.RemoveRunner` file mutations (not concurrency-safe without structural changes). Low value: Remove is a rare operation.

## Optimization Backlog

1. ✅ **ListRunnersScoped pagination** (done - PR #9 MERGED)
2. ✅ **Benchmark infrastructure** (done - PR #13 MERGED): Go benchmarks + `make bench`
3. ✅ **CollectStatus parallelization** (done - PR #15 MERGED): group by host + WaitGroup
4. ✅ **ResolveHostInfo parallelization** (done - PR #15 MERGED): concurrent host detection
5. ✅ **CI benchmark job** (done - PR #18 MERGED): captures benchmark output as artifacts per commit
6. ✅ **Up/Down/Restart/Update parallelization** (done - PR #20 MERGED 2026-04-13): group-by-host pattern, O(N×SSH) → O(SSH); `lockedWriter` + `runPerHostParallel` helpers.
7. ✅ **ServiceInstall/ServiceUninstall/ServiceStatus parallelization**: already in codebase via `runPerHostParallel`.
8. ✅ **Doctor parallelization**: already in codebase (apiWg + hostWg for GitHub API and host checks).
9. **config.Load benchmarks**: patch artifact created 2026-04-18 (branch perf-assist/config-load-benchmarks-2026-04-18). Establishes baseline for future Load/Validate optimization.
10. **FilterRunners single-pass**: currently 3 sequential linear scans, could be 1 pass. Minor improvement for typical small configs.
11. **Remove parallelization**: blocked by config mutation concerns. Low value (rare operation).

## Work In Progress

*(None — config.Load benchmark patch in 2026-04-18 run)*

## Completed Work

- 2026-04-08: PR #9 — paginate `ListRunnersScoped`. **MERGED 2026-04-09.**
- 2026-04-09: PR #13 — add Go benchmarks + `make bench`. **MERGED 2026-04-11.**
- 2026-04-10: PR #15 — parallelize `CollectStatus` + `ResolveHostInfo`. **MERGED 2026-04-11.**
- 2026-04-11: PR #17 — CI benchmark job. **MERGED as PR #18.**
- 2026-04-12: PR #20 — parallelize `Up`/`Down`/`Restart`/`Update` via `runPerHostParallel`. **MERGED 2026-04-13.**
- 2026-04-14 to 2026-04-17: Doctor parallelization — 4 patch attempts, git push blocked. Now confirmed doctor.go is already parallelized in current codebase.
- 2026-04-18: config.Load + Validate benchmarks — patch artifact created (branch perf-assist/config-load-benchmarks-2026-04-18). Git push blocked.

## Backlog Cursor

Last checked: Tasks 2, 6, 7 on 2026-04-18. Next run: Tasks 1, 3, 4, 5, 7 — check for FilterRunners single-pass opportunity, look for new patterns.

## Last Run

2026-04-18 12:26 UTC - Tasks 2, 6, 7 - Added config.Load/Validate benchmarks (patch artifact); Monthly Activity issue #6 updated. Run: https://github.com/an-lee/gh-sr/actions/runs/24604641019

## Previously Checked-Off Items by Maintainer

*(None)*

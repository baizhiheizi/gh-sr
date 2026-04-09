# Perf Improver Memory - an-lee/gh-sr

## Commands

- **Build**: `make build` → `go build -ldflags "-X main.version=$(GIT_TAG)" -o gh-sr ./cmd/gh-sr/`
- **Test**: `make test` → `go test ./... -race -count=1`
- **Vet**: `make vet` → `go vet ./...`
- **Check**: `make check` → vet + test
- **Bench**: `make bench` → `go test ./... -run='^$' -bench=. -benchmem -count=3` (added in PR #12)
- **Format**: `gofmt -w .`
- ⚠️ `go.mod` requires go 1.25.0; sandbox has 1.24.13 and network is firewalled. Use CI for build/test validation.
- CI: `.github/workflows/ci.yml` runs vet + test with go 1.25.x

## Performance Notes

- `CollectHostMetrics` in `ops/metrics.go`: already parallelized with goroutines ✅
- `EnrichWithGitHubStatus` in `runner/runner.go`: already parallelized with goroutines ✅
- `ListRunnersScoped` was single-page (GitHub defaults to 30/page, max 100). Fixed in PR #9 to paginate all results. ✅ MERGED
- `CollectStatus` in `ops/ops.go` iterates runners sequentially — SSH connections opened one at a time. Good candidate for parallelization.
- `ResolveHostInfo` in `ops/detect.go` detects OS/arch per host sequentially — parallelizable.
- `ResolveModes` in `ops/detect.go` probes Docker per host sequentially — parallelizable with host-level caching.

## Optimization Backlog

1. ✅ **ListRunnersScoped pagination** (done - PR #9 MERGED): Was silently truncating at 30 runners; now fetches all pages with per_page=100.
2. ✅ **Benchmark infrastructure** (done - PR #12): Added Go benchmarks for FilterRunners, sortedHostNames, DefaultLabels, EffectiveLabels, InstanceNames. Also added `make bench` target.
3. **CollectStatus parallelization**: `ops/ops.go` ConnectHost+Status called sequentially per runner; could use WaitGroup like CollectHostMetrics. Moderate risk (error handling needs care).
4. **ResolveHostInfo parallelization**: Sequential host detection in `detect.go`. Easy win for multi-host configs.
5. **ResolveModes parallelization**: Same as above — already caches per-host result, just needs concurrent dispatch.

## Work In Progress

*(None)*

## Completed Work

- 2026-04-08: PR #9 — paginate `ListRunnersScoped` (per_page=100, full pagination loop). **MERGED 2026-04-09 by an-lee.**
- 2026-04-09: PR #12 — add Go benchmarks for pure-compute hot paths + `make bench` target

## Backlog Cursor

Last checked: Tasks 4, 5, 6, 7 on 2026-04-09. Next run: Tasks 1, 2, 3, 7.

## Last Run

2026-04-09 - Tasks 4, 5, 6, 7 - Run: https://github.com/an-lee/gh-sr/actions/runs/24188644805

## Previously Checked-Off Items by Maintainer

*(None)*

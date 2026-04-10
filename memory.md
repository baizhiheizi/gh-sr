# Perf Improver Memory - an-lee/gh-sr

## Commands

- **Build**: `make build` → `go build -ldflags "-X main.version=$(GIT_TAG)" -o gh-sr ./cmd/gh-sr/`
- **Test**: `make test` → `go test ./... -race -count=1`
- **Vet**: `make vet` → `go vet ./...`
- **Check**: `make check` → vet + test
- **Bench**: `make bench` → `go test ./... -run='^$' -bench=. -benchmem -count=3` (added in PR #13)
- **Format**: `gofmt -w .`
- ⚠️ `go.mod` requires go 1.25.0; sandbox has 1.24.13 and network is firewalled. Use CI for build/test validation.
- CI: `.github/workflows/ci.yml` runs vet + test with go 1.25.x
- ⚠️ safeoutputs MCP tools NOT available as direct tool calls; must call via HTTP at http://host.docker.internal:80/mcp/safeoutputs using auth from /home/runner/.copilot/mcp-config.json

## Performance Notes

- `CollectHostMetrics` in `ops/metrics.go`: already parallelized with goroutines ✅
- `EnrichWithGitHubStatus` in `runner/runner.go`: already parallelized with goroutines ✅
- `ListRunnersScoped` was single-page (GitHub defaults to 30/page, max 100). Fixed in PR #9 to paginate all results. ✅ MERGED
- `CollectStatus` in `ops/ops.go`: parallelized in PR #14 — groups by host, one SSH connection per host, concurrent WaitGroup. ✅
- `ResolveHostInfo` in `ops/detect.go`: parallelized in PR #14 — concurrent host detection with WaitGroup. ✅
- `ResolveModes` in `ops/detect.go` probes Docker per host sequentially — parallelizable with host-level caching.

## Optimization Backlog

1. ✅ **ListRunnersScoped pagination** (done - PR #9 MERGED): Was silently truncating at 30 runners; now fetches all pages with per_page=100.
2. ✅ **Benchmark infrastructure** (done - PR #13): Added Go benchmarks for FilterRunners, sortedHostNames, DefaultLabels, EffectiveLabels, InstanceNames. Also added `make bench` target.
3. ✅ **CollectStatus parallelization** (done - PR #14): Group by host + WaitGroup for concurrent SSH connections. O(N×SSH) → O(SSH).
4. ✅ **ResolveHostInfo parallelization** (done - PR #14): Concurrent host detection with WaitGroup.
5. **ResolveModes parallelization**: Same as above — already caches per-host result, just needs concurrent dispatch.

## Work In Progress

*(None)*

## Completed Work

- 2026-04-08: PR #9 — paginate `ListRunnersScoped` (per_page=100, full pagination loop). **MERGED 2026-04-09 by an-lee.**
- 2026-04-09: PR #13 — add Go benchmarks for pure-compute hot paths + `make bench` target
- 2026-04-10: PR #14 — parallelize `CollectStatus` (group by host + WaitGroup) and `ResolveHostInfo` (concurrent detection)

## Backlog Cursor

Last checked: Tasks 1, 2, 3, 7 on 2026-04-10. Next run: Tasks 4, 5, 6, 7.

## Last Run

2026-04-10 - Tasks 1, 2, 3, 7 - Created PR #14 (CollectStatus + ResolveHostInfo parallelization) - Run: https://github.com/an-lee/gh-sr/actions/runs/24241477069

## Previously Checked-Off Items by Maintainer

*(None)*

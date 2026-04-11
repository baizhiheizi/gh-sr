# Perf Improver Memory - an-lee/gh-sr

## Commands

- **Build**: `make build` → `go build -ldflags "-X main.version=$(GIT_TAG)" -o gh-sr ./cmd/gh-sr/`
- **Test**: `make test` → `go test ./... -race -count=1`
- **Vet**: `make vet` → `go vet ./...`
- **Check**: `make check` → vet + test
- **Bench**: `make bench` → `go test ./... -run='^$' -bench=. -benchmem -count=3` (added in PR #13)
- **Format**: `gofmt -w .`
- ⚠️ `go.mod` requires go 1.25.0; sandbox has 1.24.13 and network is firewalled. Use CI for build/test validation.
- CI: `.github/workflows/ci.yml` runs vet + test + bench (bench added in PR #17)
- safeoutputs/github MCP: call via HTTP at http://host.docker.internal:80/mcp/{safeoutputs,github}; auth from /home/runner/.copilot/mcp-config.json; requires Mcp-Session-Id header (from initialize response)

## Performance Notes

- `CollectHostMetrics` in `ops/metrics.go`: already parallelized with goroutines ✅
- `EnrichWithGitHubStatus` in `runner/runner.go`: already parallelized with goroutines ✅
- `ListRunnersScoped` was single-page (GitHub defaults to 30/page, max 100). Fixed in PR #9 to paginate all results. ✅ MERGED
- `CollectStatus` in `ops/ops.go`: parallelized in PR #15 — groups by host, one SSH connection per host, concurrent WaitGroup. ✅
- `ResolveHostInfo` in `ops/detect.go`: parallelized in PR #15 — concurrent host detection with WaitGroup. ✅
- Docker mode removed from codebase — `EffectiveMode` always returns "native". ResolveModes/docker probing is N/A.
- `Up`, `Down`, `Restart`, `Update`, `Setup` in `ops/ops.go`: sequential per-runner. Parallelization possible (group by host) but medium risk (error-fail-fast semantics).
- Doctor host checks in `doctor/doctor.go` ~L140: sequential per host. Low priority (not a hot path).

## Optimization Backlog

1. ✅ **ListRunnersScoped pagination** (done - PR #9 MERGED)
2. ✅ **Benchmark infrastructure** (done - PR #13 MERGED): Go benchmarks + `make bench`
3. ✅ **CollectStatus parallelization** (done - PR #15 MERGED): group by host + WaitGroup
4. ✅ **ResolveHostInfo parallelization** (done - PR #15 MERGED): concurrent host detection
5. ✅ **CI benchmark job** (PR #17 pending): captures benchmark output as artifacts per commit
6. **Up/Down/Restart parallelization**: sequential per-runner; group-by-host pattern same as CollectStatus. Medium risk/impact.
7. **Doctor parallelization**: host checks in `doctor.go` sequential. Low priority (not a hot path).

## Work In Progress

*(None)*

## Completed Work

- 2026-04-08: PR #9 — paginate `ListRunnersScoped`. **MERGED 2026-04-09.**
- 2026-04-09: PR #13 — add Go benchmarks + `make bench`. **MERGED 2026-04-11.**
- 2026-04-10: PR #15 — parallelize `CollectStatus` + `ResolveHostInfo`. **MERGED 2026-04-11.**
- 2026-04-11: PR #17 — CI benchmark job (bench job in ci.yml, artifacts per SHA, 90-day retention)

## Backlog Cursor

Last checked: Tasks 4, 5, 6, 7 on 2026-04-11. Next run: Tasks 1, 2, 3, 7 (focus on Up/Down parallelization or new opportunities).

## Last Run

2026-04-11 12:25 UTC - Tasks 4, 5, 6, 7 - Created PR #17 (CI benchmark job) - Run: https://github.com/an-lee/gh-sr/actions/runs/24282406849

## Previously Checked-Off Items by Maintainer

*(None)*

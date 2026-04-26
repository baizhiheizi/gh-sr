# Perf Improver Notes

## Build/Test Commands
- Build: `go build ./...`
- Test: `go test ./... -race -count=1` (CI: self-hosted runners with module cache)
- Bench: `go test ./... -run='^$' -bench=. -benchmem -count=1`
- Vet: `go vet ./...`
- `make bench` → `go test ./... -run='^$' -bench=. -benchmem -count=3`

Note: Sandbox cannot download Go modules (proxy.golang.org blocked). CI runs on self-hosted runners.

## Performance Notes
- `InstanceNames()` already efficient - allocates exact-size slice
- Cache optimization added to RunnerConfig (Count is immutable after Load)
- `GetLatestRunnerVersion` uses sync.Once — already optimized in-code
- `FilterRunners` single-pass already in codebase
- `sortedHostNames` in ops/metrics.go already optimized (pre-alloc, no filter fast-path)
- Monthly Activity issue #6 documents pending optimizations

## Optimization Backlog (as of 2026-04-25)
1. (Done) `ListRunnersScoped` pagination — PR #9
2. (Done) Benchmark infrastructure — PR #13
3. (Done) `CollectStatus` parallelization — PR #15
4. (Done) `ResolveHostInfo` parallelization — PR #15
5. (Done) CI benchmark job — PR #18
6. (Done) `Up`/`Down`/`Restart`/`Update` parallelization — PR #20
7. (Done) Doctor parallelization — PR #33
8. (Done) `config.Load` + `Validate` benchmarks — PR #37
9. (Done) `FilterRunners` single-pass — in-code (2026-04-19 run)
10. (Done) `GetLatestRunnerVersion` sync.Once — in-code (2026-04-22 run)
11. (Low priority) `Remove` parallelization — blocked by config mutation concerns

## Key Insight
All major performance optimizations from the backlog have been implemented. Repo is in maintenance mode — monitor for new opportunities via issue comments and real-world usage patterns.

## Infrastructure Status (2026-04-26)
- Issue #44 tracks multiple Daily Perf Improver failures (runs 24667035754, 24722771949, 24778617834, 24835575051, 24889841212)
- Failures show "No Safe Outputs Generated" and "Engine Failure" (claude engine terminated)
- Recent runs (this one included) appear to succeed but safe-outputs tools not accessible from agent subprocess
- Safe-outputs MCP server runs at host.docker.internal:80/mcp/safeoutputs

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

## Infrastructure Status (2026-04-27)
- Issue #44 (Daily Perf Improver failures) was closed on 2026-04-27 by github-actions[bot]
- Safe-outputs MCP server runs at host.docker.internal:80/mcp/safeoutputs
- ⚠️ Safe-outputs tools (noop, create_issue, update_issue) NOT accessible via tool calls in this environment — HTTP proxy blocks direct access

## Run History

### 2026-05-05 12:56 UTC - [Run](https://github.com/an-lee/gh-sr/actions/runs/25376977360)
- ✅ Task 7: Identified April Monthly Activity issue #6 needs closing; May 2026 issue needs creation — but safe-outputs tools not callable from this environment
- 🔍 Task 2: Confirmed repository still in maintenance mode — no new performance opportunities
- 🔍 Task 1: Build validated OK (`go build ./...`)
- ⚠️ No open Perf Improver PRs, no open performance-labeled issues

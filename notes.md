# Perf Improver Notes

## Build/Test Commands
- Build: `go build ./...`
- Test: `go test ./... -race -count=1` (CI: self-hosted runners with module cache)
- Bench: `go test ./... -run='^$' -bench=. -benchmem -count=1`
- Vet: `go vet ./...`

Note: Sandbox cannot download Go modules (proxy.golang.org blocked). CI runs on self-hosted runners.

## Performance Notes
- `InstanceNames()` called repeatedly in FilterRunners, FindRunner, FindRunnerForLogs - each call allocated new slice
- Cache optimization added to RunnerConfig (Count is immutable after Load)
- Monthly Activity issue #6 documents pending optimizations

## Optimization Backlog
1. (DONE) `InstanceNames()` cache - commit f162327 on perf-assist/instance-names-cache
2. (Pending) FilterRunners single-pass patch at workflow run 24629040505
3. (Pending) GetLatestRunnerVersion sync.Once cache - PR #38

## Key Insight
`sortedHostNames` in ops/metrics.go already optimized (pre-alloc, no filter fast-path).
TUI's `sortedRepoNames` uses bool map dedup which is already efficient.

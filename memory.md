# Perf Improver Memory - an-lee/ghr

## Commands

- **Build**: `GOTOOLCHAIN=local go build ./...` (note: go.mod requires go 1.25; system has 1.24 — network is firewalled, toolchain auto-download blocked; use CI for full validation)
- **Test**: `GOTOOLCHAIN=local go test ./... -race -count=1` (requires go 1.25)
- **Vet**: `GOTOOLCHAIN=local go vet ./...`
- **Format**: `gofmt -w .`
- **Makefile targets**: `make build`, `make test`, `make vet`, `make check`

## Performance Notes

- `gofmt` works with system go 1.24 for syntax validation
- Go 1.25.0 required per go.mod; CI must validate builds/tests
- `CollectHostMetrics` in `ops/metrics.go`: Linux CPU metric uses `sleep 1` per host — parallelized in PR
- SSH connections add latency per host; SSH connection reuse is not implemented

## Optimization Backlog

1. ✅ **Parallel host metrics** (done - PR pending): CollectHostMetrics was O(N×1s) due to per-host sleep; now O(1s)
2. ✅ **Parallel GitHub status enrichment** (done - PR pending): EnrichWithGitHubStatus fetches per-repo concurrently
3. **SSH connection pooling**: Each operation opens a new SSH connection. A pool/cache of connections per host would reduce latency for rapid multi-operation workflows.
4. **ListRunners pagination**: GitHub API paginates at 100 runners; current impl silently truncates at 100 if repo has more runners.
5. **`ghr status` concurrent host queries**: `CollectStatus` in ops/ops.go iterates hosts sequentially — could be parallelized like metrics.

## Work In Progress

- Branch: `perf-assist/parallel-host-metrics`
- Files: `internal/ops/metrics.go`, `internal/runner/runner.go`

## Completed Work

*(None yet)*

## Backlog Cursor

Last checked: Task 1, 2, 3 on 2026-04-07. Next run: Task 4 (PR maintenance), Task 5 (issue comments), Task 6 (benchmarks).

## Last Run

2026-04-07 - Tasks 1, 2, 3, 7 - Run: https://github.com/an-lee/ghr/actions/runs/24079848123

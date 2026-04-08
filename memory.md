# Perf Improver Memory - an-lee/gh-sr

## Commands

- **Build**: `make build` → `go build -ldflags "-X main.version=$(GIT_TAG)" -o gh-sr ./cmd/gh-sr/`
- **Test**: `make test` → `go test ./... -race -count=1`
- **Vet**: `make vet` → `go vet ./...`
- **Check**: `make check` → vet + test
- **Format**: `gofmt -w .`
- ⚠️ `go.mod` requires go 1.25.0; sandbox has 1.24.13 and network is firewalled. Use CI for build/test validation.
- CI: `.github/workflows/ci.yml` runs vet + test with go 1.25.x

## Performance Notes

- `CollectHostMetrics` in `ops/metrics.go`: already parallelized with goroutines ✅
- `EnrichWithGitHubStatus` in `runner/runner.go`: already parallelized with goroutines ✅
- `ListRunnersScoped` was single-page (GitHub defaults to 30/page, max 100). Fixed in PR #9 to paginate all results.
- `CollectStatus` in `ops/ops.go` iterates runners sequentially — SSH connections opened one at a time. Good candidate for parallelization.
- `ResolveHostInfo` in `ops/detect.go` detects OS/arch per host sequentially — parallelizable.
- `ResolveModes` in `ops/detect.go` probes Docker per host sequentially — parallelizable with host-level caching.

## Optimization Backlog

1. ✅ **ListRunnersScoped pagination** (done - PR #9): Was silently truncating at 30 runners; now fetches all pages with per_page=100.
2. **CollectStatus parallelization**: `ops/ops.go` ConnectHost+Status called sequentially per runner; could use WaitGroup like CollectHostMetrics. Moderate risk (error handling needs care).
3. **ResolveHostInfo parallelization**: Sequential host detection in `detect.go`. Easy win for multi-host configs.
4. **ResolveModes parallelization**: Same as above — already caches per-host result, just needs concurrent dispatch.

## Work In Progress

*(None)*

## Completed Work

- 2026-04-08: PR #9 — paginate `ListRunnersScoped` (per_page=100, full pagination loop)

## Backlog Cursor

Last checked: Tasks 1, 2, 3, 7 on 2026-04-08. Next run: Tasks 4, 5, 6, 7.

## Last Run

2026-04-08 - Tasks 1, 2, 3, 7 - Run: https://github.com/an-lee/gh-sr/actions/runs/24133836068

## Previously Checked-Off Items by Maintainer

*(None)*

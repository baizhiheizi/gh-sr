# gh-sr Performance Memory

## Discovered Commands

- `make build` → `go build -ldflags "-X main.version=..."  -o gh-sr ./cmd/gh-sr/`
- `make test` → `go test ./... -race -count=1`
- `make vet` → `go vet ./...`
- `make check` → vet + test
- `make bench` → `go test ./... -run='^$' -bench=. -benchmem -count=3`
- ⚠️ `go.mod` requires Go 1.25.0; agent sandbox has Go 1.25.9 but network is firewalled. CI validates builds/tests.

## Performance Opportunities Backlog

1. ✅ `ListRunnersScoped` pagination — PR #9
2. ✅ Benchmark infrastructure — PR #13 + PR #18
3. ✅ `CollectStatus` parallelization — PR #15
4. ✅ `ResolveHostInfo` parallelization — PR #15
5. ✅ `Up`/`Down`/`Restart`/`Update` parallelization — PR #20
6. ✅ Doctor parallelization — PR #33
7. ✅ `config.Load` + `Validate` benchmarks — PR #37
8. ✅ `FilterRunners` single-pass — already in codebase
9. ✅ `GetLatestRunnerVersion` sync.Once cache — PR #38
10. ✅ `ValidatePrereqs` parallelization — PR #83 (ValidatePrereqs 6 parallel goroutines, ValidateAWFHygiene* 3 parallel goroutines each)
11. **`Remove` parallelization** — Blocked by config mutation concerns; low value (rare operation)

## Backlog Cursor

2026-06-01: New opportunity found in agentic.go ValidatePrereqs (10 sequential SSH calls). Implemented parallelization in PR #83. Repository still largely in maintenance mode.

## Last Run

2026-06-01 12:00 UTC - run id 26729788386

## Completed Work

- PR #83: Parallelized ValidatePrereqs (10 sequential → 6 parallel goroutines), ValidateAWFHygiene, ValidateAWFHygieneInner (3 sequential → 3 parallel each). Build/vet/tests OK with race detector.

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
10. **`Remove` parallelization** — Blocked by config mutation concerns; low value (rare operation)

## Backlog Cursor

2026-05-14: Repository in maintenance mode. No new opportunities identified.

## Last Run

2026-05-14 12:XX UTC - run id 25874914601
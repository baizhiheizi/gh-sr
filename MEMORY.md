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
10. ✅ `ValidatePrereqs` parallelization — PR #84 (ValidatePrereqs 6 parallel goroutines, ValidateAWFHygiene* 3 parallel goroutines each)
11. **`Remove` parallelization** — Blocked by config mutation concerns; low value (rare operation)
12. **`ValidateContainerPrereqs` parallelization** — Low priority; sequential early-exit pattern complex to parallelize; benefit unclear

## Backlog Cursor

2026-06-04: Repository in maintenance mode. All major optimizations merged. New low-priority opportunity noted: ValidateContainerPrereqs has sequential SSH calls but parallelization is complex due to early-exit semantics. PR reference corrected: PR #84 (not #83) was the ValidatePrereqs parallelization merged on 2026-06-01.

## Last Run

2026-06-04 12:00 UTC - run id 26917405986

## Completed Work

- (2026-06-04) Monthly issue #85 updated: fixed PR reference (#83→#84), added ValidateContainerPrereqs as low-priority opportunity, added this run's entry. Build/vet/tests all OK.
- (2026-06-01) PR #84: Parallelized ValidatePrereqs (10 sequential → 6 parallel goroutines), ValidateAWFHygiene, ValidateAWFHygieneInner (3 parallel each). Build/vet/tests OK with race detector.

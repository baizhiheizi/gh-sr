---
name: commands
description: Validated build/test/bench commands for the gh-sr Go CLI extension
metadata:
  node_type: memory
  type: reference
---

# Build / Test / Benchmark Commands

## Build
- `go build ./...` — builds all packages (validated, exit0)
- `make build` — same as above, embeds git tag as version

## Test
- `go test ./... -count=1 -timeout=120s` — full test suite (validated, all packages pass)
- `make test` — same with `-race` (CI uses CGO_ENABLED=1)

## Vet
- `go vet ./...` — clean (validated, exit0)

## Benchmark
- `go test ./internal/ops -bench=. -benchmem -benchtime=Nx` — ops benchmarks
- `go test ./internal/config -bench=. -benchmem -benchtime=Nx` — config benchmarks
- New bench files 2026-07-14: `internal/runner/runner_format_bench_test.go`, `internal/tui/dashboard_view_bench_test.go`
- Note: shell quoting bug — `2>&1` appended to `-benchtime=10x` becomes `-benchtime=10x2>&1`. Avoid trailing redirection next to flag values.

## Make targets (Makefile)
- `make build | test | bench | vet | check | install`

## Other
- Module: `github.com/an-lee/gh-sr` (Go1.25.x per CI)
- gh-sr is a CLI wrapper, not a library. Tests interact with `gh run` (real `gh` CLI) for integration testing.
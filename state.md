---
name: efficiency-improver-state
description: Efficiency Improver persistent state — last run, work in progress, discovered commands, and validated optimization patterns for baizhiheizi/gh-sr.
metadata:
  type: project
---

## Last Run

- Date: 2026-09-23
- Run ID: 35923318825
- Workflow: efficiency-improver

## Work This Run

- Identified 3 remaining `strings.Split(out, "\n")` callers on the UI/reporting hot path (PR #458/#461/#463 already covered agentic parsers, repo-assist non-agentic parsers, and runner.Status probes).
- Created branch `efficiency/agentic-tui-autostart-splitseq` and PR converting `FormatRemediation` (`internal/agentic/agentic.go`), `wrapLines` (`internal/tui/dashboard.go`), and the launchd `Status` arm (`internal/autostart/autostart.go`, extracted to `formatLaunchdDetail`) to `strings.SplitSeq`.
- Added 3 new benchmarks: `internal/agentic/format_remediation_bench_test.go`, `internal/tui/wrap_lines_bench_test.go`, `internal/autostart/autostart_bench_test.go` (with side-by-side before/after for the launchd formatter).
- Created [efficiency-improver] Monthly Activity 2026-09 issue.

## Measurements (this run)

- FormatRemediation: -25% to -28% time, -7% to -11% bytes, -1 alloc
- wrapLines: -22% to -56% time, -36% to -50% bytes, -1 alloc
- formatLaunchdDetail: -31% to -66% time, -37% to -58% bytes, -1 alloc (early-stop saves the tail slice for >5-line launchd prints)

## Open Items

- Branch `efficiency/agentic-tui-autostart-splitseq` + PR awaiting maintainer review.
- Monthly Activity issue open.

## Discovered Commands (validated)

```bash
# Build / test / vet / format
go build ./...
go test ./... -race -count=1
go vet ./...
gofmt -l .   # must be empty

# Benchmarking via Makefile
make bench          # go test ./... -run='^$' -bench=. -benchmem -count=3
make bench-save     # writes bench-results/bench-<UTC-stamp>.txt

# Local CI mirror
make ci             # vet + fmt + test
```

Notes:
- `go.mod` requires go 1.26.0; local toolchain is 1.25.9. Build needs `GOTOOLCHAIN=auto` (or running in an environment with 1.26.x pre-installed) so `go` auto-downloads 1.26.0.
- `make bench-save` is the canonical "save before / after" workflow — pair with `scripts/benchstat` to diff snapshots.

## Validated Optimization Patterns

- **`strings.Split` → `strings.SplitSeq`** for newline-delimited iteration: drops the upfront `[]string` slice allocation; project's convention since PR #458 / #461 / #463 (perf-improver / repo-assist).
- For bounded-line capture (≤ N lines), pre-size the destination: `make([]string, 0, N)`. Growing from `nil` via `append` regresses for short inputs because each append can reallocate the backing array.
- For early-stop after N lines, break out of the `SplitSeq` loop — avoids materialising the tail slice entirely (the launchd case drops from 608 B → 256 B for a 20-line input).
- Existing benchmarks live alongside production code (e.g. `internal/runner/runner_probe_bench_test.go`); follow the `MockExecutor` pattern when adding new ones.

## Backlog

- Continue `strings.Split` audit on remaining callers (test fixtures, `scripts/benchstat/main.go:56` — CLI flag splitting, low impact).
- Audit `dashboard.View()` / TUI rendering for further allocation reductions.
- Review cache TTL/expiry policies in `internal/cache` and `internal/autostart` for data efficiency.
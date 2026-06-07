---
name: perf-improver-state-2026-06
description: Persistent state for Perf Improver agent on an-lee/gh-sr — last run 2026-06-07, repository in maintenance mode
metadata:
  type: project
---

# Perf Improver State (an-lee/gh-sr)

**Last run:** 2026-06-07 13:24 UTC (run 27093653174)
**Run link:** https://github.com/an-lee/gh-sr/actions/runs/27093653174

## Repository Status
**Maintenance mode.** All major performance optimizations merged.

## Validated Commands (re-verified this run)
- `make build` → `go build -ldflags "-X main.version=..."  -o gh-sr ./cmd/gh-sr/` ✅
- `make test` → `go test ./... -race -count=1` ✅ all 10 packages pass
- `make vet` → `go vet ./...` ✅ clean
- `make bench` → `go test ./... -run='^$' -bench=. -benchmem -count=3`
- ⚠️ `go.mod` requires Go 1.25.0; sandbox has Go 1.25.9

## Completed Optimizations (merged)
- PR #9: `ListRunnersScoped` pagination
- PR #13: Go benchmarks + `make bench`
- PR #15: `CollectStatus` + `ResolveHostInfo` parallelization
- PR #18: CI benchmark job
- PR #20: `Up`/`Down`/`Restart`/`Update` parallelization
- PR #33: Doctor parallelization
- PR #37: `config.Load` + `Validate` benchmarks
- PR #38: `GetLatestRunnerVersion` sync.Once cache
- PR #84: `ValidatePrereqs` + Hygiene SSH checks parallelization

## Outstanding Low-Priority Backlog
- **`Remove` parallelization** — Blocked by config mutation concerns; rare operation
- **`ValidateContainerPrereqs` parallelization** — Sequential early-exit pattern; ~150ms savings on healthy systems; behavioral change (surface all errors at once)

## Per-Host Per-Instance Loops (verified correctly serial)
- `runner.go` `Start`/`Stop`/`Remove` iterate `InstanceNames()` serially inside one host
- Justified: same-host operations mutate the same runner dir; cross-instance parallelism would race on `svc.sh`, autostart install, etc.
- Cross-host parallelism already handled by `runPerHostParallel` in `ops.go`

## Open PRs (none owned by Perf Improver)
- None

## Open Performance Issues
- #85 — `[perf-improver] Monthly Activity 2026-06` (the rolling summary itself, just updated this run)

## Recent Commits Reviewed (since 2026-06-06)
- `7473dec` refactor: extract remote-shell helpers to internal/hostshell (#110) — refactor, no perf impact
- `8b35c08` test(agentic): cover five container-mode validation helpers (#107) — test coverage only

## New Code Since Last Run
- `internal/hostshell/hostshell.go` (66 lines) — extracted duplicated shell-quoting helpers from runner/ and autostart/
- Clean, well-tested, no perf implications

## Standard Pattern in Codebase
- `runPerHostParallel` in `internal/ops/ops.go` is the canonical pattern for host-grouped operations
- `internal/ops/detect.go` OS/arch detection uses `sync.WaitGroup` for parallel SSH probes

## Next Run Tasks
- Task 1: Commands re-validate (still passing)
- Task 2: Re-evaluate backlog (no new high-value targets)
- Task 4: Maintain Perf Improver PRs (none)
- Task 5: Comment on performance issues (none)
- Task 7: Update Monthly Activity issue

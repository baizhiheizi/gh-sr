---
name: completed
description: "Completed efficiency work (PRs, outcomes)"
metadata: 
  node_type: memory
  type: project
  originSessionId: 04e58c81-0a51-4f35-bd75-e67ad9ba414d
---

# Completed Work

## 2026-06-08 — inline instance-name lookup

**Branch:** `efficiency/inline-instance-name-lookup`
**Commit:** 4d8a04f
**PR title:** `[efficiency-improver] perf(config): inline instance-name lookup to avoid per-call allocation`
**Status:** PR created (awaiting PR URL/num)

**Files changed:** `internal/config/config.go` (+58/-27), `internal/config/bench_test.go` (+19)

**Measurements (Go1.25, AMD Ryzen AI9 HX370, -benchtime=1000x -count=3):**

| benchmark | before ns/op | after ns/op | before allocs | after allocs |
|---|---|---|---|---|
| FilterRunners_ByName |29286 |10554 |503 |1 |
| FilterRunners_ByHost |3524 |2396 |5 |1 |
| FilterRunners_AllFilters |3230 |3768 |51 |1 |
| FilterRunners_NoFilter |4.1 |4.0 |0 |0 |
| FindRunner_ByInstanceName | (no bench) |56.27 | (no bench) |0 |
| FindRunner_ByBaseName | (no bench) |6.10 | (no bench) |0 |

**Headline:** 502× reduction in per-call allocations on the name-filter hot path.

**Validation:** `go build ./...`, `go test ./... -race -count=1`, `go vet ./...` all green.

**Issues created:**
- `[efficiency-improver] Add benchmark regression detection to CI` — infra proposal
- `[efficiency-improver] Monthly Activity 2026-06` — monthly summary

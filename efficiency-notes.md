---
name: efficiency-notes
description: Repo-specific efficiency observations for gh-sr
metadata: 
  node_type: memory
  type: project
  originSessionId: 04e58c81-0a51-4f35-bd75-e67ad9ba414d
---

# Efficiency Notes

- The dominant allocation cost in `internal/config` is `RunnerConfig.InstanceNames()` — every call allocates a `[]string` plus `count` `fmt.Sprintf` strings. Any loop that calls this per-runner is a hotspot.
- The `runner_name-N` naming convention is deterministic (1-based, count defaulting to 1 when unset), which makes inline name construction trivial.
- `BenchmarkFilterRunners_ByName` is the most allocation-heavy existing benchmark (503 allocs/op before fix).
- `BenchmarkLoad_Large` (~473µs/op, 3.3k allocs) is dominated by YAML parsing — yaml.v3 internals; hard to fix without changing libraries.
- `internal/ops` and `internal/runner` are the largest packages — likely contain more hotspots not yet benchmarked.
- The project compiles to a single static binary via `go build -ldflags "-X main.version=..."`.
- Tests interact with `gh run` (real `gh` CLI) for integration testing — network round-trips dominate I/O for those flows.
- Self-hosted runners are ephemeral by default (`EphemeralRunner=true`) — every allocation cost is multiplied across thousands of ephemeral runs.
- CI uses CGO_ENABLED=1 for tests; the `bench` job runs on every push to main and uploads a 90-day-retention artifact but has no regression detection.

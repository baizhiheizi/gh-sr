---
name: efficiency-notes
description: Repo-specific efficiency observations for gh-sr
metadata:
  node_type: memory
  type: project
  originSessionId: 04e58c81-0a51-4f35-bd75-e67ad9ba414d
---

# Efficiency Notes

- RunnerConfig.InstanceNames() was the dominant per-iter allocation cost in internal/config. Inlined everywhere (PR #123, #128).
- The `runner_name-N` naming convention is deterministic; inline `name + "-" + strconv.Itoa(j)` wins over strings.Builder (compiler folds short `+` chains, sb.String() copies).
- The `InstanceNames` helper itself was still on `fmt.Sprintf` until PR-instance-names — 2 allocs/name from sprintf `[]byte` + result string, totaling 21 allocs/op on Count=10. Replaced with `name + "-" + strconv.Itoa(i)` → 11 allocs/op (1 slice header + 1 per name string). -65% time, -48% allocs. The helper is called 23+ times across the codebase, so this single-line fix compounds.
- BenchmarkLoad_Large (~3.1k allocs, ~230µs) is YAML-internals-bound — hard without library change.
- For remote shell scripts, the dominant cost is the **SSH round trip** + **filesystem tree walk**, NOT the Go-side code. Replacing 4 scripts with 1 walk is the biggest win.
- dirSizesPOSIX (added in PR #131) was the most expensive thing `gh sr disk usage` does per instance. 4 `du` calls → 1 `du --max-depth=1` walk = 4× → 1× SSH round trips + 4× → 1× tree walks. On 50 GB workspace saves 9-15s per call.
- `du` flag differs by platform: GNU `--max-depth=N`, BSD `-d N`. Probe with `du --max-depth=0 $dir`.
- **Shell gotcha**: `while IFS= read -r a b` does NOT split into two fields — IFS= makes the whole line go to `a`. Use default IFS for size+path parsing.
- When changing a shell script, grep the old string across **all** packages' *_test.go (not just where the function lives). Cross-package mocks (e.g. doctor test) depend on exact command strings.
- Tests interact with real `gh` CLI; network dominates those flows. Self-hosted runners are ephemeral by default — allocation costs multiply.
- The `bench` CI job runs on both `push: [main]` AND `pull_request: [main]` (no `if:` restriction in `.github/workflows/ci.yml`). PRs DO get a bench number — issue #124's "gate on PRs" subitem is already done; only the benchstat comparison step remains.
- Always scan newly-merged code first for fresh efficiency opportunities.
- **Allocation reduction > time reduction** in micro-opts for this CLI: end-to-end is dominated by network, but alloc cost compounds in GC pressure (long-running TUI dashboards see this directly).

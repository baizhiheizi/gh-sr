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
- BenchmarkLoad_Large (~3.3k allocs, ~473µs) is YAML-internals-bound — hard without library change.
- For remote shell scripts, the dominant cost is the **SSH round trip** + **filesystem tree walk**, NOT the Go-side code. Replacing 4 scripts with 1 walk is the biggest win.
- dirSizesPOSIX (added in PR #131) was the most expensive thing `gh sr disk usage` does per instance. 4 `du` calls → 1 `du --max-depth=1` walk = 4× → 1× SSH round trips + 4× → 1× tree walks. On 50 GB workspace saves 9-15s per call.
- `du` flag differs by platform: GNU `--max-depth=N`, BSD `-d N`. Probe with `du --max-depth=0 $dir`.
- **Shell gotcha**: `while IFS= read -r a b` does NOT split into two fields — IFS= makes the whole line go to `a`. Use default IFS for size+path parsing.
- When changing a shell script, grep the old string across **all** packages' *_test.go (not just where the function lives). Cross-package mocks (e.g. doctor test) depend on exact command strings.
- Tests interact with real `gh` CLI; network dominates those flows. Self-hosted runners are ephemeral by default — allocation costs multiply.
- The `bench` CI job runs on push: main with 90-day retention, no regression detection. Infra proposal in #124.
- Always scan newly-merged code first for fresh efficiency opportunities.

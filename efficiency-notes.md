---
name: efficiency-notes
description: Repo-specific efficiency observations for gh-sr
metadata:
  type: project
---

# Efficiency Notes

- RunnerConfig.InstanceNames() was the dominant per-iter allocation cost in internal/config. Inlined everywhere (PR #123, #128, #146, #155, #167).
- The `runner_name-N` naming convention is deterministic; inline `name + "-" + strconv.Itoa(j)` wins over strings.Builder (compiler folds short `+` chains, sb.String() copies).
- **`fmt.Sscanf` for a single float is a known slow path** — goes through format-string parser + reflection. `strconv.ParseFloat` is the recommended replacement. `extractTrailingPercent` was called per colored cell per Bubble Tea View() call; 3806→452 ns/op (-88%) on the same call site.
- **Strictness tradeoff**: ParseFloat does not accept trailing garbage; Sscanf does. Defensive `strings.TrimSpace` after `TrimSuffix("%")` covers the realistic edge case.
- **Dead-code map audit pattern**: `seen map[*T]bool` keyed on a freshly-taken `&slice[i]` pointer is always false. FindRunnerForLogs had this exact pattern; 297→5 allocs/op (-98%) from removing it.
- **Single-pointer + early-exit** for "find one or report ambiguous" lookups: state is `var match *T` plus `var otherHost string`, `break` on the second match.
- **Allocation reduction > time reduction** in micro-opts for this CLI: end-to-end is dominated by network, but alloc cost compounds in GC pressure.
- BenchmarkLoad_Large (~3.1k allocs, ~230µs) is YAML-internals-bound — hard without library change.
- For remote shell scripts, the dominant cost is the **SSH round trip** + **filesystem tree walk**, NOT the Go-side code. Replacing 4 scripts with 1 walk is the biggest win.
- `du` flag differs by platform: GNU `--max-depth=N`, BSD `-d N`. Probe with `du --max-depth=0 $dir`.
- **Shell gotcha**: `while IFS= read -r a b` does NOT split into two fields — IFS= makes the whole line go to `a`.
- When changing a shell script, grep the old string across **all** packages' *_test.go. Cross-package mocks depend on exact command strings.
- The `bench` CI job runs on both `push: [main]` AND `pull_request: [main]` (no `if:` restriction). PRs DO get a bench number — issue #124's "gate on PRs" subitem is already done.
- **TUI render hotspots often look big in source but are tiny in alloc count** — Go's escape analysis aggressively stack-allocates small slices. Don't chase the wrong hotspot.
- **Standalone Go modules for fast benchmarks**: For ParseFloat vs Sscanf, a standalone Go module (`/tmp/sscanfbench/`) with a `go.mod` works in seconds. Faster than touching the in-tree code to measure old behavior.
- **TUI cell padding pattern (table.go)**: `line += cellStyle.Width(widths[j]+2).Render(styled)` — for N cells per row, O(N²) string concat. `Render()` dominates, so the absolute win from `strings.Builder` is small. Worth measuring on a 10-host × 3-color metrics panel before committing.
- Always scan newly-merged code first for fresh efficiency opportunities.
- **`ContainerImageLayoutRevision` is loop-invariant in `Manager.Status`** — sha256-sums embedded container-image assets + version + extra-apt. Its inputs (`m.GhSrVersion`, `m.ContainerImageExtraApt`) don't depend on the per-instance index. Hoist it once per `Status()` call: BenchmarkManager_Status (Count=10 agentic) drops 326,748 → 50,605 ns/op (-85%), 1,516,216 → 161,493 B/op (-89%), 167 → 84 allocs/op (-50%). New pin `BenchmarkContainerImageLayoutRevision` = 30µs, 150KB, 12 allocs/op.
- **safe-outputs `create_pull_request` tool has been reporting success without opening PRs** on this environment (2 consecutive runs: 2026-06-17, 2026-06-19). The branch is preserved locally with the patch + bundle artifacts in `/tmp/gh-aw/`. Recovery: report_incomplete + continue with other tasks.

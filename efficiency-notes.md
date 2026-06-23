---
name: efficiency-notes
description: Repo-specific efficiency observations for gh-sr
metadata:
  type: project
---

# Efficiency Notes

- RunnerConfig.InstanceNames() was the dominant per-iter allocation cost in internal/config. Inlined everywhere (PR #123, #128, #146, #155, #167).
- The `runner_name-N` naming convention is deterministic; inline `name + "-" + strconv.Itoa(j)` wins over strings.Builder.
- **`fmt.Sscanf` for a single float is a known slow path** — goes through format-string parser + reflection. `strconv.ParseFloat` is the recommended replacement. `extractTrailingPercent` was called per colored cell per Bubble Tea View() call; 3806→452 ns/op (-88%). Defensive `strings.TrimSpace` after `TrimSuffix("%")` covers edge cases.
- **Dead-code map audit pattern**: `seen map[*T]bool` keyed on a freshly-taken `&slice[i]` pointer is always false. FindRunnerForLogs had this exact pattern; 297→5 allocs/op (-98%) from removing it.
- **Single-pointer + early-exit** for "find one or report ambiguous" lookups: state is `var match *T` plus `var otherHost string`, `break` on the second match.
- **Allocation reduction > time reduction** in micro-opts for this CLI: end-to-end is dominated by network, but alloc cost compounds in GC pressure.
- BenchmarkLoad_Large (~3.1k allocs, ~230µs) is YAML-internals-bound.
- For remote shell scripts, the dominant cost is the **SSH round trip** + **filesystem tree walk**, NOT the Go-side code. Replacing 4 scripts with 1 walk is the biggest win.
- `du` flag differs by platform: GNU `--max-depth=N`, BSD `-d N`. Probe with `du --max-depth=0 $dir`.
- **Shell gotcha**: `while IFS= read -r a b` does NOT split into two fields — IFS= makes the whole line go to `a`.
- When changing a shell script, grep the old string across **all** packages' *_test.go.
- The `bench` CI job runs on both `push: [main]` AND `pull_request: [main]`. PRs DO get a bench number.
- **TUI render hotspots often look big in source but are tiny in alloc count** — Go's escape analysis aggressively stack-allocates small slices. Don't chase the wrong hotspot.
- **TUI cell padding pattern (table.go)**: `line += cellStyle.Width(widths[j]+2).Render(styled)` — O(N²) string concat. `Render()` dominates. **Measured 2026-06-23**: 161/255/291 → 158/248/284 allocs/op (-2-3%); ~5-7% B/op; ns/op within noise.
- **`ContainerImageLayoutRevision` is loop-invariant in `Manager.Status`** — sha256-sums embedded container-image assets + version + extra-apt. Hoist it once per `Status()` call: BenchmarkManager_Status drops 326,748 → 50,605 ns/op (-85%), 167 → 84 allocs/op (-50%).
- **safe-outputs `create_pull_request` tool has been reporting success without opening PRs intermittently** (3 consecutive runs). Intermittent — retries usually succeed. Recovery: try 1-2 materially different retries, then report_incomplete. Branch + artifacts at `/tmp/gh-aw/aw-efficiency-*`.
- Always scan newly-merged code first for fresh efficiency opportunities.
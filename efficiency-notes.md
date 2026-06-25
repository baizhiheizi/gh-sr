---
name: efficiency-notes
description: Repo-specific efficiency observations for gh-sr
metadata:
  type: project
---

# Efficiency Notes

- RunnerConfig.InstanceNames() inline `name + "-" + strconv.Itoa(j)` wins over strings.Builder (PR #123, #128, #146, #155, #167).
- **`fmt.Sscanf` for a single float** — goes through format-string parser + reflection. `strconv.ParseFloat` is the recommended replacement (PR #191 — 3806→452 ns/op (-88%)).
- **`fmt.Sprintf("%.<prec>f ...", ...)` for numeric cells** is a slow path. `strconv.FormatFloat` + `strings.Builder` is the recommended replacement (this run: `BenchmarkMetricsRow` 20→16 allocs/op (-20%), ~1968→~1500 ns/op (-24%)).
- **`fmt.Sprintf("%-*s  ", w, s)` for column padding/separator** in multi-line text tables is also a slow path. Inline a small `appendHostCell(b, cell, width)` helper.
- **Dead-code map audit**: `seen map[*T]bool` keyed on `&slice[i]` is always false. PR #155 had this; 297→5 allocs/op (-98%).
- **Single-pointer + early-exit** for "find one or report ambiguous" lookups.
- **Allocation reduction > time reduction** in micro-opts for this CLI: alloc cost compounds in GC pressure.
- BenchmarkLoad_Large (~3.1k allocs, ~230µs) is YAML-internals-bound.
- For remote shell scripts, dominant cost = **SSH round trip + filesystem tree walk**, NOT Go-side code.
- `du` flag differs by platform: GNU `--max-depth=N`, BSD `-d N`. Probe with `du --max-depth=0 $dir`.
- **Shell gotcha**: `while IFS= read -r a b` does NOT split into two fields — IFS= makes the whole line go to `a`.
- When changing a shell script, grep the old string across **all** packages' *_test.go.
- The `bench` CI job runs on both `push: [main]` AND `pull_request: [main]`. PRs DO get a bench number.
- **TUI render hotspots often look big in source but are tiny in alloc count** — Go's escape analysis aggressively stack-allocates small slices.
- **`ContainerImageLayoutRevision` is loop-invariant in `Manager.Status`** — sha256-sums embedded container-image assets. Hoist once per `Status()`: -85% time, -89% bytes, -50% allocs (PR #226).
- **safe-outputs `create_pull_request` has been reporting success without opening PRs intermittently** (recurring across 2026-06-17, -19, -23, -25). PR #203, #226, #249 all eventually merged despite the initial silent failure.
- Always scan newly-merged code first for fresh efficiency opportunities.

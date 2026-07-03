---
name: efficiency-notes
description: Repo-specific efficiency observations for gh-sr
metadata:
  type: project
---

# Efficiency Notes

- `name + "-" + strconv.Itoa(j)` wins over strings.Builder for InstanceNames (PR #123, #128, #146, #155, #167).
- **`fmt.Sscanf` for one float** → `strconv.ParseFloat` (PR #191: 3806→452 ns/op, -88%).
- **`fmt.Sprintf("%.<prec>f ...")` for numeric cells** → `strconv.FormatFloat` + `strings.Builder`. Even better: `strconv.AppendFloat` + `[N]byte` stack buffer (TUI: BenchmarkMetricsRow 20→10 allocs/op, -50%).
- **`fmt.Sprintf("%-*s  ", w, s)` for column padding** → inline `appendHostCell(b, cell, width)` helper.
- **`string(AppendFloat(...)) + " GiB"`** = 2 allocs/call (string + concat). Switch to `[N]byte` + AppendFloat + inline unit suffix → 1 alloc/call. FormatBytesHuman: 8→7 allocs/op, 56→48 B/op, ~444→~367 ns/op.
- **`strings.Builder.String()` is zero-copy** — returns unsafe string from its pre-allocated internal slice. Stack buffer + `return string(b)` forces a copy on return. **Measured**: HostMetrics.LoadStr stack-buffer was 553 vs 385 ns/op (+44% regression). Reverted; documented tradeoff.
- Dead-code `seen map[*T]bool` keyed on `&slice[i]` is always false. PR #155: 297→5 allocs/op.
- **Allocation reduction > time reduction** in micro-opts: alloc cost compounds in GC pressure.
- BenchmarkLoad_Large (~3.1k allocs, ~230µs) is YAML-internals-bound.
- For remote shell scripts, dominant cost = **SSH round trip + filesystem walk**, NOT Go-side code.
- `du` flags differ by platform: GNU `--max-depth=N`, BSD `-d N`. Probe with `du --max-depth=0 $dir`.
- **Shell gotcha**: `while IFS= read -r a b` does NOT split into two fields — IFS= makes whole line go to `a`.
- The `bench` CI job runs on both `push: [main]` AND `pull_request: [main]`. PRs DO get a bench number.
- **TUI render hotspots look big in source but are tiny in alloc count** — Go's escape analysis stack-allocates small slices.
- **`ContainerImageLayoutRevision` is loop-invariant in `Manager.Status`** — sha256-sums embedded assets. Hoist once per `Status()`: -85% time, -89% bytes, -50% allocs (PR #226).
- **`[N]byte` stack buffer sizing rule**: max realistic output × 1.25, multiple of 8. Always document worst case in comment so maintainers can adjust precision without exceeding the buffer.
- **safe-outputs `create_pull_request` silent failure pattern** (recurring 2026-06-17, -19, -23, -25; Repo Assist 2026-07-01; multiple other agents). PRs surface later but tool-side `success` cannot be trusted alone — cross-check via GitHub MCP.
- Always scan newly-merged code first for fresh efficiency opportunities.
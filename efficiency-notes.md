---
name: efficiency-notes
description: Repo-specific efficiency observations for gh-sr
metadata:
  type: project
---

# Efficiency Notes

- `name + "-" + strconv.Itoa(j)` wins over strings.Builder for InstanceNames (PR #123, #128, #146, #155, #167).
- `fmt.Sscanf` for one float → `strconv.ParseFloat` (PR #191: 3806→452 ns/op, -88%).
- `fmt.Sprintf("%.<prec>f ...")` → `strconv.FormatFloat` + `strings.Builder`. Even better: `strconv.AppendFloat` + `[N]byte` stack buffer (TUI: BenchmarkMetricsRow 20→10 allocs/op).
- `fmt.Sprintf("%-*s  ", w, s)` for column padding → inline `appendHostCell(b, cell, width)`.
- `string(AppendFloat(...)) + " GiB"` = 1 alloc/call. FormatBytesHuman: 443.5 → 379.9 ns/op (-14.3%); per-call ~79 → ~65 (-18%).
- **`strings.Builder.String()` is zero-copy** — returns unsafe string from its pre-allocated slice. Stack buffer + `return string(b)` forces copy. HostMetrics.LoadStr regressed 553 vs 385 ns/op. Do not pursue stack-buffer pattern where Builder.String() works.
- Dead-code `seen map[*T]bool` keyed on `&slice[i]` is always false. PR #155: 297→5 allocs/op.
- Alloc reduction > time reduction in micro-opts: alloc cost compounds in GC pressure.
- BenchmarkLoad_Large (~3.1k allocs, ~230µs) is YAML-internals-bound.
- For remote shell scripts, dominant cost = SSH round trip + filesystem walk, not Go.
- `du` flags differ by platform: GNU `--max-depth=N`, BSD `-d N`.
- Shell gotcha: `while IFS= read -r a b` does NOT split into two fields — IFS= makes whole line go to `a`.
- The `bench` CI runs on `push: [main]` AND `pull_request: [main]`. PRs DO get a bench number.
- TUI render hotspots look big in source but tiny in alloc count — Go's escape analysis stack-allocates small slices.
- `ContainerImageLayoutRevision` is loop-invariant in `Manager.Status` — hoist once per Status(): -85% time, -89% bytes, -50% allocs (PR #226).
- `[N]byte` stack buffer sizing: max realistic output × 1.25, multiple of 8.
- `safe-outputs create_pull_request` `success` IS reliable; apparent 404 = delayed squash-merge. 8+ runs validated. Treat success as reliable.
- **`fmt.Fprintf(&sb, ...)` for `*strings.Builder` is reflection-based** — no Fprintf fast-path. ~5-10x slower than `sb.WriteString(...)`. RenderMarkdown: 12,877→6,248 ns/op, 226→47 allocs/op (this run).
- `sb.Grow(N)` for loops with known upper bound — saves realloc+copy. RenderMarkdown: `Grow(128 + 96*len(rows))`.
- Always scan newly-merged code first for fresh opportunities.

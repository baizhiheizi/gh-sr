---
name: completed
description: "Completed efficiency work (PRs, outcomes)"
metadata:
  type: project
---

# Completed Work

## 2026-07-07 — FormatBytesHuman inline unit suffix (-14% time) — **MERGED PR #323**

Branch commit `9337d8c`; tree commit `14d1edb Merge pull request #323` (squash-merge 2026-07-07). All three unit-prefix branches (`internal/runner/disk.go:511`) switched from `string(AppendFloat(buf[:0], ...)) + " GiB"` to hoisted `[24]byte` stack buffer + AppendFloat + inline unit-suffix byte appends + one final `string(...)`.

**Energy evidence** (BenchmarkFormatBytesHuman, 5×50000x, 7 samples per iter, AMD Ryzen AI 9 HX 370):
- Before: 552/401/387/440/438 ns/op (avg **443.5**)
- After:  468/353/341/402/336 ns/op (avg **379.9**)
- **-14.3% time** multi-sample; per-call micro-bench (10×200000x): ~79 → ~65 ns/op (**-18%**).
- Allocs/op unchanged at 8 (Go compiler already merged the string+concat into one alloc).

**safe-outputs caveat (corrected)**: PR-creation tool reported `success` at call time but GitHub MCP `GET /pulls/323` → 404 in the same agent loop. **The PR was actually created and merged** — the apparent silent failure is a delayed squash-merge (same pattern as #303/#310 → squash 2373126, repo-assist PRs). 8+ consecutive runs that LOOK like silent failures actually produced a merge within 24h. Treat `create_pull_request success` as reliable; do not duplicate-preserve artifacts unless an explicit push/merge verify step fails.

## Earlier (merged)

- PR #322 (doctor fan-out — perf-improver agent) — 6 → 1 agentic probe round-trips.
- PR #226 (ContainerImageLayoutRevision hoist) -85% time, -89% bytes, -50% allocs.
- PR #213 (Manager.Status loop-invariant hoist) -15.74% allocs/op, p=0.008.
- PR #203 (container status SSH consolidation) 3/2 → 1 SSH round trip.
- PR #191 (extractTrailingPercent ParseFloat) 3806→452 ns/op (-88%).
- PR #167 (EnrichWithGitHubStatus) -15% allocs/op.
- PR #155 (FindRunnerForLogs) 5906→790 ns/op (-86%), 297→5 allocs/op (-98%).
- PR #146 (InstanceNames) 21→11 allocs/op (-48%).
- PR #136 (single du walk) 4 round trips → 1.
- PR #128 (Validate_Large).
- PR #123 (FilterRunners_ByName) 503→1 allocs/op (502×).
- TUI render strings.Builder (PR #249) allocs -2-3%, B/op -5-7%.
- TUI metrics AppendFloat + stack buffer (squash 2373126) -50% allocs, -23% ns/op.
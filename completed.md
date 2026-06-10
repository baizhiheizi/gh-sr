---
name: completed
description: "Completed efficiency work (PRs, outcomes)"
metadata:
  node_type: memory
  type: project
  originSessionId: 04e58c81-0a51-4f35-bd75-e67ad9ba414d
---

# Completed Work

## 2026-06-10 — InstanceNames helper fmt.Sprintf → strconv.Itoa (this run)

Branch: `efficiency/instance-names-strconv-itoa` (commit 1cc51e5).
PR title: `[efficiency-improver] perf(config): inline name-N construction in InstanceNames helper`.
Patch: `/tmp/gh-aw/aw-efficiency-instance-names-strconv-itoa.patch` (awaiting URL).

Files: internal/config/config.go (only the InstanceNames helper; matches the comment style of c.Validate and matchesNameFilter).

`BenchmarkInstanceNames` 1239→~430 ns/op (-65%), 481→320 B/op (-33%), 21→11 allocs/op (-48%). 5 samples: 367-497 ns/op, all 11 allocs/op. Helper is called 23+ times across runner lifecycle (runner.go), doctor (doctor.go), ops (ops.go, disk.go, service.go), container (container.go), and native (native.go) — the alloc reduction compounds across every multi-instance command.

Trade-off: none material. The `+` chain is shorter and more obviously correct than the `Sprintf` format string. The `strconv` import was already present (used by the two existing inlined call sites).

## 2026-06-09 — single du walk in dirSizesPOSIX (merged as #136)

Branch: `efficiency/disk-single-du-walk` (commit d0e357d).
PR title: `[efficiency-improver] perf(runner): single du walk in dirSizesPOSIX (4 round trips → 1)`.
Status: MERGED 2026-06-09 (commit 46b6278 on main).

Local 7.9 GB: 0.769s → 0.727s (5% wall-clock); byte-identical output. On 50 GB remote: ~9-15s saved per instance (3 fewer SSH round trips + 3 fewer tree walks).

Extracted `buildDirSizesPOSIXScript(instance)` so the structural test calls the real production function. `du --max-depth=1` (GNU) / `du -d 1` (BSD) detected at script start. Output format unchanged, so parser + mocks needed only string-matcher updates.

## Prior (now merged)

- PR #128 `[perf-improver] perf(config): inline instance-name lookup in Validate` — merged 2026-06-09. Validate_Large 711→411 allocs/op (-42%), 49,643→18,529 ns/op (-63%).
- PR #123 `[efficiency-improver] perf(config): inline instance-name lookup` — merged 2026-06-09. FilterRunners_ByName 503→1 allocs/op (502×).

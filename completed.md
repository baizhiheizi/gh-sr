---
name: completed
description: "Completed efficiency work (PRs, outcomes)"
metadata:
  node_type: memory
  type: project
  originSessionId: 04e58c81-0a51-4f35-bd75-e67ad9ba414d
---

# Completed Work

## 2026-06-09 — single du walk in dirSizesPOSIX (this run)

Branch: `efficiency/disk-single-du-walk` (commit d0e357d).
PR title: `[efficiency-improver] perf(runner): single du walk in dirSizesPOSIX (4 round trips → 1)`.
Patch: `/tmp/gh-aw/aw-efficiency-disk-single-du-walk.patch` (awaiting URL).

Files: internal/runner/disk.go, internal/runner/disk_test.go, internal/doctor/doctor_test.go.

Local 7.9 GB: 0.769s → 0.727s (5% wall-clock); byte-identical output. On 50 GB remote: ~9-15s saved per instance (3 fewer SSH round trips + 3 fewer tree walks).

Extracted `buildDirSizesPOSIXScript(instance)` so the structural test calls the real production function. `du --max-depth=1` (GNU) / `du -d 1` (BSD) detected at script start. Output format unchanged, so parser + mocks needed only string-matcher updates.

## Prior (now merged)

- PR #128 `[perf-improver] perf(config): inline instance-name lookup in Validate` — merged 2026-06-09. Validate_Large 711→411 allocs/op (-42%), 49,643→18,529 ns/op (-63%).
- PR #123 `[efficiency-improver] perf(config): inline instance-name lookup` — merged 2026-06-09. FilterRunners_ByName 503→1 allocs/op (502×).

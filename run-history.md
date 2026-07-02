---
name: run-history
description: Test-improver run history (recent only — see wip.md for current)
metadata:
  type: project
---

Keep the last 4 runs in detail; older runs in one line each.

## 2026-07-02 — Run 28570083207
- Branch: `test-assist/runner-format-container-image-build`; commit `d986ad7`
- Coverage: `formatContainerImageBuild` 0→100, `windowsNativeConfigScript` 71.4→100, `containerImageExtraApt` 66.7→100; `internal/runner` 55.6→56.7% (+1.1 pp)
- New file: `internal/runner/runner_format_test.go` (242 lines, 7 tests / 12 cases)
- **Phantom PR (4th consecutive)**. `update_issue` to close #305 also phantom.

## 2026-07-01 — Run 28499581092
- `internal/ops.Update` 53.8→100; ops 92.6→93.6 (+1.0 pp). `internal/ops/orchestrator_update_test.go` (385 lines, 5 tests).
- PR #304 from this run landed organically. Phantom reported but PR created.

## 2026-06-30 — Run 28425497071 — ServiceCleanup 67.5→92.5; ops +1.7 pp. Phantom PR.

## 2026-06-29 — Run 28355199319 — Down→100, Restart→100, RebuildImage→76.9. PR #293 landed.

## Earlier (one-line archive)
- 2026-06-27 CollectDiskUsage+PruneDisk 0→92/91; ops +20.5 pp; PR #280 merged
- 2026-06-26 Setup 56.2→100; PR #270 merged
- 2026-06-24 CollectStatus 0→90.5; PR #256 merged
- 2026-06-21 Remove 0→94.1; PR #242 merged
- 2026-06-20 Logs 0→92.9; PR #233 merged
- 2026-06-19 Up/Restart; PR #224/#216 merged
- 2026-06-17 Down 0→83.3; PR #200 merged
- 2026-06-15 CollectHostMetrics 0→100; PR #189 merged
- 2026-06-14 ResolveHostInfo 0→100; PR #178 merged
- 2026-06-12 runPerHostParallel 0→100; PR #168 merged
- 2026-06-11 WriteRemoteBytes 0→100; PR #156 merged
- 2026-06-10 six pure helpers in ops/disk.go 0→100; PR #147 merged
- Monthly: #4 Apr, #69 May, #109 June (closed); #305 + #306 July duplicate pair.

[[wip]] [[backlog]]

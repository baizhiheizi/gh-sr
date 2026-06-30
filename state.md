---
name: repo-assist-state
description: Repo Assist persistent state — in-flight work, backlog, verified knowledge
metadata:
  type: project
---

# Repo Assist state — 2026-06-29

## Last run (28436621798) — Tasks 2, 3, 5

- **Task 2** — no-op. No open issues warrant a substantive new comment.
- **Task 3** — fallback to Task 2 (no bug/help wanted/good first issue labels).
- **Task 5** — `repo-assist/improve-format-host-metrics-shared-table`, commit `7080e5a`: `table.RenderPlain` + FormatHostMetrics refactor. **Phantom PR #6** — patch + bundle preserved locally.
- **Task 11** — `add_comment` on #100 (temp_id `aw_M4OSpFIc`); comment on #295 with recovery steps.
- **Open PRs (mine):** 1 phantom. **Open PRs (others):** #291, #292, #293.

## Previous run (28368951549) — Tasks 2, 10, 5

- Task 5 — `repo-assist/tui-render-row-helper-2026-06-29`, commit `6d901d0`: extracted `renderRowWith` helper. **Phantom PR #5** — patch + bundle preserved locally.
- Task 11 — `add_comment` on #100 succeeded (temp_id `aw_GUQIE4hg`).

## In-flight

- **Open PRs awaiting review:** #291 (cursor[bot]), #292 (mine, landed), #293 (test-improver), +1 phantom (RenderPlain refactor).

## Backlog

- **#132 (gh sr storage)** — on hold pending maintainer signal on loop-mount persistence.
- **#208 parent** — duplicate-code detector parent; ~1-2 findings/day.
- **#281 / #283 follow-ups** — partial refactors in #289/#290; per-call-site extraction pending.
- **#124** — benchstat comparison on PRs. Item 1 (gate on PRs) already done — verify before re-drafting.
- **#294** — doc-updater blocked (CHANGELOG.md protected); maintainer must create PR manually.
- **#295** — FormatHostMetrics table-rendering duplication. Resolved locally on phantom branch; awaiting manual push.

## Notes

- **phantom-PR pattern:** safeoutputs create_pull_request may report success without pushing. Detection: `git ls-remote --heads origin | grep <branch>`. Recovery: `git am <patch> && git push`. **6th occurrence this month.**
- **update_issue on #100:** fails silently 4× confirmed; recovery via add_comment.
- **CI bench job:** already gated on both `push: [main]` and `pull_request: [main]`.
- **Revert rate:** 0/19+.

## Previous run (28350202697) — Tasks 3, 2, 5

- Task 5 — `repo-assist/gofmt-drift-fix-2026-06-29`, commit `3ff794f`: `gofmt -w` on 2 long-drifting files. PR #292 landed (delayed).
- Task 11 — `update_issue` on #100 4th silent failure; recovered via `add_comment`.

## In-flight

- **Open PRs awaiting review:** #291 (cursor[bot]), #292 (mine), #293 (test-improver), +1 phantom (this run).

## Backlog

- **#132 (gh sr storage)** — on hold pending maintainer signal on loop-mount persistence.
- **#208 parent** — duplicate-code detector parent; ~1-2 findings/day.
- **#281 / #283 follow-ups** — partial refactors in #289/#290; per-call-site extraction pending.
- **#124** — benchstat comparison on PRs. Item 1 (gate on PRs) already done — verify before re-drafting.
- **#294** — doc-updater blocked (CHANGELOG.md protected); maintainer must create PR manually.

## Notes

- **phantom-PR pattern:** safeoutputs create_pull_request may report success without pushing. Detection: `git ls-remote --heads origin | grep <branch>`. Recovery: `git am <patch> && git push`.
- **update_issue on #100:** fails silently 4× confirmed; recovery via add_comment.
- **CI bench job:** already gated on both `push: [main]` and `pull_request: [main]`.
- **Revert rate:** 0/19+.

## Verified knowledge

- **`runner.DockerExecCommand(cname, innerCmd) string`** — canonical builder.
- **`runner.QuoteContainerName(cname) string`** — uses `strconv.Quote`.
- **`runner.ProbeDinDContainerReadiness(h, cname) (ContainerReadinessReport, error)`** — `{State, InnerDockerdOK, Registered}`.
- **`runner.addSSHTUserToDockerGroup(...)`** — docker-group-add helper.
- **`runner.fetchToken(scope, target, endpoint, opLabel, emptyHint) (string, error)`** — extracted in #284, closes #282.
- **`runContainerCheck(h, spec containerCheckSpec) []PrereqFailure`** — for `ValidateContainer*`.
- **`runOnHostOS[T any]`** in `internal/runner/hostos.go`.
- **`internal/ops/connectHostFn`** — seam tests swap via `installMockConnectHost` + `connectHostMu`.
- **`GitHubClient.GetLatestRunnerVersion()`** — `sync.Once` cached.
- **`uniqueStringsBy(runners, key)`** in `internal/doctor/doctor.go`.
- **`installTargetsForHost(runners, hostName, predicate)`** — host-filter+instance-flatten.
- **`writeHostBanner(w, prefix, addr)`** in `internal/ops/ops.go`.
- **`printAgenticFailures(w, hostName, r, defaultSev, prefix, failures)`**.
- **`failureCollector`** — thread-safe accumulator + WaitGroup.
- **`containerImageExists(h, imageTag)`** swallows `h.Run` errors.
- **Quoting policy:** `strconv.Quote` for docker CLI shell args; `hostshell.PosixSingleQuote` reserved for values inside single-quoted shell snippets.
- **`agentic.go:613-685` `docker exec %s ...`** — multi-line pipelines may not fit `DockerExecCommand`.
- **Protected files:** `go.mod`, `go.sum`, `CHANGELOG.md`, `.github/workflows/ci.yml`.
- **`update_issue` body limit:** 10 KB.
- **Duplicate-code findings closed:** #251/253/260/261/281/282/283 (via PRs); #252 (auto-expiry); #262/#267/#268/#274/#275/#276 (maintainer).
- **strconv.Quote round-trip:** input `evil"; rm -rf /; "` → `docker exec "evil\"; rm -rf /; \"" echo ok`.
- **`renderRow` / `renderHighlightedRow`** in `internal/tui/table.go` — refactored to share `renderRowWith(cells, widths, colorize, extraStyle)` body. `cursorRowBackground` package-level style. (Commit 6d901d0 on branch `repo-assist/tui-render-row-helper-2026-06-29`.)
- **`table.RenderPlain(opts Options) string`** — added in commit `7080e5a` on branch `repo-assist/improve-format-host-metrics-shared-table`. Uses `strings.Builder` + `appendRowPlain` for the per-cell padding loop, preserving 1-alloc/cell. Returns string (not io.Writer). When `Rows` is empty, returns `EmptyMsg` as the sole line (no trailing newline). `FormatHostMetrics` now delegates to it; `appendHostCell` deleted.
- **`computeColumnWidths`** in `internal/tui/table.go` — REMOVED in commit `7080e5a`. Both call sites in `dashboard_view.go` now use `table.ColumnWidths` directly.
- **`gofmt -l .` clean** since 2026-06-29.
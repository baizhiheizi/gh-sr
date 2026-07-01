---
name: repo-assist-state
description: Repo Assist persistent state — in-flight work, backlog, verified knowledge
metadata:
  type: project
---

# Repo Assist state — 2026-07-01

## Last run (28545571496) — Tasks 10, 2, 3

- **Task 10** — `repo-assist/eng-bench-compare-2026-07-01`, commit `2c76716`: 2 files (+378 lines). New `scripts/benchstat/main.go` (self-contained Go, stdlib only, `//go:build ignore`) + new `.github/workflows/bench-compare.yml`. **Closes #124 (option a, benchstat piece).** **Phantom PR #12** — patch + bundle at `/tmp/gh-aw/aw-repo-assist-eng-bench-compare-2026-07-01.{patch,bundle}` (14.5 KB / 6.2 KB).
- **Task 2** — comment on #124 (the bench-gate clarification + design rationale). Phantom comment — bridge failed.
- **Task 3** — no-op (0 open issues labeled `bug`/`help wanted`/`good first issue`).
- **Task 11** — `report_incomplete` after safe-outputs bridge failed across all write tools. The maintainer can apply the patch manually and post the #124 comment + open the new July issue themselves (full body text in the report_incomplete details payload).

## In-flight

- **Open PRs awaiting review:** none (all repo-assist drafts from prior runs are now merged or are phantom).
- **Phantom PR #12** — `repo-assist/eng-bench-compare-2026-07-01` at commit `2c76716`. Patch + bundle at `/tmp/gh-aw/aw-repo-assist-eng-bench-compare-2026-07-01.{patch,bundle}` (14.5 KB / 6.2 KB). Recovery: `git am` + push + `gh pr create`.

## Backlog

- **#132 (gh sr storage)** — on hold pending maintainer signal on loop-mount persistence.
- **#208 parent** — duplicate-code detector; ~1-2 findings/day, all recent ones closed.
- **#294** — doc-updater blocked (CHANGELOG.md protected); maintainer must create PR manually.
- **#124** — implementation ready locally; awaiting manual application of the patch.

## Notes

- **phantom-PR pattern:** **12 occurrences**. Most recent: run 28545571496 (bench-compare) — not just PRs this time, ALL safe-outputs write tools failed (create_issue, update_issue, add_comment, create_pull_request). This is a safe-outputs bridge outage, not a transient phantom.
- **safe-outputs bridge failure** (run 28545571496): every write tool returned `success` from the MCP server but produced no remote side effect. Recovery options: (a) wait for next run and retry (bridge may recover), (b) maintainer applies the patch manually, (c) investigate the agentic-workflows bridge on the platform side. **Recommendation: option (c) — the failure is too systemic to retry around.**
- **CI bench job:** already gated on both `push: [main]` and `pull_request: [main]`.
- **Revert rate:** 0/21+.

## Verified knowledge

- **`runner.DockerExecCommand(cname, innerCmd) string`** — canonical builder.
- **`runner.fetchToken(scope, target, endpoint, opLabel, emptyHint)`** — extracted in #284, closes #282.
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
- **Duplicate-code findings closed:** #251/253/260/261/281/282/283/295/299.
- **strconv.Quote round-trip:** input `evil"; rm -rf /; "` → `docker exec "evil\"; rm -rf /; \"" echo ok`.
- **`renderRow` / `renderHighlightedRow`** in `internal/tui/table.go` — share `renderRowWith(cells, widths, colorize, extraStyle)` body.
- **`table.RenderPlain(opts Options) string`** — strings.Builder + appendRowPlain; 1-alloc/cell.
- **`computeColumnWidths`** in `internal/tui/table.go` — REMOVED in commit `7080e5a`.
- **`gofmt -l .` clean** since 2026-06-29.
- **`internal/autostart.ListInstalled(h)`** dispatch contract: linux/darwin/windows/unsupported-OS.
- **`internal/autostart.parseInstanceLines(out)`** — preserves input order; dedupe is caller's job.
- **`internal/host.Host.RunShell`** wraps via `wrapCommand`: windows+non-local base64; otherwise no-op.
- **`internal/tui.hostMetricsHeaders`** — package-level `var` (commit `c2b96ad`); canonical column ordering shared by PrintHostMetricsTable / FormatHostMetrics / viewHostMetrics.
- **`internal/tui.buildHostMetricsRows(metrics)`** — shared row constructor (commit `c2b96ad`).
- **`internal/tui.hostMetricsColorize(col, cell)`** — shared colorize closure (commit `c2b96ad`); not used by FormatHostMetrics.
- **`runner.FormatBytesHuman(b int64)`** — 4× `fmt.Sprintf` → `strconv.AppendFloat`+stack `[16]byte` (3 float branches) + `strconv.FormatInt` (1 int branch). Per-call: 89→53 ns/op (−40%), 16→8 B/op (−50%), 1.9→1.14 allocs/op (−40%). Output byte-identical; parity-checked against 15 sub-tests including boundary cases at 1024 / 1MiB / 1GiB and negative-input clamping. (Commit `898e101`.)
- **`scripts/benchstat/main.go`** (commit `2c76716`) — self-contained Go, stdlib only, `//go:build ignore`. Parses `go test -bench=. -benchmem` single-line output, computes per-benchmark deltas for `ns/op` / `B/op` / `allocs/op`, prints a markdown table, exits 1 on a fail-level regression. Thresholds (warn/fail): `ns/op` 10/30%, `B/op` 15/50%, `allocs/op` 10/25%. Public surface: `parseFile`, `computeDelta`, `classify`, `formatPct`, `statusGlyph`, `main`. Excluded from `go build ./...` and `go test ./...` via the build ignore tag.
- **`.github/workflows/bench-compare.yml`** (commit `2c76716`) — runs on `pull_request: [main]`, downloads the most recent successful `bench-*` artifact from main via `gh api` against the Actions REST API, runs the comparison, posts a PR comment via `peter-evans/create-or-update-comment@v4` in `edit-mode: replace`. Self-deactivates (returns 0) when no main artifact exists.

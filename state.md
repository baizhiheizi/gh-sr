---
name: repo-assist-state
description: Repo Assist persistent state — in-flight work, backlog, verified knowledge
metadata:
  type: project
---

# Repo Assist state — 2026-07-01

## Last run (28494945299) — Tasks 2, 5, 8

- **Task 2** — no-op. #295 auto-expires today (addressed by #297). #299 closed locally via Task 5. #132/#124/#208 on hold.
- **Task 5** — `repo-assist/improve-hostmetrics-shared-builder-2026-07-01`, commit `c2b96ad`: extracted `hostMetricsHeaders` + `buildHostMetricsRows` + `hostMetricsColorize` in `internal/tui/metrics.go`. All 3 renderers (PrintHostMetricsTable / FormatHostMetrics / viewHostMetrics) share them. Diff: 2 files, +36 / −35. Closes #299. **Phantom PR #10** — patch+bundle at `/tmp/gh-aw/aw-repo-assist-improve-hostmetrics-shared-builder-2026-07-01.{patch,bundle}`.
- **Task 8** — `repo-assist/perf-formatbyteshuman-appendfloat-2026-07-01`, commit `898e101`: 4× `fmt.Sprintf` → `strconv.AppendFloat` + stack `[16]byte` in `FormatBytesHuman`. Per-call: 89→53 ns/op (−40%), 16→8 B/op (−50%), 1.9→1.14 allocs/op (−40%). Diff: 2 files, +66 / −12. Test 1→15 sub-tests; benchmark added. **Phantom PR #11** — patch+bundle at `/tmp/gh-aw/aw-repo-assist-perf-formatbyteshuman-appendfloat-2026-07-01.{patch,bundle}`.
- **Task 11** — `add_comment` on #100 succeeded (`aw_jul01_run`); `update_issue` not attempted per silent-failure pattern.

## In-flight

- **Open PRs awaiting review:** #298 (SplitSeq, mine), #300 (test-listinstalled, mine), #301 (perf-improver), + 2 phantoms (this run).

## Backlog

- **#132 (gh sr storage)** — on hold pending maintainer signal on loop-mount persistence.
- **#208 parent** — duplicate-code detector; ~1-2 findings/day.
- **#294** — doc-updater blocked (CHANGELOG.md protected); maintainer must create PR manually.
- **#295** — addressed by #297; auto-expiry 2026-07-01.

## Notes

- **phantom-PR pattern:** **11 occurrences**. Detection: `git ls-remote --heads origin | grep <branch>` or `search_pull_requests with head:` filter. Recovery: `git am <patch> && git push origin <branch> && gh pr create --base main --head <branch> --draft`.
- **update_issue on #100:** fails silently 5× confirmed; recovery via `add_comment`.
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
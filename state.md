---
name: repo-assist-state
description: Repo Assist persistent state — in-flight work, backlog, verified knowledge
metadata:
  type: project
---

# Repo Assist state — 2026-07-02

## Last run (28565949565) — Tasks 3, 10, 2

- **Task 3** — no-op fallback to Task 2. 0 open issues labeled `bug`/`help wanted`/`good first issue`.
- **Task 10** — `repo-assist/perf-metricsrow-appendfloat-2026-07-02`, commit `e0c9de9`: 1 file (`internal/tui/metrics.go`), +35 / −16. `formatPercent` + `formatUsedTotal` switched from `strings.Builder + strconv.FormatFloat` to `strconv.AppendFloat + stack [N]byte`. Same win-class as PR #303. **Measured** (3×3s, AMD Ryzen AI 9 HX 370): `BenchmarkFormatHostMetrics` ~3374→~3023 ns/op (**-10%**), 1944→1904 B/op (**-2%**); `BenchmarkMetricsRow` ~1042→~960 ns/op (**-8%**), 424→400 B/op (**-6%**). Allocs/op unchanged (the previous `strings.Builder` was already stack-allocated by escape analysis). **Phantom PR #13** — safeoutputs reported success but no PR opened; patch + bundle at `/tmp/gh-aw/aw-repo-assist-perf-metricsrow-appendfloat-2026-07-02.{patch,bundle}` (4.9 KB / 2.2 KB).
- **Task 2** — no-op.
- **Task 11** — `update_issue` to #309 (full body) — phantom. `update_issue` to #308 with status=closed — phantom.

## In-flight

- **Phantom PR #13** — `repo-assist/perf-metricsrow-appendfloat-2026-07-02` @ `e0c9de9`. Patch + bundle at `/tmp/gh-aw/aw-repo-assist-perf-metricsrow-appendfloat-2026-07-02.{patch,bundle}` (4.9 KB / 2.2 KB).
- **Phantom PR #12** — `repo-assist/eng-bench-compare-2026-07-01` @ `2c76716`. Patch + bundle at `/tmp/gh-aw/aw-repo-assist-eng-bench-compare-2026-07-01.{patch,bundle}` (14.5 KB / 6.2 KB). Bridge converted this to issue **#307** (protected file); recovery instructions in body.

## Backlog

- **#132 (gh sr storage)** — on hold.
- **#208 parent** — duplicate-code detector; ~1-2 findings/day, all recent ones closed.
- **#307** — phantom PR turned into issue; recovery in body.
- **#308** — duplicate Monthly Activity issue; close phantom-failed.
- **#124** — implementation ready in patch form.

## Notes

- **phantom-PR pattern:** **13 occurrences**. Most recent: run 28565949565 (metricsrow AppendFloat). Safe-outputs bridge is fully non-functional for write operations.
- **safe-outputs bridge failure** (runs 28545571496 + 28565949565): every write tool returned `success` but no remote side effect. **Recommendation: investigate the agentic-workflows bridge** — failure has persisted across two consecutive runs.
- **bridge partial success:** bridge successfully converted `create_pull_request` → `create_issue` for the protected-file case (#307). Bridge is *not* fully silent.
- **Revert rate:** 0/21+.

## Verified knowledge

- **`runner.DockerExecCommand(cname, innerCmd) string`** — canonical builder.
- **`runner.fetchToken(scope, target, endpoint, opLabel, emptyHint)`** — extracted in #284.
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
- **Quoting policy:** `strconv.Quote` for docker CLI shell args; `hostshell.PosixSingleQuote` for single-quoted shell snippets.
- **Protected files:** `go.mod`, `go.sum`, `CHANGELOG.md`, `.github/workflows/ci.yml`, `.github/workflows/bench-compare.yml`.
- **`update_issue` body limit:** 10 KB.
- **Duplicate-code findings closed:** #251/253/260/261/281/282/283/295/299.
- **`renderRow` / `renderHighlightedRow`** in `internal/tui/table.go` — share `renderRowWith(cells, widths, colorize, extraStyle)` body.
- **`table.RenderPlain(opts Options) string`** — strings.Builder + appendRowPlain; 1-alloc/cell.
- **`internal/autostart.ListInstalled(h)`** dispatch contract: linux/darwin/windows/unsupported-OS.
- **`internal/autostart.parseInstanceLines(out)`** — preserves input order; dedupe is caller's job.
- **`internal/host.Host.RunShell`** wraps via `wrapCommand`: windows+non-local base64; otherwise no-op.
- **`internal/tui.hostMetricsHeaders`** — package-level `var` (commit `c2b96ad`).
- **`internal/tui.buildHostMetricsRows(metrics)`** — shared row constructor (commit `c2b96ad`).
- **`internal/tui.hostMetricsColorize(col, cell)`** — shared colorize closure (commit `c2b96ad`); not used by FormatHostMetrics.
- **`runner.FormatBytesHuman(b int64)`** — 4× `fmt.Sprintf` → `strconv.AppendFloat`+stack `[16]byte` + `strconv.FormatInt`. Per-call: 89→53 ns/op (−40%), 16→8 B/op (−50%), 1.9→1.14 allocs/op (−40%). (Commit `898e101`.)
- **`tui.formatPercent(v, prec)`** (commit `e0c9de9`) — `strconv.AppendFloat` + stack `[24]byte` + `append(b, '%')`. ~92 ns/op, 5 B/op, 1 alloc/op.
- **`tui.formatUsedTotal(used, total, pct, unit)`** (commit `e0c9de9`) — `strconv.AppendFloat` + stack `[48]byte` for "used/total UNIT (pct%)". ~155 ns/op, 24 B/op, 1 alloc/op. End-to-end (`BenchmarkFormatHostMetrics`, 5 samples): 3374→3023 ns/op (**-10%**), 1944→1904 B/op (**-2%**).
- **`scripts/benchstat/main.go`** (commit `2c76716`) — self-contained Go, stdlib only, `//go:build ignore`. Parses `go test -bench=. -benchmem` output, computes per-benchmark deltas for `ns/op` / `B/op` / `allocs/op`. Thresholds (warn/fail): `ns/op` 10/30%, `B/op` 15/50%, `allocs/op` 10/25%. Public surface: `parseFile`, `computeDelta`, `classify`, `formatPct`, `statusGlyph`, `main`.
- **`.github/workflows/bench-compare.yml`** (commit `2c76716`) — runs on `pull_request: [main]`, downloads the most recent successful `bench-*` artifact from main, runs the comparison, posts a PR comment via `peter-evans/create-or-update-comment@v4` in `edit-mode: replace`. **PROTECTED FILE** — `create_pull_request` converted to `create_issue` (#307).
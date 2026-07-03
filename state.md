---
name: repo-assist-state
description: Repo Assist persistent state — in-flight work, backlog, verified knowledge
metadata:
  type: project
---

# Repo Assist state — 2026-07-03

## Last run (28638387867) — Tasks 9, 4, 8

- **Task 8** — `repo-assist/perf-loadstr-appendfloat-2026-07-03` @ `977e1c5`. LoadStr switched to `AppendFloat + stack [40]byte`. `BenchmarkLoadStr` -9.5% ns/op, -33% B/op. **NOT submitted as PR** — maintainer reverted working tree mid-run.
- **Task 9** — `repo-assist/test-editor-open-windows-default-2026-07-03` @ `6db86e8`. editor_test.go +58/-3. Coverage 53.8% → 92.3%. PR reported success. Patch `/tmp/gh-aw/aw-repo-assist-test-editor-open-windows-default-2026-07-03.{patch,bundle}`.
- **Task 4** — `repo-assist/eng-makefile-fmt-target-2026-07-03` @ `9bfa21c`. Makefile +19/-2 (make fmt, tidy, ci). PR reported success. Patch `/tmp/gh-aw/aw-repo-assist-eng-makefile-fmt-target-2026-07-03.{patch,bundle}`.
- **Task 11** — updated #309 with new full body. Reported success.

## In-flight / backlog

- **Phantom PR #12** — `repo-assist/eng-bench-compare-2026-07-01` @ `2c76716`. Converted to issue #307. Patch+bundle still on disk.
- **#124** — implementation ready in patch form (work above) but #307 issue still has recovery steps.
- **#132** (gh sr storage) — on hold.
- **#208** parent — duplicate-code detector; all recent findings closed.
- **#307** — phantom PR turned into issue; recovery in body.
- **#308** — duplicate Monthly Activity issue.
- **#309** — current Monthly Activity.

## Notes

- **phantom-PR pattern:** 14 occurrences (across this and prior runs). Cross-cutting observation: bridge silently drops writes for unprotected files but does convert PR to issue for protected files (see #307).
- **mid-run reverts are observed:** run 28638387867 saw `internal/host/metrics.go` reverted to the strings.Builder version by the user/linter between commit and PR creation. Future runs should always re-verify working-tree state before opening PRs.

## Verified knowledge

- `runner.DockerExecCommand`, `runner.fetchToken`, `runContainerCheck`, `runOnHostOS[T]`, `internal/ops.connectHostFn` + `installMockConnectHost`, `GitHubClient.GetLatestRunnerVersion()` (sync.Once), `uniqueStringsBy`, `installTargetsForHost`, `writeHostBanner`, `printAgenticFailures`, `failureCollector`, `containerImageExists`.
- **Quoting:** `strconv.Quote` for docker CLI args; `hostshell.PosixSingleQuote` for shell snippets.
- **Protected:** `go.mod`, `go.sum`, `CHANGELOG.md`, `.github/workflows/ci.yml`, `.github/workflows/bench-compare.yml`.
- **Render:** `renderRow`/`renderHighlightedRow` share `renderRowWith`; `table.RenderPlain` = strings.Builder + appendRowPlain, 1-alloc/cell.
- **autostart:** `ListInstalled(h)` linux/darwin/windows/unsupported dispatch; `parseInstanceLines(out)` preserves order.
- **host.Host.RunShell:** wraps via wrapCommand (windows+non-local base64; else no-op).
- **tui (commit c2b96ad):** `hostMetricsHeaders` var, `buildHostMetricsRows`, `hostMetricsColorize` shared by PrintHostMetricsTable + FormatHostMetrics + viewHostMetrics.
- **FormatBytesHuman (898e101):** 4× fmt.Sprintf → AppendFloat + [16]byte + FormatInt. -40% ns/op, -50% B/op, -40% allocs.
- **tui.formatPercent + formatUsedTotal (e0c9de9):** AppendFloat + [24]byte / [48]byte. `BenchmarkFormatHostMetrics` -10% ns/op, -2% B/op.
- **scripts/benchstat/main.go (2c76716):** //go:build ignore, stdlib only; parseFile, computeDelta, classify, formatPct, statusGlyph, main. Thresholds: ns/op 10/30%, B/op 15/50%, allocs/op 10/25%.
- **bench-compare.yml (2c76716):** pull_request:[main], downloads main bench-*, posts PR comment via peter-evans/create-or-update-comment@v4 edit-mode:replace. PROTECTED FILE.
- **host.LoadStr (977e1c5, branch only):** AppendFloat + [40]byte. -9.5% ns/op, -33% B/op. Maintainer reverted working tree; PR not opened.
- **editor.Preferred + Open:** Windows notepad default + exec error chain wraps `exec.ErrNotFound` (NOT fs.ErrNotExist). os/exec tests should match exec.ErrNotFound.
- **Makefile targets after 9bfa21c:** all build test bench vet fmt tidy ci check clean install uninstall. `make ci` mirrors ci.yml (vet + fmt + test).

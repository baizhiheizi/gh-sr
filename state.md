---
name: repo-assist-state
description: Repo Assist persistent state — in-flight work, backlog, verified knowledge
metadata:
  type: project
---

# Repo Assist state — 2026-07-07 (run 28878070395)

## Last run (28878070395) — Tasks 9, 3, 2
- **Task 9** — `repo-assist/test-host-metrics-collect-dispatch-2026-07-07` @ `9a63cb3`. metrics_test.go +158. 4 new tests lock the per-OS script generator picked by `Host.CollectMetrics`. PR reported success. Patch/bundle at `/tmp/gh-aw/aw-repo-assist-test-host-metrics-collect-dispatch-2026-07-07.{patch,bundle}`.
- **Task 2** — Comments on #325 (agentic probe metadata duplication; offered surgical sub-scope) and #307 (bench-compare recovery; confirmed `/tmp/gh-aw/` patch is wiped, recommended downloading from GitHub Actions artifact of run `28545571496`).
- **Task 3** — Fallback (no bug/help-wanted/good-first-issue).
- **Task 11** — Updated #309 with new full body. Reported success.

## In-flight / backlog
- **#124 / #307** — bench-compare piece. `2c76716` on `repo-assist/eng-bench-compare-2026-07-01`. Phantom PR. Underlying `agent-*` artifact on run `28545571496` is recoverable until late September 2026 (90-day retention).
- **#132** (gh sr storage) — on hold; awaiting maintainer signal on loop-mount persistence.
- **#208** parent — duplicate-code detector; recent findings closed/commented on (#325/#326/#327 from 2026-07-06).
- **#309** — current Monthly Activity.
- **Phantom PRs #312, #313** — still listed in #309's Suggested Actions; verify if landed since 2026-07-04.

## Notes
- **phantom-PR pattern:** 16+ occurrences across runs. Recovery mechanism now recommends the GitHub Actions artifact directly because `/tmp/gh-aw/` is wiped between runs.
- **mid-run reverts** (re-confirmed this run): NOT observed. The metrics test file (a non-protected `_test.go`) survived commit + bridge call cleanly. Phantom-PR pattern did not recur for the metrics test PR.

## Verified knowledge
- `runner.DockerExecCommand`, `runner.fetchToken`, `runContainerCheck`, `runOnHostOS[T]`, `internal/ops.connectHostFn` + `installMockConnectHost`, `GitHubClient.GetLatestRunnerVersion()` (sync.Once), `uniqueStringsBy`, `installTargetsForHost`, `writeHostBanner`, `printAgenticFailures`, `failureCollector`, `containerImageExists`.
- **Quoting:** `strconv.Quote` for docker CLI args; `hostshell.PosixSingleQuote` for shell snippets.
- **Protected:** `go.mod`, `go.sum`, `CHANGELOG.md`, `.github/workflows/ci.yml`, `.github/workflows/bench-compare.yml`.
- **Render:** `renderRow`/`renderHighlightedRow` share `renderRowWith`; `table.RenderPlain` = strings.Builder + appendRowPlain, 1-alloc/cell.
- **autostart:** `ListInstalled(h)` linux/darwin/windows/unsupported dispatch; `parseInstanceLines(out)` preserves order.
- **host.Host.RunShell:** wraps via wrapCommand (windows+non-local base64; else no-op).
- **tui (c2b96ad):** `hostMetricsHeaders` var, `buildHostMetricsRows`, `hostMetricsColorize` shared by PrintHostMetricsTable + FormatHostMetrics + viewHostMetrics.
- **FormatBytesHuman (898e101):** 4× fmt.Sprintf → AppendFloat + [16]byte + FormatInt. -40% ns/op, -50% B/op, -40% allocs.
- **tui.formatPercent + formatUsedTotal (e0c9de9):** AppendFloat + [24]byte / [48]byte. `BenchmarkFormatHostMetrics` -10% ns/op, -2% B/op.
- **scripts/benchstat/main.go (2c76716):** //go:build ignore, stdlib only; parseFile, computeDelta, classify, formatPct, statusGlyph, main. Thresholds: ns/op 10/30%, B/op 15/50%, allocs/op 10/25%.
- **bench-compare.yml (2c76716):** pull_request:[main], downloads main bench-*, posts PR comment via peter-evans/create-or-update-comment@v4 edit-mode:replace. PROTECTED FILE.
- **host.LoadStr (977e1c5, branch only):** AppendFloat + [40]byte. -9.5% ns/op, -33% B/op. Maintainer reverted working tree; PR not opened.
- **editor.Preferred + Open:** Windows notepad default + exec error chain wraps `exec.ErrNotFound` (NOT fs.ErrNotExist).
- **Makefile targets after 9bfa21c:** all build test bench vet fmt tidy ci check clean install uninstall. `make ci` mirrors ci.yml (vet + fmt + test).
- **host/metrics.go (9a63cb3, this run):** `Host.CollectMetrics` → `collectMetricsWindows` (if h.OS=="windows", base64-encoded via `host.wrapCommand` for non-local) or `collectMetricsUnix` (else). `unixMetricsScript(hostOS)`: darwin → `top -l 2` + `vm_stat` + `vm.loadavg`; else (linux/freebsd/etc.) → `/proc/stat` + `/proc/meminfo` + `/proc/loadavg`. `windowsMetricsScript()` → `Get-CimInstance` on Win32_Processor / Win32_OperatingSystem / Win32_LogicalDisk. Test contracts: TestCollectMetrics_DispatchOS asserts wrapper surface for Windows, body for linux/darwin; TestUnixMetricsScript_Branches locks all three branches; TestWindowsMetricsScript_Body locks the unencoded PowerShell body; TestCollectMetrics_ScriptErrorPropagates asserts `fmt.Errorf("running metrics script: %w", err)` wrap.
- **agentic/agentic.go fanout (re-confirmed via #325):** `containerAgenticFanoutCheckCommand` lines 760-805 inlines the 6 shell bodies (duplicated from `*CheckCommand` helpers at lines 945-1005). `containerAgenticFanoutSpecs` lines 815-909 repeats Name/Severity/Message/Remediation/DocRef for the 6 probes (duplicated from per-check wrappers at lines 530-688). `agentic_fanout_test.go` snapshots BOTH the assembled command string and the spec map — this snapshot is the load-bearing coupling keeping the two paths in sync today. Any unification refactor must update both snapshots in lockstep.

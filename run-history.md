---
name: run-history
description: Test-improver run history
metadata:
  type: project
---

## 2026-06-11 12:39 UTC — Run 27346830049

- Branch: `test-assist/hostshell-write-remote-bytes`; commit ab1f73f
- PR: draft via `create_pull_request` safeoutputs
- Patch: `/tmp/gh-aw/aw-test-assist-hostshell-write-remote-bytes.patch` (~14 KB / 362 lines)
- Coverage: `internal/hostshell` 23.1% → 100.0% (+76.9 pp); `WriteRemoteBytes` 0% → 100%
- New file: `internal/hostshell/hostshell_remote_test.go` (362 lines, 4 test functions, 16 sub-tests, parallel)
- Pinned contracts: PosixSingleQuote round-trips through `'\\''` escape in script, PowerShellSingleQuote round-trips through `''` doubling, executor errors propagate, Upload() never called

## 2026-06-11 04:08 UTC — Off-cycle run (PR #149 merged)

- escapePS PowerShell single-quote escape in internal/diskschedule: 0% → 100%
- internal/diskschedule coverage: 15.6% → 16.3% (+0.7 pp)
- 27 lines added to internal/diskschedule/diskschedule_test.go
- Note: not in repo-memory before this run; backfilled now.

## 2026-06-10 12:35 UTC — Run 27275755111

- Branch: `test-assist/disk-printer-helpers`; commit 5508526
- PR: draft via `create_pull_request` safeoutputs
- Patch: `/tmp/gh-aw/aw-test-assist-disk-printer-helpers.patch` (19724 B / 590 lines)
- Coverage: `internal/ops` 9.6% → 19.6% (+10.0 pp); six helpers 0% → 100% (`columnWidths`, `printRow`, `printPruneResult`, `PrintDiskUsageTable`, `diskHostInstanceKey`, `rcByInstanceForHost`)
- New file: `internal/ops/disk_printer_test.go` (543 lines, 25 sub-tests, parallel)
- Pinned contracts: `PrintDiskUsageTable` does not sort, err-row `TotalBytes` excluded from totals, `columnWidths` uses byte length
- Note: First-cut cell-parsing test broke on `InstanceNames` (Name "ci-1" with Count=1 produces "ci-1-1"). Switched to suffix-anchored `strings.TrimRight` assertions.

## 2026-06-08 12:52 UTC — Run 27138355567

- Branch: `test-assist/awf-hygiene-validators`
- Coverage: `internal/agentic` 60.5% → 83.9% (+23.4 pp); `ValidateAWFHygiene` + `ValidateAWFHygieneInner` 0% → 100%
- New file: `internal/agentic/agentic_awf_hygiene_test.go` (435 lines, 13 subtests)
- Note: First-cut test failed because inner iptables probe drops `sudo -n` — exact-string mock caught it.

## 2026-06-06 11:46 UTC — Run 27061368592

- Branch: `test-assist/agentic-validation-helpers`
- Coverage: `internal/agentic` 44.8% → 60.5% (+15.7 pp); five `Validate*` helpers 0% → 100%
- PR subsequently closed as duplicate #107/#108

## Previous test-improver PRs (most recent first)

- #68 (merged 2026-05-31) — IsServiceActive, absRunnerDir, containerRunnerPresent, containerImageExists
- #58 (merged 2026-04-29) — DetectOS/DetectArch/DetectDockerAvailable + mock infra
- #54 (merged) — nativeConfigURL, powerShellSingleQuoted, windowsDeleteRunnerConfig
- #51 (merged 2026-04-24) — applyContainerImageExtras + lockedWriter
- #40 (merged 2026-04-19) — posixSingleQuote + editor helpers
- #31 (merged 2026-04-17) — HasBlockingFailures + remediation formatters
- #11 (merged) — shellSingleQuote + docker_runner_entry_script

Monthly Activity issues: #4 (April, closed), #69 (May, closed), #109 (June, active).

[[wip]] [[backlog]]

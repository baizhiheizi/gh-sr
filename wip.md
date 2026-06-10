---
name: wip
description: Current test-improver work in progress
metadata:
  type: project
---

## Run #27275755111 (2026-06-10 12:35 UTC)

- **Branch:** `test-assist/disk-printer-helpers`
- **Commit:** `5508526 test(ops): cover disk-printer helpers and per-host instance map`
- **PR:** draft via `create_pull_request` safeoutputs; patch at `/tmp/gh-aw/aw-test-assist-disk-printer-helpers.patch` (19724 B / 590 lines)
- **Coverage delta:** `internal/ops` 9.6% → 19.6% (+10.0 pp); six helpers in `internal/ops/disk.go` 0% → 100%: `columnWidths`, `printRow`, `printPruneResult`, `PrintDiskUsageTable`, `diskHostInstanceKey`, `rcByInstanceForHost`
- **New file:** `internal/ops/disk_printer_test.go` (543 lines, 6 top-level test functions, 25 sub-tests, all parallel)
- **Status:** build ✅, vet ✅, `go test ./... -race -count=1` ✅, `gofmt` ✅

**Next:** The `ConnectHost` injection refactor for `runPerHostParallel` and the rest of the `internal/ops` orchestrators is still the biggest unblocked win. This run covered what is testable today (pure helpers); the next move is the refactor itself. Worth a focused issue-discussion before implementation — the blast radius touches every multi-host op.

[[backlog]] [[run-history]]

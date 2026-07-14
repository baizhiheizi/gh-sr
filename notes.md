---
name: perf-improver-state-2026-07
description: Persistent state for Perf Improver agent on baizhiheizi/gh-sr — run 27 on 2026-07-14, PR (parseUnixMetrics zero-alloc) opened (commit 9602a91), #358 + #361 merged
metadata:
  type: project
---

# Perf Improver State (baizhiheizi/gh-sr)

**Last run:** 2026-07-14 21:18 UTC (run 29368565728, 27th scheduled run)
**Run link:** https://github.com/baizhiheizi/gh-sr/actions/runs/29368565728

## Repository Status
Latest commit on main: `e774249 [perf-improver] perf(runner): combine Start/Stop svc+autostart probe into 1 SSH round-trip (#361)` (squash of #359 from run 26). Both run-25 PR #358 (orphan-plan-batch) and run-26 PR #361 (start-svc-autostart-batch) are now merged. 🎉

## This Run's Work (run 27, 2026-07-14)
- Re-validated commands: `go build ./...` ✅, `go vet ./...` ✅, `go test ./... -race -count=1` ✅ (17/17 packages pass), `gofmt -l .` clean
- Confirmed merges: **PR #361** (= #359 squash) on main as `e774249`, **PR #358** also confirmed merged
- Audit: backlog remaining was 0 SSH-round-trip consolidations on hot paths. Pivoted to in-process parsing (item 40, NEW).
- 🔧 **Created local commit** `9602a91` on `perf-assist/parse-unix-metrics-no-fields-alloc-2026-07-14` and opened draft PR (safe-outputs success, but branch not yet visible at origin — likely phantom-success). Replaces `strings.Fields(v)` in `parseUnixMetrics` `"load"` case with two `strings.IndexByte(v, ' ')` calls and three sub-slice `strconv.ParseFloat` invocations (all alias `v`, no copy).
  - **Drops the only heap allocation in the function**: 48 B/op → 0 B/op, 1 allocs/op → 0 allocs/op.
  - **~21% faster at ~212 ns/op** (was ~270 ns/op).
  - `parseUnixMetrics` runs once per host per TUI metric-refresh tick (5s default).
  - Adds `TestParseUnixMetrics_LoadMalformed` (3 cases: no-spaces, one-space-only, empty-value) to lock the new parser's tolerance of incomplete load lines. Skipped "trailing-space" case because upstream `strings.TrimSpace` strips trailing whitespace before the load case sees it.

## Validated Commands
- `go build ./...` ✅
- `go test ./... -race -count=1` ✅ all 17 packages pass (race-clean)
- `go vet ./...` ✅ clean
- `gofmt -l .` ✅ clean
- `go test -bench=BenchmarkParseUnixMetrics -benchmem -run=^$ ./internal/host/` ✅ used to baseline + verify this run

## Open Perf Improver PRs / Local Branches
- **PR (this run)**: `perf-assist/parse-unix-metrics-no-fields-alloc-2026-07-14` (commit `9602a91`) — parseUnixMetrics `strings.Fields(v)` → `IndexByte`. **Phantom-success** — branch not yet visible at origin. Diff: 2 files, +50 / −5.
- `perf-assist/removecontainer-state-dir-foldin-2026-07-10` (commit `e01dbfe`, **branch not pushed — no GitHub PR**) — `removeContainer` state-dir `echo $HOME` skip. **Action item for maintainer**: push branch and open PR. Diff: 2 files, +104 / −5.

## Merged This Series (recap, updated)
- PR #350 (`[perf-improver]`): `containersPresentOneShot` — phantom-success from run 22 (local commit `6fb977f`).
- PR #358 (`[perf-improver]`): `orphanLinuxPlanProbe` (PlanOrphanCleanup 3→1) — phantom-success from run 25 (local commit `1a426ee`).
- PR #361 (`[perf-improver]`): `linuxSvcAndAutostartProbe` (Start/Stop svc+autostart 2→1) — phantom-success from run 26 (local commit `e749be8`), squash-merged.
- (PR #123, #128, #146, #155, #167, #255, #264, #269, #285, #301, #317, #322, #342, #359 ⇒ #361, #358 from the same series — all merged). Plus PR #333 (benchstat CI), PR #340, PR #347 (#349), PR #348 (#346), PR #345/#357 (`FormatNumber`), PR #353 (`posixRunnerDirVar`), PR #355 (`LoadStr`), and many doctor/test coverage PRs (#329, #343, #344, #351, #352, #354, #362, #363, #364, #366, #367) from sibling workflows.

## Open Performance Issues
- #318 — `[perf-improver] Monthly Activity 2026-07` (maintained across runs, updated this run)
- #124 — `[efficiency-improver] Add benchmark regression detection to CI` — CLOSED via PR #333 🎉

## Next Run Tasks
- Task 1: Commands re-validate (passing; 17/17 packages with `-race -count=1`)
- Task 2/4: Check phantom-success branches — both `parse-unix-metrics-no-fields-alloc-2026-07-14` (this run) and `removecontainer-state-dir-foldin-2026-07-10` (run 23). Watch for maintainer push.
- Task 3: Look at next parsing hot path. Candidates after parseUnixMetrics zero-alloc:
  - `runner.diskschedule` `parseAtTime` further trimming (PR #199 already landed Sscanf→Atoi + Split→Cut). Could explore `strings.IndexByte` for the colon byte.
  - `runner.containerLocalStatusOneShot` parsing chain — any remaining `strings.Split` or `fmt.Fprintf`?
  - `host.detect` PowerShell cascade cold-path consolidation (item 43).
  - `agentic.ValidatePrereqs` cold-path chain (item 41).
- Task 7: Update Monthly Activity July issue (this run, completed)

## Memory note — IMPORTANT for future runs
Phantom-success pattern continues to fire. **9 of 14 phantom-success branches in this series have been pushed and merged by the maintainer** (PRs #342, #350, #358, #361 most recently all came from phantom-success commits in earlier runs). The recovery protocol is well-established: document the local commit, save branch ref in memory, exit via `noop` if needed.

## Memory note (research/observations)
- **General principle (validated thirteen times now)**: When a helper returns multiple correlated values from a single I/O call, never split those reads across multiple invocations. Examples closed in this series: PR #264, #269, #285, #301, #317, #322, #333, #334, #342, #350, #358, #361, agentic.ValidatePrereqs merged work (local commit `0fcd1e1`), local commits `6fb977f`, `e01dbfe`, `1a426ee`, `e749be8`, and now `9602a91`.
- **`strings.Fields` → `strings.IndexByte` for known-shape separators**: when the input shape is fixed (e.g. exactly 3 space-separated fields), two `IndexByte` calls locate separators, and ParseFloat sub-slices alias the input without copy. Drops `strings.Fields`'s `[]string` allocation. Same pattern as PR #202 (Split→Cut chain) but for a multi-field separator case.
- **Sub-slice `strconv.ParseFloat` does not allocate on success**: the parse-only path returns the float and `nil` error without any heap allocation. Only the error path allocates (and we discard errors in our case).
- **`parseUnixMetrics` is on the per-host-per-tick path**: 5s refresh × 10 hosts × 24 h × 60 min ≈ 17,280 calls/day per open TUI. Dropping the 48 B allocation per call = ~830 KB/day of short-lived GC pressure saved per open TUI. Small per-call, but compounds on long-running dashboards.
- **Repo past the hot-path ceiling for SSH round-trip consolidations** on Linux: every `gh sr up`/`down`/`setup`/`doctor`/`cleanup` hot path now does ≤1 SSH round-trip per batched probe. Remaining cold-path candidates (agentic.ValidatePrereqs, autostart.InstallSystemdSystem) and in-process parsing/rendering (parseUnixMetrics item 40 closed this run; parseAtTime further trimming possible).
- **`$HOME` literal in shell commands**: when a shell command is going to use the path with `rm -rf` or `test -f`, the shell can expand `$HOME` itself — no need to first resolve it to an absolute path via `echo $HOME`. The previous `resolveStateDirOrFallback(h, instanceName)` helper was overkill for these code paths. The `$HOME` literal form is now used in: startContainer (PR #342), containerLocalStatusOneShot, disk.go orphans listing, removeContainer state-dir (local commit `e01dbfe`).
- **Multi-marker shell output**: a single shell call can emit multiple independent boolean results on separate lines (D/S/U/Y markers), then the Go side parses them into a structured tuple. This is the model used by `containersPresentOneShot` (PR #350), `orphanLinuxPlanProbe` (PR #358), and `linuxSvcAndAutostartProbe` (PR #359).
- **Benchstat regression detection is live** (PR #333). Every PR now gets a per-benchmark ns/op, B/op, allocs/op diff against `main`. This made the parseUnixMetrics zero-alloc PR measurable on the same PR CI run.
- **Marker convention**: `D` = directory, `S` = svc.sh, `U` = user systemd, `Y` = system systemd. Used consistently across orphanLinuxPlanProbe (D/S/U/Y) and linuxSvcAndAutostartProbe (S/U/Y — no directory check needed in Start/Stop path).
- **`strings.IndexByte` over `strings.Fields`**: when input separator is a single byte and field count is known, IndexByte is both faster and zero-alloc. `strings.Fields` always allocates a `[]string` slice; two `IndexByte` calls partition the input into N+1 sub-slices that alias the original.

## Backlog cursor (for round-robin)
Last touched: 2026-07-14 (27th run). Next run should re-validate these candidates (in order of impact):
1. `runner.diskschedule` `parseAtTime` further trimming — `strings.IndexByte` for colon byte instead of `strings.Split` if benchmarks warrant.
2. `agentic.ValidatePrereqs` id -u → sudo iptables → id -un chain (3 sequential dependent probes → 1-2 round-trips — lower priority, `_test.go` caller only).
3. `host.DetectOS` / `host.DetectArch` PowerShell cascade (cold path, Windows only, very limited impact).
4. `autostart.Install` installSystemdSystem home + id tuple (already combined at `autostart.go:180` — closed in run 22, skip).
5. Check for any remaining `strings.Split` or `fmt.Fprintf` on the rendering hot paths in `internal/runner/container.go` or `internal/tui`.

**⚠️ Major SSH-round-trip consolidations on hot paths are all closed.** Remaining incremental opportunities are cold paths OR in-process parsing/rendering. Benchstat regression detection (PR #333) keeps these contributions measurable.

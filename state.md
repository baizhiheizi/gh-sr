---
name: efficiency-improver-state
description: Efficiency Improver persistent state — last run, work in progress, discovered commands, validated optimization patterns, and the energy-efficiency backlog for baizhiheizi/gh-sr.
metadata:
  type: project
---

## Last Run

- Date: 2026-11-12
- Run ID: 43130118672
- Workflow: efficiency-improver
- Round-robin slot this run: **Tasks 2+3 (implement the top backlog item: Manager.Status batch)** + Task 4 verification + Task 7.

## Work This Run

- **Manager.Status cross-instance batch (2-3N → 1 SSH round trip per host)** — the top backlog item. New `internal/runner/status_batch.go`: `buildNativeStatusBatchScript` (full `statusNativeState` machine per instance in one POSIX script: version cat → svc.sh systemctl branch → autostart user/system branch → PID-file fallback; elevation gate `id -u`+`sudo -n true` evaluated ONCE per script) and `buildContainerStatusBatchScript` (the containerLocalStatusOneShot shell verbatim + tagged emit). Both emit `@@ns:<i>|<local>|<version>` / `@@cs:<i>|<payload>` via the `internal/probe` Writer; shared `parseStatusBatch` decodes with `probe.Lines`; missing/duplicate/out-of-range index → whole batch fails → Status falls back to the per-instance probes verbatim. Non-Linux hosts skip the batch.
- `Manager.Status` in internal/runner/runner.go: batch-first (container always; native only `h.OS == "linux"`), unchanged per-instance loop as fallback. Version "" → "-" mapping preserved.
- Deliberate folds documented in code: double `ServiceActiveState` probe → one probe (second call only re-read an already-inactive state); `.runner-version` whitespace collapsed to single words (mirrors V-marker space-join).
- **The real-shell scenario suite caught a genuine bug**: my first mapping used one `case` for both branches, but `statusNativeState` maps svc-branch "inactive" → stopped while the kind-branch leaves it EMPTY so the PID-file check decides. Fixed: `elif [ "$src" = svc ] → stopped` / `elif failed|activating → service error` / else fall through. The shell tests earn their keep — keep writing them for any new remote state machine.
- Created draft PR on branch `efficiency/status-batch-one-shot` (number not visible during run).
- Monthly Activity 2026-11 issue identified as **#602** (found via list_issues + labels:["efficiency"] — the filter-removal note reveals the number). Appended this run's entry (append-only; cannot read filtered bodies).
- Task 4: **PR #601 (cache.Inspect 5→1 fold) was closed UNMERGED 2026-11-05** — do NOT re-propose it. The maintainer generalized the one-shot pattern into `internal/probe` during the #609 architecture refactor (probe module #613/#632, strfmt dissolved, ops fanout unified, Manager rewiring — those PRs were also closed unmerged with commits squashed onto main). No open `[efficiency-improver]` PRs remain.

## Measurements

Manager.Status round trips (exact proxy metric, 0 % variance):

| Benchmark | Before (main) | After (branch) |
| --------- | ------------- | -------------- |
| ManagerStatusRoundTripsNative1 | 2.000 | **1.000** |
| ManagerStatusRoundTripsNative4 | 8.000 | **1.000** |
| ManagerStatusRoundTripsNative16 | 32.00 | **1.000** |
| ManagerStatusRoundTripsContainer4 | 4.000 | **1.000** |
| ManagerStatusRoundTripsNative4Fallback | — | 9.000 (1 failed batch + 2×4) |

Mock-local ns/op regressed (~1.2→~2.4 µs N1, ~15→~28 µs N16; B/op 16.8→94.5 KB N16) — script-text build cost, same CPU-for-network trade-off as the cache fold. Real-host saving: ~31 RTs/tick for 16 instances.

Prior baselines: cache.Inspect 1 RT (PR #601 closed; pattern now in internal/probe), MeasureDiskUsageBatch 1 RT, agentic fanout 1 RT.

## Tooling limits (important for future runs)

- **safeoutputs-created issues/PRs cannot be read** through the MCP GitHub tools — integrity policy filters them. `list_issues` DOES reveal their **numbers** in the filter-removal note; filtering by label narrows which number belongs to which issue (used to identify #602).
  - Consequence: use `update_issue` with `operation: append` for monthly issues, never `replace`.
  - PR/issue numbers created via safeoutputs are **not visible** during the run — reference by branch name / title.
- `mcp__github__ui_get` denied; `list_label` works.
- search_pull_requests `fields` enum: `head` and `merged` are NOT valid; use number/title/state/draft/html_url/updated_at/closed_at.
- Maintainer pattern: agent PRs get closed unmerged with the changes squashed onto main themselves (e.g. #632 → commit df2eadb). Check `git log` before assuming a PR was rejected on merit.

## Open Items

- Draft PR on branch `efficiency/status-batch-one-shot` — Manager.Status batch (awaiting maintainer review; recommend a real-VPS spot-check of the elevation gate).
- Open issue #595 (2026-10-21): benchstat drops sub-benchmark rows (noise hardening) — still awaiting maintainer input.
- `[efficiency-improver] Monthly Activity 2026-11` = issue **#602** (open).
- Other agent PRs (#570/#566/#610 etc.) — not ours to maintain.

## Discovered Commands (validated)

```bash
# Build / test / vet / format
GOTOOLCHAIN=auto go build ./...
GOTOOLCHAIN=auto go test ./... -race -count=1
GOTOOLCHAIN=auto go vet ./...
gofmt -l .   # must be empty

# Benchmarking via Makefile
make bench          # go test ./... -run='^$' -bench=. -benchmem -count=3
make bench-save     # writes bench-results/bench-<UTC-stamp>.txt
make ci             # vet + fmt + test

# Round-trip metric benchmarks only
GOTOOLCHAIN=auto go test ./internal/runner/ ./internal/agentic/ ./internal/cache/ \
  -run='^$' -bench='RoundTrips' -benchmem -count=3 -benchtime=500ms

# Diff two snapshots
go run scripts/benchstat/main.go -base base.txt -head head.txt -packages internal/runner -output out.md
```

Notes:
- `go.mod` requires go 1.26.0; local toolchain older → `GOTOOLCHAIN=auto` everywhere.
- Full bench run ~4 min → run in background. `scripts/benchstat/main.go` has `//go:build ignore`; run via `go run`.
- Benchmarks must be FLAT-named (benchstat regex drops `/` sub-benchmarks).

## Validated Optimization Patterns

- **`internal/probe` is the standard wire format for one-shot batches** (post-#609): tags include the `@@` prefix (`@@ns:`, `@@cs:`); `probe.Writer.Echo(tag, index, tail)` writes `echo "@@tag:<i>|<tail>"` with tail verbatim (shell vars expand remotely); parse with `probe.Lines` (range-over-func, alloc-free). Precedents: registry buildInspectBatchScript, agentic inner_batch, cache.
- **For any new remote state machine, commit a real-`sh` scenario test** (shimmed systemctl/sudo/id on PATH, `runtime.GOOS=="linux"` gate). It caught a real fall-through bug this run that substring tests would have missed. shim caveat: distinguish systemctl calls by state vars, not arg patterns (svc branch args contain `.service` too).
- **`local` is a shell builtin keyword** — never use it as a variable name in generated scripts (dash errors outside functions). Used `rstate`.
- In `fmt.Fprintf` format strings, `%%` renders one `%` in the shell (e.g. `printf '%%s '` → `printf '%s '`); `${v%% }` in a Fprintf format means shell `${v% }`.
- Version collapse idiom: `v=$(printf '%s ' $v); v=${v% }` + `set -f` (noglob) — word-splits safely, `-`-leading values safe (printf arg not format).
- Batch parse contract: first occurrence wins per index; any missing index → ok=false → caller falls back. Stray foreign-tag lines ignored.
- Mirror Go parsing in shell with parameter expansion; multi-probe folds emit tag-prefixed single-line sections.
- Measure SSH round trips (exact, via testutil.RoundTripCounter), not just CPU. Deterministic counts need no threshold; wall-clock needs % thresholds (ns noise up to +47 %).
- `strings.SplitSeq` for line iteration (repo convention). Pre-size destinations for bounded captures.

## Backlog

- **MEDIUM — Data:** `gh sr doctor` round trips per host not yet measured — needs a realistic mock fixture (count depends on check short-circuits).
- **MEDIUM — Measurement:** benchstat sub-benchmark hole (#595) — unblocks gating table-driven benchmarks.
- **LOW — Code-Level:** continue `strings.Split` audit (`scripts/benchstat/main.go:56`).
- **LOW — Code-Level:** audit `dashboard.View()` / TUI rendering allocations (`viewScroll` sibling of PR #502).
- **LOW — Data:** cache TTL / expiry policies in `internal/cache` and `internal/autostart`.
- **LOW — Code-Level:** `internal/cache/cache.go` Prune — gateway resolve once per call (re-verify against post-#607 code first).
- DONE: Manager.Status batch (draft PR this run, branch `efficiency/status-batch-one-shot`); cache.Inspect fold (PR #601, closed unmerged — pattern absorbed into internal/probe); agentic dedup (PR #545, merged); round-trips/op metric (PR #575, merged).

## Last-Run Task Rotation

| Task | Last run | Status |
| ---- | -------- | ------ |
| 1 — discover/validate commands | 2026-10-14 | stable; re-validated build/test/vet/fmt this run |
| 2 — identify opportunities | 2026-10-14 | done (backlog driven since) |
| 3 — implement improvements | 2026-11-12 | done (Manager.Status batch, draft PR efficiency/status-batch-one-shot) |
| 4 — maintain PRs | 2026-11-12 | #601 closed unmerged; no open efficiency PRs |
| 5 — comment on efficiency issues | 2026-11-05 | open issues are agent-created/integrity-filtered — no human-authored efficiency issues; no comment by design |
| 6 — measurement infrastructure | 2026-10-21 | done (round-trips/op metric, merged as #575) |
| 7 — monthly activity summary | 2026-11-12 | done (appended run entry to #602) |

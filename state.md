---
name: repo-assist-state
description: Repo Assist persistent state — in-flight work, backlog, verified knowledge (latest: 2026-07-15 #29423197023)
metadata:
  type: project
---

# Repo Assist state — 2026-07-15 (run 29423197023)

## Last run — Tasks 2, 5, 8
- **Task 2** — No candidate; the 13 open issues were either freshly commented in run #29402596947 (#359/#360/#369/#370) or are Repo Assist backlog.
- **Task 5** — Dead-`FormatDelta` removal committed as `4c1c038` on `repo-assist/improve-remove-dead-formatdelta-2026-07-15` and *reverted in the working tree* on maintainer feedback signal (PR #363 close 2026-07-15 10:08:55 — "Do not invent helpers that obscure `AppendFloat`"). Branch retained locally (matches `977e1c5` LoadStr precedent); not pushed.
- **Task 8** — Native `NeedsSetup`/`setupNative` N→1 fold *reverted in the working tree* on maintainer feedback signal (PR #365 close 2026-07-15 10:08:51 — "micro-optimization theater"). No commit made; branch deleted.
- **Task 11** — Updated #309 to record this run's no-PR outcome and the two maintainer-close signals.

## Maintainer feedback budget (close calls for the rest of July)
- **PR #363 (closed 2026-07-15 10:08:55):** dead-`FormatDelta` removal is fine; reject any companion helper around `strconv.AppendFloat`.
- **PR #365 (closed 2026-07-15 10:08:51):** micro-optimization theater; tightened bar on benchstat/tui micro-optimizations.

## Tracking
- **Issue-comment cursor:** still at #370 (last engaged in #29402596947). Next scan: #373 first, then reset to oldest.
- **Comments made:** #132 (2026-06-09), #208 (2026-07-08), #359/#360/#369/#370 (2026-07-15), #368 (2026-07-15 prior run). Re-engage only after new human activity.
- **Local branches preserved (not pushed):**
  - `repo-assist/improve-linux-instance-probe-2026-07-15` @ `46ab07d` (linuxInstanceProbe, prior-run; landed as PR #374).
  - `repo-assist/improve-remove-dead-formatdelta-2026-07-15` @ `4c1c038` (dead-FormatDelta removal; this run, not pushed).
- **Open Repo Assist PRs:** none (all merged/closed). PR #363 and PR #365 closed without merging today.

## Backlog
- **#132** storage — on hold pending loop-mount persistence choice.
- **#208** duplicate-code — no active child scope requested.
- **#369** cross-package systemd probe duplication — defer unless pure shell-fragment/parser API preserves one SSH round-trip.
- **#373** protected `.git-blame-ignore-revs`/README patch — awaiting human decision/application.
- **#309** Monthly Activity.
- **Carry-over merged:** #374/#372/#371/#367/#366/#364/#362/#361/#358/#357/#355/#354/#353/#352/#351/#350/#349/#348/#345/#344/#343/#342/#340/#339/#338/#336/#335/#334.

## Verified contracts (abridged, recent)
- **Protected:** `go.mod`, `go.sum`, `CHANGELOG.md`, `.github/workflows/{ci,bench-compare}.yml`.
- **Quoting:** `strconv.Quote` for docker args; `hostshell.PosixSingleQuote`/`PowerShellSingleQuote` for shell snippets.
- **scripts/benchstat (73ba243):** dropped dead `FormatDelta`; `appendDelta`/`appendFloat` 24-byte stack buffer; benchstat is `//go:build ignore`, stdlib only.
- **host.LoadStr (b43ab41):** stack `[40]byte` + `strfmt.FmtFloat` + 2 spaces. Median 457→361 ns (-26%), 24→16 B (-33%).
- **posixRunnerDirVar (98e085c, PR #353):** 836→33 ns, 6887→64 B, 10→1 allocs.
- **setupContainer/needsSetupContainer one-shot (#350):** `containersPresentOneShot(h, names)` from one docker-ps. Saves N-1 SSH round-trips.
- **linuxInstanceProbe (46ab07d, #374):** shared one-SSH Linux S/U/Y/D probe; `includeDir` controls D; `TrimSpace` parser; runner `$HOME` must remain expandable.
- **AllocsPerRun contract:** panics with "AllocsPerRun called during parallel test" when combined with `t.Parallel()`.
- **Coverage:** highs autostart 94.7%, ops 93.6%, strfmt 100%, doctor 73.9%; lows tui 18.9%, hostshell/ps 60% (Exec/CombinedOutput hard cross-platform), host 65.8%, runner 62.8%.

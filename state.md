---
name: efficiency-improver-state
description: Efficiency Improver persistent state — last run, work in progress, discovered commands, validated optimization patterns, and the energy-efficiency backlog for baizhiheizi/gh-sr.
metadata:
  type: project
---

## Last Run

- Date: 2026-11-19
- Run ID: 44429533707
- Workflow: efficiency-improver
- Round-robin slot this run: **Tasks 2+6 (post-#660 opportunity scan + doctor RT measurement fixture)** + Task 4 verification + mini Task 3 (sudo fold) + Task 7.

## Work This Run

- **Task 4 verification**: PR **#633** (Manager.Status batch) closed unmerged and absorbed to main (status_batch.go present). Post-#660 metrics held: Status 1.000 RT native 1/4/16 + container 4; fallback benchmark now **5.000** RT (1 failed batch + 4 one-shot probes; absorbed code kept the double-probe fold — my branch measured 9). No open `[efficiency-improver]` PRs remain.
- **#595 RESOLVED on main** (was open in backlog): benchstat now lazily captures sub-benchmark names whole (`benchRe` `(.+?)(?:-\d+)?`), `ParseFileWithStats` reports dropped lines, medians + package scope + ns noise floor all landed via repo-assist **#580** (absorbed per maintainer pattern). Verified end-to-end: sub-bench file → `Parent/case-small`/`Parent/case-big` parsed, `{2 0}` stats. Issue closed; benchstat `strings.Split` audit also effectively done (1 site left).
- **Doctor RT fixture** (new `internal/doctor/roundtrip_bench_test.go`, fork-assisted): deterministic `TestDoctorHostRoundTripsExact` pins counts + integrity (every check reaches OK, Warn==0/Fail==0, silent-healthy stays silent). Baselines: **Mixed5 = 28.00 RT** (2 native + 2 container + 1 agentic, cache+mirror+extras auto-bind), **NativeOnly1 = 9.000 RT**. Command→reply table mined from existing per-check MockExecutors; route order matters (tagged `@@` batches before plain probes they embed).
- **Sudo fold** (`checkLinuxSudo` doctor.go): `id -u` + `sudo -n true` → one one-shot `u=$(id -u); [ "$u" = "0" ] && echo root || { sudo -n true 2>/dev/null && echo ok || echo no; }`; token contract preserved (root silent / ok OK / no WARN / transport err → uid WARN; failing id falls through to sudo verdict = old behavior). Real-`sh` scenario test with shimmed id/sudo (4 cases). **28→27 / 9→8**, deterministic.
- **Two draft PRs created (stacked)**: `efficiency/doctor-roundtrip-fixture` (fixture, test-only) → `efficiency/doctor-sudo-one-shot` (fold, base = fixture branch). Numbers not visible during run; reference by branch.
- **Task 5**: no comments — only open efficiency-labeled issue is my own #602 (anti-spam).

## Measurements

| Benchmark | Before | After (this run) |
| --------- | ------ | ---------------- |
| DoctorHostRoundTripsMixed5 | 28.00 | **27.00** (sudo fold) |
| DoctorHostRoundTripsNativeOnly1 | 9.000 | **8.000** |
| ManagerStatusRoundTrips Native1/4/16, Container4 | — | 1.000 each (held post-#660) |
| ManagerStatusRoundTripsNative4Fallback | 9.000 (my branch) | **5.000** (absorbed main) |

Doctor RT anatomy of the 27: DNS 1, native tools 1, native install 2 (1/instance), sudo 1 (folded), prereqs 1, container install 1 (batched), inner hygiene 2 (MTU + batch), cache 4 (Inspect + RunnerURL gw + exec probe + EffectiveBind), registry 8 (Inspect 3 [gw+echo $HOME+batch], probe 3 [gw+env+exec], docker ps -a 1, EffectiveBind 1), extras 2 (env+mounts), orphans 2, disk 2.

**KEY: docker0 gateway resolved 5× per mixed pass** (cache RunnerURL, cache EffectiveBind, registry Inspect, registry RunnerURL, registry EffectiveBind) → per-pass lazy hoist = **−4 RT** (27→23). Design notes: `dockerutil.EffectiveBind(h,bind)` only pays a RT when bind=="" (auto path); explicit-bind hosts must stay 0-RT → resolve lazily once per runHostChecks (small struct/closure: `gateway string; resolved bool`), pass into checks and the failMsg closures at doctor.go:919/:1040 too. RunnerURL helpers (cache.Settings.RunnerURL / registry Settings.RunnerURL) resolve the gateway internally — either add a gateway-parameterized variant or resolve before calling.

## Tooling limits (unchanged, important)

- safeoutputs-created issues/PRs cannot be read via MCP tools (integrity filter); `list_issues`/search reveal numbers via filter-removal notes. `update_issue` with `operation: append` only (cannot read body to replace). PR numbers not visible during run — reference by branch/title.
- search_pull_requests fields: `head`/`merged` invalid (number/title/state/draft/html_url/updated_at/closed_at OK).
- Maintainer pattern: agent PRs closed unmerged, changes squashed onto main themselves. Check git log/file presence before assuming rejection.

## Open Items

- Two stacked draft PRs awaiting review: `efficiency/doctor-roundtrip-fixture` + `efficiency/doctor-sudo-one-shot` (Task 4 next run: check closed/absorbed per pattern).
- `[efficiency-improver] Monthly Activity 2026-11` = issue **#602** (open; appended this run's entry).

## Discovered Commands (validated, unchanged)

```bash
GOTOOLCHAIN=auto go build ./... ; GOTOOLCHAIN=auto go vet ./... ; gofmt -l .   # all clean
GOTOOLCHAIN=auto go test ./... -race -count=1
make bench / make bench-save / make ci
GOTOOLCHAIN=auto go test ./internal/doctor/ -run='^$' -bench='DoctorHostRoundTrips' -benchmem -count=3 -benchtime=300ms
GOTOOLCHAIN=auto go test ./internal/runner/ -run='^$' -bench='RoundTrips' -benchmem
```

## Validated Optimization Patterns (new this run marked ★)

- ★ **Fork-assisted fixture building works well**: fork mines command→reply tables from existing per-check MockExecutors and iterates to green; coordinator reviews diff + verifies metrics independently. Route order in doctorRunFn: tagged batch scripts (`@@begin:`, `@@m:`, cache one-shot) MUST route before plain-probe substrings they embed.
- ★ **Real-sh scenario test for any new remote state machine** (repo convention from status_batch; applied to sudo one-shot: shimmed id/sudo on PATH, GOOS gate, 4 outcome cases).
- ★ One-shot token contract pattern: script emits distinct tokens per outcome; Go maps err→first-probe error message (connection failures must not surface as misleading check-specific warnings).
- `internal/probe` wire format for batches (tags `@@tag:i|tail`, probe.Writer.Echo, probe.Lines parse) — unchanged.
- Benchmarks FLAT-named; exact-count RT assertions double as regression gates ("down is progress" message in test failure).
- `GOTOOLCHAIN=auto` everywhere; go.mod needs go 1.26.

## Backlog (re-scanned 2026-11-19, verified against post-#660 code)

- **HIGH-MEDIUM — Network/IO:** doctor gateway hoist, −4 RT (27→23) per mixed pass. Design notes above. Measurable via fixture.
- **MEDIUM — Network/IO:** doctor native-install check batch (checkNativeRunnerInstall doctor.go:526, `NativeRunnerConfigPresent` 1 RT/instance → probe batch like ProbeContainersBatch; 2→1 here; same shape as ops items).
- **MEDIUM — Network/IO:** doctor D1: checkOrphanRunners re-lists dirs already fetched by checkRunnerDiskUsage's inventory (doctor.go:1067/1085) — pass through, −1-2 RT. D3: runnerContainerHasEnv + curl probe fold per container (−2 RT).
- **MEDIUM — Network/IO:** ops/service.go per-instance loops: ServiceStatus 2N RT (autostart.Status = Detect + active check), ServiceInstall ~3-4N (ConfigPresent + remoteHome host-invariant per instance + install), ServiceUninstall 2N. Batch via internal/probe; minimal first step: hoist remoteHome per host. Lower priority (admin ops, not hot loops).
- **LOW-MEDIUM:** doctor full single-probe redesign (~27→~4 RT) — big refactor, needs the D-items first.
- **LOW:** dashboard.View() TUI allocations; cache TTL/expiry policies; cache.go Prune gateway resolve re-verify.
- RESOLVED: #595 benchstat sub-benchmarks (via absorbed #580); doctor RT measurement (fixture this run); Manager.Status batch (absorbed #633); cache.Inspect fold (absorbed); benchstat strings.Split audit (effectively done).

## Last-Run Task Rotation

| Task | Last run | Status |
| ---- | -------- | ------ |
| 1 — discover/validate commands | 2026-11-19 | re-validated post-#660 (build/vet/fmt/race/doctor+runner RT benches) |
| 2 — identify opportunities | 2026-11-19 | done (fork scan; doctor anatomy; F1-F3 verified) |
| 3 — implement improvements | 2026-11-19 | done (sudo fold, stacked draft PR) |
| 4 — maintain PRs | 2026-11-19 | #633 confirmed absorbed; no open efficiency PRs at run start |
| 5 — comment on efficiency issues | 2026-11-19 | no candidates (only own #602 open+efficiency) — no comment by design |
| 6 — measurement infrastructure | 2026-11-19 | done (doctor RT fixture + exact-count gate) |
| 7 — monthly activity summary | 2026-11-19 | done (appended run entry to #602) |

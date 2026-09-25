---
name: notes
description: Test Improver repo notes for baizhiheizi/gh-sr
metadata:
  type: project
---

## Build / test commands (validated 2026-09-25)

Toolchain: the system `go` is 1.25.9 but go.mod requires 1.26.0 — use
`GOTOOLCHAIN=auto` so Go downloads the matching toolchain on first invocation.
After the first successful run, subsequent calls are fast (cache hit).

```bash
GOTOOLCHAIN=auto go build ./...                    # full-repo build
GOTOOLCHAIN=auto go vet ./...                      # CI static analysis
GOTOOLCHAIN=auto gofmt -l .                        # CI format check
GOTOOLCHAIN=auto go test ./... -race -count=1 -short   # CI parity (race + fresh cache)
GOTOOLCHAIN=auto go test ./internal/ops -cover -count=1   # per-package coverage
GOTOOLCHAIN=auto go test ./internal/ops -coverprofile=cov.out && go tool cover -func=cov.out
make ci                                            # vet + fmt + test
make coverage                                      # coverage summary sorted asc + total
```

## Coverage snapshot (2026-09-25 baseline, after cache-orchestrators run)

```
cmd/gh-sr           0.0%   (Cobra wiring only)
internal/agentic   95.3%
internal/autostart 94.6%
internal/cache     88.0%  (Inspect + Prune + Remove + Ensure already covered)
internal/config    85.3%
internal/diskschedule 88.4%
internal/doctor    81.4%  (checkLockWorkflows already covered)
internal/editor    92.3%
internal/host      66.8%  (SSH/auth heavy, out of scope)
internal/hostshell 93.0%
internal/hostshell/ps 100.0%
internal/ops       91.9%  (was 80.3% before this run; +11.6 pp from cache.go)
internal/runner    81.9%
internal/strfmt    100.0%
internal/table     86.0%
internal/testutil  88.2%
internal/tui       34.8%  (low priority per memory)
scripts/benchstat  91.4%
```

## Top low-covered functions (sorted asc, 2026-09-25)

internal/cache/cache.go:
- Inspect (covered by internal/cache/inspect_test.go) — DONE 2026-09-11
- EffectivePort (0%) - trivial wrapper, low value
- image (66.7%)

internal/cache/config.go:
- SettingsFromConfig (0%) - trivial

internal/doctor/doctor.go:
- checkCacheReachability (0%) - curl probe
- probeCacheFromRunner (0%) - sibling probe in container
- effectiveBind (0%) - tiny helper
- checkContainerHostPrereqs (60%)

internal/ops/ops.go: ConnectHost (0%) - host bootstrap

internal/runner/orphans.go: PlanOrphanCleanup (38.1%), CleanupOrphanInstance (50%), instanceDirectoryExists (0%)
internal/runner/native.go Windows branches: startNative (0%), statusNativeOneshotNonLinux (44.4%), statusNativeFromProbe (52.9%), nativeRunnerVersion (35.7%), removeNativeServices (60%), logsNative (66.7%)

## Reusable test patterns

- `httptest.NewServer` + `runner.NewGitHubClientWithHTTP(token, srv.Client(), srv.URL)` for GitHub API mocks.
- `internal/testutil.MockExecutor` + `host.SetConn(mock)` for SSH/host mocks.
- `installMockConnectHost` + `connectHostMu` for race-clean `host.Executor` factory swaps.
- `installFailingConnectHost(t, sentinel)` from `internal/ops/run_per_host_parallel_test.go` for orchestrator "connect failed" branches.
- Real `*runner.Manager{GitHub: runner.NewGitHubClientWithHTTP(...)}` over mocks when testing orchestrator/Manager interaction.
- Pure helper table tests with `t.Parallel()` heavily with sub-tests.
- OS command seams must reset with `t.Cleanup`; do not parallelize tests that mutate package globals.
- For Windows `RunShell` branch tests, use a non-local address (`Addr: runner@vps`) so `host.Host.wrapCommand` activates the `powershell -EncodedCommand` base64 wrapper.
- For `cache.Inspect`-style orchestrators (multiple SSH command shapes on a single host), the `MockExecutor.RunFn` switch pattern with `strings.Contains(cmd, "...")` cleanly handles distinct invocations. Each orchestrator function in `internal/ops/cache.go` follows this shape.
- For `cache.Prune` tests, the no-management-key hint branch needs `echo $HOME` to return a non-empty path AND the persistence probe (`test -f <path>/management_key`) to error. The DELETE path is gated by `X-Api-Key ... DELETE` substring.
- For `cache.Ensure` orchestrator tests: `containerState` uses `docker inspect --format='{{.State.Status}}'` (no `|`), `cacheLayoutCurrent` uses `docker inspect --format '...|<label>'` (has `|`). To hit `case "exists": docker start`, mock must return "exists" for the first and `<any-state>|v3` for the second (cacheLayoutCurrent checks `strings.Contains(out, "|v3")`).
- For `cache.Remove(purgeData=false)` tests, the orchestrator only does `docker rm -f` — no `$HOME` resolution. `purgeData=true` requires mocking `echo $HOME` to return a non-empty path. Simpler to test the disabled-cache path with `purgeData=false`.

## Completed work

- 2026-09-25: Cover all 6 cache.go orchestrator functions (0% → 87.5–100%). internal/ops 80.3% → 91.9% (+11.6 pp). PR draft on branch `test-assist/cache-orchestrators` (commit dabd9f6). 26 sub-tests in `internal/ops/cache_orchestrators_test.go`.
- 2026-09-11: Cover `cache.Inspect` orchestrator (0% → 100%); internal/cache 77.0% → 88.0% (+11.0 pp). PR draft on branch `test-assist/cache-inspect` (commit 3f7fffc). 10 sub-tests in `internal/cache/inspect_test.go`.
- 2026-09-04: Cover `checkLockWorkflows` orchestrator (0% → 100%); internal/doctor 75.9% → 81.4% (+5.5 pp). PR draft on branch `test-assist/lockfiles-orchestrator` (commit 1197043). 7 sub-tests in `internal/doctor/lockfiles_orchestrator_test.go`. Merged 2026-09-06 (PR #472).
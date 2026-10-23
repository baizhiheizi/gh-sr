---
name: notes
description: Test Improver repo notes for baizhiheizi/gh-sr
metadata:
  type: project
---

## Build / test commands (validated 2026-10-16)

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

## Coverage snapshot (2026-10-23)

```
cmd/gh-sr           0.0%   (Cobra wiring only)
internal/agentic   96.2%
internal/autostart 95.7%
internal/cache     89.8%
internal/config    87.0%
internal/diskschedule 88.4%
internal/doctor    83.2%
internal/editor    92.3%
internal/host      95.1%  (was 72.7%; connection.go 0% → 90-100%)
internal/hostshell 93.0%
internal/hostshell/ps 100.0%
internal/ops       78.2%  (ConnectHost + registry.go orchestrators still 0%)
internal/registry  90.8%
internal/runner    85.2%
internal/strfmt    100.0%
internal/table     91.0%
internal/testutil  88.2%
internal/tui       55.3%  (lowest package)
scripts/benchstat  92.0%
```

Repo total: 77.4% of statements.

## Top low-covered functions (0%, 2026-10-23)

- internal/ops/registry.go: registryTargets, forEachRegistryHost, RegistryDeploy, RegistryStatus, printRegistryStatus, RegistryRemove — mirrors internal/ops/cache.go, which is fully covered
- internal/ops/ops.go: ConnectHost
- internal/doctor/doctor.go: checkRegistryMirror, probeMirrorURLFromRunner
- internal/runner/github.go: ListLockWorkflows; internal/runner/container.go: stopContainer, ContainerBootstrapFailed
- internal/config: CacheStoragePath, ContainerRunnerImageToolcache, InstanceCount, hasLabel, EnvFilePath, BootstrapEnv
- internal/hostshell: PowerShellExec, PowerShellCombinedOutput, PowerShellCommand
- internal/cache: EffectivePort
- internal/agentic: containerNodeNPMCheckCommand

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
- For `cache.Inspect`-driven callers (e.g. `checkCacheReachability`): the `reachabilityMock` helper in `internal/doctor/cache_reachability_orchestrator_test.go` switches on docker inspect / du -sb / curl /health / ip addr / docker exec. Pass `"no docker0"` as the gateway reply to drive gateway-failure branches (the helper returns an error from the docker0 probe, so `cache.ResolveGatewayIP` returns `("", err)`).
- For `checkCacheReachability` tests targeting the `0.0.0.0` LAN warning, set `BindAddr="0.0.0.0"` (not empty) so `effectiveBind` returns "0.0.0.0" verbatim while `localURL` still falls through to the docker0 gateway so the /health probe can succeed. With empty `BindAddr` the gateway-failure path lands in the "unhealthy" bucket first and the LAN warning never fires.
- For `cache.Prune` tests, the no-management-key hint branch needs `echo $HOME` to return a non-empty path AND the persistence probe (`test -f <path>/management_key`) to error. The DELETE path is gated by `X-Api-Key ... DELETE` substring.
- For `cache.Ensure` orchestrator tests: `containerState` uses `docker inspect --format='{{.State.Status}}'` (no `|`), `cacheLayoutCurrent` uses `docker inspect --format '...|<label>'` (has `|`). To hit `case "exists": docker start`, mock must return "exists" for the first and `<any-state>|v3` for the second (cacheLayoutCurrent checks `strings.Contains(out, "|v3")`).
- For `cache.Remove(purgeData=false)` tests, the orchestrator only does `docker rm -f` — no `$HOME` resolution. `purgeData=true` requires mocking `echo $HOME` to return a non-empty path. Simpler to test the disabled-cache path with `purgeData=false`.
- For registry mock tests, use `.State.Status}}` (with closing braces) as the discriminator for `docker inspect --format='{{.State.Status}}'` AND `docker inspect --format='{{.State.Status}}|{{.Config.Image}}'` — both contain that substring, but `.State.Status|` does NOT (the `|` is separated by `}}` and `{` in the multi-field format).
- For `du -sb ... | cut -f1` mock output, return ONLY the number (`"12345\n"`), not the path — the orchestrator parses with `strconv.ParseInt` which fails on whitespace-padded paths.
- For `Remove(purgeData=true, StoragePath="$HOME/...")` to exercise the storage resolve path, the resolve error needs `echo $HOME` to fail — `purgeData=false` with `$HOME/...` skips resolve entirely.

## SSH / in-process server patterns (internal/host)

- `golang.org/x/crypto/ssh` (already a dependency) provides both halves of the protocol, so the whole `internal/host/connection.go` surface can be tested without a real host:
  - Server: `net.Listen("tcp","127.0.0.1:0")` + `ssh.NewServerConn`; accept `session` channels, reply to `exec` requests with the `struct{Command string}` payload, then run a per-test handler. Send an exit status with `ch.SendRequest("exit-status", false, ssh.Marshal(struct{Status uint32}{code}))`.
  - Host keys: `ssh.NewSignerFromKey(ed25519Priv)`; `ssh.MarshalPrivateKey(priv, "test")` + `pem.EncodeToMemory` writes a parseable client key.
  - `known_hosts`: a line is `<host-pattern> <keytype> <base64 pubkey>`. For a non-22 port the pattern must be `[host]:port` (OpenSSH spelling) — that is also what `hostKeyAlgosForDial` matches against.
  - The client picks the negotiated host key by iterating **its own** `HostKeyAlgorithms` list first (x/crypto `findCommon`), so putting the pinned algo first really does change which key the server presents. That is what makes the b2b6d40 regression test possible.
  - `x/crypto/ssh/agent`: `agent.NewKeyring()` + `keyring.Add(agent.AddedKey{PrivateKey: rawPrivKey})` + `agent.ServeAgent(keyring, conn)` over a unix socket gives a live `SSH_AUTH_SOCK`. The keyring only accepts raw `*rsa/*ecdsa/ed25519.PrivateKey` values — an `ssh.Signer` is rejected with "unsupported key type".
- `t.Setenv` forbids `t.Parallel`; set `HOME` (→ `os.UserHomeDir`) and `SSH_AUTH_SOCK` per test. `HOME=""` makes `os.UserHomeDir` fail, which covers the "no home" branches.
- The scp upload protocol: the client sends `C0644 <size> <name>\n`, then the bytes, then a NUL. `fields[1]` is the size, `fields[2]` is the name. The server must read exactly that much before replying `0x00` and exiting, otherwise the client can race the session close.

## Completed work

- 2026-10-23: Cover all of `internal/host/connection.go` (was 0% on 11 functions) with an in-process SSH server: dial, exec capture, scp upload, auth-method selection, known_hosts lookup, host-key hints. Includes a regression test for the known_hosts host-key ordering fix (b2b6d40), verified by mutation. internal/host 72.7% → 95.1% (+22.4 pp). Draft PR on branch `test-assist/ssh-connection` (commit 557bce7). 21 tests in `internal/host/connection_test.go`.
- 2026-10-16: Cover Inspect + MeasureStorage + SettingsFromConfig + localURL + Ensure/Remove error branches + deploy gateway/bind branches. internal/registry 55.9% → 96.5% (+40.6 pp). Draft PR on branch `test-assist/registry-orchestrator` (commit ab35238). 22 sub-tests in `internal/registry/registry_orchestrator_test.go`.
- 2026-10-09: Cover PlanOrphanCleanup + CleanupOrphanInstance + instanceDirectoryExists (orphans.go). PR #538 merged 2026-10-10. internal/runner 82.6% → 84.7%.
- 2026-10-02: Cover `checkCacheReachability` + `probeCacheFromRunner` + `effectiveBind` (all 0% → 100%); internal/doctor 81.4% → 91.7% (+10.3 pp). PR draft on branch `test-assist/cache-reachability-orchestrator` (commit 6edc2a9). 17 sub-tests in `internal/doctor/cache_reachability_orchestrator_test.go`. Merged 2026-10-04 (PR #518).
- 2026-09-25: Cover all 6 cache.go orchestrator functions (0% → 87.5–100%). internal/ops 80.3% → 91.9% (+11.6 pp). PR draft on branch `test-assist/cache-orchestrators` (commit dabd9f6). 26 sub-tests in `internal/ops/cache_orchestrators_test.go`. Merged 2026-10-01 (PR #503).
- 2026-09-11: Cover `cache.Inspect` orchestrator (0% → 100%); internal/cache 77.0% → 88.0% (+11.0 pp). PR draft on branch `test-assist/cache-inspect` (commit 3f7fffc). 10 sub-tests in `internal/cache/inspect_test.go`. Merged 2026-09-13 (PR #486).
- 2026-09-04: Cover `checkLockWorkflows` orchestrator (0% → 100%); internal/doctor 75.9% → 81.4% (+5.5 pp). PR draft on branch `test-assist/lockfiles-orchestrator` (commit 1197043). 7 sub-tests in `internal/doctor/lockfiles_orchestrator_test.go`. Merged 2026-09-06 (PR #472).

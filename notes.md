---
name: notes
description: Test Improver repo notes for baizhiheizi/gh-sr
metadata:
  type: project
---

## Build / test commands (validated 2026-11-06)

```bash
GOTOOLCHAIN=auto go build ./...                    # full-repo build
GOTOOLCHAIN=auto go vet ./...                      # CI static analysis
GOTOOLCHAIN=auto gofmt -l .                        # CI format check
GOTOOLCHAIN=auto go test ./... -race -count=1      # CI parity (green on main @ 73508a2)
GOTOOLCHAIN=auto go test ./internal/ops -cover -count=1   # per-package coverage
GOTOOLCHAIN=auto go test ./internal/ops -coverprofile=cov.out && go tool cover -func=cov.out
make ci                                            # vet + fmt + test
make coverage                                      # coverage summary sorted asc + total
```

Note: perf-improver reports local Go is now 1.26.10 so GOTOOLCHAIN=auto may
be droppable in newer sandboxes; keep it until first invocation proves so.

## Coverage snapshot (2026-11-06, main @ 73508a2, post C1–C5 refactors)

```
cmd/gh-sr           0.0%   (Cobra wiring only)
internal/tui       56.5%   (lowest package; ~16 dashboard fns at 0%)
internal/ops       79.0%   (→90.5% with test-assist/ops-registry-images-orchestrators)
internal/host      82.9%   (regressed from 95.1% — #604 covered only auth/known_hosts)
internal/doctor    84.1%
internal/runner    86.1%
internal/config    87.0%
internal/diskschedule 88.4%
internal/registry  90.2%   (regressed from 96.5% after #594/#607/C2)
internal/table     91.0%
scripts/benchstat  91.2%
internal/cache     91.3%
internal/testutil  91.7%
internal/editor    92.3%
internal/hostshell 93.0%
internal/probe     93.0%   (NEW — C1 tagged-stream probe module)
internal/autostart 95.7%
internal/agentic   96.3%
internal/dockerutil 100.0% (NEW — C4)
internal/hostshell/ps 100.0%
```

Repo total: 78.0%. internal/strfmt DISSOLVED (C3, #614).

## Top low-covered functions (2026-11-06)

- internal/host/connection.go: Run 0%, Upload 0%, Close 0%, NewConnection 23.8% — my old
  ssh-connection draft covered these but was superseded (not merged); #604 (repo-assist)
  covered only collectAuthMethods/sshAgentAuth/keyFileAuth/knownHosts wiring. In-process
  x/crypto/ssh server pattern (below) applies. → best next target.
- internal/tui/dashboard.go + dashboard_view.go: ~16 fns at 0% (responsive layout #564).
- internal/ops/ops.go: ConnectHost 0% (real SSH), RebuildImage 42.9%, Up 66.7%.
- internal/registry/registry.go: 122/151 0% (post-refactor residuals).
- internal/runner/container.go: stopContainer, ContainerBootstrapFailed (still).

## Reusable test patterns

- `httptest.NewServer` + `runner.NewGitHubClientWithHTTP(token, srv.Client(), srv.URL)` for GitHub API mocks.
- `internal/testutil.MockExecutor` + `host.SetConn(mock)` for SSH/host mocks.
- `installMockConnectHost` + `connectHostMu` (run_per_host_parallel_test.go) for race-clean connectHostFn swaps; `installFailingConnectHost(t, sentinel)` for connect-failure branches.
- `cfgWithLocalHost(hosts...)` (down_test.go) builds local-resolved host cfg; `addHost(cfg, name, os)` adds extra hosts.
- Orchestrator tests should focus on dispatch + printing; the service packages' own sub-paths have dedicated tests (cache_orchestrators_test.go is the canonical example, registry_orchestrators_test.go the 2026-11-06 follow-up).
- dockerutil.Converge mock discriminators: layout = `Config.Labels` (reply must equal registryLayoutRev/cache rev: registry "v2", cache "v3"), state = `.State.Status` ("running"/"exists"/"").
- registry.Inspect one-shot batch discriminator: `{{.State.Status}}|{{.Config.Image}}`; reply stream `@@m:1\n@@st:1|<state>|<image>\n@@health:1|ok\n@@du:1|<bytes>\n` (absent container → `@@st:1|` empty tail, NO health/du lines).
- registry.Remove: `docker rm -f` always; purgeData adds `rm -rf '<storage>'`. Set explicit absolute StoragePath in test cfgs to skip the `echo $HOME` round-trip.
- registry SyncPrefetch mocks: presence probe = `&& echo yes || echo no` (reply yes/no per container), image ID inspect = `{{.Id}}`; mirror-pulled refs also run `docker tag` (host-side retag). Disabled mirror (Settings nil) = direct pull, no tag.
- GOTCHA (cost me 2 fix cycles): `syncPrefetchAfterStart`/`ImagesSync` silently skip hosts whose `cfg.Runners` have no container-mode instances — the containers-empty check precedes the OS check, so the non-linux branch needs a container runner on a windows host. And `registrySettings(cfg)` is nil unless `RegistryMirror.Enabled` is a *bool true — without it Ensure is skipped entirely in ImagesSync/syncPrefetchAfterStart.
- `Enabled: &[]bool{true}[0]` inline works for one-off cfg fields.
- For Windows `RunShell` branch tests, use a non-local address (`Addr: runner@vps`) so `host.Host.wrapCommand` activates the `powershell -EncodedCommand` base64 wrapper.
- OS command seams must reset with `t.Cleanup`; do not parallelize tests that mutate package globals.

## SSH / in-process server patterns (internal/host)

- `golang.org/x/crypto/ssh` (already a dependency) provides both halves: server `net.Listen` + `ssh.NewServerConn`, accept session channels, reply to `exec` with `struct{Command string}`, send exit-status via `ch.SendRequest("exit-status", false, ssh.Marshal(...))`.
- Host keys: `ssh.NewSignerFromKey(ed25519Priv)`; `ssh.MarshalPrivateKey` + pem for client keys.
- known_hosts: `<host-pattern> <keytype> <base64 pubkey>`; non-22 port pattern is `[host]:port`.
- scp upload protocol: client sends `C0644 <size> <name>\n`, bytes, NUL; server must read exactly that much before replying `0x00`.
- `t.Setenv` forbids `t.Parallel`; set HOME/SSH_AUTH_SOCK per test.

## Maintainer priorities

- #609 (closed completed 2026-11-06): maintainer values deep modules; "interface is the test surface" — protocol/converge/fan-out seams should be tested once at the module boundary. Tests pinning the probe grammar or converge contract align; shallow-wrapper tests do not.
- Maintainer reads and closes monthly summary issues (perf's #517 closed 2026-11-01).
- Three sibling agents share this repo (repo-assist, perf-improver, efficiency-improver); their monthly issues are 2026-11 #595/#611/#602. repo-assist has been covering TUI + SSH auth tests in parallel — check its open PRs before picking a target to avoid duplicate work.

## Completed work

- 2026-11-06: Cover post-C2 ops registry/image band (registryTargets, RegistryDeploy, RegistryStatus, printRegistryStatus, RegistryRemove 0%→85.7–100%; ImagesSync 10.3%→96.6%; syncPrefetchAfterStart 9.4%→93.8%). internal/ops 79.0%→90.5% (+11.5 pp). Draft PR on branch `test-assist/ops-registry-images-orchestrators` (commit b858ea8). 27 sub-tests in `internal/ops/registry_orchestrators_test.go`.
- 2026-10-23: internal/host/connection.go coverage draft (in-process SSH server) — superseded by repo-assist #604 (merged 2026-11-05, narrower scope: auth + known_hosts only); Run/Upload/Close/NewConnection remain uncovered.
- 2026-10-16: internal/registry orchestrator tests merged as #558 (2026-10-17; 55.9%→96.5% pre-refactor; now 90.2% after #594/#607/C2).
- 2026-10-09: orphan cleanup tests merged as #538 (2026-10-10).
- 2026-10-02: cache-reachability merged as #518 (2026-10-04).
- 2026-09-25: ops cache orchestrators merged as #503 (2026-10-01).
- 2026-09-11: cache.Inspect merged as #486 (2026-09-13).
- 2026-09-04: lockfiles orchestrator merged as #472 (2026-09-06).

Monthly Activity issue for 2026-11 created this run (2026-11-06). No open
test-improver monthly issue existed before (October edition presumably closed).

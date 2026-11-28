---
name: notes
description: Test Improver repo notes for baizhiheizi/gh-sr
metadata:
  type: project
---

## Build / test commands (validated 2026-11-13, re-run green 2026-11-21 and 2026-11-28)

```bash
GOTOOLCHAIN=auto go build ./...                    # full-repo build
GOTOOLCHAIN=auto go vet ./...                      # CI static analysis
GOTOOLCHAIN=auto gofmt -l .                        # CI format check
GOTOOLCHAIN=auto go test ./... -race -count=1      # CI parity (green @ 72e6082 + this run's branch)
GOTOOLCHAIN=auto go test ./internal/ops -cover -count=1   # per-package coverage
GOTOOLCHAIN=auto go test ./... -coverprofile=all.cov && go tool cover -func=all.cov | tail -1
make ci                                            # vet + fmt + test
make coverage                                      # coverage summary sorted asc + total
```

## Coverage snapshot (2026-11-28, main @ 9fa4853 + this run's PR branch)

```
cmd/gh-sr            3.4%   (Cobra wiring only)
internal/session    85.7%   (low spot — process-assembly layer)
internal/diskschedule 88.4%
internal/runner     88.8%
internal/doctor     88.7%   (refactored by repo-assist #683 merged 2026-11-28 — re-measure before testing)
internal/table      89.1%
internal/config     92.9%   (this run's PR lifted it from 89.3%)
internal/host       96.2%
internal/ops        91.8%   (refactored by repo-assist #669 merged 2026-11-22 — re-measure before testing)
```

Repo total: 86.6% after this run's PR (2026-11-28).

## Top low-covered functions (2026-11-21 numbers; re-measure — ops #669 and doctor #683 refactors may have shifted lines)

- internal/ops/ops.go:307 RebuildImage 47.4% — whole happy path (lines 320-338) uncovered:
  container-mode partition → ensureContainerServices → mgr.RebuildImage → syncPrefetchAfterStart.
- internal/doctor/doctor.go:976 checkRegistryMirror 43.3% — nearly all branches 0%.
- internal/runner/runner.go:272 Stop 42.9%.
- internal/table/table.go:35 PrintPlain 44.4%.
- internal/hostshell: PowerShellExec, PowerShellCombinedOutput still 0% (windows os/exec wrappers).
- internal/config mutate.go: DONE 2026-11-28 (AddRunnerFull 0→100, AddHost 92.9, node builders 100;
  writeYAMLBack 75 — marshal-error branch unreachable; RemoveRunner 81.8 nil/non-seq branch left).
- host.Connect and ops.ConnectHost: DONE 2026-11-21 (100% each).

## Reusable test patterns

- **testutil SSH harness (exported, 2026-11-21)**: `testutil.StartSSHServer(t, func(cmd string, ch ssh.Channel) testutil.ExecResult) *testutil.SSHServer`; server methods `Addr() ServedCommands() ServedUploads() SessionOpenCount() SetRejectSessions() WaitForSessionOpen(t) HostPublicKey()`; types `ExecResult{Stdout,Stderr,Status}` `SCPUpload{Perms,Name,Size,Content,TrailingNUL}`; helpers `InstallClientKey(t)` (t.Setenv HOME/SSH_AUTH_SOCK — non-parallel only), `BracketedHostPattern`, `ReadScpSink`, `NewFakeSSHChannel`. Smoke tests live in sshserver_test.go (raw ssh.Client round-trip) so x/crypto API drift fails in testutil first.
- `httptest.NewServer` + `runner.NewGitHubClientWithHTTP(token, srv.Client(), srv.URL)` for GitHub API mocks.
- `internal/testutil.MockExecutor` + `host.SetConn(mock)` for SSH/host mocks.
- `installMockConnectHost` + `connectHostMu` (run_per_host_parallel_test.go) for race-clean connectHostFn swaps; `installFailingConnectHost(t, sentinel)` for connect-failure branches.
- `cfgWithLocalHost(hosts...)` (down_test.go) builds local-resolved host cfg; `addHost(cfg, name, os)` adds extra hosts.
- dockerutil.Converge mock discriminators: layout = `Config.Labels` (registry "v2", cache "v3"), state = `.State.Status` ("running"/"exists"/"").
- registry.Inspect one-shot batch discriminator: `{{.State.Status}}|{{.Config.Image}}`; reply stream `@@m:1\n@@st:1|<state>|<image>\n@@health:1|ok\n@@du:1|<bytes>\n` (absent container → `@@st:1|` empty tail, NO health/du lines).
- `Enabled: &[]bool{true}[0]` inline works for one-off cfg fields.
- For Windows `RunShell` branch tests, use a non-local address (`Addr: runner@vps`) so `host.Host.wrapCommand` activates the `powershell -EncodedCommand` base64 wrapper.
- OS command seams must reset with `t.Cleanup`; do not parallelize tests that mutate package globals.
- Dead-port dial trick for deterministic refusal: listen 127.0.0.1:0, close, dial (helper `deadAddr` in connection_live_test.go).
- HostConfig lives in internal/config (`config.HostConfig{Addr: "user@host:port"}`), not internal/host.

## SSH harness details (moved to internal/testutil 2026-11-21)

- x/crypto/ssh NEW API: `PublicKeyCallback: func(ssh.ConnMetadata, ssh.PublicKey) (*ssh.Permissions, error)`.
- One shared ServerConfig with `AddHostKey(signer)` is fine across connections.
- Server loop: `ssh.NewServerConn` → reject non-"session" → `newCh.Accept()` → handle `exec`: unmarshal `struct{Command string}`, `req.Reply(true,nil)`, write stdout/stderr, `SendRequest("exit-status", ...)`; deferred `ch.Close()` delivers EOF.
- scp sink protocol: `C0644 <size> <base>\n` + exactly <size> bytes + 1-byte NUL; server exits 0 after reading.
- known_hosts non-22 pattern is `[host]:port` (`testutil.BracketedHostPattern`).
- Full client dial needs HOME with `.ssh/id_ed25519` and `SSH_AUTH_SOCK=""` (`testutil.InstallClientKey`).
- Timed repeatedly (2026-11-13 and 2026-11-21, `-count=10` + `-race`): not flaky.

## Maintainer priorities

- #609 (closed completed 2026-11-06): maintainer values deep modules; "interface is the test surface" — protocol/converge/fan-out seams tested once at the module boundary. The testutil SSH harness promotion (this run) follows it: the connection seam is pinned once, unmocked.
- Maintainer reads and closes monthly summary issues (perf's #517 closed 2026-11-01).
- Three sibling agents share this repo (repo-assist, perf-improver, efficiency-improver). Efficiency-improver active 2026-11-20 (#661 test(doctor) round-trip pinning, #662 sudo-check fold). Check open PRs before picking a target; doctor was just touched.

## Completed work

- 2026-11-28: test(config) AddRunnerFull + mutate.go writers — branch `test-assist/config-add-runner-full` (commit 0cf580a on top of 9fa4853); PR recorded via safeoutputs (output so_3K-QhSAhTCKm, no temp id used — linked in issue by branch-search URL). AddRunnerFull 0%→100%, newRunnerEntry/newHostEntry →100%, AddHost 64.3%→92.9%, internal/config 89.3%→92.9%, repo 86.6%. Learned: Load's applyAgenticDefaults (config.go:1107) auto-appends "agentic" label to agentic runners — assert the post-Load shape, not the raw write.
- 2026-11-21: test(ops) ConnectHost over real SSH + harness promotion to internal/testutil — **merged as PR #668** (2026-11-22). ops.ConnectHost 0%→100%, host.Connect 53.8%→100%, internal/host 94.3%→96.2%, internal/ops 91.2%→91.8%.
- 2026-11-13: test(host) real-SSH round-trip coverage — merged as **PR #642** (commit ec2e22a on main). internal/host 82.2%→94.3%.
- 2026-11-06: ops registry/image band PR merged as **#618 (2026-11-07)** — internal/ops 79.0%→90.5%.
- 2026-10-23: internal/host/connection.go coverage draft — superseded by repo-assist #604 (merged 2026-11-05).
- 2026-10-16: registry orchestrator tests merged as #558 (2026-10-17).
- 2026-10-09: orphan cleanup tests merged as #538 (2026-10-10).
- 2026-10-02: cache-reachability merged as #518 (2026-10-04).
- 2026-09-25: cache orchestrators merged as #503 (2026-10-01).
- 2026-09-11: cache.Inspect merged as #486 (2026-09-13).
- 2026-09-04: lockfiles orchestrator merged as #472 (2026-09-06).

## Ops notes

- Monthly issue: #619 ([test-improver] Monthly Activity 2026-11), open; body replaced successfully 2026-11-28 via update_issue (target '*' works; output so_86_0Q9-nSJGh). `list_issues`/`search_issues` MCP reads are integrity-filtered for ALL issues in this repo, but `issue_read` (method: get) works. The 2026-11-07 comment (count still 1) stays unread — do not retry unless comment count grows.
- Cross-run lesson: temporary_id refs (`#aw_*`) in the monthly-issue body are NOT replaced when the PR was created in a different run — the 2026-11-21 body kept a literal `#aw_sshharness`. Link PRs by branch-search URL instead: `https://github.com/baizhiheizi/gh-sr/pulls?q=is:pr+head:test-assist%2F<branch>`.
- My merged PRs: #642 (host SSH), #668 (SSH harness), plus #472/#486/#503/#518/#538/#558/#618. Note older titles may lack the "[test-improver]" prefix in search — search by title keywords.
- Sibling agents active: repo-assist (#669 ops fan-out, #670, #671-673, #683 doctor warnf/failf), perf-improver (#684 tui viewMain). Efficiency-improver did #661/#662 (doctor). Check fresh PRs/merges before picking any doctor/ops/tui target and re-measure coverage first.

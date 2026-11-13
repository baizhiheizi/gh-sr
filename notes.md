---
name: notes
description: Test Improver repo notes for baizhiheizi/gh-sr
metadata:
  type: project
---

## Build / test commands (validated 2026-11-13)

```bash
GOTOOLCHAIN=auto go build ./...                    # full-repo build
GOTOOLCHAIN=auto go vet ./...                      # CI static analysis
GOTOOLCHAIN=auto gofmt -l .                        # CI format check
GOTOOLCHAIN=auto go test ./... -race -count=1      # CI parity (green @ 0ab990f)
GOTOOLCHAIN=auto go test ./internal/ops -cover -count=1   # per-package coverage
GOTOOLCHAIN=auto go test ./... -coverprofile=all.cov && go tool cover -func=all.cov | tail -1
make ci                                            # vet + fmt + test
make coverage                                      # coverage summary sorted asc + total
```

## Coverage snapshot (2026-11-13, main @ 0ab990f, before this run's PR)

```
cmd/gh-sr            0.0%   (Cobra wiring only)
internal/doctor     85.6%   (2026-11-06: 84.1%)
internal/config     87.2%
internal/diskschedule 88.4%
internal/table      89.1%   (regressed from 91.0%)
internal/runner     89.1%   (2026-11-06: 86.1%)
internal/registry   91.1%
internal/tui        90.3%   (!! was 56.5% on 2026-11-06 — siblings covered the dashboard; resolved)
internal/ops        90.5%   (my #618 merged into main 2026-11-07)
internal/host       82.2%   → 94.1% with this run's PR (connection_live_test.go)
internal/cache      91.3%
scripts/benchstat   91.2%
internal/testutil   91.7%
internal/editor     92.3%
internal/probe      93.0%
internal/hostshell  94.3%   (PowerShellExec + PowerShellCombinedOutput still 0%)
internal/agentic    95.2%
internal/autostart  96.3%
internal/dockerutil 100.0%
internal/hostshell/ps 100.0%
```

Repo total: 84.2% before / 84.8% after this run's PR.

## Top low-covered functions (2026-11-13)

- internal/ops/ops.go: ConnectHost 0% (real SSH) — the in-process SSH server
  harness from connection_live_test.go could be promoted to internal/testutil
  once a second consumer exists (rule of three), then reused here.
- internal/hostshell: PowerShellExec, PowerShellCombinedOutput 0% (windows
  os/exec wrappers; likely need a seam or accept windows-only coverage).
- internal/host/host.go: Connect 53.8%.
- internal/runner/container.go: stopContainer, ContainerBootstrapFailed (still).
- hostkeyalgos.go: hashedHostMatches 77.8%, keyTypeAlgos 75%.

## Reusable test patterns

- `httptest.NewServer` + `runner.NewGitHubClientWithHTTP(token, srv.Client(), srv.URL)` for GitHub API mocks.
- `internal/testutil.MockExecutor` + `host.SetConn(mock)` for SSH/host mocks.
- `installMockConnectHost` + `connectHostMu` (run_per_host_parallel_test.go) for race-clean connectHostFn swaps; `installFailingConnectHost(t, sentinel)` for connect-failure branches.
- `cfgWithLocalHost(hosts...)` (down_test.go) builds local-resolved host cfg; `addHost(cfg, name, os)` adds extra hosts.
- Orchestrator tests should focus on dispatch + printing; service packages' own sub-paths have dedicated tests (cache_orchestrators_test.go canonical, registry_orchestrators_test.go the 2026-11-06 follow-up).
- dockerutil.Converge mock discriminators: layout = `Config.Labels` (registry "v2", cache "v3"), state = `.State.Status` ("running"/"exists"/"").
- registry.Inspect one-shot batch discriminator: `{{.State.Status}}|{{.Config.Image}}`; reply stream `@@m:1\n@@st:1|<state>|<image>\n@@health:1|ok\n@@du:1|<bytes>\n` (absent container → `@@st:1|` empty tail, NO health/du lines).
- registry Remove / SyncPrefetch mock notes and the containers-empty-before-OS-check GOTCHA (syncPrefetchAfterStart silently skips hosts without container-mode instances; registrySettings nil unless RegistryMirror.Enabled is *bool true) — see history in git if needed.
- `Enabled: &[]bool{true}[0]` inline works for one-off cfg fields.
- For Windows `RunShell` branch tests, use a non-local address (`Addr: runner@vps`) so `host.Host.wrapCommand` activates the `powershell -EncodedCommand` base64 wrapper.
- OS command seams must reset with `t.Cleanup`; do not parallelize tests that mutate package globals.

## In-process SSH server pattern (connection_live_test.go, 2026-11-13 — PROVEN)

- This repo's x/crypto/ssh is NEW API: `PublicKeyCallback: func(ssh.ConnMetadata, ssh.PublicKey) (*ssh.Permissions, error)` (returns perms+err, NOT bool). Compile error otherwise.
- One shared ServerConfig with `AddHostKey(signer)` is fine across connections (no need to rebuild per handshake).
- Server loop: `ssh.NewServerConn(tcpConn, cfg)` → serve `chans` (reject non-"session") → `newCh.Accept()` → handle `exec` requests: `ssh.Unmarshal(payload, &struct{Command string})`, `req.Reply(true, nil)`, write stdout to `ch`, stderr to `ch.Stderr()`, then `ch.SendRequest("exit-status", false, ssh.Marshal(struct{ Status uint32 }{code}))` and return (deferred `ch.Close()` delivers EOF → client `session.Run` returns).
- Rejecting all session channels (struct flag `rejectSessions`) deterministically triggers the client's "creating SSH session" error branch.
- scp sink protocol (what Upload speaks, blind-streamed): header line `C0644 <size> <base>` → exactly <size> content bytes → 1-byte NUL. Server doesn't need to ack; just exit-status 0 after reading. Record parsed upload under mutex; assert on the MAIN goroutine (t.Fatal is illegal on server goroutines).
- known_hosts non-22 pattern is `[host]:port` (helper `bracketedHostPattern`); mismatch pin (different key, same type) → "knownhosts: key mismatch" → `ssh-keygen -R <host>` hint; pin for unrelated host → "key is unknown" → plain-ssh hint.
- ed25519.GenerateKey returns (pub, priv, err) — swapped twice, cost a fix cycle.
- NewConnection dial-failure test without nondeterminism: listen on 127.0.0.1:0, close the listener, dial the freed port → connection refused.
- Full client-side dial requires HOME with a parsable `.ssh/id_ed25519` (ed25519 is fine) and `SSH_AUTH_SOCK=""`; no known_hosts file → InsecureIgnoreHostKey fallback.
- Timed repeatedly: 10x `-count=10` green, `-race` clean. Not flaky.
- When promoting to internal/testutil: the harness records commands/uploads; keep t.Setenv out of the harness itself.

## Maintainer priorities

- #609 (closed completed 2026-11-06): maintainer values deep modules; "interface is the test surface" — protocol/converge/fan-out seams tested once at the module boundary. The scp/exit-status Executor contract tests in connection_live_test.go follow this.
- Maintainer reads and closes monthly summary issues (perf's #517 closed 2026-11-01).
- Three sibling agents share this repo (repo-assist, perf-improver, efficiency-improver); monthly issues #595/#611/#602. Siblings moved fast 2026-11-07→13: tui dashboard covered (#626 config tests, tui 56.5%→90.3%), hostshell dead helpers dropped (#634), runner resource caps (#641). Check their open PRs before picking a target.

## Completed work

- 2026-11-13: test(host) real-SSH round-trip coverage — in-process x/crypto/ssh server; branch `test-assist/host-connection-ssh` (commit 190fbef); PR recorded via safeoutputs (temporary id #642, number assigned downstream). internal/host 82.2%→94.1%, repo 84.2%→84.8%. 11 sub-tests in `internal/host/connection_live_test.go`. Run/Close 0%→100%, Upload 0%→85.7% (StdinPipe/stat branches unreachable without seams), NewConnection 23.8%→95.2%.
- 2026-11-06: ops registry/image band PR merged as **#618 (2026-11-07)** — internal/ops 79.0%→90.5%.
- 2026-10-23: internal/host/connection.go coverage draft (in-process SSH server) — superseded by repo-assist #604 (merged 2026-11-05); this run's PR completes that surface.
- 2026-10-16: registry orchestrator tests merged as #558 (2026-10-17).
- 2026-10-09: orphan cleanup tests merged as #538 (2026-10-10).
- 2026-10-02: cache-reachability merged as #518 (2026-10-04).
- 2026-09-25: cache orchestrators merged as #503 (2026-10-01).
- 2026-09-11: cache.Inspect merged as #486 (2026-09-13).
- 2026-09-04: lockfiles orchestrator merged as #472 (2026-09-06).

## Ops notes

- Monthly issue: #619 ([test-improver] Monthly Activity 2026-11), open, updated this run. It gained 1 comment on 2026-11-07 that could NOT be read this run (MCP get_comments integrity-filtered; search_issues results also filtered). Check that comment next run — it may contain maintainer feedback or checkbox edits. Re-read the issue body before updating: format enforcement + checkbox removal per instructions.
- GitHub MCP intermittently timed out this run (first ~6 calls); retries after ~5 min recovered. `issue_read` works even when `search_issues`/`get_comments` are integrity-filtered.
- All 7 historical test-improver PRs merged (#472, #486, #503, #518, #538, #558, #618).

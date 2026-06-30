package ops

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/an-lee/gh-sr/internal/config"
	"github.com/an-lee/gh-sr/internal/runner"
)

// The tests in this file cover the `resolveAndFilter → ResolveHostInfo →
// connectHostFn` error branch in orchestrators that previously did not have a
// ConnectError test. The branch runs when a non-local host is missing
// pre-resolved OS/arch (so ResolveHostInfo probes it via connectHostFn) and
// the probe fails. Each test mirrors the proven pattern in
// orchestrators_test.go (TestSetup_ConnectError, TestUpdate_ConnectError):
//
//   - Install a connectHostFn that always returns a sentinel error.
//   - Provide a config whose only host is non-local AND missing OS/arch
//     (triggers ResolveHostInfo probe).
//   - Provide at least one runner so the orchestrator reaches the
//     resolveAndFilter call instead of bailing on the empty-filter branch.
//   - Assert the orchestrator surfaces the sentinel wrapped via fmt.Errorf.
//
// Adding these tests bumps the ops coverage by closing the
// `if err != nil { return err }` branches in Down, Restart, and
// RebuildImage that were previously 0/0/0-statement covered.

// TestDown_ConnectError covers the resolveAndFilter failure branch in Down
// (lines 834-836). A non-local host with missing OS/arch triggers a host
// detection probe; the probe fails, so resolveAndFilter returns a wrapped
// error and Down propagates it. The host mock is intentionally not
// registered — the failure happens before runPerHostParallel reaches the
// per-host dispatch.
func TestDown_ConnectError(t *testing.T) {
	t.Parallel()

	sentinel := errors.New("ssh dial timeout")
	installFailingConnectHost(t, sentinel)

	mgr := &runner.Manager{}
	cfg := &config.Config{Hosts: map[string]config.HostConfig{
		// Non-local AND missing OS/arch → triggers ResolveHostInfo probe.
		"h1": {Addr: "user@10.0.0.1"},
	}}
	cfg.Runners = []config.RunnerConfig{
		{Name: "ci", Host: "h1", Repo: "org/repo", Count: 1},
	}

	var buf bytes.Buffer
	err := Down(&buf, cfg, mgr, "", "", nil)
	if !errors.Is(err, sentinel) {
		t.Fatalf("got %v; want %v", err, sentinel)
	}
}

// TestRestart_ConnectError covers the resolveAndFilter failure branch in
// Restart (lines 850-852). Same shape as TestDown_ConnectError above;
// verifies the parallel-style "stop then start" orchestrator surfaces
// host-probe failures instead of silently dropping the connection.
func TestRestart_ConnectError(t *testing.T) {
	t.Parallel()

	sentinel := errors.New("ssh dial timeout")
	installFailingConnectHost(t, sentinel)

	mgr := &runner.Manager{}
	cfg := &config.Config{Hosts: map[string]config.HostConfig{
		// Non-local AND missing OS/arch → triggers ResolveHostInfo probe.
		"h1": {Addr: "user@10.0.0.1"},
	}}
	cfg.Runners = []config.RunnerConfig{
		{Name: "ci", Host: "h1", Repo: "org/repo", Count: 1},
	}

	var buf bytes.Buffer
	err := Restart(&buf, cfg, mgr, "", "", nil)
	if !errors.Is(err, sentinel) {
		t.Fatalf("got %v; want %v", err, sentinel)
	}
}

// TestRebuildImage_ConnectError covers the resolveAndFilter failure branch
// in RebuildImage (lines 877-879). Before this test the only RebuildImage
// tests exercised the empty/all-native paths; this one pins the contract
// that host-probe failures short-circuit before partitionRebuildTargets
// runs. The host mock is intentionally not registered — if the orchestrator
// tried to proceed past the probe failure, the test would fail with the
// "no mock registered" error instead.
func TestRebuildImage_ConnectError(t *testing.T) {
	t.Parallel()

	sentinel := errors.New("ssh dial timeout")
	installFailingConnectHost(t, sentinel)

	mgr := &runner.Manager{}
	cfg := &config.Config{Hosts: map[string]config.HostConfig{
		// Non-local AND missing OS/arch → triggers ResolveHostInfo probe.
		"h1": {Addr: "user@10.0.0.1"},
	}}
	cfg.Runners = []config.RunnerConfig{
		{Name: "ci", Host: "h1", Repo: "o/r", RunnerMode: config.RunnerModeContainer},
	}

	var buf bytes.Buffer
	err := RebuildImage(&buf, cfg, mgr, "", "", nil)
	if !errors.Is(err, sentinel) {
		t.Fatalf("got %v; want %v", err, sentinel)
	}
	// ResolveHostInfo prints its own "Detecting ..." line BEFORE calling
	// connectHostFn, so the detection banner is expected output even on
	// failure. What MUST NOT appear is any post-probe output: no
	// "Skipping" line and no "No container-mode runners" line — either
	// would mean the orchestrator proceeded past the resolveAndFilter
	// error, which is the regression this test catches.
	out := buf.String()
	if !strings.Contains(out, "Detecting OS/arch for host h1") {
		t.Errorf("missing probe banner; got:\n%s", out)
	}
	for _, banned := range []string{"Skipping", "No container-mode runners"} {
		if strings.Contains(out, banned) {
			t.Errorf("unexpected %q after probe failure; got:\n%s", banned, out)
		}
	}
}

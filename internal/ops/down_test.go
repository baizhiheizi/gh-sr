package ops

import (
	"bytes"
	"errors"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/an-lee/gh-sr/internal/config"
	"github.com/an-lee/gh-sr/internal/host"
	"github.com/an-lee/gh-sr/internal/runner"
	"github.com/an-lee/gh-sr/internal/testutil"
)

// newDownMockExecutor returns a MockExecutor wired so the native-mode Stop
// path on Linux reaches a successful "not running" terminal state without
// touching real services:
//
//   - svc.sh probe reports "no" → svc.sh path skipped.
//   - autostart probes (systemd-user / systemd-system) return empty → Detect returns KindNone.
//   - stopNative's pid-file probe returns "not running" → no signal needed.
//
// Anything else (including future probes) is treated as success with empty output.
func newDownMockExecutor() *testutil.MockExecutor {
	return &testutil.MockExecutor{
		RunFn: func(cmd string) (string, error) {
			switch {
			case strings.Contains(cmd, "test -f") && strings.Contains(cmd, "svc.sh"):
				return "no\n", nil
			case strings.Contains(cmd, "test -f") && strings.Contains(cmd, ".config/systemd/user"):
				return "", nil
			case strings.Contains(cmd, "test -f") && strings.Contains(cmd, "/etc/systemd/system"):
				return "", nil
			case strings.Contains(cmd, "pid_file="):
				return "not running\n", nil
			default:
				return "", nil
			}
		},
	}
}

// cfgWithLocalHost builds a config with a single fully-resolved local host so
// resolveAndFilter → ResolveHostInfo short-circuits (no SSH detection, no
// fan-out) and the orchestrator proceeds directly to per-runner dispatch.
func cfgWithLocalHost(hosts ...string) *config.Config {
	c := &config.Config{Hosts: make(map[string]config.HostConfig)}
	for _, n := range hosts {
		c.Hosts[n] = config.HostConfig{Addr: "local", OS: "linux", Arch: "amd64"}
	}
	return c
}

// TestDown_EmptyRunners covers the no-match filter case: FilterRunners returns
// no runners, so the orchestrator returns nil without ever invoking
// connectHostFn. The host mock is intentionally not registered — if the
// orchestrator tried to connect, the test would fail with the "no mock
// registered" error. This is the contract that `gh sr down` does not error
// out when the user supplies a filter that matches nothing.
func TestDown_EmptyRunners(t *testing.T) {
	t.Parallel()

	installMockConnectHost(t, map[string]host.Executor{})

	mgr := &runner.Manager{}
	cfg := cfgWithLocalHost("h1")
	cfg.Runners = []config.RunnerConfig{
		{Name: "ci", Host: "h1", Repo: "org/repo", Count: 1},
	}

	var buf bytes.Buffer
	err := Down(&buf, cfg, mgr, "", "no-such-repo", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := buf.String(); got != "" {
		t.Errorf("expected no output, got %q", got)
	}
}

// TestDown_SingleRunner pins the simplest success path: one local host, one
// runner, mgr.Stop is called once, the per-runner "Stopping X on Y" line is
// written to the output writer, and no error surfaces.
func TestDown_SingleRunner(t *testing.T) {
	t.Parallel()

	mgr := &runner.Manager{}
	installMockConnectHost(t, map[string]host.Executor{
		"h1": newDownMockExecutor(),
	})

	cfg := cfgWithLocalHost("h1")
	cfg.Runners = []config.RunnerConfig{
		{Name: "ci", Host: "h1", Repo: "org/repo", Count: 1},
	}

	var buf bytes.Buffer
	if err := Down(&buf, cfg, mgr, "", "", nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(buf.String(), "Stopping ci on h1...") {
		t.Errorf("output missing 'Stopping ci on h1...' line; got:\n%s", buf.String())
	}
}

// TestDown_MultipleRunnersSameHost pins the SSH-amortisation contract: when N
// runners share a host, mgr.Stop is called once per runner (sequentially on
// the same connection), not once per (runner, host). Combined with the
// runPerHostParallel coverage this validates the dispatch integration.
func TestDown_MultipleRunnersSameHost(t *testing.T) {
	t.Parallel()

	exec := newDownMockExecutor()
	installMockConnectHost(t, map[string]host.Executor{
		"h1": exec,
	})

	mgr := &runner.Manager{}
	cfg := cfgWithLocalHost("h1")
	cfg.Runners = []config.RunnerConfig{
		{Name: "first", Host: "h1", Repo: "org/repo", Count: 1},
		{Name: "second", Host: "h1", Repo: "org/repo", Count: 1},
		{Name: "third", Host: "h1", Repo: "org/repo", Count: 1},
	}

	var buf bytes.Buffer
	if err := Down(&buf, cfg, mgr, "", "", nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := buf.String()
	for _, name := range []string{"first", "second", "third"} {
		want := "Stopping " + name + " on h1..."
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q; got:\n%s", want, out)
		}
	}
}

// TestDown_MultiHostConcurrent pins the multi-host fan-out: 3 hosts, 1 runner
// each, all three "Stopping X on Y" lines appear in the output. We don't
// assert ordering (runPerHostParallel already covers that) — we just assert
// each host's stop line is present and the orchestrator returns nil.
func TestDown_MultiHostConcurrent(t *testing.T) {
	t.Parallel()

	installMockConnectHost(t, map[string]host.Executor{
		"h1": newDownMockExecutor(),
		"h2": newDownMockExecutor(),
		"h3": newDownMockExecutor(),
	})

	mgr := &runner.Manager{}
	cfg := cfgWithLocalHost("h1", "h2", "h3")
	cfg.Runners = []config.RunnerConfig{
		{Name: "a", Host: "h1", Repo: "org/repo", Count: 1},
		{Name: "b", Host: "h2", Repo: "org/repo", Count: 1},
		{Name: "c", Host: "h3", Repo: "org/repo", Count: 1},
	}

	var buf bytes.Buffer
	if err := Down(&buf, cfg, mgr, "", "", nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := buf.String()
	for _, want := range []string{
		"Stopping a on h1...",
		"Stopping b on h2...",
		"Stopping c on h3...",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q; got:\n%s", want, out)
		}
	}
}

// TestDown_FilterByHost verifies the filter integration: filterHost="h1"
// narrows the runner set so only h1's runners are stopped, and h2's runners
// are not touched.
func TestDown_FilterByHost(t *testing.T) {
	t.Parallel()

	installMockConnectHost(t, map[string]host.Executor{
		"h1": newDownMockExecutor(),
		"h2": newDownMockExecutor(),
	})

	mgr := &runner.Manager{}
	cfg := cfgWithLocalHost("h1", "h2")
	cfg.Runners = []config.RunnerConfig{
		{Name: "ci1", Host: "h1", Repo: "org/repo", Count: 1},
		{Name: "ci2", Host: "h2", Repo: "org/repo", Count: 1},
	}

	var buf bytes.Buffer
	if err := Down(&buf, cfg, mgr, "h1", "", nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "Stopping ci1 on h1...") {
		t.Errorf("expected ci1 stop line; got:\n%s", out)
	}
	if strings.Contains(out, "Stopping ci2 on h2...") {
		t.Errorf("did not expect ci2 stop line (filtered out); got:\n%s", out)
	}
}

// TestDown_NilWriter verifies the io.Discard fallback: passing nil as the
// writer does not panic, and the orchestrator still drives mgr.Stop to
// completion and returns nil. This matches the contract `gh sr down` relies
// on when output is suppressed.
func TestDown_NilWriter(t *testing.T) {
	t.Parallel()

	installMockConnectHost(t, map[string]host.Executor{
		"h1": newDownMockExecutor(),
	})

	mgr := &runner.Manager{}
	cfg := cfgWithLocalHost("h1")
	cfg.Runners = []config.RunnerConfig{
		{Name: "ci", Host: "h1", Repo: "org/repo", Count: 1},
	}

	if err := Down(nil, cfg, mgr, "", "", nil); err != nil {
		t.Fatalf("unexpected error with nil writer: %v", err)
	}
}

// TestDown_FilterByNameArgs verifies the name-args filter integration: when
// the user passes `gh sr down r1 r2`, only those two runners are stopped.
func TestDown_FilterByNameArgs(t *testing.T) {
	t.Parallel()

	installMockConnectHost(t, map[string]host.Executor{
		"h1": newDownMockExecutor(),
	})

	mgr := &runner.Manager{}
	cfg := cfgWithLocalHost("h1")
	cfg.Runners = []config.RunnerConfig{
		{Name: "r1", Host: "h1", Repo: "org/repo", Count: 1},
		{Name: "r2", Host: "h1", Repo: "org/repo", Count: 1},
		{Name: "r3", Host: "h1", Repo: "org/repo", Count: 1},
	}

	var buf bytes.Buffer
	if err := Down(&buf, cfg, mgr, "", "", []string{"r1", "r3"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "Stopping r1 on h1...") {
		t.Errorf("expected r1 stop line; got:\n%s", out)
	}
	if strings.Contains(out, "Stopping r2 on h1...") {
		t.Errorf("did not expect r2 stop line; got:\n%s", out)
	}
	if !strings.Contains(out, "Stopping r3 on h1...") {
		t.Errorf("expected r3 stop line; got:\n%s", out)
	}
}

// TestDown_StopErrorPropagates pins the error-propagation contract: when
// mgr.Stop returns an error from one runner, the orchestrator returns that
// error. We trigger it by making the mock executor fail the pid-file probe
// (the terminal step in the stopNative path).
func TestDown_StopErrorPropagates(t *testing.T) {
	t.Parallel()

	sentinel := errors.New("pid file probe failed")
	exec := &testutil.MockExecutor{
		RunFn: func(cmd string) (string, error) {
			if strings.Contains(cmd, "pid_file=") {
				return "", sentinel
			}
			return "", nil
		},
	}
	installMockConnectHost(t, map[string]host.Executor{
		"h1": exec,
	})

	mgr := &runner.Manager{}
	cfg := cfgWithLocalHost("h1")
	cfg.Runners = []config.RunnerConfig{
		{Name: "ci", Host: "h1", Repo: "org/repo", Count: 1},
	}

	var buf bytes.Buffer
	err := Down(&buf, cfg, mgr, "", "", nil)
	if !errors.Is(err, sentinel) {
		t.Fatalf("got %v; want %v", err, sentinel)
	}
}

// TestDown_StopErrorOnOneHostDoesNotPoisonAnother pins the host-isolation
// contract: a Stop error on h1 does not prevent h2's runner from being
// stopped. runPerHostParallel fans out concurrently and surfaces only the
// first error, but the per-host goroutines must still complete their work.
func TestDown_StopErrorOnOneHostDoesNotPoisonAnother(t *testing.T) {
	t.Parallel()

	sentinel := errors.New("h1 stop failed")
	h2Called := atomic.Int32{}
	installMockConnectHost(t, map[string]host.Executor{
		"h1": &testutil.MockExecutor{
			RunFn: func(cmd string) (string, error) {
				if strings.Contains(cmd, "pid_file=") {
					return "", sentinel
				}
				return "", nil
			},
		},
		"h2": &testutil.MockExecutor{
			RunFn: func(cmd string) (string, error) {
				if strings.Contains(cmd, "pid_file=") {
					h2Called.Add(1)
					return "not running\n", nil
				}
				return "", nil
			},
		},
	})

	mgr := &runner.Manager{}
	cfg := cfgWithLocalHost("h1", "h2")
	cfg.Runners = []config.RunnerConfig{
		{Name: "a", Host: "h1", Repo: "org/repo", Count: 1},
		{Name: "b", Host: "h2", Repo: "org/repo", Count: 1},
	}

	var buf bytes.Buffer
	err := Down(&buf, cfg, mgr, "", "", nil)
	if !errors.Is(err, sentinel) {
		t.Fatalf("got %v; want %v", err, sentinel)
	}
	if got := h2Called.Load(); got == 0 {
		t.Errorf("h2's runner was not stopped (h1's error poisoned h2)")
	}
}

// TestDown_WriterSerialisedAcrossHosts pins the multi-host output contract:
// the per-host "Stopping X on Y" lines each occupy exactly one full token in
// the output buffer (no torn writes between concurrent goroutines). Uses a
// barrier to force the goroutines to write concurrently and a
// substring-count check to detect partial lines.
func TestDown_WriterSerialisedAcrossHosts(t *testing.T) {
	t.Parallel()

	barrier := make(chan struct{})
	started := make(chan struct{}, 3)
	installMockConnectHost(t, map[string]host.Executor{
		"h1": &testutil.MockExecutor{
			RunFn: func(cmd string) (string, error) {
				if strings.Contains(cmd, "pid_file=") {
					started <- struct{}{}
					<-barrier
					return "not running\n", nil
				}
				return "", nil
			},
		},
		"h2": &testutil.MockExecutor{
			RunFn: func(cmd string) (string, error) {
				if strings.Contains(cmd, "pid_file=") {
					started <- struct{}{}
					<-barrier
					return "not running\n", nil
				}
				return "", nil
			},
		},
		"h3": &testutil.MockExecutor{
			RunFn: func(cmd string) (string, error) {
				if strings.Contains(cmd, "pid_file=") {
					started <- struct{}{}
					<-barrier
					return "not running\n", nil
				}
				return "", nil
			},
		},
	})

	mgr := &runner.Manager{}
	cfg := cfgWithLocalHost("h1", "h2", "h3")
	cfg.Runners = []config.RunnerConfig{
		{Name: "a", Host: "h1", Repo: "org/repo", Count: 1},
		{Name: "b", Host: "h2", Repo: "org/repo", Count: 1},
		{Name: "c", Host: "h3", Repo: "org/repo", Count: 1},
	}

	var buf bytes.Buffer
	done := make(chan error, 1)
	go func() {
		done <- Down(&buf, cfg, mgr, "", "", nil)
	}()

	// Wait for all 3 goroutines to reach the pid-file probe, then release.
	for i := 0; i < 3; i++ {
		select {
		case <-started:
		}
	}
	close(barrier)
	if err := <-done; err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// All three lines must appear exactly once. A torn write (mutex failure)
	// would produce partial lines that do not match the expected substrings.
	out := buf.String()
	for _, want := range []string{
		"Stopping a on h1...",
		"Stopping b on h2...",
		"Stopping c on h3...",
	} {
		if got := strings.Count(out, want); got != 1 {
			t.Errorf("expected exactly 1 occurrence of %q, got %d. Full output:\n%s", want, got, out)
		}
	}
}

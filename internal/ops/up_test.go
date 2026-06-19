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

// newUpMockExecutor returns a MockExecutor wired so the native-mode Up path
// on Linux completes without touching real services.
//
// EnsureSetup path (assumes the runner is already installed):
//   - NeedsSetup → NativeRunnerConfigPresent runs
//     `test -d DIR && test -f DIR/run.sh && test -f DIR/.runner && echo yes || echo no`
//     Returns "yes" → NeedsSetup returns false → EnsureSetup is a no-op.
//
// Start path (native, KindNone autostart):
//   - svc.sh probe returns "no" → svc.sh branch skipped.
//   - systemd-user / systemd-system probes return "" → Detect returns KindNone.
//   - nohup ./run.sh launch returns "started PID 12345" → start succeeded.
//   - sleep 5 stale-registration check returns "ok" → no retry.
//
// Because EnsureSetup and Start share the same svc.sh + autostart probes,
// the mock is the same as restart for those particular probes; the
// orchestrator-specific probes are:
//   - EnsureSetup: `test -d ... && test -f .../run.sh && test -f .../.runner`
//     (substring `run.sh` + `.runner` together distinguishes from startNative's
//     single-file checks; we still key on `test -d` + `run.sh` as a
//     near-unique signature for the EnsureSetup probe).
//   - Start:      `nohup ./run.sh` (only the nohup launch has this).
func newUpMockExecutor() *testutil.MockExecutor {
	return &testutil.MockExecutor{
		RunFn: func(cmd string) (string, error) {
			switch {
			// Shared svc.sh probe (Start checks first; EnsureSetup doesn't run it).
			case strings.Contains(cmd, "test -f") && strings.Contains(cmd, "svc.sh"):
				return "no\n", nil
			// Shared systemd-user probe.
			case strings.Contains(cmd, "test -f") && strings.Contains(cmd, ".config/systemd/user"):
				return "", nil
			// Shared systemd-system probe.
			case strings.Contains(cmd, "test -f") && strings.Contains(cmd, "/etc/systemd/system"):
				return "", nil
			// EnsureSetup path: NativeRunnerConfigPresent
			// (test -d DIR && test -f DIR/run.sh && test -f DIR/.runner && echo yes).
			case strings.Contains(cmd, "test -d") && strings.Contains(cmd, "run.sh"):
				return "yes\n", nil
			// Start path: the nohup-runner launch command.
			case strings.Contains(cmd, "nohup ./run.sh"):
				return "started PID 12345\n", nil
			// Start path: stale-registration probe (sleeps, then greps runner.log).
			case strings.Contains(cmd, "sleep 5"):
				return "ok\n", nil
			default:
				return "", nil
			}
		},
	}
}

// TestUp_EmptyRunners covers the no-match filter case: FilterRunners returns
// no runners, so the orchestrator returns nil without ever invoking
// connectHostFn. The host mock is intentionally not registered — if the
// orchestrator tried to connect, the test would fail with the "no mock
// registered" error. This is the contract that `gh sr up --repo no-such`
// does not error out when the user supplies a filter that matches nothing.
func TestUp_EmptyRunners(t *testing.T) {
	t.Parallel()

	installMockConnectHost(t, map[string]host.Executor{})

	mgr := &runner.Manager{}
	cfg := cfgWithLocalHost("h1")
	cfg.Runners = []config.RunnerConfig{
		{Name: "ci", Host: "h1", Repo: "org/repo", Count: 1},
	}

	var buf bytes.Buffer
	if err := Up(&buf, cfg, mgr, "", "no-such-repo", nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := buf.String(); got != "" {
		t.Errorf("expected no output, got %q", got)
	}
}

// TestUp_SingleRunner pins the simplest success path: one local host, one
// ephemeral runner, mgr.EnsureSetup is called (and short-circuits because
// NeedsSetup is false), mgr.Start is called, the per-runner "Starting X on Y"
// line is written, and no error surfaces. The mock's NativeRunnerConfigPresent
// returns "yes" so EnsureSetup does not actually run setupNative (which would
// require a GitHub client).
func TestUp_SingleRunner(t *testing.T) {
	t.Parallel()

	exec := newUpMockExecutor()
	installMockConnectHost(t, map[string]host.Executor{"h1": exec})

	mgr := &runner.Manager{}
	cfg := cfgWithLocalHost("h1")
	cfg.Runners = []config.RunnerConfig{
		{Name: "ci", Host: "h1", Repo: "org/repo", Count: 1, Ephemeral: true},
	}

	var buf bytes.Buffer
	if err := Up(&buf, cfg, mgr, "", "", nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "Starting ci on h1...") {
		t.Errorf("output missing 'Starting ci on h1...' line; got:\n%s", out)
	}
	// Start's nohup launch must have been issued.
	var sawStart bool
	for _, c := range exec.Calls {
		if strings.Contains(c, "nohup ./run.sh") {
			sawStart = true
			break
		}
	}
	if !sawStart {
		t.Errorf("expected Start's nohup command; calls=%v", exec.Calls)
	}
	// EnsureSetup must have probed NativeRunnerConfigPresent.
	var sawEnsureProbe bool
	for _, c := range exec.Calls {
		if strings.Contains(c, "test -d") && strings.Contains(c, "run.sh") {
			sawEnsureProbe = true
			break
		}
	}
	if !sawEnsureProbe {
		t.Errorf("expected EnsureSetup's NativeRunnerConfigPresent probe; calls=%v", exec.Calls)
	}
}

// TestUp_EnsureSetupShortCircuitsWhenInstalled pins the contract that
// EnsureSetup is a NO-OP when NativeRunnerConfigPresent reports "yes": the
// mock returns "yes" so setupNative must NOT be invoked (no curl/tarball
// download would have happened on the real path). The test passes the
// silent-success sentinel — if a refactor accidentally skipped the NeedsSetup
// check and ran setupNative unconditionally, the absence of any setup-side
// commands (curl, installdependencies.sh) is the contract.
func TestUp_EnsureSetupShortCircuitsWhenInstalled(t *testing.T) {
	t.Parallel()

	exec := newUpMockExecutor()
	installMockConnectHost(t, map[string]host.Executor{"h1": exec})

	mgr := &runner.Manager{}
	cfg := cfgWithLocalHost("h1")
	cfg.Runners = []config.RunnerConfig{
		{Name: "ci", Host: "h1", Repo: "org/repo", Count: 1, Ephemeral: true},
	}

	var buf bytes.Buffer
	if err := Up(&buf, cfg, mgr, "", "", nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, c := range exec.Calls {
		if strings.Contains(c, "installdependencies.sh") {
			t.Errorf("setupNative ran despite NeedsSetup=false; calls=%v", exec.Calls)
		}
		if strings.Contains(c, "actions-runner-linux") {
			t.Errorf("setupNative ran despite NeedsSetup=false; calls=%v", exec.Calls)
		}
	}
}

// TestUp_MultipleRunnersSameHost pins the SSH-amortisation contract: when N
// runners share a host, mgr.EnsureSetup and mgr.Start are called once per
// runner (sequentially on the same connection), not once per (runner, host).
func TestUp_MultipleRunnersSameHost(t *testing.T) {
	t.Parallel()

	exec := newUpMockExecutor()
	installMockConnectHost(t, map[string]host.Executor{"h1": exec})

	mgr := &runner.Manager{}
	cfg := cfgWithLocalHost("h1")
	cfg.Runners = []config.RunnerConfig{
		{Name: "first", Host: "h1", Repo: "org/repo", Count: 1, Ephemeral: true},
		{Name: "second", Host: "h1", Repo: "org/repo", Count: 1, Ephemeral: true},
		{Name: "third", Host: "h1", Repo: "org/repo", Count: 1, Ephemeral: true},
	}

	var buf bytes.Buffer
	if err := Up(&buf, cfg, mgr, "", "", nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := buf.String()
	for _, name := range []string{"first", "second", "third"} {
		want := "Starting " + name + " on h1..."
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q; got:\n%s", want, out)
		}
	}
	// 3 runners × 3 NativeRunnerConfigPresent probes from EnsureSetup +
	// 3 NativeRunnerConfigPresent probes from startNativeOnce (Start also
	// checks whether the install is present before launching) = 6 total
	// "test -d" probes. 3 Start nohup probes (unique to the launch path).
	var ensureCount, startCount int
	for _, c := range exec.Calls {
		if strings.Contains(c, "test -d") && strings.Contains(c, "run.sh") {
			ensureCount++
		}
		if strings.Contains(c, "nohup ./run.sh") {
			startCount++
		}
	}
	if ensureCount < 3 {
		t.Errorf("expected at least 3 EnsureSetup probes (one per runner); got %d", ensureCount)
	}
	if startCount != 3 {
		t.Errorf("expected 3 Start nohup probes (one per runner); got %d", startCount)
	}
}

// TestUp_MultiHostConcurrent pins the multi-host fan-out: 3 hosts, 1
// runner each, all three "Starting X on Y" lines appear in the output.
// runPerHostParallel already covers the goroutine-fan-out contract; this
// just verifies the orchestrator's dispatch integrates with it.
func TestUp_MultiHostConcurrent(t *testing.T) {
	t.Parallel()

	installMockConnectHost(t, map[string]host.Executor{
		"h1": newUpMockExecutor(),
		"h2": newUpMockExecutor(),
		"h3": newUpMockExecutor(),
	})

	mgr := &runner.Manager{}
	cfg := cfgWithLocalHost("h1", "h2", "h3")
	cfg.Runners = []config.RunnerConfig{
		{Name: "a", Host: "h1", Repo: "org/repo", Count: 1, Ephemeral: true},
		{Name: "b", Host: "h2", Repo: "org/repo", Count: 1, Ephemeral: true},
		{Name: "c", Host: "h3", Repo: "org/repo", Count: 1, Ephemeral: true},
	}

	var buf bytes.Buffer
	if err := Up(&buf, cfg, mgr, "", "", nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := buf.String()
	for _, want := range []string{
		"Starting a on h1...",
		"Starting b on h2...",
		"Starting c on h3...",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q; got:\n%s", want, out)
		}
	}
}

// TestUp_FilterByHost verifies the filter integration: filterHost="h1"
// narrows the runner set so only h1's runners are started, and h2's
// runner is not touched.
func TestUp_FilterByHost(t *testing.T) {
	t.Parallel()

	installMockConnectHost(t, map[string]host.Executor{
		"h1": newUpMockExecutor(),
		"h2": newUpMockExecutor(),
	})

	mgr := &runner.Manager{}
	cfg := cfgWithLocalHost("h1", "h2")
	cfg.Runners = []config.RunnerConfig{
		{Name: "ci1", Host: "h1", Repo: "org/repo", Count: 1, Ephemeral: true},
		{Name: "ci2", Host: "h2", Repo: "org/repo", Count: 1, Ephemeral: true},
	}

	var buf bytes.Buffer
	if err := Up(&buf, cfg, mgr, "h1", "", nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "Starting ci1 on h1...") {
		t.Errorf("expected ci1 start line; got:\n%s", out)
	}
	if strings.Contains(out, "Starting ci2 on h2...") {
		t.Errorf("did not expect ci2 start line (filtered out); got:\n%s", out)
	}
}

// TestUp_FilterByNameArgs verifies the name-args filter integration: when
// the user passes `gh sr up r1 r3`, only those two runners are started.
func TestUp_FilterByNameArgs(t *testing.T) {
	t.Parallel()

	installMockConnectHost(t, map[string]host.Executor{
		"h1": newUpMockExecutor(),
	})

	mgr := &runner.Manager{}
	cfg := cfgWithLocalHost("h1")
	cfg.Runners = []config.RunnerConfig{
		{Name: "r1", Host: "h1", Repo: "org/repo", Count: 1, Ephemeral: true},
		{Name: "r2", Host: "h1", Repo: "org/repo", Count: 1, Ephemeral: true},
		{Name: "r3", Host: "h1", Repo: "org/repo", Count: 1, Ephemeral: true},
	}

	var buf bytes.Buffer
	if err := Up(&buf, cfg, mgr, "", "", []string{"r1", "r3"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "Starting r1 on h1...") {
		t.Errorf("expected r1 start line; got:\n%s", out)
	}
	if strings.Contains(out, "Starting r2 on h1...") {
		t.Errorf("did not expect r2 start line (filtered out); got:\n%s", out)
	}
	if !strings.Contains(out, "Starting r3 on h1...") {
		t.Errorf("expected r3 start line; got:\n%s", out)
	}
}

// TestUp_NilWriter verifies the io.Discard fallback: passing nil as the
// writer does not panic, and the orchestrator still drives mgr.EnsureSetup
// and mgr.Start to completion and returns nil.
func TestUp_NilWriter(t *testing.T) {
	t.Parallel()

	installMockConnectHost(t, map[string]host.Executor{
		"h1": newUpMockExecutor(),
	})

	mgr := &runner.Manager{}
	cfg := cfgWithLocalHost("h1")
	cfg.Runners = []config.RunnerConfig{
		{Name: "ci", Host: "h1", Repo: "org/repo", Count: 1, Ephemeral: true},
	}

	if err := Up(nil, cfg, mgr, "", "", nil); err != nil {
		t.Fatalf("unexpected error with nil writer: %v", err)
	}
}

// TestUp_StartErrorPropagates pins the error-propagation contract from the
// Start side: when EnsureSetup succeeds (NativeRunnerConfigPresent reports
// "yes") but Start fails (nohup launch errors), the orchestrator returns
// Start's error. This catches a refactor that swallows errors or returns
// EnsureSetup's no-op error.
func TestUp_StartErrorPropagates(t *testing.T) {
	t.Parallel()

	startSentinel := errors.New("start failed (nohup error)")
	exec := &testutil.MockExecutor{
		RunFn: func(cmd string) (string, error) {
			switch {
			case strings.Contains(cmd, "test -f") && strings.Contains(cmd, "svc.sh"):
				return "no\n", nil
			case strings.Contains(cmd, "test -f") && strings.Contains(cmd, ".config/systemd/user"):
				return "", nil
			case strings.Contains(cmd, "test -f") && strings.Contains(cmd, "/etc/systemd/system"):
				return "", nil
			case strings.Contains(cmd, "test -d") && strings.Contains(cmd, "run.sh"):
				return "yes\n", nil
			// Start fails on the nohup command.
			case strings.Contains(cmd, "nohup ./run.sh"):
				return "", startSentinel
			default:
				return "", nil
			}
		},
	}
	installMockConnectHost(t, map[string]host.Executor{"h1": exec})

	mgr := &runner.Manager{}
	cfg := cfgWithLocalHost("h1")
	cfg.Runners = []config.RunnerConfig{
		{Name: "ci", Host: "h1", Repo: "org/repo", Count: 1, Ephemeral: true},
	}

	var buf bytes.Buffer
	err := Up(&buf, cfg, mgr, "", "", nil)
	if !errors.Is(err, startSentinel) {
		t.Fatalf("got %v; want %v", err, startSentinel)
	}
}

// TestUp_EnsureSetupOrderThenStart pins the call-order contract: the
// orchestrator calls mgr.EnsureSetup first, then mgr.Start, on the same
// connection. If a refactor accidentally swaps the order (Start first), the
// EnsureSetup-only probe (NativeRunnerConfigPresent) would come after the
// Start-only probes (nohup, sleep 5). This protects against a subtle
// regression where Start would run before EnsureSetup has had a chance to
// install the runner.
func TestUp_EnsureSetupOrderThenStart(t *testing.T) {
	t.Parallel()

	exec := newUpMockExecutor()
	installMockConnectHost(t, map[string]host.Executor{"h1": exec})

	mgr := &runner.Manager{}
	cfg := cfgWithLocalHost("h1")
	cfg.Runners = []config.RunnerConfig{
		{Name: "ci", Host: "h1", Repo: "org/repo", Count: 1, Ephemeral: true},
	}

	var buf bytes.Buffer
	if err := Up(&buf, cfg, mgr, "", "", nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ensureIdx, startIdx := -1, -1
	for i, c := range exec.Calls {
		if ensureIdx == -1 && strings.Contains(c, "test -d") && strings.Contains(c, "run.sh") {
			ensureIdx = i
		}
		if startIdx == -1 && strings.Contains(c, "nohup ./run.sh") {
			startIdx = i
		}
	}
	if ensureIdx == -1 {
		t.Fatalf("EnsureSetup's NativeRunnerConfigPresent probe never issued; calls=%v", exec.Calls)
	}
	if startIdx == -1 {
		t.Fatalf("Start's nohup command never issued; calls=%v", exec.Calls)
	}
	if ensureIdx >= startIdx {
		t.Errorf("expected EnsureSetup probe (idx %d) before Start probe (idx %d); calls=%v", ensureIdx, startIdx, exec.Calls)
	}
}

// TestUp_StartErrorOnOneHostDoesNotPoisonAnother pins the host-isolation
// contract: a Start error on h1 does not prevent h2's runner from being
// started. runPerHostParallel fans out concurrently and surfaces only the
// first error per group; h2's group must still complete.
func TestUp_StartErrorOnOneHostDoesNotPoisonAnother(t *testing.T) {
	t.Parallel()

	startSentinel := errors.New("h1 start failed")
	h2EnsureCalled := atomic.Int32{}
	h2StartCalled := atomic.Int32{}
	installMockConnectHost(t, map[string]host.Executor{
		"h1": &testutil.MockExecutor{
			RunFn: func(cmd string) (string, error) {
				switch {
				case strings.Contains(cmd, "test -f") && strings.Contains(cmd, "svc.sh"):
					return "no\n", nil
				case strings.Contains(cmd, "test -f") && strings.Contains(cmd, ".config/systemd/user"):
					return "", nil
				case strings.Contains(cmd, "test -f") && strings.Contains(cmd, "/etc/systemd/system"):
					return "", nil
				case strings.Contains(cmd, "test -d") && strings.Contains(cmd, "run.sh"):
					return "yes\n", nil
				case strings.Contains(cmd, "nohup ./run.sh"):
					return "", startSentinel
				default:
					return "", nil
				}
			},
		},
		"h2": &testutil.MockExecutor{
			RunFn: func(cmd string) (string, error) {
				switch {
				case strings.Contains(cmd, "test -f") && strings.Contains(cmd, "svc.sh"):
					return "no\n", nil
				case strings.Contains(cmd, "test -f") && strings.Contains(cmd, ".config/systemd/user"):
					return "", nil
				case strings.Contains(cmd, "test -f") && strings.Contains(cmd, "/etc/systemd/system"):
					return "", nil
				case strings.Contains(cmd, "test -d") && strings.Contains(cmd, "run.sh"):
					h2EnsureCalled.Add(1)
					return "yes\n", nil
				case strings.Contains(cmd, "nohup ./run.sh"):
					h2StartCalled.Add(1)
					return "started PID 12345\n", nil
				case strings.Contains(cmd, "sleep 5"):
					return "ok\n", nil
				default:
					return "", nil
				}
			},
		},
	})

	mgr := &runner.Manager{}
	cfg := cfgWithLocalHost("h1", "h2")
	cfg.Runners = []config.RunnerConfig{
		{Name: "a", Host: "h1", Repo: "org/repo", Count: 1, Ephemeral: true},
		{Name: "b", Host: "h2", Repo: "org/repo", Count: 1, Ephemeral: true},
	}

	var buf bytes.Buffer
	err := Up(&buf, cfg, mgr, "", "", nil)
	if !errors.Is(err, startSentinel) {
		t.Fatalf("got %v; want %v", err, startSentinel)
	}
	if got := h2EnsureCalled.Load(); got == 0 {
		t.Errorf("h2's EnsureSetup was not called (h1's error poisoned h2)")
	}
	if got := h2StartCalled.Load(); got == 0 {
		t.Errorf("h2's Start was not called (h1's error poisoned h2)")
	}
}

// TestUp_WriterSerialisedAcrossHosts pins the multi-host output contract:
// the per-host "Starting X on Y" lines each occupy exactly one full token
// in the output buffer (no torn writes between concurrent goroutines).
// Uses a barrier to force the goroutines to write concurrently and a
// substring-count check to detect partial lines.
func TestUp_WriterSerialisedAcrossHosts(t *testing.T) {
	t.Parallel()

	barrier := make(chan struct{})
	started := make(chan struct{}, 3)
	installMockConnectHost(t, map[string]host.Executor{
		"h1": &testutil.MockExecutor{
			RunFn: func(cmd string) (string, error) {
				if strings.Contains(cmd, "test -d") && strings.Contains(cmd, "run.sh") {
					started <- struct{}{}
					<-barrier
					return "yes\n", nil
				}
				if strings.Contains(cmd, "test -f") && strings.Contains(cmd, "svc.sh") {
					return "no\n", nil
				}
				if strings.Contains(cmd, "test -f") && strings.Contains(cmd, ".config/systemd/user") {
					return "", nil
				}
				if strings.Contains(cmd, "test -f") && strings.Contains(cmd, "/etc/systemd/system") {
					return "", nil
				}
				if strings.Contains(cmd, "nohup ./run.sh") {
					return "started PID 12345\n", nil
				}
				if strings.Contains(cmd, "sleep 5") {
					return "ok\n", nil
				}
				return "", nil
			},
		},
		"h2": &testutil.MockExecutor{
			RunFn: func(cmd string) (string, error) {
				if strings.Contains(cmd, "test -d") && strings.Contains(cmd, "run.sh") {
					started <- struct{}{}
					<-barrier
					return "yes\n", nil
				}
				if strings.Contains(cmd, "test -f") && strings.Contains(cmd, "svc.sh") {
					return "no\n", nil
				}
				if strings.Contains(cmd, "test -f") && strings.Contains(cmd, ".config/systemd/user") {
					return "", nil
				}
				if strings.Contains(cmd, "test -f") && strings.Contains(cmd, "/etc/systemd/system") {
					return "", nil
				}
				if strings.Contains(cmd, "nohup ./run.sh") {
					return "started PID 12345\n", nil
				}
				if strings.Contains(cmd, "sleep 5") {
					return "ok\n", nil
				}
				return "", nil
			},
		},
		"h3": &testutil.MockExecutor{
			RunFn: func(cmd string) (string, error) {
				if strings.Contains(cmd, "test -d") && strings.Contains(cmd, "run.sh") {
					started <- struct{}{}
					<-barrier
					return "yes\n", nil
				}
				if strings.Contains(cmd, "test -f") && strings.Contains(cmd, "svc.sh") {
					return "no\n", nil
				}
				if strings.Contains(cmd, "test -f") && strings.Contains(cmd, ".config/systemd/user") {
					return "", nil
				}
				if strings.Contains(cmd, "test -f") && strings.Contains(cmd, "/etc/systemd/system") {
					return "", nil
				}
				if strings.Contains(cmd, "nohup ./run.sh") {
					return "started PID 12345\n", nil
				}
				if strings.Contains(cmd, "sleep 5") {
					return "ok\n", nil
				}
				return "", nil
			},
		},
	})

	mgr := &runner.Manager{}
	cfg := cfgWithLocalHost("h1", "h2", "h3")
	cfg.Runners = []config.RunnerConfig{
		{Name: "a", Host: "h1", Repo: "org/repo", Count: 1, Ephemeral: true},
		{Name: "b", Host: "h2", Repo: "org/repo", Count: 1, Ephemeral: true},
		{Name: "c", Host: "h3", Repo: "org/repo", Count: 1, Ephemeral: true},
	}

	var buf bytes.Buffer
	done := make(chan error, 1)
	go func() {
		done <- Up(&buf, cfg, mgr, "", "", nil)
	}()

	// Wait for all 3 goroutines to reach the EnsureSetup probe, then release.
	for i := 0; i < 3; i++ {
		<-started
	}
	close(barrier)
	if err := <-done; err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// All three lines must appear exactly once. A torn write (mutex failure)
	// would produce partial lines that do not match the expected substrings.
	out := buf.String()
	for _, want := range []string{
		"Starting a on h1...",
		"Starting b on h2...",
		"Starting c on h3...",
	} {
		if got := strings.Count(out, want); got != 1 {
			t.Errorf("expected exactly 1 occurrence of %q, got %d. Full output:\n%s", want, got, out)
		}
	}
}

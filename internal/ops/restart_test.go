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

// newRestartMockExecutor returns a MockExecutor wired so the native-mode
// Restart path on Linux completes Stop then Start without touching real
// services. The orchestrator calls mgr.Stop then mgr.Start on the same
// connection; both share the svc.sh and autostart probes but otherwise
// have distinct command patterns that the mock disambiguates by substring.
//
// Stop path:
//   - svc.sh probe reports "no" → svc.sh branch skipped.
//   - autostart probes return empty → Detect returns KindNone.
//   - stopNative's pid-file probe returns "not running" → terminal state.
//
// Start path (assumes rc.Ephemeral == true to skip autostart install):
//   - svc.sh probe reports "no" → svc.sh branch skipped.
//   - autostart probes return empty → Detect returns KindNone.
//   - NativeRunnerConfigPresent returns "yes" → no setupNative.
//   - start command returns "started PID 12345" → start succeeded.
//   - stale registration check returns "ok" → no retry.
//
// Disambiguation between the Stop and Start pid-file commands hinges on
// "rm -f" appearing only in Stop (start's cmd has no "rm -f") and
// "nohup ./run.sh" appearing only in Start.
func newRestartMockExecutor() *testutil.MockExecutor {
	return &testutil.MockExecutor{
		RunFn: func(cmd string) (string, error) {
			switch {
			// Both Stop and Start share the svc.sh probe.
			case strings.Contains(cmd, "test -f") && strings.Contains(cmd, "svc.sh"):
				return "no\n", nil
			// Both Stop and Start share the systemd-user probe.
			case strings.Contains(cmd, "test -f") && strings.Contains(cmd, ".config/systemd/user"):
				return "", nil
			// Both Stop and Start share the systemd-system probe.
			case strings.Contains(cmd, "test -f") && strings.Contains(cmd, "/etc/systemd/system"):
				return "", nil
			// Stop path: stopNative's pid-file probe (rm -f only appears here).
			case strings.Contains(cmd, "rm -f") && strings.Contains(cmd, "pid_file"):
				return "not running\n", nil
			// Start path: NativeRunnerConfigPresent (test -d + run.sh).
			case strings.Contains(cmd, "test -d") && strings.Contains(cmd, "run.sh"):
				return "yes\n", nil
			// Start path: the nohup-runner launch command (Stop has no nohup).
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

// TestRestart_EmptyRunners covers the no-match filter case: FilterRunners
// returns no runners, so the orchestrator returns nil without ever invoking
// connectHostFn. The host mock is intentionally not registered — if the
// orchestrator tried to connect, the test would fail with the "no mock
// registered" error. This is the contract that `gh sr restart` does not
// error out when the user supplies a filter that matches nothing.
func TestRestart_EmptyRunners(t *testing.T) {
	t.Parallel()

	installMockConnectHost(t, map[string]host.Executor{})

	mgr := &runner.Manager{}
	cfg := cfgWithLocalHost("h1")
	cfg.Runners = []config.RunnerConfig{
		{Name: "ci", Host: "h1", Repo: "org/repo", Count: 1, Ephemeral: true},
	}

	var buf bytes.Buffer
	if err := Restart(&buf, cfg, mgr, "", "no-such-repo", nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := buf.String(); got != "" {
		t.Errorf("expected no output, got %q", got)
	}
}

// TestRestart_SingleRunner pins the simplest success path: one local host,
// one ephemeral runner, mgr.Stop is called once, mgr.Start is called once,
// the per-runner "Restarting X on Y" line is written, and no error surfaces.
// This is the contract that `gh sr restart ci` (single name arg) drives the
// full Stop→Start chain on a single connection.
func TestRestart_SingleRunner(t *testing.T) {
	t.Parallel()

	exec := newRestartMockExecutor()
	installMockConnectHost(t, map[string]host.Executor{"h1": exec})

	mgr := &runner.Manager{}
	cfg := cfgWithLocalHost("h1")
	cfg.Runners = []config.RunnerConfig{
		{Name: "ci", Host: "h1", Repo: "org/repo", Count: 1, Ephemeral: true},
	}

	var buf bytes.Buffer
	if err := Restart(&buf, cfg, mgr, "", "", nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "Restarting ci on h1...") {
		t.Errorf("output missing 'Restarting ci on h1...' line; got:\n%s", out)
	}
	// Stop and Start each print a per-instance "started PID" or "not running"
	// line via m.out() (which falls back to os.Stdout when m.Out is nil), so we
	// can't assert on those here — but we can assert that the host mock saw
	// both a Stop-side pid-file probe and a Start-side nohup probe. The Stop
	// probe has "rm -f"; the Start probe has "nohup ./run.sh".
	var sawStop, sawStart bool
	for _, c := range exec.Calls {
		if strings.Contains(c, "rm -f") && strings.Contains(c, "pid_file") {
			sawStop = true
		}
		if strings.Contains(c, "nohup ./run.sh") {
			sawStart = true
		}
	}
	if !sawStop {
		t.Errorf("expected Stop's pid-file probe (with rm -f) to be issued; calls=%v", exec.Calls)
	}
	if !sawStart {
		t.Errorf("expected Start's nohup command to be issued; calls=%v", exec.Calls)
	}
}

// TestRestart_MultipleRunnersSameHost pins the SSH-amortisation contract:
// when N runners share a host, mgr.Stop then mgr.Start are called once per
// runner (sequentially on the same connection), not once per (runner, host).
func TestRestart_MultipleRunnersSameHost(t *testing.T) {
	t.Parallel()

	exec := newRestartMockExecutor()
	installMockConnectHost(t, map[string]host.Executor{"h1": exec})

	mgr := &runner.Manager{}
	cfg := cfgWithLocalHost("h1")
	cfg.Runners = []config.RunnerConfig{
		{Name: "first", Host: "h1", Repo: "org/repo", Count: 1, Ephemeral: true},
		{Name: "second", Host: "h1", Repo: "org/repo", Count: 1, Ephemeral: true},
		{Name: "third", Host: "h1", Repo: "org/repo", Count: 1, Ephemeral: true},
	}

	var buf bytes.Buffer
	if err := Restart(&buf, cfg, mgr, "", "", nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := buf.String()
	for _, name := range []string{"first", "second", "third"} {
		want := "Restarting " + name + " on h1..."
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q; got:\n%s", want, out)
		}
	}

	// 3 runners × 2 Stop pid-file probes + 2 Start nohup probes (Stop and
	// Start both call svc.sh + autostart Detect too, but those are shared
	// service probes — we count only the per-instance dispatch probes here).
	var stopCount, startCount int
	for _, c := range exec.Calls {
		if strings.Contains(c, "rm -f") && strings.Contains(c, "pid_file") {
			stopCount++
		}
		if strings.Contains(c, "nohup ./run.sh") {
			startCount++
		}
	}
	if stopCount != 3 {
		t.Errorf("expected 3 Stop pid-file probes (one per runner); got %d", stopCount)
	}
	if startCount != 3 {
		t.Errorf("expected 3 Start nohup probes (one per runner); got %d", startCount)
	}
}

// TestRestart_MultiHostConcurrent pins the multi-host fan-out: 3 hosts, 1
// runner each, all three "Restarting X on Y" lines appear in the output.
// We don't assert ordering (runPerHostParallel already covers that) — we
// just assert each host's line is present and the orchestrator returns nil.
func TestRestart_MultiHostConcurrent(t *testing.T) {
	t.Parallel()

	installMockConnectHost(t, map[string]host.Executor{
		"h1": newRestartMockExecutor(),
		"h2": newRestartMockExecutor(),
		"h3": newRestartMockExecutor(),
	})

	mgr := &runner.Manager{}
	cfg := cfgWithLocalHost("h1", "h2", "h3")
	cfg.Runners = []config.RunnerConfig{
		{Name: "a", Host: "h1", Repo: "org/repo", Count: 1, Ephemeral: true},
		{Name: "b", Host: "h2", Repo: "org/repo", Count: 1, Ephemeral: true},
		{Name: "c", Host: "h3", Repo: "org/repo", Count: 1, Ephemeral: true},
	}

	var buf bytes.Buffer
	if err := Restart(&buf, cfg, mgr, "", "", nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := buf.String()
	for _, want := range []string{
		"Restarting a on h1...",
		"Restarting b on h2...",
		"Restarting c on h3...",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q; got:\n%s", want, out)
		}
	}
}

// TestRestart_FilterByHost verifies the filter integration: filterHost="h1"
// narrows the runner set so only h1's runners are restarted, and h2's
// runner is not touched.
func TestRestart_FilterByHost(t *testing.T) {
	t.Parallel()

	installMockConnectHost(t, map[string]host.Executor{
		"h1": newRestartMockExecutor(),
		"h2": newRestartMockExecutor(),
	})

	mgr := &runner.Manager{}
	cfg := cfgWithLocalHost("h1", "h2")
	cfg.Runners = []config.RunnerConfig{
		{Name: "ci1", Host: "h1", Repo: "org/repo", Count: 1, Ephemeral: true},
		{Name: "ci2", Host: "h2", Repo: "org/repo", Count: 1, Ephemeral: true},
	}

	var buf bytes.Buffer
	if err := Restart(&buf, cfg, mgr, "h1", "", nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "Restarting ci1 on h1...") {
		t.Errorf("expected ci1 restart line; got:\n%s", out)
	}
	if strings.Contains(out, "Restarting ci2 on h2...") {
		t.Errorf("did not expect ci2 restart line (filtered out); got:\n%s", out)
	}
}

// TestRestart_NilWriter verifies the io.Discard fallback: passing nil as
// the writer does not panic, and the orchestrator still drives mgr.Stop
// and mgr.Start to completion and returns nil.
func TestRestart_NilWriter(t *testing.T) {
	t.Parallel()

	installMockConnectHost(t, map[string]host.Executor{
		"h1": newRestartMockExecutor(),
	})

	mgr := &runner.Manager{}
	cfg := cfgWithLocalHost("h1")
	cfg.Runners = []config.RunnerConfig{
		{Name: "ci", Host: "h1", Repo: "org/repo", Count: 1, Ephemeral: true},
	}

	if err := Restart(nil, cfg, mgr, "", "", nil); err != nil {
		t.Fatalf("unexpected error with nil writer: %v", err)
	}
}

// TestRestart_FilterByNameArgs verifies the name-args filter integration:
// when the user passes `gh sr restart r1 r3`, only those two runners are
// restarted.
func TestRestart_FilterByNameArgs(t *testing.T) {
	t.Parallel()

	installMockConnectHost(t, map[string]host.Executor{
		"h1": newRestartMockExecutor(),
	})

	mgr := &runner.Manager{}
	cfg := cfgWithLocalHost("h1")
	cfg.Runners = []config.RunnerConfig{
		{Name: "r1", Host: "h1", Repo: "org/repo", Count: 1, Ephemeral: true},
		{Name: "r2", Host: "h1", Repo: "org/repo", Count: 1, Ephemeral: true},
		{Name: "r3", Host: "h1", Repo: "org/repo", Count: 1, Ephemeral: true},
	}

	var buf bytes.Buffer
	if err := Restart(&buf, cfg, mgr, "", "", []string{"r1", "r3"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "Restarting r1 on h1...") {
		t.Errorf("expected r1 restart line; got:\n%s", out)
	}
	if strings.Contains(out, "Restarting r2 on h1...") {
		t.Errorf("did not expect r2 restart line; got:\n%s", out)
	}
	if !strings.Contains(out, "Restarting r3 on h1...") {
		t.Errorf("expected r3 restart line; got:\n%s", out)
	}
}

// TestRestart_StopErrorIgnoredThenStartSucceeds pins the "ignore first
// error" contract that makes Restart different from sequential Stop+Start:
// when mgr.Stop returns an error (e.g. the runner was already stopped), the
// orchestrator must NOT propagate that error. It must still call mgr.Start
// and surface Start's error if any. This is the contract that `gh sr
// restart` works correctly on already-stopped runners — the user-visible
// behaviour is "make sure it's running", not "fail if it wasn't running".
func TestRestart_StopErrorIgnoredThenStartSucceeds(t *testing.T) {
	t.Parallel()

	stopSentinel := errors.New("stop failed (already stopped)")
	exec := &testutil.MockExecutor{
		RunFn: func(cmd string) (string, error) {
			switch {
			case strings.Contains(cmd, "test -f") && strings.Contains(cmd, "svc.sh"):
				return "no\n", nil
			case strings.Contains(cmd, "test -f") && strings.Contains(cmd, ".config/systemd/user"):
				return "", nil
			case strings.Contains(cmd, "test -f") && strings.Contains(cmd, "/etc/systemd/system"):
				return "", nil
			// Stop fails on the pid-file probe.
			case strings.Contains(cmd, "rm -f") && strings.Contains(cmd, "pid_file"):
				return "", stopSentinel
			// Start path still completes successfully.
			case strings.Contains(cmd, "test -d") && strings.Contains(cmd, "run.sh"):
				return "yes\n", nil
			case strings.Contains(cmd, "nohup ./run.sh"):
				return "started PID 12345\n", nil
			case strings.Contains(cmd, "sleep 5"):
				return "ok\n", nil
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
	if err := Restart(&buf, cfg, mgr, "", "", nil); err != nil {
		t.Fatalf("Restart should ignore Stop errors when Start succeeds; got %v", err)
	}

	// Confirm Start was actually invoked after Stop failed. If a refactor
	// accidentally short-circuits on Stop error, the nohup probe would be
	// missing.
	var sawStart bool
	for _, c := range exec.Calls {
		if strings.Contains(c, "nohup ./run.sh") {
			sawStart = true
			break
		}
	}
	if !sawStart {
		t.Errorf("expected Start's nohup command after Stop error; calls=%v", exec.Calls)
	}
}

// TestRestart_StartErrorPropagates pins the error-propagation contract
// from the Start side: when Stop succeeds but Start fails, the orchestrator
// returns Start's error. This catches a refactor that swallows errors or
// returns Stop's discarded error instead.
func TestRestart_StartErrorPropagates(t *testing.T) {
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
			case strings.Contains(cmd, "rm -f") && strings.Contains(cmd, "pid_file"):
				return "not running\n", nil
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
	err := Restart(&buf, cfg, mgr, "", "", nil)
	if !errors.Is(err, startSentinel) {
		t.Fatalf("got %v; want %v", err, startSentinel)
	}
}

// TestRestart_BothStopAndStartAreCalled pins the call-order contract: the
// orchestrator calls Stop first, then Start, on the same connection. If a
// refactor accidentally swaps the order (Start first), the test catches it
// because the Start-only probes (nohup, sleep 5) would precede the
// Stop-only probe (rm -f pid_file). This protects against a subtle
// regression where Stop no longer cleans up before Start brings it back.
func TestRestart_BothStopAndStartAreCalled(t *testing.T) {
	t.Parallel()

	exec := newRestartMockExecutor()
	installMockConnectHost(t, map[string]host.Executor{"h1": exec})

	mgr := &runner.Manager{}
	cfg := cfgWithLocalHost("h1")
	cfg.Runners = []config.RunnerConfig{
		{Name: "ci", Host: "h1", Repo: "org/repo", Count: 1, Ephemeral: true},
	}

	var buf bytes.Buffer
	if err := Restart(&buf, cfg, mgr, "", "", nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	stopIdx, startIdx := -1, -1
	for i, c := range exec.Calls {
		if stopIdx == -1 && strings.Contains(c, "rm -f") && strings.Contains(c, "pid_file") {
			stopIdx = i
		}
		if startIdx == -1 && strings.Contains(c, "nohup ./run.sh") {
			startIdx = i
		}
	}
	if stopIdx == -1 {
		t.Fatalf("Stop's pid-file probe never issued; calls=%v", exec.Calls)
	}
	if startIdx == -1 {
		t.Fatalf("Start's nohup command never issued; calls=%v", exec.Calls)
	}
	if stopIdx >= startIdx {
		t.Errorf("expected Stop probe (idx %d) before Start probe (idx %d); calls=%v", stopIdx, startIdx, exec.Calls)
	}
}

// TestRestart_StartErrorOnOneHostDoesNotPoisonAnother pins the host-
// isolation contract: a Start error on h1 does not prevent h2's runner
// from being restarted. runPerHostParallel fans out concurrently and
// surfaces only the first error per group; h2's group must still complete.
func TestRestart_StartErrorOnOneHostDoesNotPoisonAnother(t *testing.T) {
	t.Parallel()

	startSentinel := errors.New("h1 start failed")
	h2StopCalled := atomic.Int32{}
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
				case strings.Contains(cmd, "rm -f") && strings.Contains(cmd, "pid_file"):
					return "not running\n", nil
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
				case strings.Contains(cmd, "rm -f") && strings.Contains(cmd, "pid_file"):
					h2StopCalled.Add(1)
					return "not running\n", nil
				case strings.Contains(cmd, "test -d") && strings.Contains(cmd, "run.sh"):
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
	err := Restart(&buf, cfg, mgr, "", "", nil)
	if !errors.Is(err, startSentinel) {
		t.Fatalf("got %v; want %v", err, startSentinel)
	}
	if got := h2StopCalled.Load(); got == 0 {
		t.Errorf("h2's Stop was not called (h1's error poisoned h2)")
	}
	if got := h2StartCalled.Load(); got == 0 {
		t.Errorf("h2's Start was not called (h1's error poisoned h2)")
	}
}

// TestRestart_WriterSerialisedAcrossHosts pins the multi-host output
// contract: the per-host "Restarting X on Y" lines each occupy exactly one
// full token in the output buffer (no torn writes between concurrent
// goroutines). Uses a barrier to force the goroutines to write concurrently
// and a substring-count check to detect partial lines.
func TestRestart_WriterSerialisedAcrossHosts(t *testing.T) {
	t.Parallel()

	barrier := make(chan struct{})
	started := make(chan struct{}, 3)
	installMockConnectHost(t, map[string]host.Executor{
		"h1": &testutil.MockExecutor{
			RunFn: func(cmd string) (string, error) {
				if strings.Contains(cmd, "rm -f") && strings.Contains(cmd, "pid_file") {
					started <- struct{}{}
					<-barrier
					return "not running\n", nil
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
				if strings.Contains(cmd, "test -d") && strings.Contains(cmd, "run.sh") {
					return "yes\n", nil
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
				if strings.Contains(cmd, "rm -f") && strings.Contains(cmd, "pid_file") {
					started <- struct{}{}
					<-barrier
					return "not running\n", nil
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
				if strings.Contains(cmd, "test -d") && strings.Contains(cmd, "run.sh") {
					return "yes\n", nil
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
				if strings.Contains(cmd, "rm -f") && strings.Contains(cmd, "pid_file") {
					started <- struct{}{}
					<-barrier
					return "not running\n", nil
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
				if strings.Contains(cmd, "test -d") && strings.Contains(cmd, "run.sh") {
					return "yes\n", nil
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
		done <- Restart(&buf, cfg, mgr, "", "", nil)
	}()

	// Wait for all 3 goroutines to reach the pid-file probe, then release.
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
		"Restarting a on h1...",
		"Restarting b on h2...",
		"Restarting c on h3...",
	} {
		if got := strings.Count(out, want); got != 1 {
			t.Errorf("expected exactly 1 occurrence of %q, got %d. Full output:\n%s", want, got, out)
		}
	}
}

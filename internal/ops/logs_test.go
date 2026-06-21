package ops

import (
	"errors"
	"strings"
	"testing"

	"github.com/an-lee/gh-sr/internal/config"
	"github.com/an-lee/gh-sr/internal/host"
	"github.com/an-lee/gh-sr/internal/runner"
	"github.com/an-lee/gh-sr/internal/testutil"
)

// newLogsMockExecutor returns a MockExecutor wired so the native-mode Logs
// path on Linux completes without touching real files. logsNative runs:
//
//	tail -50 $HOME/.gh-sr/runners/<instance>/runner.log 2>/dev/null || echo 'no logs found'
//
// We pin on `tail -50` + `/runner.log` as the unique signature of this
// command (no other orchestrator in this package invokes tail).
func newLogsMockExecutor(logOutput string) *testutil.MockExecutor {
	return &testutil.MockExecutor{
		RunFn: func(cmd string) (string, error) {
			if strings.Contains(cmd, "tail -50") && strings.Contains(cmd, "/runner.log") {
				return logOutput, nil
			}
			return "", nil
		},
	}
}

// cfgWithLogRunner builds a single-runner config on the given local hosts so
// ResolveHostInfo short-circuits (no SSH detection, no fan-out) and Logs
// proceeds straight to FindRunnerForLogs.
func cfgWithLogRunner(hosts []string, runners []config.RunnerConfig) *config.Config {
	c := &config.Config{Hosts: make(map[string]config.HostConfig)}
	for _, n := range hosts {
		c.Hosts[n] = config.HostConfig{Addr: "local", OS: "linux", Arch: "amd64"}
	}
	c.Runners = runners
	return c
}

// TestLogs_RunnerNotFound covers the FindRunnerForLogs "not in config" branch:
// the runner name does not match anything in cfg.Runners, so FindRunnerForLogs
// returns an error before connectHostFn is invoked. The host mock is
// intentionally not registered — if Logs tried to connect, the test would fail
// with the "no mock registered" error. This is the contract that
// `gh sr logs no-such-runner` fails fast with a clear error.
func TestLogs_RunnerNotFound(t *testing.T) {
	t.Parallel()

	installMockConnectHost(t, map[string]host.Executor{})

	mgr := &runner.Manager{}
	cfg := cfgWithLogRunner([]string{"h1"}, []config.RunnerConfig{
		{Name: "ci", Host: "h1", Repo: "org/repo", Count: 1},
	})

	_, err := Logs(cfg, mgr, "", "no-such-runner")
	if err == nil {
		t.Fatalf("expected error for missing runner, got nil")
	}
	if !strings.Contains(err.Error(), "no-such-runner") {
		t.Errorf("expected error message to mention runner name; got %q", err.Error())
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected error message to mention 'not found'; got %q", err.Error())
	}
}

// TestLogs_AmbiguousRunnerName pins the ambiguity contract: when the same
// runner name exists on two hosts and the user did not supply --host,
// FindRunnerForLogs returns the "matches multiple hosts" error and Logs
// surfaces it before any host connection. The host mocks are intentionally
// not registered.
func TestLogs_AmbiguousRunnerName(t *testing.T) {
	t.Parallel()

	installMockConnectHost(t, map[string]host.Executor{})

	mgr := &runner.Manager{}
	cfg := cfgWithLogRunner([]string{"h1", "h2"}, []config.RunnerConfig{
		{Name: "ci", Host: "h1", Repo: "org/repo", Count: 1},
		{Name: "ci", Host: "h2", Repo: "org/repo", Count: 1},
	})

	_, err := Logs(cfg, mgr, "", "ci")
	if err == nil {
		t.Fatalf("expected ambiguous-match error, got nil")
	}
	if !strings.Contains(err.Error(), "multiple hosts") {
		t.Errorf("expected 'multiple hosts' in error; got %q", err.Error())
	}
}

// TestLogs_HostFilterNarrowsAmbiguous pins the --host filter integration:
// same ambiguity as above, but supplying --host h1 narrows the match and
// Logs proceeds to the success path. Catches a refactor that drops the
// filter and re-introduces the ambiguity error.
func TestLogs_HostFilterNarrowsAmbiguous(t *testing.T) {
	t.Parallel()

	exec := newLogsMockExecutor("line 1\nline 2\n")
	installMockConnectHost(t, map[string]host.Executor{
		"h1": exec,
		"h2": newLogsMockExecutor("h2 output"),
	})

	mgr := &runner.Manager{}
	cfg := cfgWithLogRunner([]string{"h1", "h2"}, []config.RunnerConfig{
		{Name: "ci", Host: "h1", Repo: "org/repo", Count: 1},
		{Name: "ci", Host: "h2", Repo: "org/repo", Count: 1},
	})

	out, err := Logs(cfg, mgr, "h1", "ci")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "line 1") {
		t.Errorf("expected h1's log content; got %q", out)
	}
	if strings.Contains(out, "h2 output") {
		t.Errorf("did not expect h2's log content; got %q", out)
	}
}

// TestLogs_HostFilterNotFound pins the "runner on a different host" branch:
// the runner exists, but only on h2; passing --host h1 yields the
// "runner %q not found for host %q" error.
func TestLogs_HostFilterNotFound(t *testing.T) {
	t.Parallel()

	installMockConnectHost(t, map[string]host.Executor{
		"h1": newLogsMockExecutor(""),
		"h2": newLogsMockExecutor(""),
	})

	mgr := &runner.Manager{}
	cfg := cfgWithLogRunner([]string{"h1", "h2"}, []config.RunnerConfig{
		{Name: "ci", Host: "h2", Repo: "org/repo", Count: 1},
	})

	_, err := Logs(cfg, mgr, "h1", "ci")
	if err == nil {
		t.Fatalf("expected not-found-for-host error, got nil")
	}
	if !strings.Contains(err.Error(), "not found") || !strings.Contains(err.Error(), "h1") {
		t.Errorf("expected error to mention h1 and 'not found'; got %q", err.Error())
	}
}

// TestLogs_ConnectHostError pins the connectHostFn error-propagation contract:
// when the factory returns a sentinel error, Logs surfaces it before invoking
// mgr.Logs. The mock executor's RunFn must never be called.
func TestLogs_ConnectHostError(t *testing.T) {
	t.Parallel()

	sentinel := errors.New("ssh handshake failed")
	installFailingConnectHost(t, sentinel)

	mgr := &runner.Manager{}
	cfg := cfgWithLogRunner([]string{"h1"}, []config.RunnerConfig{
		{Name: "ci", Host: "h1", Repo: "org/repo", Count: 1},
	})

	_, err := Logs(cfg, mgr, "", "ci")
	if !errors.Is(err, sentinel) {
		t.Fatalf("got %v; want %v", err, sentinel)
	}
}

// TestLogs_SuccessByInstanceName covers the happy path: user passes an
// instance name (`ci-1`), the mock executor returns log lines, and Logs
// returns the same lines. The exec.Calls assertion pins that exactly one
// `tail -50 .../runner.log` command was issued.
func TestLogs_SuccessByInstanceName(t *testing.T) {
	t.Parallel()

	wantLogs := "[2026-06-20T10:00:00Z] Job started\n[2026-06-20T10:00:01Z] Step: build\n"
	exec := newLogsMockExecutor(wantLogs)
	installMockConnectHost(t, map[string]host.Executor{"h1": exec})

	mgr := &runner.Manager{}
	cfg := cfgWithLogRunner([]string{"h1"}, []config.RunnerConfig{
		{Name: "ci", Host: "h1", Repo: "org/repo", Count: 1},
	})

	out, err := Logs(cfg, mgr, "", "ci-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out != wantLogs {
		t.Errorf("got %q; want %q", out, wantLogs)
	}
	// Verify the exact command was issued.
	var saw bool
	for _, c := range exec.Calls {
		if strings.Contains(c, "tail -50") && strings.Contains(c, "/runner.log") {
			saw = true
			break
		}
	}
	if !saw {
		t.Errorf("expected tail -50 .../runner.log; calls=%v", exec.Calls)
	}
}

// TestLogs_SuccessByBaseNameForSingleInstance pins the contract that for a
// Count=1 runner, the user may pass either the base name or the instance
// name — both resolve to the same instance.
func TestLogs_SuccessByBaseNameForSingleInstance(t *testing.T) {
	t.Parallel()

	exec := newLogsMockExecutor("base-name resolved\n")
	installMockConnectHost(t, map[string]host.Executor{"h1": exec})

	mgr := &runner.Manager{}
	cfg := cfgWithLogRunner([]string{"h1"}, []config.RunnerConfig{
		{Name: "ci", Host: "h1", Repo: "org/repo", Count: 1},
	})

	out, err := Logs(cfg, mgr, "", "ci")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "base-name resolved") {
		t.Errorf("got %q", out)
	}
}

// TestLogs_BaseNameAmbiguousForMultiInstance pins the ResolveRunnerInstance
// ambiguity error: a Count=3 runner's base name "ci" matches 3 instances,
// and the helper returns an error naming all three. This is the "specify one
// of: ci-1, ci-2, ci-3" branch.
func TestLogs_BaseNameAmbiguousForMultiInstance(t *testing.T) {
	t.Parallel()

	installMockConnectHost(t, map[string]host.Executor{"h1": newLogsMockExecutor("")})

	mgr := &runner.Manager{}
	cfg := cfgWithLogRunner([]string{"h1"}, []config.RunnerConfig{
		{Name: "ci", Host: "h1", Repo: "org/repo", Count: 3},
	})

	_, err := Logs(cfg, mgr, "", "ci")
	if err == nil {
		t.Fatalf("expected error for ambiguous base name, got nil")
	}
	if !strings.Contains(err.Error(), "ci-1") || !strings.Contains(err.Error(), "ci-3") {
		t.Errorf("expected error to list instance names (ci-1 ... ci-3); got %q", err.Error())
	}
}

// TestLogs_InvalidInstanceName pins the ResolveRunnerInstance "not a valid
// name or instance" branch: user passes "ci-99" but the runner only has
// Count=2 instances.
func TestLogs_InvalidInstanceName(t *testing.T) {
	t.Parallel()

	installMockConnectHost(t, map[string]host.Executor{"h1": newLogsMockExecutor("")})

	mgr := &runner.Manager{}
	cfg := cfgWithLogRunner([]string{"h1"}, []config.RunnerConfig{
		{Name: "ci", Host: "h1", Repo: "org/repo", Count: 2},
	})

	_, err := Logs(cfg, mgr, "", "ci-99")
	if err == nil {
		t.Fatalf("expected error for invalid instance name, got nil")
	}
	if !strings.Contains(err.Error(), "ci-99") {
		t.Errorf("expected error to mention bad name ci-99; got %q", err.Error())
	}
}

// TestLogs_MultiInstanceByInstanceName pins that for a Count>1 runner, the
// user may pass the instance name directly to read that instance's log
// file. The mock's tail command must reference the right instance path.
func TestLogs_MultiInstanceByInstanceName(t *testing.T) {
	t.Parallel()

	wantLogs := "second-instance log\n"
	exec := newLogsMockExecutor(wantLogs)
	installMockConnectHost(t, map[string]host.Executor{"h1": exec})

	mgr := &runner.Manager{}
	cfg := cfgWithLogRunner([]string{"h1"}, []config.RunnerConfig{
		{Name: "ci", Host: "h1", Repo: "org/repo", Count: 3},
	})

	out, err := Logs(cfg, mgr, "", "ci-2")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "second-instance log") {
		t.Errorf("got %q", out)
	}
	// Confirm the path includes the right instance dir.
	var sawCi2 bool
	for _, c := range exec.Calls {
		if strings.Contains(c, "/ci-2/runner.log") {
			sawCi2 = true
			break
		}
	}
	if !sawCi2 {
		t.Errorf("expected /ci-2/runner.log in calls; got %v", exec.Calls)
	}
}

// TestLogs_ManagerLogsError pins the error-propagation contract from the
// mgr.Logs side: when the mock executor returns an error from the tail
// command, Logs surfaces that error.
func TestLogs_ManagerLogsError(t *testing.T) {
	t.Parallel()

	sentinel := errors.New("permission denied")
	exec := &testutil.MockExecutor{
		RunFn: func(cmd string) (string, error) {
			if strings.Contains(cmd, "tail -50") && strings.Contains(cmd, "/runner.log") {
				return "", sentinel
			}
			return "", nil
		},
	}
	installMockConnectHost(t, map[string]host.Executor{"h1": exec})

	mgr := &runner.Manager{}
	cfg := cfgWithLogRunner([]string{"h1"}, []config.RunnerConfig{
		{Name: "ci", Host: "h1", Repo: "org/repo", Count: 1},
	})

	_, err := Logs(cfg, mgr, "", "ci-1")
	if !errors.Is(err, sentinel) {
		t.Fatalf("got %v; want %v", err, sentinel)
	}
}

// TestLogs_NoLogsFoundMessage pins the "no logs found" fallback contract:
// when the runner.log file does not exist, tail fails and the shell's
// `|| echo 'no logs found'` clause kicks in. The mock returns exactly that
// fallback string, and Logs propagates it verbatim.
func TestLogs_NoLogsFoundMessage(t *testing.T) {
	t.Parallel()

	exec := newLogsMockExecutor("no logs found\n")
	installMockConnectHost(t, map[string]host.Executor{"h1": exec})

	mgr := &runner.Manager{}
	cfg := cfgWithLogRunner([]string{"h1"}, []config.RunnerConfig{
		{Name: "ci", Host: "h1", Repo: "org/repo", Count: 1},
	})

	out, err := Logs(cfg, mgr, "", "ci-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "no logs found") {
		t.Errorf("expected 'no logs found' fallback; got %q", out)
	}
}

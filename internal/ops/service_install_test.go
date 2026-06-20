package ops

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/an-lee/gh-sr/internal/config"
	"github.com/an-lee/gh-sr/internal/host"
	"github.com/an-lee/gh-sr/internal/testutil"
)

func TestServiceInstallSkipsContainerRunners(t *testing.T) {
	t.Parallel()
	installMockConnectHost(t, map[string]host.Executor{
		"x1": &testutil.MockExecutor{},
	})

	cfg := &config.Config{
		Hosts: map[string]config.HostConfig{
			"x1": {Addr: "local", OS: "linux"},
		},
		Runners: []config.RunnerConfig{
			{Name: "baizhiheizi", Host: "x1", Repo: "o/r", Count: 1, RunnerMode: config.RunnerModeContainer},
		},
	}
	var buf bytes.Buffer
	if err := ServiceInstall(&buf, cfg, "", "", nil, false); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "runner_mode: container") {
		t.Fatalf("expected container skip message, got:\n%s", out)
	}
	if strings.Contains(out, "not configured on host") {
		t.Fatalf("should not fail for container runner, got:\n%s", out)
	}
}

// autostartInstallMock returns a *testutil.MockExecutor wired to the sequence
// of remote commands the Linux-user-mode autostart.Install path issues when
// everything succeeds: home resolution, runner-presence probe (yes), unit-file
// write, and the three systemctl invocations. `present` flips the probe to
// "no" so tests can exercise the "runner not configured" error branch.
func autostartInstallMock(present bool) *testutil.MockExecutor {
	return &testutil.MockExecutor{
		RunFn: func(cmd string) (string, error) {
			switch {
			case strings.Contains(cmd, `printf %s "$HOME"`):
				return "/home/test\n", nil
			case strings.Contains(cmd, "test -d") && strings.Contains(cmd, "run.sh"):
				if present {
					return "yes\n", nil
				}
				return "no\n", nil
			case strings.Contains(cmd, "base64 -d"):
				return "", nil // WriteRemoteBytes
			case strings.Contains(cmd, "systemctl --user"):
				return "", nil
			}
			return "", nil
		},
	}
}

// TestServiceInstall_nativeSuccess exercises the happy path: NativeRunnerConfigPresent
// reports yes, autostart.Install issues its systemctl sequence, and the runner
// appears in the output as installed. Locks in the per-instance iteration loop.
func TestServiceInstall_nativeSuccess(t *testing.T) {
	t.Parallel()
	installMockConnectHost(t, map[string]host.Executor{
		"h1": autostartInstallMock(true),
	})

	cfg := &config.Config{
		Hosts: map[string]config.HostConfig{
			"h1": {Addr: "local", OS: "linux"},
		},
		Runners: []config.RunnerConfig{
			{Name: "ci", Host: "h1", Repo: "o/r", Count: 1},
		},
	}
	var buf bytes.Buffer
	if err := ServiceInstall(&buf, cfg, "", "", nil, false); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "Autostart for ci on h1 (local)") {
		t.Fatalf("expected banner with local addr, got:\n%s", out)
	}
	if !strings.Contains(out, "ci-1: autostart installed") {
		t.Fatalf("expected per-instance install confirmation, got:\n%s", out)
	}
}

// TestServiceInstall_nativeRemoteAddr verifies the banner uses the remote
// address (not the literal "local") when hcfg.Addr is set, so multi-host
// operators can tell which host each line refers to.
func TestServiceInstall_nativeRemoteAddr(t *testing.T) {
	t.Parallel()
	mock := autostartInstallMock(true)
	// Remote hosts trigger OS/arch detection via ResolveHostInfo; mock must
	// answer the arch probe with a recognised value.
	mock.RunFn = func(cmd string) (string, error) {
		if strings.Contains(cmd, "uname -m") {
			return "x86_64\n", nil
		}
		// Delegate everything else to the standard install mock.
		switch {
		case strings.Contains(cmd, `printf %s "$HOME"`):
			return "/home/test\n", nil
		case strings.Contains(cmd, "test -d") && strings.Contains(cmd, "run.sh"):
			return "yes\n", nil
		case strings.Contains(cmd, "base64 -d"):
			return "", nil
		case strings.Contains(cmd, "systemctl --user"):
			return "", nil
		}
		return "", nil
	}
	installMockConnectHost(t, map[string]host.Executor{
		"h1": mock,
	})

	cfg := &config.Config{
		Hosts: map[string]config.HostConfig{
			"h1": {Addr: "runner@box.example", OS: "linux"},
		},
		Runners: []config.RunnerConfig{
			{Name: "ci", Host: "h1", Repo: "o/r", Count: 1},
		},
	}
	var buf bytes.Buffer
	if err := ServiceInstall(&buf, cfg, "", "", nil, false); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "runner@box.example") {
		t.Fatalf("expected remote address in banner, got:\n%s", buf.String())
	}
}

// TestServiceInstall_runnerNotConfigured pins the "user forgot to run setup"
// error path. The orchestrator must surface an actionable error ("run: gh sr
// setup <name>") instead of silently succeeding or producing a confusing
// systemctl failure deep inside autostart.Install.
func TestServiceInstall_runnerNotConfigured(t *testing.T) {
	t.Parallel()
	installMockConnectHost(t, map[string]host.Executor{
		"h1": autostartInstallMock(false),
	})

	cfg := &config.Config{
		Hosts: map[string]config.HostConfig{
			"h1": {Addr: "local", OS: "linux"},
		},
		Runners: []config.RunnerConfig{
			{Name: "ci", Host: "h1", Repo: "o/r", Count: 1},
		},
	}
	var buf bytes.Buffer
	err := ServiceInstall(&buf, cfg, "", "", nil, false)
	if err == nil {
		t.Fatal("expected error when runner not configured on host")
	}
	if !strings.Contains(err.Error(), "run: gh sr setup ci") {
		t.Fatalf("expected setup-hint error, got: %v", err)
	}
	if strings.Contains(buf.String(), "autostart installed") {
		t.Fatalf("should not confirm install on error path, got:\n%s", buf.String())
	}
}

// TestServiceInstall_systemFlagLinuxOnly validates that --system is rejected
// on non-Linux hosts. The flag controls a system-level systemd install that
// only makes sense on Linux; on darwin/windows it must fail loudly so the user
// doesn't think a system-wide install actually happened.
func TestServiceInstall_systemFlagLinuxOnly(t *testing.T) {
	t.Parallel()
	installMockConnectHost(t, map[string]host.Executor{
		"m1": &testutil.MockExecutor{},
	})

	cfg := &config.Config{
		Hosts: map[string]config.HostConfig{
			"m1": {Addr: "local", OS: "darwin"},
		},
		Runners: []config.RunnerConfig{
			{Name: "ci", Host: "m1", Repo: "o/r", Count: 1},
		},
	}
	var buf bytes.Buffer
	err := ServiceInstall(&buf, cfg, "", "", nil, true)
	if err == nil {
		t.Fatal("expected --system error on darwin host")
	}
	if !strings.Contains(err.Error(), "--system applies only to Linux") {
		t.Fatalf("expected --system error, got: %v", err)
	}
	if !strings.Contains(err.Error(), "darwin") {
		t.Fatalf("expected darwin in error, got: %v", err)
	}
}

// TestServiceInstall_remoteErrorPropagates locks in that errors from the
// remote probe (e.g. SSH disconnect during NativeRunnerConfigPresent) wrap
// the instance name and the underlying error so the operator can tell which
// instance on which host failed.
func TestServiceInstall_remoteErrorPropagates(t *testing.T) {
	t.Parallel()
	sentinel := errors.New("ssh: connection reset")
	installMockConnectHost(t, map[string]host.Executor{
		"h1": &testutil.MockExecutor{
			RunFn: func(cmd string) (string, error) {
				if strings.Contains(cmd, "test -d") && strings.Contains(cmd, "run.sh") {
					return "", sentinel
				}
				return "", nil
			},
		},
	})

	cfg := &config.Config{
		Hosts: map[string]config.HostConfig{
			"h1": {Addr: "local", OS: "linux"},
		},
		Runners: []config.RunnerConfig{
			{Name: "ci", Host: "h1", Repo: "o/r", Count: 1},
		},
	}
	var buf bytes.Buffer
	err := ServiceInstall(&buf, cfg, "", "", nil, false)
	if err == nil {
		t.Fatal("expected error from remote probe")
	}
	if !strings.Contains(err.Error(), "ci-1") {
		t.Fatalf("error should mention instance name, got: %v", err)
	}
	if !strings.Contains(err.Error(), "connection reset") {
		t.Fatalf("error should wrap underlying cause, got: %v", err)
	}
}

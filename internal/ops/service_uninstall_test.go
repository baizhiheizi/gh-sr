package ops

import (
	"bytes"
	"strings"
	"testing"

	"github.com/an-lee/gh-sr/internal/config"
	"github.com/an-lee/gh-sr/internal/host"
	"github.com/an-lee/gh-sr/internal/testutil"
)

// TestServiceUninstallSkipsContainerRunners mirrors the ServiceInstall container
// skip — `gh sr service uninstall` is a no-op for container-mode runners
// because their lifecycle is managed by Docker's restart policy, not by an
// OS autostart unit.
func TestServiceUninstallSkipsContainerRunners(t *testing.T) {
	t.Parallel()
	installMockConnectHost(t, map[string]host.Executor{
		"x1": &testutil.MockExecutor{},
	})

	cfg := &config.Config{
		Hosts: map[string]config.HostConfig{
			"x1": {Addr: "local", OS: "linux"},
		},
		Runners: []config.RunnerConfig{
			{Name: "agentic", Host: "x1", Repo: "o/r", Count: 1, RunnerMode: config.RunnerModeContainer},
		},
	}
	var buf bytes.Buffer
	if err := ServiceUninstall(&buf, cfg, "", "", nil); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "runner_mode: container") {
		t.Fatalf("expected container skip message, got:\n%s", out)
	}
	if strings.Contains(out, "Removing autostart") {
		t.Fatalf("should not attempt removal on container runner, got:\n%s", out)
	}
}

// TestServiceUninstall_nativeNoAutostartInstalled exercises the "Detect
// returns KindNone" path. On a clean host the systemd timer file does not
// exist; the orchestrator must report "no autostart to remove" instead of
// invoking any systemctl commands.
func TestServiceUninstall_nativeNoAutostartInstalled(t *testing.T) {
	t.Parallel()
	mock := &testutil.MockExecutor{}
	installMockConnectHost(t, map[string]host.Executor{"h1": mock})

	cfg := &config.Config{
		Hosts: map[string]config.HostConfig{
			"h1": {Addr: "local", OS: "linux"},
		},
		Runners: []config.RunnerConfig{
			{Name: "ci", Host: "h1", Repo: "o/r", Count: 1},
		},
	}
	var buf bytes.Buffer
	if err := ServiceUninstall(&buf, cfg, "", "", nil); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "Removing autostart for ci on h1") {
		t.Fatalf("expected uninstall banner, got:\n%s", out)
	}
	if !strings.Contains(out, "ci-1: no autostart to remove") {
		t.Fatalf("expected per-instance skip message, got:\n%s", out)
	}
	// No systemctl calls should have been issued.
	for _, call := range mock.Calls {
		if strings.Contains(call, "systemctl") {
			t.Fatalf("should not invoke systemctl when autostart absent, got: %s", call)
		}
	}
}

// TestServiceUninstall_nativeRemoteAddr verifies the uninstall banner uses
// the remote address (not "local") when hcfg.Addr is set, matching the
// install-side behaviour for symmetric output.
func TestServiceUninstall_nativeRemoteAddr(t *testing.T) {
	t.Parallel()
	mock := &testutil.MockExecutor{
		RunFn: func(cmd string) (string, error) {
			if strings.Contains(cmd, "uname -m") {
				return "x86_64\n", nil
			}
			return "", nil
		},
	}
	installMockConnectHost(t, map[string]host.Executor{"h1": mock})

	cfg := &config.Config{
		Hosts: map[string]config.HostConfig{
			"h1": {Addr: "runner@box.example", OS: "linux"},
		},
		Runners: []config.RunnerConfig{
			{Name: "ci", Host: "h1", Repo: "o/r", Count: 1},
		},
	}
	var buf bytes.Buffer
	if err := ServiceUninstall(&buf, cfg, "", "", nil); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "runner@box.example") {
		t.Fatalf("expected remote address in uninstall banner, got:\n%s", buf.String())
	}
}

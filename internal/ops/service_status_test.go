package ops

import (
	"bytes"
	"strings"
	"testing"

	"github.com/an-lee/gh-sr/internal/config"
	"github.com/an-lee/gh-sr/internal/host"
	"github.com/an-lee/gh-sr/internal/testutil"
)

// TestServiceStatus_containerModeNoAutostart pins the row shape for a
// container-mode runner on a host with no autostart unit installed. The
// orchestrator passes "native" as the autostart mode argument regardless of
// rc.RunnerMode, so the row shows `[native]` and the detail reports the
// not-installed state. (The container-mode skip lives in ServiceInstall /
// ServiceUninstall; ServiceStatus is intentionally a uniform read across
// both runner modes.)
func TestServiceStatus_containerModeNoAutostart(t *testing.T) {
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
	if err := ServiceStatus(&buf, cfg, "", "", nil); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "agentic-1 @ x1") {
		t.Fatalf("expected status row for container runner, got:\n%s", out)
	}
	if !strings.Contains(out, "[native]") {
		t.Fatalf("expected [native] mode column (ServiceStatus always reports native), got:\n%s", out)
	}
	if !strings.Contains(out, "autostart not installed") {
		t.Fatalf("expected not-installed detail, got:\n%s", out)
	}
}

// TestServiceStatus_nativeNoAutostart verifies the native-mode row when no
// autostart is installed (the common case on a freshly-provisioned host).
// The orchestrator must surface "autostart not installed" so operators can
// distinguish "never installed" from "installed but inactive".
func TestServiceStatus_nativeNoAutostart(t *testing.T) {
	t.Parallel()
	installMockConnectHost(t, map[string]host.Executor{
		"h1": &testutil.MockExecutor{},
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
	if err := ServiceStatus(&buf, cfg, "", "", nil); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "ci-1 @ h1 [native]") {
		t.Fatalf("expected native-mode status row, got:\n%s", out)
	}
	if !strings.Contains(out, "autostart not installed") {
		t.Fatalf("expected 'autostart not installed' detail, got:\n%s", out)
	}
}

// TestServiceStatus_perInstanceEnumeration locks in the per-instance loop:
// every entry in rc.InstanceNames() must produce one row, in order, so a
// `gh sr service status` report on a Count=3 runner shows three lines.
func TestServiceStatus_perInstanceEnumeration(t *testing.T) {
	t.Parallel()
	installMockConnectHost(t, map[string]host.Executor{
		"h1": &testutil.MockExecutor{},
	})

	cfg := &config.Config{
		Hosts: map[string]config.HostConfig{
			"h1": {Addr: "local", OS: "linux"},
		},
		Runners: []config.RunnerConfig{
			{Name: "ci", Host: "h1", Repo: "o/r", Count: 3},
		},
	}
	var buf bytes.Buffer
	if err := ServiceStatus(&buf, cfg, "", "", nil); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	for _, inst := range []string{"ci-1", "ci-2", "ci-3"} {
		if !strings.Contains(out, inst+" @ h1") {
			t.Fatalf("expected row for %s, got:\n%s", inst, out)
		}
	}
}

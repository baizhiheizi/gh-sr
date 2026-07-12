package ops

import (
	"bytes"
	"strings"
	"testing"

	"github.com/an-lee/gh-sr/internal/config"
	"github.com/an-lee/gh-sr/internal/host"
	"github.com/an-lee/gh-sr/internal/testutil"
)

func TestServiceCleanupDryRun(t *testing.T) {
	t.Parallel()
	installMockConnectHost(t, map[string]host.Executor{
		"h1": &testutil.MockExecutor{
			RunFn: func(cmd string) (string, error) {
				switch {
				case strings.Contains(cmd, `ls -1 "$HOME/.gh-sr/runners"`):
					return "stale-1\nactive-1\n", nil
				case strings.Contains(cmd, "for f in \"$HOME/.config/systemd/user/ghsr-runner-\""):
					return "ghsr-runner-stale-2\n", nil
				case strings.Contains(cmd, "echo D") && strings.Contains(cmd, "echo S"):
					// Combined Linux orphan-plan probe (orphanLinuxPlanProbe).
					// Dir present, no svc.sh, user systemd unit detected.
					return "D\nU\n", nil
				default:
					return "", nil
				}
			},
		},
	})

	cfg := &config.Config{
		Hosts: map[string]config.HostConfig{
			"h1": {Addr: "local", OS: "linux"},
		},
		Runners: []config.RunnerConfig{
			{Name: "active", Host: "h1", Repo: "o/r", Count: 1},
		},
	}
	var buf bytes.Buffer
	err := ServiceCleanup(&buf, cfg, "", true)
	if err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if strings.Contains(out, "active-1:") {
		t.Fatalf("should not touch configured instance, got:\n%s", out)
	}
	if !strings.Contains(out, "would remove autostart") {
		t.Fatalf("expected autostart cleanup preview, got:\n%s", out)
	}
	if !strings.Contains(out, "would remove orphan directory") {
		t.Fatalf("expected directory cleanup preview, got:\n%s", out)
	}
	if !strings.Contains(out, "Found 2 orphan instance(s)") {
		t.Fatalf("expected orphan count, got:\n%s", out)
	}
}

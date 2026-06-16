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
				if strings.Contains(cmd, "for f in \"$HOME/.config/systemd/user/ghsr-runner-\"") {
					return "ghsr-runner-stale-1\n", nil
				}
				if strings.Contains(cmd, "test -d") {
					return "yes\n", nil
				}
				return "", nil
			},
		},
	})

	cfg := &config.Config{Hosts: map[string]config.HostConfig{
		"h1": {Addr: "local", OS: "linux"},
	}}
	var buf bytes.Buffer
	err := ServiceCleanup(&buf, cfg, "", true)
	if err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "would remove stale autostart") {
		t.Fatalf("output: %s", out)
	}
	if !strings.Contains(out, "Found 1 stale autostart unit(s)") {
		t.Fatalf("output: %s", out)
	}
}

package ops

import (
	"bytes"
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

package runner

import (
	"strings"
	"testing"

	"github.com/an-lee/gh-sr/internal/config"
	"github.com/an-lee/gh-sr/internal/host"
	"github.com/an-lee/gh-sr/internal/testutil"
)

func TestOrphanInstances(t *testing.T) {
	t.Parallel()
	mock := &testutil.MockExecutor{
		RunFn: func(cmd string) (string, error) {
			switch {
			case strings.Contains(cmd, `ls -1 "$HOME/.gh-sr/runners"`):
				return "active-1\nold-1\n", nil
			case strings.Contains(cmd, "for f in \"$HOME/.config/systemd/user/ghsr-runner-\""):
				return "ghsr-runner-old-2\n", nil
			default:
				return "", nil
			}
		},
	}
	h := host.NewHost("linux", config.HostConfig{OS: "linux"})
	h.SetConn(mock)

	configured := map[string]struct{}{"active-1": {}}
	orphans, err := OrphanInstances(h, configured)
	if err != nil {
		t.Fatal(err)
	}
	if len(orphans) != 2 || orphans[0] != "old-1" || orphans[1] != "old-2" {
		t.Fatalf("got %v", orphans)
	}
}

func TestConfiguredInstanceSet(t *testing.T) {
	t.Parallel()
	runners := []config.RunnerConfig{
		{Name: "ci", Host: "h1", Repo: "o/r", Count: 2},
		{Name: "web", Host: "h2", Repo: "o/r", Count: 1},
	}
	set := ConfiguredInstanceSet(runners, "h1")
	if len(set) != 2 {
		t.Fatalf("got %d entries", len(set))
	}
	if _, ok := set["ci-1"]; !ok {
		t.Fatal("missing ci-1")
	}
	if _, ok := set["ci-2"]; !ok {
		t.Fatal("missing ci-2")
	}
}

func TestCleanupOrphanInstanceDryRun(t *testing.T) {
	t.Parallel()
	mock := &testutil.MockExecutor{
		RunFn: func(cmd string) (string, error) {
			switch {
			case strings.Contains(cmd, "&& echo user || true"):
				return "user\n", nil
			case strings.Contains(cmd, "test -d"):
				return "yes\n", nil
			case strings.Contains(cmd, "test -f") && strings.Contains(cmd, "svc.sh"):
				return "no\n", nil
			default:
				return "", nil
			}
		},
	}
	h := host.NewHost("linux", config.HostConfig{OS: "linux"})
	h.SetConn(mock)
	m := NewManager("")
	plan, err := m.CleanupOrphanInstance(h, "old-1", true)
	if err != nil {
		t.Fatal(err)
	}
	if !plan.Autostart || !plan.Directory {
		t.Fatalf("plan = %+v", plan)
	}
}

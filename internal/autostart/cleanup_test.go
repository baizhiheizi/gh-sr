package autostart

import (
	"strings"
	"testing"

	"github.com/an-lee/gh-sr/internal/config"
	"github.com/an-lee/gh-sr/internal/testutil"
)

func TestInstanceFromServiceBasename(t *testing.T) {
	t.Parallel()
	inst, ok := instanceFromServiceBasename("ghsr-runner-rune-x1-2")
	if !ok || inst != "rune-x1-2" {
		t.Fatalf("got %q ok=%v", inst, ok)
	}
	if _, ok := instanceFromServiceBasename("other.service"); ok {
		t.Fatal("expected false for unrelated basename")
	}
}

func TestDedupeInstances(t *testing.T) {
	t.Parallel()
	got := dedupeInstances([]string{"b", "a", "b"})
	if len(got) != 2 || got[0] != "a" || got[1] != "b" {
		t.Fatalf("got %v", got)
	}
}

func TestListInstalledLinux(t *testing.T) {
	t.Parallel()
	mock := &testutil.MockExecutor{Output: "ghsr-runner-ci-1\nghsr-runner-ci-2\n"}
	h := newMockHost("linux", config.HostConfig{OS: "linux"}, mock)
	got, err := ListInstalled(h)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0] != "ci-1" || got[1] != "ci-2" {
		t.Fatalf("got %v", got)
	}
	if len(mock.Calls) == 0 || !strings.Contains(mock.Calls[0], "ghsr-runner-") {
		t.Fatalf("unexpected cmd: %v", mock.Calls)
	}
}

func TestIsStaleLinux(t *testing.T) {
	t.Parallel()
	mock := &testutil.MockExecutor{Output: "yes\n"}
	h := newMockHost("linux", config.HostConfig{OS: "linux"}, mock)
	stale, err := IsStale(h, "missing-1")
	if err != nil {
		t.Fatal(err)
	}
	if !stale {
		t.Fatal("expected stale")
	}
}

func TestCleanupStaleDryRun(t *testing.T) {
	t.Parallel()
	mock := &testutil.MockExecutor{
		RunFn: func(cmd string) (string, error) {
			switch {
			case strings.Contains(cmd, "for f in \"$HOME/.config/systemd/user/ghsr-runner-\""):
				return "ghsr-runner-old-1\n", nil
			case strings.Contains(cmd, "test -d"):
				return "yes\n", nil
			default:
				return "", nil
			}
		},
	}
	h := newMockHost("linux", config.HostConfig{OS: "linux"}, mock)
	removed, found, err := CleanupStale(h, true)
	if err != nil {
		t.Fatal(err)
	}
	if found != 1 || len(removed) != 1 || removed[0] != "old-1" {
		t.Fatalf("found=%d removed=%v", found, removed)
	}
}

func TestCleanupStaleRemoves(t *testing.T) {
	t.Parallel()
	var uninstallCalled bool
	mock := &testutil.MockExecutor{
		RunFn: func(cmd string) (string, error) {
			switch {
			case strings.Contains(cmd, "systemctl --user disable"):
				uninstallCalled = true
				return "", nil
			case strings.Contains(cmd, "&& echo user || true"):
				return "user\n", nil
			case strings.Contains(cmd, "for f in \"$HOME/.config/systemd/user/ghsr-runner-\""):
				return "ghsr-runner-old-1\n", nil
			case strings.Contains(cmd, "test -d"):
				return "yes\n", nil
			default:
				return "", nil
			}
		},
	}
	h := newMockHost("linux", config.HostConfig{OS: "linux"}, mock)
	removed, found, err := CleanupStale(h, false)
	if err != nil {
		t.Fatal(err)
	}
	if found != 1 || len(removed) != 1 {
		t.Fatalf("found=%d removed=%v", found, removed)
	}
	if !uninstallCalled {
		t.Fatal("expected uninstall to run")
	}
}

func TestServiceActiveStateFailed(t *testing.T) {
	t.Parallel()
	mock := &testutil.MockExecutor{Output: "failed\n"}
	h := newMockHost("linux", config.HostConfig{OS: "linux"}, mock)
	state, err := ServiceActiveState(h, "ci-1", KindSystemdUser)
	if err != nil {
		t.Fatal(err)
	}
	if state != "failed" {
		t.Fatalf("got %q", state)
	}
}

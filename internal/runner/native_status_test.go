package runner

import (
	"strings"
	"testing"

	"github.com/an-lee/gh-sr/internal/config"
	"github.com/an-lee/gh-sr/internal/host"
	"github.com/an-lee/gh-sr/internal/testutil"
)

func TestStatusNativeServiceError(t *testing.T) {
	t.Parallel()
	mock := &testutil.MockExecutor{
		RunFn: func(cmd string) (string, error) {
			switch {
			case strings.Contains(cmd, "test -f") && strings.Contains(cmd, ".service"):
				return "user\n", nil
			case strings.Contains(cmd, "is-active"):
				return "activating\n", nil
			default:
				return "", nil
			}
		},
	}
	h := host.NewHost("linux", config.HostConfig{OS: "linux"})
	h.SetConn(mock)
	m := NewManager("")
	got := m.statusNative(h, "ci-1")
	if got != "service error" {
		t.Fatalf("got %q, want service error", got)
	}
}

func TestStatusNativeAutostartActive(t *testing.T) {
	t.Parallel()
	mock := &testutil.MockExecutor{
		RunFn: func(cmd string) (string, error) {
			switch {
			case strings.Contains(cmd, "test -f") && strings.Contains(cmd, ".service"):
				return "user\n", nil
			case strings.Contains(cmd, "is-active"):
				return "active\n", nil
			default:
				return "", nil
			}
		},
	}
	h := host.NewHost("linux", config.HostConfig{OS: "linux"})
	h.SetConn(mock)
	m := NewManager("")
	got := m.statusNative(h, "ci-1")
	if got != "running" {
		t.Fatalf("got %q, want running", got)
	}
}

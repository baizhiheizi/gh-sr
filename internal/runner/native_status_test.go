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
	calls := 0
	mock := &testutil.MockExecutor{
		RunFn: func(cmd string) (string, error) {
			calls++
			switch {
			case strings.Contains(cmd, ".config/systemd/user/") && strings.Contains(cmd, "/etc/systemd/system/"):
				return "U\n", nil
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
	// 1 combined probe + 1 is-active = 2 SSH round-trips (was 4 before the fold).
	if calls != 2 {
		t.Fatalf("SSH round-trips: got %d want 2", calls)
	}
}

func TestStatusNativeAutostartActive(t *testing.T) {
	t.Parallel()
	calls := 0
	mock := &testutil.MockExecutor{
		RunFn: func(cmd string) (string, error) {
			calls++
			switch {
			case strings.Contains(cmd, ".config/systemd/user/") && strings.Contains(cmd, "/etc/systemd/system/"):
				return "U\n", nil
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
	// 1 combined probe + 1 is-active = 2 SSH round-trips (was 4 before the fold).
	if calls != 2 {
		t.Fatalf("SSH round-trips: got %d want 2", calls)
	}
}

// TestStatusNative_LinuxSshRoundTripPins pins the post-fold contract: the
// svc.sh + autostart pre-check is a SINGLE SSH round-trip, not the previous
// pair (svcShPresent + autostart.Detect). A future refactor that re-introduces
// the second probe must update this test and surface the regression via the
// TUI's per-tick metrics.
func TestStatusNative_LinuxSshRoundTripPins(t *testing.T) {
	t.Parallel()
	mock := &testutil.MockExecutor{
		RunFn: func(cmd string) (string, error) {
			switch {
			case strings.Contains(cmd, ".config/systemd/user/") && strings.Contains(cmd, "/etc/systemd/system/"):
				return "U\n", nil
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
	calls := 0
	mock.RunFn = func(cmd string) (string, error) {
		calls++
		switch {
		case strings.Contains(cmd, ".config/systemd/user/") && strings.Contains(cmd, "/etc/systemd/system/"):
			return "U\n", nil
		case strings.Contains(cmd, "is-active"):
			return "active\n", nil
		default:
			return "", nil
		}
	}
	if got := m.statusNative(h, "ci-1"); got != "running" {
		t.Fatalf("got %q want running", got)
	}
	// Pre-fold would have been 3 (svcShPresent + Detect + is-active).
	// Post-fold is 2 (combined probe + is-active).
	if calls != 2 {
		t.Fatalf("SSH round-trips: got %d want 2 (combined probe must fold svc.sh+autostart)", calls)
	}
}

// TestStatusNative_NoSvcShellPath verifies the codepath when the combined probe
// emits no markers (runner not installed): only the combined probe runs, no
// is-active is issued (no autostart kind), and the function falls through to
// the PID-file block.
func TestStatusNative_NoSvcNoAutostart(t *testing.T) {
	t.Parallel()
	calls := 0
	mock := &testutil.MockExecutor{
		RunFn: func(cmd string) (string, error) {
			calls++
			switch {
			case strings.Contains(cmd, ".config/systemd/user/") && strings.Contains(cmd, "/etc/systemd/system/"):
				return "", nil
			case strings.Contains(cmd, "is-active"):
				return "active\n", nil
			default:
				return "not installed\n", nil
			}
		},
	}
	h := host.NewHost("linux", config.HostConfig{OS: "linux"})
	h.SetConn(mock)
	m := NewManager("")
	got := m.statusNative(h, "ci-1")
	if got != "not installed" {
		t.Fatalf("got %q want not installed", got)
	}
	// 1 combined probe + 1 PID-file probe = 2 SSH round-trips.
	if calls != 2 {
		t.Fatalf("SSH round-trips: got %d want 2", calls)
	}
}

// TestStatusNative_DarwinSingleProbe pins the non-Linux path: the helper
// delegates to autostart.Detect (1 SSH round-trip); svcSh is always false on
// darwin so the svc.sh branch is skipped. Pre-existing launchd double-check
// (ServiceActiveState + IsServiceActive) is left for a separate PR.
func TestStatusNative_DarwinSingleProbe(t *testing.T) {
	t.Parallel()
	calls := 0
	mock := &testutil.MockExecutor{
		RunFn: func(cmd string) (string, error) {
			calls++
			switch {
			case strings.Contains(cmd, "LaunchAgents"):
				return "yes\n", nil
			case strings.Contains(cmd, "launchctl print"):
				return "... state = running\n", nil
			default:
				return "", nil
			}
		},
	}
	h := host.NewHost("darwin", config.HostConfig{OS: "darwin"})
	h.SetConn(mock)
	m := NewManager("")
	if got := m.statusNative(h, "ci-1"); got != "running" {
		t.Fatalf("got %q want running", got)
	}
	if calls < 2 {
		t.Fatalf("SSH round-trips: got %d want at least 2 (1 Detect + 1 launchctl print)", calls)
	}
}

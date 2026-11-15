package runner

import (
	"errors"
	"strings"
	"testing"

	"github.com/an-lee/gh-sr/internal/config"
	"github.com/an-lee/gh-sr/internal/host"
	"github.com/an-lee/gh-sr/internal/testutil"
)

// The Linux per-instance tests mock the one-shot wire shape: one SSH round
// trip whose output is the same probe-tagged line the Status batch emits, so
// the per-instance path and the batch path are provably the same state
// machine (nativeStatusProbeBody) with two emit shapes.

func TestStatusNativeServiceError(t *testing.T) {
	t.Parallel()
	calls := 0
	mock := &testutil.MockExecutor{
		RunFn: func(cmd string) (string, error) {
			calls++
			if strings.Contains(cmd, nativeStatusBatchTag) {
				return nativeStatusBatchTag + "1|service error|\n", nil
			}
			return "", nil
		},
	}
	h := host.NewHost("linux", config.HostConfig{OS: "linux"})
	h.SetConn(mock)
	got, _ := statusNativeAndVersion(h, "ci-1")
	if got != "service error" {
		t.Fatalf("got %q, want service error", got)
	}
	// The one-shot answers state AND version in a single SSH round trip
	// (was 2 before the fragment unification, 3 before the fold).
	if calls != 1 {
		t.Fatalf("SSH round-trips: got %d want 1", calls)
	}
}

func TestStatusNativeAutostartActive(t *testing.T) {
	t.Parallel()
	calls := 0
	mock := &testutil.MockExecutor{
		RunFn: func(cmd string) (string, error) {
			calls++
			if strings.Contains(cmd, nativeStatusBatchTag) {
				return nativeStatusBatchTag + "1|running|\n", nil
			}
			return "", nil
		},
	}
	h := host.NewHost("linux", config.HostConfig{OS: "linux"})
	h.SetConn(mock)
	got, _ := statusNativeAndVersion(h, "ci-1")
	if got != "running" {
		t.Fatalf("got %q, want running", got)
	}
	if calls != 1 {
		t.Fatalf("SSH round-trips: got %d want 1", calls)
	}
}

// TestStatusNative_LinuxSshRoundTripPins pins the post-fragment contract: the
// per-instance Linux status is ONE SSH round trip running the same shell
// fragment the batch script embeds. A future refactor that re-introduces the
// Go-orchestrated per-branch probes must update this test and surface the
// regression via the TUI's per-tick metrics.
func TestStatusNative_LinuxSshRoundTripPins(t *testing.T) {
	t.Parallel()
	calls := 0
	mock := &testutil.MockExecutor{
		RunFn: func(cmd string) (string, error) {
			calls++
			if strings.Contains(cmd, nativeStatusBatchTag) {
				return nativeStatusBatchTag + "1|running|\n", nil
			}
			return "", nil
		},
	}
	h := host.NewHost("linux", config.HostConfig{OS: "linux"})
	h.SetConn(mock)
	if got, _ := statusNativeAndVersion(h, "ci-1"); got != "running" {
		t.Fatalf("got %q want running", got)
	}
	if calls != 1 {
		t.Fatalf("SSH round-trips: got %d want 1 (per-instance path must run the batch fragment one-shot)", calls)
	}
}

// TestStatusNative_OneShotErrorFallsBack preserves the transient-failure
// contract: a one-shot SSH error falls through to the Go-orchestrated
// combined-probe path instead of blanking the Status row.
func TestStatusNative_OneShotErrorFallsBack(t *testing.T) {
	t.Parallel()
	calls := 0
	mock := &testutil.MockExecutor{
		RunFn: func(cmd string) (string, error) {
			calls++
			if strings.Contains(cmd, nativeStatusBatchTag) {
				return "", errors.New("transient SSH failure")
			}
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
	got, _ := statusNativeAndVersion(h, "ci-1")
	if got != "running" {
		t.Fatalf("got %q, want running from the fallback path", got)
	}
	// 1 failed one-shot + 1 combined probe + 1 is-active + 1 version probe.
	if calls != 4 {
		t.Fatalf("SSH round-trips: got %d want 4", calls)
	}
}

// TestStatusNativeAndVersion_LinuxVersionFold pins the version-fold contract:
// the one-shot answers BOTH local state and runner version in the same SSH
// round trip (pre-fold the call had to invoke nativeRunnerVersion separately,
// a second SSH per instance).
func TestStatusNativeAndVersion_LinuxVersionFold(t *testing.T) {
	t.Parallel()
	calls := 0
	mock := &testutil.MockExecutor{
		RunFn: func(cmd string) (string, error) {
			calls++
			if strings.Contains(cmd, nativeStatusBatchTag) {
				return nativeStatusBatchTag + "1|running|1.0.0\n", nil
			}
			return "", nil
		},
	}
	h := host.NewHost("linux", config.HostConfig{OS: "linux"})
	h.SetConn(mock)

	local, version := statusNativeAndVersion(h, "ci-1")
	if local != "running" {
		t.Fatalf("local: got %q want running", local)
	}
	if version != "1.0.0" {
		t.Fatalf("version: got %q want 1.0.0", version)
	}
	if calls != 1 {
		t.Fatalf("SSH round-trips: got %d want 1 (version folded into the one-shot)", calls)
	}
}

// TestStatusNativeAndVersion_LinuxVersionMissing pins the contract that an
// absent .runner-version file surfaces as "-" (the prior nativeRunnerVersion
// default), keeping the TUI BUILD-cell render identical.
func TestStatusNativeAndVersion_LinuxVersionMissing(t *testing.T) {
	t.Parallel()
	mock := &testutil.MockExecutor{
		RunFn: func(cmd string) (string, error) {
			if strings.Contains(cmd, nativeStatusBatchTag) {
				return nativeStatusBatchTag + "1|stopped|\n", nil
			}
			return "", nil
		},
	}
	h := host.NewHost("linux", config.HostConfig{OS: "linux"})
	h.SetConn(mock)

	local, version := statusNativeAndVersion(h, "ci-1")
	if local != "stopped" {
		t.Fatalf("local: got %q want stopped", local)
	}
	if version != "-" {
		t.Fatalf("version: got %q want -", version)
	}
}

// TestStatusNative_DarwinSingleProbe pins the non-Linux path: the helper
// delegates to autostart.Detect (1 SSH round-trip); svcSh is always false on
// darwin so the svc.sh branch is skipped. Pre-existing launchd double-check
// (ServiceActiveState + IsServiceActive) is left for a separate PR.
// Status() on the non-Linux path additionally issues one version probe
// (.runner-version), so the total is 1 Detect + 1 launchctl print + 1
// version = 3 SSH round-trips.
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
			case strings.Contains(cmd, ".runner-version"):
				return "1.0.0\n", nil
			default:
				return "", nil
			}
		},
	}
	h := host.NewHost("darwin", config.HostConfig{OS: "darwin"})
	h.SetConn(mock)
	if got, _ := statusNativeAndVersion(h, "ci-1"); got != "running" {
		t.Fatalf("got %q want running", got)
	}
	if calls < 2 {
		t.Fatalf("SSH round-trips: got %d want at least 2 (1 Detect + 1 launchctl print)", calls)
	}
}

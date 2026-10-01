package autostart

import (
	"strings"
	"testing"

	"github.com/an-lee/gh-sr/internal/config"
	"github.com/an-lee/gh-sr/internal/testutil"
)

// TestDispatchAction covers dispatchAction directly: unknown kinds must
// error, and the KindNone precondition stays with the callers (Start/Stop
// error via resolveInstalledTarget, Uninstall no-ops).
func TestDispatchAction(t *testing.T) {
	t.Parallel()

	t.Run("unknown kind returns error", func(t *testing.T) {
		t.Parallel()
		h := newMockHost("h1", config.HostConfig{OS: "linux"}, &testutil.MockExecutor{})
		err := dispatchAction(h, Kind("bogus"), "ci-1", "ghsr-runner-ci-1", actionStart)
		if err == nil {
			t.Fatal("expected error for unknown kind, got nil")
		}
		if !strings.Contains(err.Error(), "unknown autostart kind") {
			t.Errorf("error = %q; want mention of unknown kind", err.Error())
		}
	})

	t.Run("systemd-user start and stop share the verb-dispatch path", func(t *testing.T) {
		t.Parallel()
		mock := &testutil.MockExecutor{}
		h := newMockHost("h1", config.HostConfig{OS: "linux"}, mock)
		if err := dispatchAction(h, KindSystemdUser, "ci-1", "ghsr-runner-ci-1", actionStart); err != nil {
			t.Fatalf("start: %v", err)
		}
		if err := dispatchAction(h, KindSystemdUser, "ci-1", "ghsr-runner-ci-1", actionStop); err != nil {
			t.Fatalf("stop: %v", err)
		}
		if len(mock.Calls) != 2 {
			t.Fatalf("got %d calls; want 2", len(mock.Calls))
		}
		if !strings.Contains(mock.Calls[0], "systemctl --user start ghsr-runner-ci-1.service") {
			t.Errorf("call[0] = %q; want user start", mock.Calls[0])
		}
		if !strings.Contains(mock.Calls[1], "systemctl --user stop ghsr-runner-ci-1.service") {
			t.Errorf("call[1] = %q; want user stop", mock.Calls[1])
		}
	})

	t.Run("resolveInstalledTarget rejects KindNone", func(t *testing.T) {
		t.Parallel()
		// Mock output "\n" → Detect resolves KindNone → Start/Stop preamble fails.
		h := newMockHost("h1", config.HostConfig{OS: "linux"}, &testutil.MockExecutor{Output: "\n"})
		_, _, _, err := resolveInstalledTarget(h, "ci-1")
		if err == nil {
			t.Fatal("expected error for KindNone, got nil")
		}
		if !strings.Contains(err.Error(), "ci-1") {
			t.Errorf("error = %q; want instance name", err.Error())
		}
	})
}

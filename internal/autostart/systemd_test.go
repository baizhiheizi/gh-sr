package autostart

import (
	"strings"
	"testing"
)

// These tests pin the exact shell output of the four systemd script helpers
// added in issue #276. The scripts are byte-for-byte equivalents of the inline
// copies they replace in autostart.go, so any future drift in the
// daemon-reload + enable/disable + restart||start choreography trips a golden-
// string assertion here rather than silently diverging between the user and
// system install/uninstall paths.

func TestSystemdEnableUserScript(t *testing.T) {
	t.Parallel()
	got := systemdEnableUserScript("ghsr-runner-ci-1")
	const want = `set -e
systemctl --user daemon-reload
systemctl --user enable ghsr-runner-ci-1.service
systemctl --user restart ghsr-runner-ci-1.service || systemctl --user start ghsr-runner-ci-1.service
`
	if got != want {
		t.Errorf("systemdEnableUserScript mismatch:\ngot:  %q\nwant: %q", got, want)
	}
}

func TestSystemdEnableSystemScript(t *testing.T) {
	t.Parallel()
	got := systemdEnableSystemScript("ghsr-runner-ci-1", "/home/runner/.gh-sr/ghsr-runner-ci-1.service.tmp", "/etc/systemd/system/ghsr-runner-ci-1.service")
	// The sudo prelude is contributed by sudoPrelude() (already tested via
	// hostshell.LinuxElevatePrelude); assert only the install-specific tail so
	// this test does not break when the failure-message wording changes.
	const wantTail = `
set -e
$SUDO mv '/home/runner/.gh-sr/ghsr-runner-ci-1.service.tmp' '/etc/systemd/system/ghsr-runner-ci-1.service'
$SUDO systemctl daemon-reload
$SUDO systemctl enable ghsr-runner-ci-1.service
$SUDO systemctl restart ghsr-runner-ci-1.service || $SUDO systemctl start ghsr-runner-ci-1.service
`
	if !strings.HasSuffix(got, wantTail) {
		t.Errorf("systemdEnableSystemScript tail mismatch:\ngot:  %q\nwant tail: %q", got, wantTail)
	}
}

func TestSystemdDisableUserScript(t *testing.T) {
	t.Parallel()
	got := systemdDisableUserScript("ghsr-runner-ci-1")
	const want = `set -e
systemctl --user disable --now ghsr-runner-ci-1.service 2>/dev/null || true
rm -f "$HOME/.config/systemd/user/ghsr-runner-ci-1.service"
systemctl --user daemon-reload
`
	if got != want {
		t.Errorf("systemdDisableUserScript mismatch:\ngot:  %q\nwant: %q", got, want)
	}
}

func TestSystemdDisableSystemScript(t *testing.T) {
	t.Parallel()
	got := systemdDisableSystemScript("ghsr-runner-ci-1")
	const wantTail = `
set -e
$SUDO systemctl disable --now ghsr-runner-ci-1.service 2>/dev/null || true
$SUDO rm -f /etc/systemd/system/ghsr-runner-ci-1.service
$SUDO systemctl daemon-reload
`
	if !strings.HasSuffix(got, wantTail) {
		t.Errorf("systemdDisableSystemScript tail mismatch:\ngot:  %q\nwant tail: %q", got, wantTail)
	}
}

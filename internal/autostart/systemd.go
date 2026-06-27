package autostart

import (
	"fmt"

	"github.com/an-lee/gh-sr/internal/hostshell"
)

// systemdEnableUserScript returns the `set -e` shell snippet that activates a
// freshly-written user-level systemd unit: daemon-reload, enable, then restart
// (falling back to start when the unit has no prior instance to restart). base
// is the unit basename without the .service suffix.
//
// This is the user-scoped sibling of systemdEnableSystemScript. Both share the
// daemon-reload + enable + `restart || start` choreography; they are factored
// apart so the user/system distinction (no $SUDO prefix vs. $SUDO prefix, plus
// the system variant's tmp→/etc mv) stays readable (see issue #276).
func systemdEnableUserScript(base string) string {
	return fmt.Sprintf(`set -e
systemctl --user daemon-reload
systemctl --user enable %s.service
systemctl --user restart %s.service || systemctl --user start %s.service
`, base, base, base)
}

// systemdEnableSystemScript returns the sudo-prefixed shell snippet that
// installs and activates a system-level systemd unit: it moves the staged unit
// from tmpPath to sysPath (quoted via PosixSingleQuote), then daemon-reloads,
// enables, and restart||start the unit. tmpPath and sysPath are the staged and
// final unit paths; base is the unit basename without the .service suffix.
func systemdEnableSystemScript(base, tmpPath, sysPath string) string {
	return sudoPrelude() + fmt.Sprintf(`
set -e
$SUDO mv %s %s
$SUDO systemctl daemon-reload
$SUDO systemctl enable %s.service
$SUDO systemctl restart %s.service || $SUDO systemctl start %s.service
`,
		hostshell.PosixSingleQuote(tmpPath),
		hostshell.PosixSingleQuote(sysPath),
		base, base, base)
}

// systemdDisableUserScript returns the `set -e` shell snippet that uninstalls
// a user-level systemd unit: disable --now (best-effort), remove the unit file
// from $HOME/.config/systemd/user/, and daemon-reload. base is the unit
// basename without the .service suffix.
func systemdDisableUserScript(base string) string {
	return fmt.Sprintf(`set -e
systemctl --user disable --now %s.service 2>/dev/null || true
rm -f "$HOME/.config/systemd/user/%s.service"
systemctl --user daemon-reload
`, base, base)
}

// systemdDisableSystemScript returns the sudo-prefixed shell snippet that
// uninstalls a system-level systemd unit: disable --now (best-effort), remove
// /etc/systemd/system/<base>.service, and daemon-reload. base is the unit
// basename without the .service suffix.
func systemdDisableSystemScript(base string) string {
	return sudoPrelude() + fmt.Sprintf(`
set -e
$SUDO systemctl disable --now %s.service 2>/dev/null || true
$SUDO rm -f /etc/systemd/system/%s.service
$SUDO systemctl daemon-reload
`, base, base)
}

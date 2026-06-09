package autostart

import "github.com/an-lee/gh-sr/internal/hostshell"

// sudoPrelude returns the hostshell.LinuxElevatePrelude fragment used by all
// autostart-side shell helpers that need non-interactive root or passwordless
// sudo. The user-facing failure message is the autostart-specific one.
func sudoPrelude() string {
	return hostshell.LinuxElevatePrelude(
		"gh sr: system-level autostart needs root SSH or passwordless sudo (non-interactive)",
	)
}

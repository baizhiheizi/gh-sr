package runner

import "github.com/an-lee/gh-sr/internal/hostshell"

// sudoPrelude returns the hostshell.LinuxElevatePrelude fragment used by all
// runner-side shell helpers that need non-interactive root or passwordless sudo
// over SSH. The user-facing failure message is the runner-specific one (it
// mentions `gh sr doctor`).
func sudoPrelude() string {
	return hostshell.LinuxElevatePrelude(
		"gh sr: remote Linux commands need root SSH or passwordless sudo (non-interactive); SSH has no TTY for sudo passwords. Use NOPASSWD, connect as root, or install software manually. Run: gh sr doctor",
	)
}

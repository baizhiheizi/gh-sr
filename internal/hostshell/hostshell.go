// Package hostshell provides shell-quoting and remote-write helpers shared by
// the runner and autostart packages. It centralises the byte-for-byte identical
// helpers that used to live in both packages so they cannot drift apart.
package hostshell

import (
	"encoding/base64"
	"fmt"
	"strings"

	"github.com/an-lee/gh-sr/internal/host"
)

// PosixSingleQuote wraps s in single quotes for POSIX shell (safe for remote sh -c).
// Single quotes inside s are escaped using the standard POSIX idiom: ' -> '\”.
func PosixSingleQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
}

// PowerShellSingleQuote wraps s in single quotes for PowerShell (safe for remote powershell -c).
// Single quotes inside s are doubled, which is the PowerShell escape rule.
func PowerShellSingleQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "''") + "'"
}

// PlistEscape escapes s for embedding in XML/plist string values.
func PlistEscape(s string) string {
	s = strings.ReplaceAll(s, `&`, `&amp;`)
	s = strings.ReplaceAll(s, `"`, `&quot;`)
	s = strings.ReplaceAll(s, `<`, `&lt;`)
	s = strings.ReplaceAll(s, `>`, `&gt;`)
	return s
}

// WriteRemoteBytes writes data to a remote path using base64 (POSIX) or PowerShell (Windows).
// Parent directories are created on the host.
func WriteRemoteBytes(h *host.Host, remotePath string, data []byte) error {
	if h.OS == "windows" {
		b64 := base64.StdEncoding.EncodeToString(data)
		ps := fmt.Sprintf(
			"$p = %s; $d = [Convert]::FromBase64String(%s); $dir = Split-Path -Parent $p; New-Item -ItemType Directory -Force -Path $dir | Out-Null; [IO.File]::WriteAllBytes($p, $d)",
			PowerShellSingleQuote(remotePath),
			PowerShellSingleQuote(b64),
		)
		_, err := h.RunShell(ps)
		return err
	}
	b64 := base64.StdEncoding.EncodeToString(data)
	qpath := PosixSingleQuote(remotePath)
	cmd := fmt.Sprintf(`set -e; d=$(dirname %s); mkdir -p "$d"; printf '%%s' %s | base64 -d > %s`,
		qpath, PosixSingleQuote(b64), qpath)
	_, err := h.Run(cmd)
	return err
}

// LinuxElevatePrelude returns a shell fragment for non-interactive SSH sessions
// that sets $SUDO to ” when already root, or to "sudo -n" when passwordless
// sudo works. If neither is true, it prints failureMsg to stderr and exits 1.
// Plain sudo is unsafe here because SSH Run() has no TTY.
//
// failureMsg is package-specific so each caller can keep its own user-facing
// wording (e.g. runner mentions `gh sr doctor`; autostart mentions autostart).
func LinuxElevatePrelude(failureMsg string) string {
	return `
SUDO=''
if [ "$(id -u)" -ne 0 ]; then
	if command -v sudo >/dev/null 2>&1 && sudo -n true 2>/dev/null; then
		SUDO='sudo -n'
	else
		echo ` + PosixSingleQuote(failureMsg) + ` >&2
		exit 1
	fi
fi
`
}

// LinuxElevatePreludeSoft is the soft-failure sibling of LinuxElevatePrelude:
// it sets $SUDO to "sudo -n" when passwordless sudo works, or leaves it empty
// otherwise. It never prints or exits — callers are expected to gate usage of
// "$SUDO" with `if [ -n "$SUDO" ] || [ "$(id -u)" -eq 0 ]` (or similar) so the
// surrounding script can keep going and report the per-command failure rather
// than aborting the entire pipeline.
//
// Use this in scripts that run several elevated commands sequentially and need
// to surface each one's failure individually (e.g. disk prune, dir removal).
// For commands that have a single natural failure mode, prefer the strict
// LinuxElevatePrelude so the user gets a clear, early error.
func LinuxElevatePreludeSoft() string {
	return `
SUDO=''
if [ "$(id -u)" -ne 0 ]; then
	if command -v sudo >/dev/null 2>&1 && sudo -n true 2>/dev/null; then
		SUDO='sudo -n'
	fi
fi
`
}

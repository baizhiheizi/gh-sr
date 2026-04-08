package runner

// linuxElevatePrelude is a shell fragment for non-interactive SSH sessions: set SUDO to
// empty when already root, or to "sudo -n" when passwordless sudo works; otherwise print
// to stderr and exit 1. Plain sudo is unsafe here because SSH Run() has no TTY.
const linuxElevatePrelude = `
SUDO=''
if [ "$(id -u)" -ne 0 ]; then
	if command -v sudo >/dev/null 2>&1 && sudo -n true 2>/dev/null; then
		SUDO='sudo -n'
	else
		echo 'gh wm: remote Linux commands need root SSH or passwordless sudo (non-interactive); SSH has no TTY for sudo passwords. Use NOPASSWD, connect as root, or install software manually. Run: gh wm doctor' >&2
		exit 1
	fi
fi
`

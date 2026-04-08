package autostart

// linuxElevatePrelude matches runner.linuxElevatePrelude for non-interactive sudo on remote Linux.
const linuxElevatePrelude = `
SUDO=''
if [ "$(id -u)" -ne 0 ]; then
	if command -v sudo >/dev/null 2>&1 && sudo -n true 2>/dev/null; then
		SUDO='sudo -n'
	else
		echo 'gh wm: system-level autostart needs root SSH or passwordless sudo (non-interactive)' >&2
		exit 1
	fi
fi
`

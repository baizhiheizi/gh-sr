package autostart

import (
	"fmt"
	"strings"

	"github.com/an-lee/gh-sr/internal/host"
)

// IsServiceActive reports whether the autostart supervisor considers the job running.
func IsServiceActive(h *host.Host, instance string, kind Kind) (bool, error) {
	san, err := SanitizeInstance(instance)
	if err != nil {
		return false, err
	}
	base := ServiceBasename(san)

	switch kind {
	case KindSystemdUser:
		out, err := h.Run(fmt.Sprintf(`systemctl --user is-active %s.service 2>/dev/null || echo inactive`, base))
		if err != nil {
			return false, err
		}
		return strings.TrimSpace(out) == "active", nil

	case KindSystemdSystem:
		script := linuxElevatePrelude + fmt.Sprintf(`
$SUDO systemctl is-active %s.service 2>/dev/null || echo inactive
`, base)
		out, err := h.Run(script)
		if err != nil {
			return false, err
		}
		return strings.TrimSpace(out) == "active", nil

	case KindLaunchd:
		label := LaunchdLabel(san)
		out, err := h.Run(launchdPrintScript(posixSingleQuote(label)))
		if err != nil {
			return false, err
		}
		return strings.Contains(out, "state = running"), nil

	case KindWindowsTask:
		name := WindowsTaskName(san)
		ps := fmt.Sprintf(
			`(Get-ScheduledTask -TaskName %s -ErrorAction SilentlyContinue | Select-Object -ExpandProperty State)`,
			powerShellSingleQuoted(name),
		)
		out, err := h.RunShell(ps)
		if err != nil {
			return false, err
		}
		return strings.EqualFold(strings.TrimSpace(out), "Running"), nil

	default:
		return false, nil
	}
}

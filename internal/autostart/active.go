package autostart

import (
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

	out, err := runActiveCheck(h, kind, san, base)
	if err != nil {
		return false, err
	}

	switch kind {
	case KindSystemdUser, KindSystemdSystem:
		return strings.TrimSpace(out) == "active", nil
	case KindLaunchd:
		return strings.Contains(out, "state = running"), nil
	case KindWindowsTask:
		return strings.EqualFold(strings.TrimSpace(out), "Running"), nil
	}
	return false, nil
}

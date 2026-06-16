package autostart

import (
	"strings"

	"github.com/an-lee/gh-sr/internal/host"
)

// IsServiceActive reports whether the autostart supervisor considers the job running.
func IsServiceActive(h *host.Host, instance string, kind Kind) (bool, error) {
	state, err := ServiceActiveState(h, instance, kind)
	if err != nil {
		return false, err
	}

	switch kind {
	case KindSystemdUser, KindSystemdSystem:
		return state == "active", nil
	case KindLaunchd:
		return strings.Contains(state, "state = running"), nil
	case KindWindowsTask:
		return strings.EqualFold(state, "Running"), nil
	}
	return false, nil
}

// ServiceActiveState returns the raw supervisor state string (e.g. active, inactive, failed, activating).
func ServiceActiveState(h *host.Host, instance string, kind Kind) (string, error) {
	san, err := SanitizeInstance(instance)
	if err != nil {
		return "", err
	}
	base := ServiceBasename(san)

	out, err := runActiveCheck(h, kind, san, base)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}

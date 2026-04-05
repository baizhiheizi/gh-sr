package autostart

import (
	"fmt"
	"strings"
)

// SanitizeInstance maps a runner instance name to a safe token for unit filenames,
// launchd labels, and scheduled task names.
func SanitizeInstance(instance string) (string, error) {
	var b strings.Builder
	for _, r := range instance {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-':
			b.WriteRune(r)
		default:
			b.WriteByte('-')
		}
	}
	s := strings.Trim(b.String(), "-")
	for strings.Contains(s, "--") {
		s = strings.ReplaceAll(s, "--", "-")
	}
	if s == "" {
		return "", fmt.Errorf("invalid instance name %q after sanitization", instance)
	}
	return s, nil
}

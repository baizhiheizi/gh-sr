package autostart

import "strings"

// posixSingleQuote wraps s in single quotes for POSIX shell (safe for remote sh -c).
func posixSingleQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
}

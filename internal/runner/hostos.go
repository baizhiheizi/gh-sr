package runner

import (
	"fmt"

	"github.com/an-lee/gh-sr/internal/host"
)

// runOnHostOS dispatches between a Windows and a POSIX callback based on
// h.OS. The "linux, darwin" POSIX allow-list matches the set of host OSes
// DetectOS reports (see internal/host/detect.go); any other h.OS value
// returns an explicit error rather than silently falling into the POSIX
// branch — the latter would be a footgun for future DetectOS extensions
// (freebsd, illumos, ...) or typos in runner config. Closes #135.
func runOnHostOS[T any](h *host.Host, win, posix func() (T, error)) (T, error) {
	var zero T
	switch h.OS {
	case "windows":
		return win()
	case "linux", "darwin":
		return posix()
	default:
		return zero, fmt.Errorf("unsupported host OS %q", h.OS)
	}
}

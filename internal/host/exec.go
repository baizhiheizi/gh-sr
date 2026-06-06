package host

import (
	"bytes"
	"fmt"
	"io"
	"strings"
)

// runWithCapture is the shared contract for Executor's Run implementations: it
// captures stdout/stderr into in-memory buffers, returns the trimmed stdout on
// success, and returns a wrapped error (including the failed command and the
// captured stderr) on failure.
//
// The run callback wires the supplied writers to whatever process or session is
// running the command. Each Executor implementation provides a small closure
// that performs that wiring and invokes the underlying Run, leaving the
// capture, error-wrapping, and trim policy in one place.
func runWithCapture(cmd string, run func(stdout, stderr io.Writer) error) (string, error) {
	var stdout, stderr bytes.Buffer
	if err := run(&stdout, &stderr); err != nil {
		return stdout.String(), fmt.Errorf("running %q: %w\nstderr: %s", cmd, err, stderr.String())
	}
	return strings.TrimSpace(stdout.String()), nil
}

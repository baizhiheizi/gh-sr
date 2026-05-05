package agentic

import (
	"strings"
	"testing"
)

func TestContainerInnerNetworkCheckCommand(t *testing.T) {
	t.Parallel()

	cmd := containerInnerNetworkCheckCommand("gh-sr-rune-agentic-1")

	for _, want := range []string{
		"docker exec",
		"gh-sr-rune-agentic-1",
		"getent hosts host.docker.internal",
		"docker run --rm alpine",
		"docker run --rm --network host alpine",
		"set -- \\$(getent hosts host.docker.internal 2>/dev/null || true); ip=\\${1:-}",
		"--add-host=host.docker.internal:host-gateway",
		"hg_ok",
	} {
		if !strings.Contains(cmd, want) {
			t.Fatalf("expected command to contain %q, got:\n%s", want, cmd)
		}
	}
}

func TestContainerAWFCheckCommand(t *testing.T) {
	t.Parallel()

	cmd := containerAWFCheckCommand("gh-sr-rune-agentic-1")

	for _, want := range []string{
		"docker exec",
		"gh-sr-rune-agentic-1",
		"command -v awf",
		"sudo -n -E awf --version",
	} {
		if !strings.Contains(cmd, want) {
			t.Fatalf("expected command to contain %q, got:\n%s", want, cmd)
		}
	}
}

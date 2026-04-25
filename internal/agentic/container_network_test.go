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
		"wget -qO- --timeout=2 http://host.docker.internal:",
	} {
		if !strings.Contains(cmd, want) {
			t.Fatalf("expected command to contain %q, got:\n%s", want, cmd)
		}
	}
}

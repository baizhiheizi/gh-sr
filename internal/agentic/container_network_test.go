package agentic

import (
	"strings"
	"testing"
)

func TestContainerInnerNetworkCheckCommand(t *testing.T) {
	t.Parallel()

	cmd := containerInnerNetworkCheckCommand("gh-sr-rune-agentic-1")

	// Must require the real baked-DNS path (plain default-bridge resolution),
	// rejecting loopback, and must NOT mask failure with an --add-host fallback.
	for _, want := range []string{
		"docker exec",
		"gh-sr-rune-agentic-1",
		"docker run --rm alpine getent hosts host.docker.internal",
		"127.*",
		"ok=1",
	} {
		if !strings.Contains(cmd, want) {
			t.Fatalf("expected command to contain %q, got:\n%s", want, cmd)
		}
	}
	if strings.Contains(cmd, "--add-host") {
		t.Fatalf("inner-network check must not accept an --add-host fallback (masks broken baked DNS), got:\n%s", cmd)
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

func TestContainerInnerResolvCheckCommand(t *testing.T) {
	t.Parallel()

	cmd := containerInnerResolvCheckCommand("gh-sr-rune-agentic-1")

	// Must read the runner container's resolv.conf and require the live inner docker0
	// gateway (default 10.200.0.1) as the nameserver — the pin that makes gh-aw's
	// auto-detected sandbox DNS authoritative for host.docker.internal.
	for _, want := range []string{
		"docker exec",
		"gh-sr-rune-agentic-1",
		"/etc/resolv.conf",
		"docker0",
		"10.200.0.1",
		"^nameserver",
	} {
		if !strings.Contains(cmd, want) {
			t.Fatalf("expected command to contain %q, got:\n%s", want, cmd)
		}
	}
}

func TestContainerAWFServiceRoutingCheckCommand(t *testing.T) {
	t.Parallel()

	cmd := containerAWFServiceRoutingCheckCommand("gh-sr-rune-agentic-1")

	for _, want := range []string{
		"docker exec",
		"gh-sr-rune-agentic-1",
		"iptables -t nat -S PREROUTING",
		"-A PREROUTING -s 172.30.0.0/24 -m addrtype --dst-type LOCAL -j RETURN",
	} {
		if !strings.Contains(cmd, want) {
			t.Fatalf("expected command to contain %q, got:\n%s", want, cmd)
		}
	}
}

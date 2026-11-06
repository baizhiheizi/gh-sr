// Package dockerutil holds the Docker plumbing shared by the per-host service
// deploy paths (cache server, registry mirror): docker0 gateway resolution,
// bind-address resolution, image pulls, and the idempotent container
// lifecycle. Its deep core is Converge — the converge state machine and its
// helpers (containerState, layoutCurrent) live here so the cache and registry
// packages cannot drift. The inspection helpers are implementation details;
// the exported surface is Converge plus the small set of probe/deploy helpers
// the two adapters actually call. Gateway resolution and bind selection share
// one core (bindGateway) so reporting and deploy paths cannot disagree.
package dockerutil

import (
	"fmt"
	"io"
	"strings"

	"github.com/an-lee/gh-sr/internal/host"
	"github.com/an-lee/gh-sr/internal/hostshell"
)

// ResolveGatewayIP returns the docker0 bridge gateway IPv4 on the host — the
// address runner containers use to reach host-published ports. Empty when the
// host has no docker0 interface (rootless/podman setups).
func ResolveGatewayIP(h *host.Host) (string, error) {
	out, err := h.Run("ip -4 -o addr show docker0 2>/dev/null")
	if err != nil {
		return "", err
	}
	return ParseGatewayIPOutput(out), nil
}

// ParseGatewayIPOutput extracts the IPv4 address from `ip -4 -o addr show
// docker0` output: "2: docker0    inet 172.17.0.1/16 brd ... " → "172.17.0.1".
// Exported because the cache one-shot Inspect script resolves the gateway
// shell-side and parses the captured line host-side.
func ParseGatewayIPOutput(out string) string {
	// SplitSeq avoids the upfront []string allocation that strings.Split makes.
	for line := range strings.SplitSeq(out, "\n") {
		fields := strings.Fields(line)
		for i, f := range fields {
			if f != "inet" || i+1 >= len(fields) {
				continue
			}
			ip := fields[i+1]
			if idx := strings.Index(ip, "/"); idx >= 0 {
				ip = ip[:idx]
			}
			return ip
		}
	}
	return ""
}

// containerState reports "running", "exists" (present but not running) or ""
// (absent) for the named container. It is an implementation detail of
// Converge, not part of the package interface.
func containerState(h *host.Host, cname string) (string, error) {
	out, err := h.Run(fmt.Sprintf(
		"docker inspect --format='{{.State.Status}}' %s 2>/dev/null || true", cname))
	if err != nil {
		return "", fmt.Errorf("inspecting container %s: %w", cname, err)
	}
	state := strings.TrimSpace(out)
	switch state {
	case "running":
		return "running", nil
	case "":
		return "", nil
	default:
		return "exists", nil
	}
}

// layoutCurrent reports whether an existing container carries the current
// deploy layout label revision. Used by Converge to converge deployments made
// by older gh-sr versions (or after a layout change): a stale container cannot
// be updated in place (its env and ports are baked at create), so it is
// recreated. The label value must equal rev exactly — prefix matches (rev v3
// vs a label "v33") are stale, not current. It is an implementation detail
// of Converge, not part of the package interface.
func layoutCurrent(h *host.Host, cname, label, rev string) (bool, error) {
	out, err := h.Run(fmt.Sprintf(
		"docker inspect --format '{{index .Config.Labels %q}}' %s 2>/dev/null || true", label, cname))
	if err != nil {
		return false, fmt.Errorf("inspecting container %s layout: %w", cname, err)
	}
	return strings.TrimSpace(out) == rev, nil
}

// bindGateway resolves the service bind address and the docker0 gateway in one
// place. An explicit bind wins. Otherwise the gateway is the bind address; when
// none is found it falls back to 0.0.0.0 and gatewayMissing is true. gw carries
// the resolved gateway only on the auto path that found one, so callers that
// build a runner-facing URL skip a second round-trip; it is empty for an
// explicit bind or the fallback. Both the reporting (EffectiveBind) and
// deploy-time (ResolveBind) paths share this core so their gateway/fallback
// logic cannot drift.
func bindGateway(h *host.Host, bind string) (addr, gw string, gatewayMissing bool) {
	if bind != "" {
		return bind, "", false
	}
	gwIP, err := ResolveGatewayIP(h)
	if err != nil || gwIP == "" {
		return "0.0.0.0", "", true
	}
	return gwIP, gwIP, false
}

// EffectiveBind resolves the bind address a service deploy actually chose: an
// explicit bind wins; the auto path binds the docker0 gateway, or 0.0.0.0 when
// no gateway was found. Silent — for reporting and remediation paths that
// describe an existing deployment rather than make one.
func EffectiveBind(h *host.Host, bind string) string {
	addr, _, _ := bindGateway(h, bind)
	return addr
}

// ResolveBind is the deploy-time counterpart of EffectiveBind: it prints the
// 0.0.0.0 fallback warning (svc prefixes the progress line, exposure describes
// what an all-interfaces bind exposes) and returns the gateway IP alongside
// the chosen bind so callers that need it for a runner-facing URL skip a
// second round-trip. bind "" selects the auto path.
func ResolveBind(w io.Writer, h *host.Host, svc, bind, exposure string) (string, string) {
	addr, gw, gatewayMissing := bindGateway(h, bind)
	if gatewayMissing {
		fmt.Fprintf(w, "  %s: warning: docker0 gateway not found; binding %s (%s)\n", svc, addr, exposure)
	}
	return addr, gw
}

// PullImage pulls the service image best-effort: a pre-pulled image must
// still deploy on hosts without registry access. A genuinely missing image
// fails at docker run, so the pull error is deliberately ignored.
func PullImage(w io.Writer, h *host.Host, svc, image string) {
	fmt.Fprintf(w, "  %s: pulling %s...\n", svc, image)
	_, _ = h.Run("docker pull " + hostshell.PosixSingleQuote(image))
}

package runner

import (
	"fmt"
	"io"
	"strings"

	"github.com/an-lee/gh-sr/internal/config"
	"github.com/an-lee/gh-sr/internal/host"
)

// validateAgenticPrereqs returns an error if any hard prerequisite for agentic
// workflows is not met. This is called by setupNative before any host modifications.
func (m *Manager) validateAgenticPrereqs(h *host.Host, rc config.RunnerConfig) error {
	if h.OS != "linux" {
		return fmt.Errorf("agentic profile is only supported on Linux")
	}
	// Docker CLI check
	out, err := h.Run(`docker --version 2>/dev/null`)
	if err != nil || !strings.Contains(out, "Docker version") {
		return fmt.Errorf("docker CLI not found on PATH")
	}
	// Docker daemon check
	out, err = h.Run(`docker info 2>/dev/null`)
	if err != nil {
		return fmt.Errorf("docker daemon not running")
	}
	// RUNNER_TEMP check
	out, err = h.Run(`echo "${RUNNER_TEMP:-}"`)
	if err != nil {
		return fmt.Errorf("could not read RUNNER_TEMP env var")
	}
	rt := strings.TrimSpace(out)
	if rt == "" {
		return fmt.Errorf("RUNNER_TEMP is not set; gh-aw requires it to be set to a path other than /tmp")
	}
	if rt == "/tmp" {
		return fmt.Errorf("RUNNER_TEMP=/tmp conflicts with gh-aw runtime tree at /tmp/gh-aw; set RUNNER_TEMP to a different path (e.g. ~/.gh-sr/runners/<name>/_work/_temp)")
	}
	return nil
}

// warnAgenticSudoPrereqs checks whether the runner user has passwordless sudo and, if not,
// prints a warning with remediation instructions. gh-aw requires sudo to manage iptables
// (DOCKER-USER chain) and the awf-net Docker bridge. This is a non-blocking warning: setup
// continues regardless so that the user can fix sudo and re-run without restarting from scratch.
func warnAgenticSudoPrereqs(h *host.Host, w io.Writer, runnerName string) {
	uid, err := h.Run(`id -u`)
	if err != nil {
		return // cannot determine, skip silently
	}
	if strings.TrimSpace(uid) == "0" {
		return // running as root, no sudo needed
	}
	out, err := h.Run(`sudo -n true 2>/dev/null && echo ok || echo no`)
	if err != nil || strings.TrimSpace(out) != "ok" {
		userName, _ := h.Run(`id -un`)
		userName = strings.TrimSpace(userName)
		fmt.Fprintf(w, "  %s: WARNING: passwordless sudo not available for user %q\n", runnerName, userName)
		fmt.Fprintf(w, "  %s:   gh-aw requires passwordless sudo to manage iptables and Docker bridge networks.\n", runnerName)
		fmt.Fprintf(w, "  %s:   To fix, run as root on the host:\n", runnerName)
		fmt.Fprintf(w, "  %s:     echo \"%s ALL=(ALL) NOPASSWD:ALL\" > /etc/sudoers.d/gh-sr-%s\n", runnerName, userName, userName)
		fmt.Fprintf(w, "  %s:     chmod 0440 /etc/sudoers.d/gh-sr-%s\n", runnerName, userName)
	}
}

// setupAgenticDNSConfigure configures Docker DNS on a Linux host so that agent containers
// (gh-aw) can resolve host.docker.internal to the docker0 bridge IP and also reach
// external domains (model providers, GitHub, etc.). It is idempotent: safe to re-run.
func (m *Manager) setupAgenticDNSConfigure(h *host.Host, runnerName string) error {
	// Step 1: Check if Docker is available at all.
	dockerCheck, err := h.Run(`docker info >/dev/null 2>&1 && echo ok || echo missing`)
	if err != nil || strings.TrimSpace(dockerCheck) != "ok" {
		return fmt.Errorf("docker daemon not available on host; skipping DNS setup")
	}

	// Step 2: Detect docker0 bridge IP. Fall back to 172.17.0.1 if detection fails.
	bridgeIP, err := h.Run(`ip -4 addr show docker0 2>/dev/null | grep -oP 'inet \K[\d.]+'`)
	if err != nil || strings.TrimSpace(bridgeIP) == "" {
		bridgeIP = "172.17.0.1"
	} else {
		bridgeIP = strings.TrimSpace(bridgeIP)
	}

	// Step 3: Detect if host.docker.internal already resolves inside containers.
	// If it resolves to a non-loopback IP, skip DNS setup (user may have their own solution).
	checkCmd := `docker run --rm alpine sh -c "getent hosts host.docker.internal 2>/dev/null" 2>/dev/null`
	out, err := h.Run(checkCmd)
	if err == nil && strings.TrimSpace(out) != "" {
		fields := strings.Fields(strings.TrimSpace(out))
		if len(fields) > 0 && fields[0] != "127.0.0.1" && fields[0] != "::1" {
			// Already configured; nothing to do.
			return nil
		}
	}

	// Step 4: Detect the package manager.
	detectPM := `if command -v apt-get >/dev/null 2>&1; then echo apt
elif command -v dnf >/dev/null 2>&1; then echo dnf
elif command -v yum >/dev/null 2>&1; then echo yum
elif command -v apk >/dev/null 2>&1; then echo apk
else echo unknown; fi`
	pmOut, err := h.Run(detectPM)
	pm := strings.TrimSpace(pmOut)
	if pm == "unknown" {
		return fmt.Errorf("could not detect package manager; skipping DNS setup")
	}

	// Step 5: Install dnsmasq if missing.
	dnsmasqInstalled, err := h.Run(`command -v dnsmasq >/dev/null 2>&1 && echo yes || echo no`)
	dnsmasqInstalled = strings.TrimSpace(dnsmasqInstalled)
	if dnsmasqInstalled != "yes" {
		var installCmd string
		switch pm {
		case "apt":
			installCmd = fmt.Sprintf(`%s && $SUDO apt-get update && $SUDO apt-get install -y dnsmasq`, linuxElevatePrelude)
		case "dnf", "yum":
			installCmd = fmt.Sprintf(`%s && $SUDO %s install -y dnsmasq`, linuxElevatePrelude, pm)
		case "apk":
			installCmd = fmt.Sprintf(`%s && $SUDO apk add dnsmasq`, linuxElevatePrelude)
		}
		if _, err := h.Run(installCmd); err != nil {
			return fmt.Errorf("installing dnsmasq: %w", err)
		}
	}

	// Step 6: Write dnsmasq config for gh-sr.
	// Resolves host.docker.internal to the docker0 bridge IP and forwards everything else
	// to systemd-resolved (127.0.0.53) and 8.8.8.8. The config file is prefixed "gh-sr-"
	// so we can detect and manage it separately from any user-provided config.
	dnsmasqConf := fmt.Sprintf(`address=/host.docker.internal/%s
listen-address=%s
bind-interfaces
server=127.0.0.53
server=8.8.8.8
`, bridgeIP, bridgeIP)
	// Use linuxElevatePrelude once at the top to set $SUDO, then use $SUDO throughout.
	confWrite := linuxElevatePrelude + fmt.Sprintf(`
CONF=/etc/dnsmasq.d/gh-sr-docker.conf
TMPCONF=$(mktemp)
cat > "$TMPCONF" << 'GHSREOF'
%sGHSREOF
if ! cmp -s "$TMPCONF" "$CONF" 2>/dev/null; then
    $SUDO cp "$TMPCONF" "$CONF"
    $SUDO systemctl restart dnsmasq
    echo "dnsmasq configured"
else
    echo "dnsmasq config unchanged"
fi
rm -f "$TMPCONF"`, dnsmasqConf)
	out, err = h.Run(confWrite)
	if err != nil {
		return fmt.Errorf("writing dnsmasq config: %w", err)
	}
	if out != "" {
		fmt.Fprintf(m.out(), "  %s: dnsmasq: %s\n", runnerName, strings.TrimSpace(out))
	}

	// Step 7: Configure Docker daemon DNS if not already set to use our dnsmasq.
	// linuxElevatePrelude is prepended once to set $SUDO; all elevated ops use $SUDO thereafter.
	daemonDNSConfigured, err := h.Run(linuxElevatePrelude + fmt.Sprintf(`
DOCKER_CONF=/etc/docker/daemon.json
BRIDGE_IP='%s'
if [ ! -f "$DOCKER_CONF" ]; then
    printf '{"dns":["%s","8.8.8.8"]}\n' "$BRIDGE_IP" | $SUDO tee "$DOCKER_CONF" > /dev/null
    $SUDO systemctl restart docker
    echo "daemon.json created with DNS"
else
    # Check if our dnsmasq IP is already in the dns list.
    if grep -q '"'"'"$BRIDGE_IP"'"'"' "$DOCKER_CONF" 2>/dev/null; then
        echo "daemon.json DNS already configured"
    else
        # Merge: add our dnsmasq IP at the front of the existing dns array, preserve other keys.
        # Try python3 first, then fall back to a shell-based approach.
        if command -v python3 >/dev/null 2>&1; then
            python3 -c "
import json, sys
path = '$DOCKER_CONF'
try:
    with open(path) as f:
        data = json.load(f)
except:
    data = {}
dns = data.get('dns', [])
if '$BRIDGE_IP' not in dns:
    dns.insert(0, '$BRIDGE_IP')
    dns = [d for d in dns if d]
data['dns'] = dns
with open(path, 'w') as f:
    json.dump(data, f, indent=2)
print('daemon.json DNS merged')
" 2>/dev/null && $SUDO systemctl restart docker && echo "daemon.json DNS updated"
        else
            # Fallback: use jq if available, otherwise just overwrite dns array safely.
            if command -v jq >/dev/null 2>&1; then
                $SUDO jq '.dns = ["'"'"'$BRIDGE_IP'"'"'", (.dns // [])[0:5]] | .dns += ["8.8.8.8"] | .dns = (.dns | unique)' "$DOCKER_CONF" > "${DOCKER_CONF}.new" && $SUDO mv "${DOCKER_CONF}.new" "$DOCKER_CONF" && $SUDO systemctl restart docker && echo "daemon.json DNS updated via jq"
            else
                # Last resort: read existing content and rebuild the dns array.
                # This is fragile but works on minimal systems without python3 or jq.
                $SUDO sh -c 'grep -v dns "$1" > "${1}.new"' -- "$DOCKER_CONF" && echo "  \"dns\": [\"$BRIDGE_IP\",\"8.8.8.8\"]" | $SUDO tee -a "${DOCKER_CONF}.new" > /dev/null && echo "}" | $SUDO tee -a "${DOCKER_CONF}.new" > /dev/null && $SUDO mv "${DOCKER_CONF}.new" "$DOCKER_CONF" && $SUDO systemctl restart docker && echo "daemon.json DNS updated via shell"
            fi
        fi
    fi
fi`, bridgeIP, bridgeIP))
	if err != nil && !strings.Contains(daemonDNSConfigured, "already configured") && !strings.Contains(daemonDNSConfigured, "configured") && !strings.Contains(daemonDNSConfigured, "updated") {
		// Non-fatal: docker restart may fail if there are running containers.
		fmt.Fprintf(m.out(), "  %s: warning: Docker daemon DNS merge failed: %v\n", runnerName, err)
	} else if daemonDNSConfigured != "" {
		fmt.Fprintf(m.out(), "  %s: docker: %s\n", runnerName, strings.TrimSpace(daemonDNSConfigured))
	}

	return nil
}

// verifyAgenticDNS checks whether Docker DNS is correctly configured for agentic
// workflows. Returns error if host.docker.internal does not resolve inside containers
// or if external DNS fails.
func (m *Manager) verifyAgenticDNS(h *host.Host) error {
	// Check host.docker.internal resolution inside containers
	out, err := h.Run(`docker run --rm alpine sh -c "getent hosts host.docker.internal || echo failed" 2>/dev/null`)
	out = strings.TrimSpace(out)
	if err != nil || out == "failed" || out == "" {
		return fmt.Errorf("host.docker.internal does not resolve inside containers")
	}
	if strings.Contains(out, "127.0.0.1") || strings.Contains(out, "::1") {
		return fmt.Errorf("host.docker.internal resolves to loopback inside containers; must be the Docker bridge gateway IP")
	}
	// Check external DNS resolution
	out, err = h.Run(`docker run --rm alpine sh -c "nslookup github.com >/dev/null 2>&1 && echo ok || echo failed" 2>/dev/null`)
	out = strings.TrimSpace(out)
	if err != nil || out != "ok" {
		return fmt.Errorf("external DNS (github.com) does not resolve inside containers")
	}
	return nil
}

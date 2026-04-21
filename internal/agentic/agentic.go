package agentic

import (
	"fmt"
	"strings"

	"github.com/an-lee/gh-sr/internal/host"
)

// PrereqFailure represents a single prerequisite check that failed.
type PrereqFailure struct {
	// Name is a short identifier for the failure, e.g. "sudo-iptables".
	Name string
	// Severity is "error" (blocks setup) or "warning" (non-blocking).
	Severity string
	// Message is a short human-readable description.
	Message string
	// Remediation is the exact shell command(s) to run to fix this failure.
	Remediation string
	// DocRef is an optional documentation reference, e.g. "agentic-workflows.md §5".
	DocRef string
}

// SeverityError indicates a hard failure that blocks setup.
const SeverityError = "error"

// SeverityWarning indicates a non-blocking warning.
const SeverityWarning = "warning"

// ValidatePrereqs checks all agentic prerequisites on the host and returns
// a list of failures. Returns an empty slice if all checks pass.
func ValidatePrereqs(h *host.Host) []PrereqFailure {
	var failures []PrereqFailure

	if h.OS != "linux" {
		failures = append(failures, PrereqFailure{
			Name:     "linux-required",
			Severity: SeverityError,
			Message:  "agentic profile is only supported on Linux",
			Remediation: "Use a Linux host for agentic runners. gh-aw requires Linux for its " +
				"network egress control (iptables DOCKER-USER chain) and Docker-based sandbox.",
			DocRef: "agentic-workflows.md §2",
		})
		return failures
	}

	// Docker CLI check
	out, err := h.Run(`docker --version 2>/dev/null`)
	if err != nil || !strings.Contains(out, "Docker version") {
		failures = append(failures, PrereqFailure{
			Name:     "docker-cli",
			Severity: SeverityError,
			Message:  "docker CLI not found on PATH",
			Remediation: `On the host, install Docker:

  sudo apt-get update && sudo apt-get install -y docker.io
  sudo systemctl enable --now docker
  sudo usermod -aG docker $USER
  # Log out and back in for group membership to take effect`,
			DocRef: "agentic-workflows.md §3g",
		})
	} else {
		// Docker daemon check
		out, err = h.Run(`docker info 2>/dev/null`)
		if err != nil {
			failures = append(failures, PrereqFailure{
				Name:     "docker-daemon",
				Severity: SeverityError,
				Message:  "docker daemon not running",
				Remediation: `Start the Docker daemon on the host:

  sudo systemctl start docker
  sudo systemctl enable docker  # persist across reboots`,
				DocRef: "agentic-workflows.md §3g",
			})
		} else {
			// Docker socket access check (Docker-in-Docker via DooD)
			out, err = h.Run(`docker run --rm -v /var/run/docker.sock:/var/run/docker.sock docker:cli docker ps 2>/dev/null`)
			if err != nil {
				failures = append(failures, PrereqFailure{
					Name:     "docker-socket",
					Severity: SeverityError,
					Message:  "cannot spawn containers via Docker socket; MCP gateway will fail",
					Remediation: `The MCP Gateway needs access to the Docker socket to spawn MCP server containers.
Ensure the runner user is in the docker group:

  sudo usermod -aG docker $USER
  # Log out and back in for group membership to take effect`,
					DocRef: "agentic-workflows.md §4c",
				})
			}
		}
	}

	// iptables availability check
	out, err = h.Run(`command -v iptables >/dev/null 2>&1 && echo ok || echo missing`)
	if err != nil || strings.TrimSpace(out) != "ok" {
		failures = append(failures, PrereqFailure{
			Name:     "iptables-missing",
			Severity: SeverityError,
			Message:  "iptables not found on PATH; gh-aw needs it for network egress control",
			Remediation: `Install iptables on the host:

  sudo apt-get update && sudo apt-get install -y iptables`,
			DocRef: "agentic-workflows.md §5",
		})
	}

	// RUNNER_TEMP check: read from .env files directly (not shell env, since
	// setupRunnerTemp writes to the .env file and ValidatePrereqs may run before
	// a shell session has sourced it).
	out, err = h.Run(`
FOUND_BAD=0
for ENV_FILE in $(find "$HOME/.gh-sr/runners" -maxdepth 2 -name ".env" 2>/dev/null); do
  RUNNER_TEMP=$(grep "^RUNNER_TEMP=" "$ENV_FILE" 2>/dev/null | cut -d= -f2)
  INSTANCE=$(basename "$(dirname "$ENV_FILE")")
  if [ -z "$RUNNER_TEMP" ]; then
    echo "unset:$INSTANCE"
    FOUND_BAD=1
  elif [ "$RUNNER_TEMP" = "/tmp" ]; then
    echo "tmp:$INSTANCE"
    FOUND_BAD=1
  fi
done
[ $FOUND_BAD -eq 0 ] && echo "ok"
`)
	if err == nil {
		lines := strings.TrimSpace(out)
		if lines != "ok" {
			for _, line := range strings.Split(lines, "\n") {
				if strings.HasPrefix(line, "unset:") {
					instance := strings.TrimPrefix(line, "unset:")
					failures = append(failures, PrereqFailure{
						Name:     "runner-temp-unset",
						Severity: SeverityWarning,
						Message:  fmt.Sprintf("RUNNER_TEMP is not set in %s's .env; gh-aw requires it to be set to a path other than /tmp", instance),
						Remediation: `Set RUNNER_TEMP in the runner's .env file:

  echo "RUNNER_TEMP=$HOME/.gh-sr/runners/_temp" >> ~/actions-runner/.env
  mkdir -p "$HOME/.gh-sr/runners/_temp"`,
						DocRef: "agentic-workflows.md §6",
					})
				} else if strings.HasPrefix(line, "tmp:") {
					instance := strings.TrimPrefix(line, "tmp:")
					failures = append(failures, PrereqFailure{
						Name:     "runner-temp-tmp",
						Severity: SeverityWarning,
						Message:  fmt.Sprintf("RUNNER_TEMP=/tmp in %s's .env conflicts with gh-aw runtime tree at /tmp/gh-aw", instance),
						Remediation: `Set RUNNER_TEMP to a different path in the runner's .env file:

  sed -i 's|RUNNER_TEMP=/tmp|RUNNER_TEMP='"$HOME"'/.gh-sr/runners/_temp|' ~/actions-runner/.env
  mkdir -p "$HOME/.gh-sr/runners/_temp"`,
						DocRef: "agentic-workflows.md §6",
					})
				}
			}
		}
	}

	// Passwordless sudo for iptables check
	uid, err := h.Run(`id -u`)
	if err == nil && strings.TrimSpace(uid) != "0" {
		out, err = h.Run(`sudo -n iptables -L DOCKER-USER >/dev/null 2>&1 && echo ok || echo no`)
		if err != nil || strings.TrimSpace(out) != "ok" {
			userName, _ := h.Run(`id -un`)
			userName = strings.TrimSpace(userName)
			failures = append(failures, PrereqFailure{
				Name:     "sudo-iptables",
				Severity: SeverityWarning,
				Message:  "passwordless sudo for iptables not available; gh-aw may fail to set egress rules",
				Remediation: fmt.Sprintf(`On the host, create a sudoers rule for iptables:

  echo "%s ALL=(ALL) NOPASSWD: /usr/sbin/iptables, /usr/sbin/ip6tables" | \\
    sudo tee /etc/sudoers.d/gh-sr-iptables
  sudo chmod 0440 /etc/sudoers.d/gh-sr-iptables`, userName),
				DocRef: "agentic-workflows.md §5",
			})
		}
	}

	// host.docker.internal DNS check inside containers
	out, err = h.Run(`docker run --rm alpine sh -c "getent hosts host.docker.internal 2>/dev/null" 2>/dev/null`)
	if err != nil || strings.TrimSpace(out) == "" || strings.TrimSpace(out) == "failed" {
		failures = append(failures, PrereqFailure{
			Name:     "host-docker-internal",
			Severity: SeverityError,
			Message:  "host.docker.internal does not resolve inside containers; MCP gateway unreachable",
			Remediation: `Run 'gh sr setup' for this runner, which automatically configures Docker DNS via dnsmasq.
If you already ran setup, manually configure dnsmasq:

  # Detect docker0 bridge IP
  BRIDGE_IP=$(docker inspect bridge --format='{{(index .IPAM.Config 0).Gateway}}')

  # Install and configure dnsmasq
  sudo apt-get update && sudo apt-get install -y dnsmasq

  echo "address=/host.docker.internal/$BRIDGE_IP
listen-address=$BRIDGE_IP
bind-interfaces
server=127.0.0.53
server=8.8.8.8" | sudo tee /etc/dnsmasq.d/gh-sr-docker.conf

  sudo systemctl restart dnsmasq
  sudo systemctl restart docker`,
			DocRef: "agentic-workflows.md §4b",
		})
	} else if strings.Contains(out, "127.0.0.1") || strings.Contains(out, "::1") {
		failures = append(failures, PrereqFailure{
			Name:     "host-docker-internal-loopback",
			Severity: SeverityError,
			Message:  "host.docker.internal resolves to loopback (127.0.0.1) inside containers; breaks MCP gateway",
			Remediation: `The /etc/hosts entry for host.docker.internal is pointing to 127.0.0.1, which is
the container's own loopback. It must point to the Docker bridge gateway.

Fix by running on the host:

  # Get the Docker bridge gateway IP
  BRIDGE_IP=$(docker inspect bridge --format='{{(index .IPAM.Config 0).Gateway}}')

  # Update /etc/hosts (remove any existing host.docker.internal entry first)
  grep -v "host.docker.internal" /etc/hosts | sudo tee /etc/hosts.tmp
  echo "$BRIDGE_IP  host.docker.internal" | sudo tee -a /etc/hosts.tmp
  sudo mv /etc/hosts.tmp /etc/hosts`,
			DocRef: "agentic-workflows.md §4b",
		})
	}

	// External DNS check from containers
	out, err = h.Run(`docker run --rm alpine sh -c "nslookup github.com >/dev/null 2>&1 && echo ok || echo failed" 2>/dev/null`)
	if err != nil || strings.TrimSpace(out) != "ok" {
		failures = append(failures, PrereqFailure{
			Name:     "external-dns",
			Severity: SeverityError,
			Message:  "external DNS (github.com) does not resolve inside containers",
			Remediation: `Docker containers cannot resolve external domains. This usually means dnsmasq
is not configured with upstream DNS servers, or Docker's DNS config is missing.

Check your dnsmasq config has upstream servers:

  cat /etc/dnsmasq.d/gh-sr-docker.conf
  # Should contain: server=127.0.0.53 and/or server=8.8.8.8

If missing, update the config and restart:

  echo "server=8.8.8.8" | sudo tee -a /etc/dnsmasq.d/gh-sr-docker.conf
  sudo systemctl restart dnsmasq
  sudo systemctl restart docker`,
			DocRef: "agentic-workflows.md §4b",
		})
	}

	return failures
}

// ValidateContainerPrereqs checks prerequisites for container-mode (DinD) agentic runners.
// Unlike ValidatePrereqs (for native mode), container mode only needs:
//   - Docker available on the host (to run the outer container)
//   - Support for --privileged containers (required for the inner dockerd)
//
// dnsmasq, sudoers/iptables, host.docker.internal, and RUNNER_TEMP setup all
// live inside the container image and are not host requirements.
func ValidateContainerPrereqs(h *host.Host) []PrereqFailure {
	var failures []PrereqFailure

	if h.OS != "linux" {
		failures = append(failures, PrereqFailure{
			Name:     "linux-required",
			Severity: SeverityError,
			Message:  "runner_mode: container is only supported on Linux hosts",
			Remediation: "Use a Linux host. Container-mode agentic runners require a Linux host with Docker.",
			DocRef: "agentic-workflows.md §8b",
		})
		return failures
	}

	// Docker CLI check
	out, err := h.Run(`docker --version 2>/dev/null`)
	if err != nil || !strings.Contains(out, "Docker version") {
		failures = append(failures, PrereqFailure{
			Name:     "docker-cli",
			Severity: SeverityError,
			Message:  "docker CLI not found on PATH; required to manage runner containers",
			Remediation: `Install Docker on the host:

  sudo apt-get update && sudo apt-get install -y docker.io
  sudo systemctl enable --now docker
  sudo usermod -aG docker $USER
  # Log out and back in for group membership to take effect`,
			DocRef: "agentic-workflows.md §8b",
		})
		return failures
	}

	// Docker daemon check
	if _, err = h.Run(`docker info >/dev/null 2>&1`); err != nil {
		failures = append(failures, PrereqFailure{
			Name:     "docker-daemon",
			Severity: SeverityError,
			Message:  "docker daemon not running",
			Remediation: `Start and enable Docker:

  sudo systemctl start docker
  sudo systemctl enable docker`,
			DocRef: "agentic-workflows.md §8b",
		})
		return failures
	}

	// --privileged support check (required for DinD)
	// We try to create a short-lived privileged container; if the daemon or kernel
	// security policy rejects --privileged, the inner dockerd will not start.
	privOut, err := h.Run(`docker run --rm --privileged alpine sh -c "echo privileged-ok" 2>/dev/null`)
	if err != nil || strings.TrimSpace(privOut) != "privileged-ok" {
		failures = append(failures, PrereqFailure{
			Name:     "docker-privileged",
			Severity: SeverityError,
			Message:  "docker --privileged containers are not supported; required for DinD (inner dockerd)",
			Remediation: `Privileged containers may be blocked by:
  - A non-root Docker daemon with userns-remap enabled (disable it for this use-case)
  - A Kubernetes/container runtime security policy
  - Seccomp/AppArmor profile restrictions

  Verify with: docker run --rm --privileged alpine echo ok
  For Sysbox (rootless-compatible alternative): see agentic-workflows.md §12`,
			DocRef: "agentic-workflows.md §8b",
		})
	}

	return failures
}

// ValidateAWFHygiene checks for leftover AWF/gh-aw Docker artefacts from crashed jobs.
// These are not blocking failures, only warnings.
func ValidateAWFHygiene(h *host.Host) []PrereqFailure {
	var failures []PrereqFailure

	if h.OS != "linux" {
		return failures
	}

	// Orphan awf-* containers (AWF agent sandbox containers left by crashed jobs)
	out, _ := h.Run(`docker ps -a --filter "name=awf-" --filter "name=gh-aw" --format '{{.Names}}' 2>/dev/null | head -20`)
	if strings.TrimSpace(out) != "" {
		failures = append(failures, PrereqFailure{
			Name:     "awf-orphan-containers",
			Severity: SeverityWarning,
			Message:  "orphan gh-aw/awf containers found from previously crashed jobs",
			Remediation: `Clean up orphan containers to free resources and avoid port conflicts:

  docker ps -a --filter "name=awf-" --format '{{.ID}}' | xargs -r docker rm -f
  docker ps -a --filter "name=gh-aw" --format '{{.ID}}' | xargs -r docker rm -f`,
			DocRef: "agentic-workflows.md §12",
		})
	}

	// Stale DOCKER-USER iptables rules referencing removed containers
	out, _ = h.Run(`sudo -n iptables -L DOCKER-USER --line-numbers -n 2>/dev/null | grep -i "awf\|gh-aw" | head -20`)
	if strings.TrimSpace(out) != "" {
		failures = append(failures, PrereqFailure{
			Name:     "stale-docker-user-rules",
			Severity: SeverityWarning,
			Message:  "stale DOCKER-USER iptables rules referencing gh-aw/awf containers",
			Remediation: `Flush stale AWF egress rules (only safe to do when no agentic jobs are running):

  sudo iptables -F DOCKER-USER`,
			DocRef: "agentic-workflows.md §12",
		})
	}

	// Orphan gh-aw-mcpg-* containers (MCP gateway containers)
	out, _ = h.Run(`docker ps -a --filter "name=gh-aw-mcpg-" --format '{{.Names}}' 2>/dev/null | head -20`)
	if strings.TrimSpace(out) != "" {
		failures = append(failures, PrereqFailure{
			Name:     "mcpg-orphan-containers",
			Severity: SeverityWarning,
			Message:  "orphan gh-aw-mcpg-* containers found from previously crashed jobs",
			Remediation: `Clean up orphan MCP gateway containers:

  docker ps -a --filter "name=gh-aw-mcpg-" --format '{{.ID}}' | xargs -r docker rm -f`,
			DocRef: "agentic-workflows.md §12",
		})
	}

	return failures
}

// HasBlockingFailures returns true if any failure has severity "error".
func HasBlockingFailures(failures []PrereqFailure) bool {
	for _, f := range failures {
		if f.Severity == SeverityError {
			return true
		}
	}
	return false
}

// FormatRemediation returns a formatted remediation string for a single failure.
func FormatRemediation(failure PrereqFailure) string {
	var sb strings.Builder
	sb.WriteString("\n  ")
	if failure.DocRef != "" {
		fmt.Fprintf(&sb, "[%s] ", failure.DocRef)
	}
	sb.WriteString(failure.Message)
	sb.WriteString("\n\n")
	lines := strings.Split(failure.Remediation, "\n")
	for i, line := range lines {
		if i == 0 {
			sb.WriteString("  To fix:\n")
		}
		sb.WriteString("    ")
		sb.WriteString(line)
		sb.WriteString("\n")
	}
	return sb.String()
}

// FormatAllRemediations returns a formatted string with all failures and their remediations.
func FormatAllRemediations(failures []PrereqFailure) string {
	if len(failures) == 0 {
		return ""
	}
	var sb strings.Builder
	fmt.Fprintln(&sb, "╔════════════════════════════════════════════════════════════════════════════╗")
	fmt.Fprintln(&sb, "║  Agentic Prerequisite Failures                                            ║")
	fmt.Fprintln(&sb, "╠════════════════════════════════════════════════════════════════════════════╣")
	fmt.Fprintf(&sb, "║  %d failure(s) need to be resolved before agentic workflows can run.      ║\n", len(failures))
	fmt.Fprintln(&sb, "╚════════════════════════════════════════════════════════════════════════════╝")
	for i, f := range failures {
		sev := "FAIL"
		if f.Severity == SeverityWarning {
			sev = "WARN"
		}
		fmt.Fprintf(&sb, "\n[%d] %s: %s", i+1, sev, f.Name)
		sb.WriteString(FormatRemediation(f))
	}
	return sb.String()
}

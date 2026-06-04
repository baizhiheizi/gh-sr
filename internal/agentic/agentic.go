package agentic

import (
	"fmt"
	"strconv"
	"strings"
	"sync"

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
//
// Checks are parallelized across all independent SSH calls to minimize
// wall-clock latency. Dependent chains (docker version → daemon → socket;
// id -u → sudo iptables; host.docker.internal → host-network variant) run
// sequentially within each goroutine.
func ValidatePrereqs(h *host.Host) []PrereqFailure {
	if h.OS != "linux" {
		return []PrereqFailure{{
			Name:     "linux-required",
			Severity: SeverityError,
			Message:  "agentic profile is only supported on Linux",
			Remediation: "Use a Linux host for agentic runners. gh-aw requires Linux for its " +
				"network egress control (iptables DOCKER-USER chain) and Docker-based sandbox.",
			DocRef: "agentic-workflows.md §2",
		}}
	}

	var (
		failures []PrereqFailure
		mu       sync.Mutex
		wg       sync.WaitGroup
	)

	hostDockerInternalOK := make(chan bool, 1)

	appendFailure := func(f PrereqFailure) {
		mu.Lock()
		failures = append(failures, f)
		mu.Unlock()
	}

	// ── Independent checks (all run in parallel) ──────────────────────────────

	// docker --version
	wg.Add(1)
	go func() {
		defer wg.Done()
		out, err := h.Run(`docker --version 2>/dev/null`)
		if err != nil || !strings.Contains(out, "Docker version") {
			appendFailure(PrereqFailure{
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
			return
		}
		// docker info — only if version check passed
		out, err = h.Run(`docker info 2>/dev/null`)
		if err != nil {
			appendFailure(PrereqFailure{
				Name:     "docker-daemon",
				Severity: SeverityError,
				Message:  "docker daemon not running",
				Remediation: `Start the Docker daemon on the host:

  sudo systemctl start docker
  sudo systemctl enable docker  # persist across reboots`,
				DocRef: "agentic-workflows.md §3g",
			})
			return
		}
		// docker socket — only if daemon check passed
		out, err = h.Run(`docker run --rm -v /var/run/docker.sock:/var/run/docker.sock docker:cli docker ps 2>/dev/null`)
		if err != nil {
			appendFailure(PrereqFailure{
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
	}()

	// iptables availability
	wg.Add(1)
	go func() {
		defer wg.Done()
		out, err := h.Run(`command -v iptables >/dev/null 2>&1 && echo ok || echo missing`)
		if err != nil || strings.TrimSpace(out) != "ok" {
			appendFailure(PrereqFailure{
				Name:     "iptables-missing",
				Severity: SeverityError,
				Message:  "iptables not found on PATH; gh-aw needs it for network egress control",
				Remediation: `Install iptables on the host:

  sudo apt-get update && sudo apt-get install -y iptables`,
				DocRef: "agentic-workflows.md §5",
			})
		}
	}()

	// RUNNER_TEMP check
	wg.Add(1)
	go func() {
		defer wg.Done()
		out, err := h.Run(`
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
						appendFailure(PrereqFailure{
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
						appendFailure(PrereqFailure{
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
	}()

	// id -u → sudo iptables (dependent chain: only if non-root)
	wg.Add(1)
	go func() {
		defer wg.Done()
		uidOut, err := h.Run(`id -u`)
		if err == nil && strings.TrimSpace(uidOut) != "0" {
			out, err := h.Run(`sudo -n iptables -L DOCKER-USER >/dev/null 2>&1 && echo ok || echo no`)
			if err != nil || strings.TrimSpace(out) != "ok" {
				userName, _ := h.Run(`id -un`)
				userName = strings.TrimSpace(userName)
				appendFailure(PrereqFailure{
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
	}()

	// host.docker.internal DNS check (default bridge)
	wg.Add(1)
	go func() {
		defer wg.Done()
		out, err := h.Run(`docker run --rm alpine sh -c "getent hosts host.docker.internal 2>/dev/null" 2>/dev/null`)
		out = strings.TrimSpace(out)
		if err != nil || out == "" || out == "failed" {
			hostDockerInternalOK <- false
			appendFailure(PrereqFailure{
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
			return
		}
		if strings.Contains(out, "127.0.0.1") || strings.Contains(out, "::1") {
			hostDockerInternalOK <- false
			appendFailure(PrereqFailure{
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
			return
		}
		hostDockerInternalOK <- true
	}()

	// host-network DNS check (depends on host-docker-internal passing)
	wg.Add(1)
	go func() {
		defer wg.Done()
		if ok := <-hostDockerInternalOK; !ok {
			return
		}
		outHN, errHN := h.Run(`docker run --rm --network host alpine sh -c "getent hosts host.docker.internal 2>/dev/null" 2>/dev/null`)
		outHN = strings.TrimSpace(outHN)
		fields := strings.Fields(outHN)
		badHN := errHN != nil || len(fields) == 0 || fields[0] == "127.0.0.1" || fields[0] == "::1"
		if badHN {
			appendFailure(PrereqFailure{
				Name:     "host-docker-internal-host-network",
				Severity: SeverityWarning,
				Message:  "`host.docker.internal` not usable from `docker run --network host` (same mode as gh-aw-mcpg); MCP gateway or in-sandbox MCP clients may still fail",
				Remediation: `Verify on the host (must not be 127.0.0.1):

  getent hosts host.docker.internal
  docker run --rm --network host alpine sh -c "getent hosts host.docker.internal"

Map host.docker.internal to the docker0 bridge gateway; see agentic-workflows.md §4b.`,
				DocRef: "agentic-workflows.md §4b",
			})
		}
	}()

	// external DNS check
	wg.Add(1)
	go func() {
		defer wg.Done()
		out, err := h.Run(`docker run --rm alpine sh -c "nslookup github.com >/dev/null 2>&1 && echo ok || echo failed" 2>/dev/null`)
		if err != nil || strings.TrimSpace(out) != "ok" {
			appendFailure(PrereqFailure{
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
	}()

	wg.Wait()
	return failures
}

// ValidateContainerPrereqs checks host prerequisites for runner_mode: container (DinD).
// Unlike ValidatePrereqs (for native agentic), the host only needs:
//   - Docker available on the host (to run the outer runner container)
//   - Support for --privileged containers (required for the inner dockerd)
//
// dnsmasq, sudoers/iptables, host.docker.internal, gh-aw tooling, and RUNNER_TEMP
// live inside the runner image (or apply only to native agentic) and are not
// validated here.
func ValidateContainerPrereqs(h *host.Host) []PrereqFailure {
	var failures []PrereqFailure

	if h.OS != "linux" {
		failures = append(failures, PrereqFailure{
			Name:        "linux-required",
			Severity:    SeverityError,
			Message:     "runner_mode: container is only supported on Linux hosts",
			Remediation: "Use a Linux host with Docker. Container-mode self-hosted runners require Linux on the host.",
			DocRef:      "agentic-workflows.md §8b",
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
//
// All three checks run in parallel to minimize wall-clock latency.
func ValidateAWFHygiene(h *host.Host) []PrereqFailure {
	if h.OS != "linux" {
		return nil
	}

	var (
		failures []PrereqFailure
		mu       sync.Mutex
		wg       sync.WaitGroup
	)

	appendFailure := func(f PrereqFailure) {
		mu.Lock()
		failures = append(failures, f)
		mu.Unlock()
	}

	wg.Add(3)

	// Orphan awf-* containers (AWF agent sandbox containers left by crashed jobs)
	go func() {
		defer wg.Done()
		out, _ := h.Run(`docker ps -a --filter "name=awf-" --filter "name=gh-aw" --format '{{.Names}}' 2>/dev/null | head -20`)
		if strings.TrimSpace(out) != "" {
			appendFailure(PrereqFailure{
				Name:     "awf-orphan-containers",
				Severity: SeverityWarning,
				Message:  "orphan gh-aw/awf containers found from previously crashed jobs",
				Remediation: `Clean up orphan containers to free resources and avoid port conflicts:

  docker ps -a --filter "name=awf-" --format '{{.ID}}' | xargs -r docker rm -f
  docker ps -a --filter "name=gh-aw" --format '{{.ID}}' | xargs -r docker rm -f`,
				DocRef: "agentic-workflows.md §12",
			})
		}
	}()

	// Stale DOCKER-USER iptables rules referencing removed containers
	go func() {
		defer wg.Done()
		out, _ := h.Run(`sudo -n iptables -L DOCKER-USER --line-numbers -n 2>/dev/null | grep -i "awf\|gh-aw" | head -20`)
		if strings.TrimSpace(out) != "" {
			appendFailure(PrereqFailure{
				Name:     "stale-docker-user-rules",
				Severity: SeverityWarning,
				Message:  "stale DOCKER-USER iptables rules referencing gh-aw/awf containers",
				Remediation: `Flush stale AWF egress rules (only safe to do when no agentic jobs are running):

  sudo iptables -F DOCKER-USER`,
				DocRef: "agentic-workflows.md §12",
			})
		}
	}()

	// Orphan gh-aw-mcpg-* containers (MCP gateway containers)
	go func() {
		defer wg.Done()
		out, _ := h.Run(`docker ps -a --filter "name=gh-aw-mcpg-" --format '{{.Names}}' 2>/dev/null | head -20`)
		if strings.TrimSpace(out) != "" {
			appendFailure(PrereqFailure{
				Name:     "mcpg-orphan-containers",
				Severity: SeverityWarning,
				Message:  "orphan gh-aw-mcpg-* containers found from previously crashed jobs",
				Remediation: `Clean up orphan MCP gateway containers:

  docker ps -a --filter "name=gh-aw-mcpg-" --format '{{.ID}}' | xargs -r docker rm -f`,
				DocRef: "agentic-workflows.md §12",
			})
		}
	}()

	wg.Wait()
	return failures
}

// ValidateAWFHygieneInner runs the same orphan/stale checks as ValidateAWFHygiene
// against the inner Docker daemon inside a running DinD runner container (outerContainer
// is the host-visible name, e.g. gh-sr-myinstance).
//
// All three checks run in parallel to minimize wall-clock latency.
func ValidateAWFHygieneInner(h *host.Host, outerContainer string) []PrereqFailure {
	if h.OS != "linux" {
		return nil
	}

	q := strconv.Quote(outerContainer)
	pfx := "docker exec " + q + " "

	var (
		failures []PrereqFailure
		mu       sync.Mutex
		wg       sync.WaitGroup
	)

	appendFailure := func(f PrereqFailure) {
		mu.Lock()
		failures = append(failures, f)
		mu.Unlock()
	}

	wg.Add(3)

	go func() {
		defer wg.Done()
		out, _ := h.Run(pfx + `docker ps -a --filter "name=awf-" --filter "name=gh-aw" --format '{{.Names}}' 2>/dev/null | head -20`)
		if strings.TrimSpace(out) != "" {
			appendFailure(PrereqFailure{
				Name:     "awf-orphan-containers-inner",
				Severity: SeverityWarning,
				Message:  fmt.Sprintf("orphan gh-aw/awf containers in inner Docker (runner container %s)", outerContainer),
				Remediation: fmt.Sprintf(`Clean up inside the runner container (outer name %s):

  docker exec -it %s bash
  docker ps -a --filter "name=awf-" --format '{{.ID}}' | xargs -r docker rm -f
  docker ps -a --filter "name=gh-aw" --format '{{.ID}}' | xargs -r docker rm -f`, outerContainer, outerContainer),
				DocRef: "agentic-workflows.md §12",
			})
		}
	}()

	go func() {
		defer wg.Done()
		out, _ := h.Run(pfx + `iptables -L DOCKER-USER --line-numbers -n 2>/dev/null | grep -i "awf\|gh-aw" | head -20`)
		if strings.TrimSpace(out) != "" {
			appendFailure(PrereqFailure{
				Name:     "stale-docker-user-rules-inner",
				Severity: SeverityWarning,
				Message:  fmt.Sprintf("stale DOCKER-USER iptables rules in inner netns (runner container %s)", outerContainer),
				Remediation: fmt.Sprintf(`Flush inner AWF egress rules when no agentic job is using this runner:

  docker exec %s iptables -F DOCKER-USER`, outerContainer),
				DocRef: "agentic-workflows.md §12",
			})
		}
	}()

	go func() {
		defer wg.Done()
		out, _ := h.Run(pfx + `docker ps -a --filter "name=gh-aw-mcpg-" --format '{{.Names}}' 2>/dev/null | head -20`)
		if strings.TrimSpace(out) != "" {
			appendFailure(PrereqFailure{
				Name:     "mcpg-orphan-containers-inner",
				Severity: SeverityWarning,
				Message:  fmt.Sprintf("orphan gh-aw-mcpg-* containers in inner Docker (runner container %s)", outerContainer),
				Remediation: fmt.Sprintf(`Clean up inside the runner container:

  docker exec -it %s bash
  docker ps -a --filter "name=gh-aw-mcpg-" --format '{{.ID}}' | xargs -r docker rm -f`, outerContainer),
				DocRef: "agentic-workflows.md §12",
			})
		}
	}()

	wg.Wait()
	return failures
}

// ValidateContainerInnerNetwork checks the network paths gh-aw depends on inside
// a container-mode runner. The MCP gateway runs in the inner host network, while
// agent/AWF child containers use the default bridge and reach the gateway via
// host.docker.internal.
func ValidateContainerInnerNetwork(h *host.Host, outerContainer, runnerName string) []PrereqFailure {
	if h.OS != "linux" {
		return nil
	}

	if _, err := h.Run(containerInnerNetworkCheckCommand(outerContainer)); err != nil {
		return []PrereqFailure{{
			Name:     "container-inner-host-docker-internal",
			Severity: SeverityWarning,
			Message:  fmt.Sprintf("host.docker.internal does not resolve to a usable non-loopback address inside runner container %s (baked dnsmasq/daemon.json DNS)", outerContainer),
			Remediation: fmt.Sprintf(`Inspect the runner container's inner Docker DNS and restart/rebuild it if stale:

  docker exec -it %s bash
  getent hosts host.docker.internal
  docker run --rm alpine sh -c 'getent hosts host.docker.internal'
  docker run --rm --network host alpine sh -c 'getent hosts host.docker.internal'
  docker run --rm --add-host=host.docker.internal:host-gateway alpine sh -c 'getent hosts host.docker.internal'

If add-host resolution is empty or loopback, restart the runner container. If it persists, run:

  gh sr rebuild %s`, outerContainer, runnerName),
			DocRef: "agentic-workflows.md §11a",
		}}
	}
	return nil
}

// ValidateContainerInnerResolv checks that the runner container's /etc/resolv.conf
// points at the bundled dnsmasq (the inner docker0 gateway). entrypoint.sh repoints it
// there at startup so gh-aw's firewall — which auto-detects the agent sandbox's DNS
// servers from this very file — propagates an authoritative, AWF-exempt resolver for
// host.docker.internal to the sandbox.
//
// If a stale (pre-fix) image or a read-only resolv.conf leaves the OUTER host resolver
// in place, the sandbox resolves host.docker.internal there and intermittently gets a
// non-exempt IP. AWF's sandbox iptables only exempts the inner-bridge gateway from the
// transparent Squid redirect, so a non-exempt answer force-proxies the MCP gateway POST
// into Squid, which rejects the origin-form request (ERR_INVALID_URL → HTTP 400) and the
// agent reports "MCP server(s) failed to launch". This check surfaces that latent
// misconfiguration before a job hits it.
func ValidateContainerInnerResolv(h *host.Host, outerContainer, runnerName string) []PrereqFailure {
	if h.OS != "linux" {
		return nil
	}

	if _, err := h.Run(containerInnerResolvCheckCommand(outerContainer)); err != nil {
		return []PrereqFailure{{
			Name:     "container-inner-resolv",
			Severity: SeverityWarning,
			Message:  fmt.Sprintf("runner container %s /etc/resolv.conf is not pinned to the bundled dnsmasq (inner bridge gateway); the gh-aw agent sandbox can inherit the host resolver and intermittently fail MCP launch (host.docker.internal force-proxied into Squid)", outerContainer),
			Remediation: fmt.Sprintf(`Rebuild the runner image so entrypoint.sh repoints resolv.conf at the bundled dnsmasq:

  gh sr rebuild %s

Verify after restart (expect a single nameserver equal to the inner docker0 gateway, e.g. 10.200.0.1):

  docker exec %s cat /etc/resolv.conf
  docker exec %s ip -4 -o addr show docker0`, runnerName, outerContainer, outerContainer),
			DocRef: "agentic-workflows.md §11a",
		}}
	}
	return nil
}

// ValidateContainerAWFServiceRouting checks that the runner container has the
// AWF service-routing bypass installed in NAT PREROUTING.
//
// Without this rule, AWF agents on awf-net cannot reach workflow `services:`
// containers (postgres/redis/etc.) via host.docker.internal:<port>. Inner dockerd
// DNATs port-published traffic from the host gateway IP to the service container
// IP *before* AWF's FW_WRAPPER chain in FORWARD inspects it. AWF's
// `--allow-host-service-ports` rules match on the host gateway IP, so the post-
// DNAT packet falls through to FW_WRAPPER's catch-all REJECT (ICMP port-
// unreachable), surfacing as `Connection refused` for libpq/redis/etc.
//
// The bypass (installed by entrypoint.sh) is a single rule at the head of the
// runner container's NAT PREROUTING chain:
//
//	iptables -t nat -I PREROUTING -s 172.30.0.0/24 -m addrtype --dst-type LOCAL -j RETURN
//
// Traffic from awf-net to any local IP of the runner container then skips DNAT
// and is delivered to the userland docker-proxy, which forwards to the service.
func ValidateContainerAWFServiceRouting(h *host.Host, outerContainer, runnerName string) []PrereqFailure {
	if h.OS != "linux" {
		return nil
	}

	if _, err := h.Run(containerAWFServiceRoutingCheckCommand(outerContainer)); err != nil {
		return []PrereqFailure{{
			Name:     "container-awf-service-routing",
			Severity: SeverityWarning,
			Message:  fmt.Sprintf("AWF service-routing bypass not installed in runner container %s; agentic workflows that use `services:` (postgres, redis, etc.) will see `Connection refused` on host.docker.internal", outerContainer),
			Remediation: fmt.Sprintf(`Rebuild the runner image (entrypoint.sh installs the bypass at startup):

  gh sr rebuild %s

Or apply the rule live without restart (lasts until docker daemon restart):

  docker exec %s iptables -t nat -I PREROUTING -s 172.30.0.0/24 -m addrtype --dst-type LOCAL -j RETURN

Verify:

  docker exec %s iptables -t nat -S PREROUTING | head -3`, runnerName, outerContainer, outerContainer),
			DocRef: "agentic-workflows.md §11b",
		}}
	}
	return nil
}

// ValidateContainerMTU warns when the runner container's egress interface (eth0) or the
// inner dockerd bridge (docker0) carries an MTU larger than the host's egress MTU. A
// container MTU above the host path MTU silently drops large packets when PMTUD is
// black-holed, breaking TLS handshakes — workflow downloads such as actions/setup-go
// then fail with "Client network socket disconnected before secure TLS connection was
// established" even though the host downloads fine. hostEgressMTU is the host's primary
// egress interface MTU (from runner.DetectHostEgressMTU); 0 or >= 1500 means there is
// nothing to pin, so the check is skipped (standard 1500 networks never see this).
func ValidateContainerMTU(h *host.Host, outerContainer, runnerName string, hostEgressMTU int) []PrereqFailure {
	// hostEgressMTU 0 (unknown) or >= 1500 (standard) means there is nothing to pin; skip
	// BEFORE dereferencing h so callers may short-circuit with a nil host.
	if hostEgressMTU <= 0 || hostEgressMTU >= 1500 {
		return nil
	}
	if h == nil || h.OS != "linux" {
		return nil
	}

	if _, err := h.Run(containerMTUCheckCommand(outerContainer, hostEgressMTU)); err != nil {
		return []PrereqFailure{{
			Name:     "container-mtu",
			Severity: SeverityWarning,
			Message:  fmt.Sprintf("runner container %s has a Docker interface MTU larger than the host egress MTU (%d); large-packet TLS handshakes (e.g. actions/setup-go) can fail with \"Client network socket disconnected before secure TLS connection was established\"", outerContainer, hostEgressMTU),
			Remediation: fmt.Sprintf(`Rebuild the runner so it pins the inner/outer Docker MTU to the host egress MTU:

  gh sr rebuild %s

Verify (both must be <= %d):

  docker exec %s cat /sys/class/net/eth0/mtu
  docker exec %s cat /sys/class/net/docker0/mtu

If the host's real path MTU is below its NIC MTU (a tunnel the NIC is unaware of), set it explicitly in runners.yml and rebuild:

  container_runner_image:
    mtu: %d`, runnerName, hostEgressMTU, outerContainer, outerContainer, hostEgressMTU),
			DocRef: "agentic-workflows.md §11c",
		}}
	}
	return nil
}

// ValidateContainerAWF checks that the gh-aw firewall CLI is available exactly
// the way compiled workflows invoke it.
func ValidateContainerAWF(h *host.Host, outerContainer, runnerName string) []PrereqFailure {
	if h.OS != "linux" {
		return nil
	}

	if _, err := h.Run(containerAWFCheckCommand(outerContainer)); err != nil {
		return []PrereqFailure{{
			Name:     "container-awf",
			Severity: SeverityWarning,
			Message:  fmt.Sprintf("awf is not available via sudo inside runner container %s", outerContainer),
			Remediation: fmt.Sprintf(`Rebuild the runner image so it includes github/gh-aw-firewall:

  gh sr rebuild %s

For a temporary live-container unblock only:

  docker exec %s sh -lc 'curl -sSL https://raw.githubusercontent.com/github/gh-aw-firewall/main/install.sh | AWF_FORCE_BINARY=1 bash'`, runnerName, outerContainer),
			DocRef: "agentic-workflows.md §12",
		}}
	}
	return nil
}

func containerInnerNetworkCheckCommand(outerContainer string) string {
	q := strconv.Quote(outerContainer)
	// Require the PRODUCTION baked-DNS path: a plain default-bridge child container must
	// resolve host.docker.internal to a non-loopback address purely via the image-baked
	// daemon DNS (daemon.json `dns` -> bundled dnsmasq). This is exactly what AWF agent
	// containers rely on to reach the MCP gateway. The shim no longer injects --add-host,
	// so we must NOT accept an --add-host fallback here — doing so would mask broken baked
	// DNS and let a runner pass health checks while real MCP traffic fails.
	return `docker exec ` + q + ` sh -c 'set -eu
ok=0
for i in 1 2 3 4 5; do
  ip=$(docker run --rm alpine getent hosts host.docker.internal 2>/dev/null | awk "{print \$1; exit}")
  case "$ip" in
    "" | 127.* | ::1) ;;
    *) ok=1; break ;;
  esac
  sleep 1
done
[ "$ok" -eq 1 ]'`
}

func containerAWFCheckCommand(outerContainer string) string {
	q := strconv.Quote(outerContainer)
	return `docker exec ` + q + ` sh -lc 'set -eu
command -v awf >/dev/null
sudo -n -E awf --version >/dev/null'`
}

// containerMTUCheckCommand exits non-zero when any of the runner container's Docker
// interfaces (eth0, docker0) has an MTU greater than the host egress MTU — the
// signature of a stale image built before MTU pinning. A missing interface file reads
// as 0, so it never triggers a false positive.
func containerMTUCheckCommand(outerContainer string, hostEgressMTU int) string {
	q := strconv.Quote(outerContainer)
	mtu := strconv.Itoa(hostEgressMTU)
	return `docker exec ` + q + ` sh -c 'host=` + mtu + `
for ifc in eth0 docker0; do
  m=$(cat /sys/class/net/$ifc/mtu 2>/dev/null || echo 0)
  [ "$m" -le "$host" ] || exit 1
done'`
}

// containerInnerResolvCheckCommand verifies the runner container's /etc/resolv.conf
// lists the live inner docker0 gateway as a nameserver. The gateway is normally
// 10.200.0.1 but entrypoint.sh's collision-avoidance may pick another candidate, so we
// resolve it live (falling back to 10.200.0.1) rather than hardcoding it.
func containerInnerResolvCheckCommand(outerContainer string) string {
	q := strconv.Quote(outerContainer)
	return `docker exec ` + q + ` sh -c 'set -eu
gw=$(ip -4 -o addr show docker0 2>/dev/null | awk "{print \$4}" | cut -d/ -f1 | head -n1)
[ -n "$gw" ] || gw=10.200.0.1
grep -Eq "^nameserver[[:space:]]+$gw([[:space:]]|$)" /etc/resolv.conf'`
}

// containerAWFServiceRoutingCheckCommand verifies the runner container has the
// PREROUTING bypass rule that exempts AWF subnet traffic targeting local IPs
// from inner dockerd's DOCKER chain DNAT. iptables -S normalises rule output,
// so an exact-line match is reliable.
func containerAWFServiceRoutingCheckCommand(outerContainer string) string {
	q := strconv.Quote(outerContainer)
	return `docker exec ` + q + ` sh -c 'iptables -t nat -S PREROUTING 2>/dev/null | grep -Fq -e "-A PREROUTING -s 172.30.0.0/24 -m addrtype --dst-type LOCAL -j RETURN"'`
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

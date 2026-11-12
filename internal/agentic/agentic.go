package agentic

import (
	"fmt"
	"strconv"
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

// ValidateContainerPrereqs checks host prerequisites for runner_mode: container (DinD).
// The host only needs:
//   - Docker available on the host (to run the outer runner container)
//   - Support for --privileged containers (required for the inner dockerd)
//
// dnsmasq, sudoers/iptables, host.docker.internal, gh-aw tooling, and RUNNER_TEMP
// live inside the runner image and are not validated here.
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

	// docker CLI → daemon → --privileged chain — all three sub-probes run in
	// one SSH round-trip via dockerChainCheckCommand. Exit codes are captured
	// by parseDockerChainOutput and mapped to PrereqFailure entries via
	// dockerChainSpecs. Replaces 3 sequential h.Run calls with 1.
	out, _ := h.Run(dockerChainCheckCommand("privileged"))
	failures = append(failures, parseDockerChainOutput(out, dockerChainSpecs("privileged"))...)

	return failures
}

// awfHygieneCheck describes one AWF/gh-aw hygiene probe run inside the inner
// Docker of a DinD runner container. The batched inner probe executes each
// check's Cmd via `docker exec "gh-sr-$i"` and reports any probe that prints
// output as a `-inner`-suffixed failure.
type awfHygieneCheck struct {
	// Name is the base failure name; the batch parser appends `-inner`.
	Name string
	// Cmd is the probe shell command WITHOUT the docker-exec wrapper, so the
	// same definition works for any instance the batch loops over.
	Cmd string
	// Message is the fully-rendered PrereqFailure.Message for this variant.
	// The caller pre-formats any per-variant wording (e.g. outer container name).
	Message string
	// Remediation is the fully-rendered PrereqFailure.Remediation for this
	// variant. Same pre-format rule as Message.
	Remediation string
}

// awfInnerHygieneChecks returns the inner-Docker (DinD runner) agentic hygiene
// check definitions. gh-aw v0.88+ runs AWF rootless: the job leaves behind
// awmg-mcpg / awf- / gh-aw containers rather than iptables state, so only
// container leakage is probed. Each remediation is pre-formatted with the outer
// container name so the operator running `gh sr doctor` knows which runner
// to ssh into.
func awfInnerHygieneChecks(outerContainer string) []awfHygieneCheck {
	return []awfHygieneCheck{
		{
			Name:    "orphan-agentic-containers",
			Cmd:     `docker ps -a --filter "name=awmg-mcpg" --filter "name=awf-" --filter "name=gh-aw" --format '{{.Names}}' 2>/dev/null | head -20`,
			Message: fmt.Sprintf("orphan awmg-mcpg/awf/gh-aw containers in inner Docker (runner container %s); normally reaped by the job-completed hook", outerContainer),
			Remediation: fmt.Sprintf(`Clean up inside the runner container (outer name %s):

  docker exec -it %s bash
  docker ps -a --filter "name=awmg-mcpg" --filter "name=awf-" --filter "name=gh-aw" --format '{{.ID}}' | xargs -r docker rm -f`, outerContainer, outerContainer),
		},
		{
			Name:    "orphan-agentic-networks",
			Cmd:     `docker network ls --filter "name=awf-net" --format '{{.Name}}' 2>/dev/null | head -20`,
			Message: fmt.Sprintf("orphan awf-net networks in inner Docker (runner container %s)", outerContainer),
			Remediation: fmt.Sprintf(`Prune leftover job networks inside the runner container:

  docker exec %s docker network prune -f --filter "name=awf-net"`, outerContainer),
		},
	}
}

// containerCheckDef is the single source of truth for each agentic container
// probe. The batched inner probe (ValidateAgenticInnerBatch) derives its shell
// command, failure metadata, and tagged output from the same definition, so a
// wording or logic change only needs to be made in one place.
type containerCheckDef struct {
	Name        string
	LoginShell  bool   // sh -lc (true) vs sh -c (false) for the probe body
	InnerBody   string // raw shell statements (no docker-exec wrapper)
	Message     string
	Remediation string
	DocRef      string
}

// Per-def constructors. Each helper builds exactly one containerCheckDef with
// Message/Remediation pre-rendered against the supplied outerContainer /
// runnerName (and hostEgressMTU where applicable). containerCheckDefs composes
// them into the batch's fanout slice, so they are the single source of truth
// for probe metadata.
//
// gh-aw v0.88+ rootless notes: no resolv.conf/dnsmasq probe (no baked DNS), no
// iptables service-routing probe (network isolation is the inner bridge
// topology), no awf CLI/sudo probe (AWF is rootless and fetched by the job).
func innerHostDockerInternalDef(outerContainer, runnerName string) containerCheckDef {
	return containerCheckDef{
		Name:       "container-inner-host-docker-internal",
		LoginShell: false,
		InnerBody: `set -eu
ok=0
for i in 1 2 3 4 5; do
  ip=$(docker run --rm --add-host=host.docker.internal:host-gateway alpine getent hosts host.docker.internal 2>/dev/null | awk "{print \$1; exit}")
  case "$ip" in
    "" | 127.* | ::1) ;;
    *) ok=1; break ;;
  esac
  sleep 1
done
[ "$ok" -eq 1 ]`,
		Message: fmt.Sprintf("inner dockerd in runner container %s does not map --add-host=host.docker.internal:host-gateway to a usable non-loopback address; gh-aw v0.88+ job containers (agent sandbox, awmg-mcpg gateway) rely on it", outerContainer),
		Remediation: fmt.Sprintf(`Inspect the runner container's inner Docker and restart/rebuild it if stale:

  docker exec -it %s bash
  docker run --rm --add-host=host.docker.internal:host-gateway alpine sh -c 'getent hosts host.docker.internal'

If resolution is empty or loopback, restart the runner container. If it persists, run:

  gh sr rebuild %s`, outerContainer, runnerName),
		DocRef: "agentic-workflows.md §11a",
	}
}

func nodeNPMDef(outerContainer, runnerName string) containerCheckDef {
	return containerCheckDef{
		Name:       "container-node-npm",
		LoginShell: true,
		InnerBody:  `command -v node >/dev/null && command -v npm >/dev/null`,
		Message:    fmt.Sprintf("node LTS/npm are not on PATH inside runner container %s", outerContainer),
		Remediation: fmt.Sprintf(`Rebuild the runner image so it includes Node.js LTS:

  gh sr rebuild %s`, runnerName),
		DocRef: "agentic-workflows.md §8",
	}
}

func dockerSocketUserDef(outerContainer, runnerName string) containerCheckDef {
	return containerCheckDef{
		Name:       "container-docker-socket-user",
		LoginShell: false,
		InnerBody:  `su -s /bin/sh runner -c 'docker info >/dev/null 2>&1'`,
		Message:    fmt.Sprintf("the runner user inside runner container %s cannot talk to the inner dockerd socket; the awmg-mcpg gateway mounts that socket and needs non-root access (fork image puts runner in the docker group, gid 123)", outerContainer),
		Remediation: fmt.Sprintf(`Rebuild the runner image so the inner dockerd socket is group-accessible to the runner user:

  gh sr rebuild %s

Verify:

  docker exec %s su -s /bin/sh runner -c 'docker info >/dev/null && echo ok'`, runnerName, outerContainer),
		DocRef: "agentic-workflows.md §11a",
	}
}

func zstdDef(outerContainer, runnerName string) containerCheckDef {
	return containerCheckDef{
		Name:       "container-zstd",
		LoginShell: true,
		InnerBody:  `command -v zstd >/dev/null`,
		Message:    fmt.Sprintf("zstd is not on PATH inside runner container %s; actions/cache restore/save and tool-cache archives need it", outerContainer),
		Remediation: fmt.Sprintf(`Rebuild the runner image so it includes zstd:

  gh sr rebuild %s`, runnerName),
		DocRef: "agentic-workflows.md §8",
	}
}

func cacheEnvDef(outerContainer, runnerName string) containerCheckDef {
	return containerCheckDef{
		Name:       "container-cache-env",
		LoginShell: false,
		InnerBody:  `grep -q '^CUSTOM_ACTIONS_RESULTS_URL=' /home/runner/.env`,
		Message:    fmt.Sprintf("runner container %s has no CUSTOM_ACTIONS_RESULTS_URL in .env; with the cache enabled the runner must point actions/cache at the per-host cache server", outerContainer),
		Remediation: fmt.Sprintf(`Redeploy the runner so the entrypoint writes the cache URL (check that the cache is enabled and reachable on this host):

  gh sr cache deploy
  gh sr up %s`, runnerName),
		DocRef: "guides/local-cache.md",
	}
}

func mtuDef(outerContainer, runnerName string, hostEgressMTU int) containerCheckDef {
	mtu := strconv.Itoa(hostEgressMTU)
	return containerCheckDef{
		Name:       "container-mtu",
		LoginShell: false,
		InnerBody: "host=" + mtu + `
for ifc in eth0 docker0; do
  m=$(cat /sys/class/net/$ifc/mtu 2>/dev/null || echo 0)
  [ "$m" -le "$host" ] || exit 1
done`,
		Message: fmt.Sprintf("runner container %s has a Docker interface MTU larger than the host egress MTU (%d); large-packet TLS handshakes (e.g. actions/setup-go) can fail with \"Client network socket disconnected before secure TLS connection was established\"", outerContainer, hostEgressMTU),
		Remediation: fmt.Sprintf(`Rebuild the runner so it pins the inner/outer Docker MTU to the host egress MTU:

  gh sr rebuild %s

Verify (both must be <= %d):

  docker exec %s cat /sys/class/net/eth0/mtu
  docker exec %s cat /sys/class/net/docker0/mtu

If the host's real path MTU is below its NIC MTU (a tunnel the NIC is unaware of), set it explicitly in runners.yml and rebuild:

  container_runner_image:
    mtu: %d`, runnerName, hostEgressMTU, outerContainer, outerContainer, hostEgressMTU),
		DocRef: "agentic-workflows.md §11c",
	}
}

// mtuProbeInWindow reports whether the MTU probe is in scope for the supplied
// host egress MTU. The pinning window is 0 < MTU < 1500: an MTU of 0 (unknown /
// undetectable) or >= 1500 (standard Ethernet, jumbo frames) has nothing to pin,
// so the probe is skipped entirely.
//
// This predicate is the single source of truth for that window: widening or
// narrowing it changes containerCheckDefs' slice membership in one place.
func mtuProbeInWindow(hostEgressMTU int) bool {
	return hostEgressMTU > 0 && hostEgressMTU < 1500
}

// containerCheckDefs returns the definitions for all in-scope agentic
// container probes, with Message/Remediation pre-rendered against the supplied
// outerContainer / runnerName / hostEgressMTU. The MTU probe is included only
// when hostEgressMTU falls in the pinning window (0 < MTU < 1500); the
// cache-env probe only when cacheEnabled (runners without a local cache keep
// GitHub's cache service and must not carry CUSTOM_ACTIONS_RESULTS_URL).
//
// The slice is composed from the per-def helpers above, which are the single
// source of truth for probe wording and gating.
func containerCheckDefs(outerContainer, runnerName string, hostEgressMTU int, cacheEnabled bool) []containerCheckDef {
	defs := []containerCheckDef{
		innerHostDockerInternalDef(outerContainer, runnerName),
		nodeNPMDef(outerContainer, runnerName),
		dockerSocketUserDef(outerContainer, runnerName),
		zstdDef(outerContainer, runnerName),
	}
	if cacheEnabled {
		defs = append(defs, cacheEnvDef(outerContainer, runnerName))
	}
	if mtuProbeInWindow(hostEgressMTU) {
		defs = append(defs, mtuDef(outerContainer, runnerName, hostEgressMTU))
	}
	return defs
}

// containerAgenticFanoutInnerScript renders the tagged probe blocks (and the
// trailing `true`) WITHOUT the docker-exec wrapper. The batched inner-hygiene
// probe reuses it verbatim — the probe bodies are instance-independent, so the
// batch wraps this same script in `docker exec "gh-sr-$i" sh -lc` per instance.
func containerAgenticFanoutInnerScript(defs []containerCheckDef) string {
	var inner strings.Builder
	for _, d := range defs {
		inner.WriteString("{ ")
		inner.WriteString(d.InnerBody)
		inner.WriteString("\n} >/dev/null 2>&1; echo \"#")
		inner.WriteString(d.Name)
		inner.WriteString(":$?\"\n")
	}
	inner.WriteString("true")
	return inner.String()
}

// containerAgenticFanoutSpecsFromDefs renders the per-check metadata map
// from a pre-built defs slice.
func containerAgenticFanoutSpecsFromDefs(defs []containerCheckDef) map[string]PrereqFailure {
	specs := make(map[string]PrereqFailure, len(defs))
	for _, d := range defs {
		specs[d.Name] = PrereqFailure{
			Name:        d.Name,
			Severity:    SeverityWarning,
			Message:     d.Message,
			Remediation: d.Remediation,
			DocRef:      d.DocRef,
		}
	}
	return specs
}

// dockerChainCheckCommand builds the single shell command that runs the
// three docker-chain prereq probes (CLI → daemon → <third>) and tags each
// with `#<name>:<exit>` on stdout. All three sub-probes run unconditionally
// so a single failing prerequisite surfaces every dependent failure in one
// pass — the Go parser maps each non-zero tag back to its PrereqFailure
// via the chain's specs. Replaces 3 sequential h.Run calls with 1 SSH
// round-trip on the `gh sr doctor` ValidateContainerPrereqs hot path.
//
// variant selects the third probe and its associated spec key:
//   - "privileged": --privileged support probe (used by ValidateContainerPrereqs)
//
// The first two probes (CLI version, daemon info) are shared; only the third
// differs to match the caller's intent.
func dockerChainCheckCommand(variant string) string {
	var third string
	switch variant {
	case "privileged":
		// Mirrors the original probe's dual check: docker must exit 0 AND
		// the inner shell must echo "privileged-ok". Either failing the
		// block emits a non-zero tag.
		third = `{ out=$(docker run --rm --privileged alpine sh -c "echo privileged-ok" 2>/dev/null); rc=$?; if [ "$rc" -ne 0 ] || [ "$out" != "privileged-ok" ]; then exit 1; fi; } >/dev/null 2>&1; echo "#docker-privileged:$?"`
	default:
		return ""
	}
	return `{ docker --version >/dev/null 2>&1; } >/dev/null 2>&1; echo "#docker-cli:$?"
{ docker info >/dev/null 2>&1; } >/dev/null 2>&1; echo "#docker-daemon:$?"
` + third
}

// dockerChainSpecs returns the per-probe metadata the docker-chain parser
// uses to convert a `#<name>:<non-zero>` tag into the corresponding
// PrereqFailure. The CLI/daemon specs are shared across both variants; the
// third probe spec varies. Pass the same variant string to both
// dockerChainCheckCommand and dockerChainSpecs so they line up.
func dockerChainSpecs(variant string) map[string]PrereqFailure {
	specs := map[string]PrereqFailure{
		"docker-cli": {
			Name:     "docker-cli",
			Severity: SeverityError,
			Message:  "docker CLI not found on PATH",
			Remediation: `On the host, install Docker:

  sudo apt-get update && sudo apt-get install -y docker.io
  sudo systemctl enable --now docker
  sudo usermod -aG docker $USER
  # Log out and back in for group membership to take effect`,
			DocRef: "agentic-workflows.md §3g",
		},
		"docker-daemon": {
			Name:     "docker-daemon",
			Severity: SeverityError,
			Message:  "docker daemon not running",
			Remediation: `Start the Docker daemon on the host:

  sudo systemctl start docker
  sudo systemctl enable docker  # persist across reboots`,
			DocRef: "agentic-workflows.md §3g",
		},
	}
	switch variant {
	case "privileged":
		specs["docker-privileged"] = PrereqFailure{
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
		}
	}
	return specs
}

// parseDockerChainOutput converts the stdout of dockerChainCheckCommand
// into the per-probe failure list. Each `#<name>:N` tag is mapped through
// the supplied specs; tags with N==0 (success) are dropped, non-zero tags
// produce a failure entry. Tags absent from the output (e.g. shell
// short-circuited away — never happens with dockerChainCheckCommand's
// unconditional block layout, but defensive against future tightening) are
// silently ignored. Order of returned failures matches tag order on stdout,
// which matches probe-block order in dockerChainCheckCommand.
func parseDockerChainOutput(out string, specs map[string]PrereqFailure) []PrereqFailure {
	var failures []PrereqFailure
	// SplitSeq avoids the upfront []string allocation that strings.Split makes
	// for the full tagged stdout. parseDockerChainOutput powers the docker
	// CLI / daemon / privileged probe trio consumed by
	// ValidateContainerPrereqs on the `gh sr doctor` hot path.
	for line := range strings.SplitSeq(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || !strings.HasPrefix(line, "#") {
			continue
		}
		rest := strings.TrimPrefix(line, "#")
		name, status, ok := strings.Cut(rest, ":")
		if !ok {
			continue
		}
		// Non-zero exit code → failure. Covers docker CLI missing (127),
		// daemon down (1), permission denied (1), image pull failure (125/1),
		// and any other error mode without the parser needing to know the
		// specific code.
		code, err := strconv.Atoi(strings.TrimSpace(status))
		if err != nil || code == 0 {
			continue
		}
		spec, ok := specs[name]
		if !ok {
			continue
		}
		failures = append(failures, spec)
	}
	return failures
}

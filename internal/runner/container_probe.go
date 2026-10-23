package runner

import (
	"fmt"
	"strings"

	"github.com/an-lee/gh-sr/internal/host"
)

// ContainerHostProbe is the per-instance result of the batched host-side
// container probe. It carries the same facts the per-instance probe chain
// (bootstrap marker + readiness) produced, but for every instance of a host
// in one shot.
type ContainerHostProbe struct {
	// State is the Docker container state ("running", "restarting",
	// "exited", "missing", ...). "missing" means the container does not
	// exist on the host.
	State string
	// InnerDockerdOK is true iff the inner dockerd answered `docker info`.
	// Only meaningful when State is "running" or "restarting".
	InnerDockerdOK bool
	// Registered is true iff /home/runner/.runner is present inside the
	// container. Only meaningful when State is "running" or "restarting".
	Registered bool
	// BootstrapFailed reports whether the host-side bootstrap-failed marker
	// exists in the instance's state directory.
	BootstrapFailed bool
}

// ProbeContainersBatch probes every instance on a host in ONE SSH round-trip,
// regardless of how many instances the host runs.
//
// It replaces the per-instance chain that `gh sr doctor` used to run for
// every container-mode runner:
//
//	state-dir resolve + marker -> 2 round-trips (echo $HOME + test -f)
//	ContainerStateStatus        -> 1 round-trip (docker inspect)
//	ProbeDinDContainerReadiness -> 1 round-trip (docker exec)
//
// That is four round-trips per instance, so a 12-container host spent ~48
// SSH round-trips on one doctor check — and each round-trip is a full
// connection setup against a host that is, on metered/WAN setups, the
// slowest part of the command. Batching the whole loop into one remote
// script turns that into a constant one round-trip per host, in the same
// win-class as the earlier per-command batchings (PRs #264, #269, #285,
// #358, #361, #372, #379, #380).
//
// The probe is best-effort per instance: a container that is missing, or an
// exec that fails, is reported as such rather than aborting the batch.
func ProbeContainersBatch(h *host.Host, instances []string) (map[string]ContainerHostProbe, error) {
	if len(instances) == 0 {
		return map[string]ContainerHostProbe{}, nil
	}
	if h.OS == "windows" {
		return nil, fmt.Errorf("batched container probe is POSIX-only (host %s is windows)", h.Name)
	}
	// An instance name reaches the remote shell as a literal, so only names
	// that are safe to embed are probed. A rejected name simply gets no
	// result, which the caller reports as a missing probe result.
	safe := make([]string, 0, len(instances))
	for _, inst := range instances {
		if err := SafeRunnerInstanceName(inst); err != nil {
			continue
		}
		safe = append(safe, inst)
	}
	if len(safe) == 0 {
		return map[string]ContainerHostProbe{}, nil
	}
	script := buildContainerProbeScript(h.RunnerBaseDir(), safe)
	out, err := h.Run(script)
	if err != nil {
		return nil, err
	}
	return parseContainerProbeOutput(out), nil
}

// buildContainerProbeScript builds the single remote shell command that
// probes every instance. The remote loop does the per-instance work (one
// `docker inspect`, one `docker exec`, one `test -f`) so the whole batch
// costs one SSH round-trip.
//
// The instance name is the LAST field of each output line and the other
// fields are fixed tokens, so an instance name containing the field
// separator cannot corrupt the flags — the parser splits into a fixed
// number of fields and keeps the remainder as the name.
func buildContainerProbeScript(baseDir string, instances []string) string {
	var b strings.Builder
	b.WriteString("for i in")
	for _, inst := range instances {
		b.WriteString(" " + quotePOSIXWord(inst))
	}
	b.WriteString("; do\n")
	b.WriteString("c=\"gh-sr-$i\"\n")
	b.WriteString("st=$(docker inspect --format '{{.State.Status}}' \"$c\" 2>/dev/null || echo missing)\n")
	b.WriteString("dd=no\n")
	b.WriteString("rg=no\n")
	b.WriteString("case \"$st\" in\n")
	b.WriteString("running|restarting)\n")
	b.WriteString("out=$(docker exec \"$c\" sh -c 'docker info >/dev/null 2>&1 && echo GH_SR_DOCKERD_OK || echo GH_SR_DOCKERD_NO; test -f /home/runner/.runner && echo GH_SR_RUNNER_OK || echo GH_SR_RUNNER_NO' 2>/dev/null)\n")
	b.WriteString("case \"$out\" in *GH_SR_DOCKERD_OK*) dd=yes ;; esac\n")
	b.WriteString("case \"$out\" in *GH_SR_RUNNER_OK*) rg=yes ;; esac\n")
	b.WriteString(";;\n")
	b.WriteString("esac\n")
	b.WriteString("bf=no\n")
	b.WriteString("if test -f \"" + baseDir + "/$i/" + bootstrapFailedMarker + "\" 2>/dev/null; then bf=yes; fi\n")
	b.WriteString("printf '%s|%s|%s|%s|%s\\n' \"$st\" \"$dd\" \"$rg\" \"$bf\" \"$i\"\n")
	b.WriteString("done")
	return b.String()
}

// parseContainerProbeOutput parses the batched probe output into one entry
// per instance. Lines that do not carry the expected shape are skipped, so a
// stray line from the remote shell cannot invent a result.
func parseContainerProbeOutput(out string) map[string]ContainerHostProbe {
	probes := make(map[string]ContainerHostProbe)
	for line := range strings.SplitSeq(out, "\n") {
		line = strings.TrimRight(line, "\r")
		if strings.TrimSpace(line) == "" {
			continue
		}
		parts := strings.SplitN(line, "|", 5)
		if len(parts) != 5 {
			continue
		}
		state, dd, rg, bf, inst := parts[0], parts[1], parts[2], parts[3], parts[4]
		if inst == "" {
			continue
		}
		probes[inst] = ContainerHostProbe{
			State:           state,
			InnerDockerdOK:  dd == "yes",
			Registered:      rg == "yes",
			BootstrapFailed: bf == "yes",
		}
	}
	return probes
}

// quotePOSIXWord renders s as a single POSIX shell word, safe to embed in a
// double-quoted string. Instance names come from the runner config, so they
// are escaped rather than trusted: a name containing a quote or a shell
// metacharacter must not be able to reshape the probe script.
func quotePOSIXWord(s string) string {
	return `"` + posixInstanceEscaper.Replace(s) + `"`
}

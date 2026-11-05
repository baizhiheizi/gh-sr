package agentic

import (
	"fmt"
	"strings"

	"github.com/an-lee/gh-sr/internal/host"
	"github.com/an-lee/gh-sr/internal/hostshell"
	"github.com/an-lee/gh-sr/internal/runner"
	"github.com/an-lee/gh-sr/internal/strfmt"
)

// AgenticInnerTarget identifies one agentic container-mode runner instance for
// the batched inner-hygiene sweep.
type AgenticInnerTarget struct {
	// Instance is the runner instance name; its container is "gh-sr-<instance>".
	Instance string
	// RunnerName is the runner config name used in remediation wording.
	RunnerName string
}

// AgenticInnerProbe is the per-instance result of ValidateAgenticInnerBatch.
type AgenticInnerProbe struct {
	// Reported is false when the instance produced no result section: the
	// remote script was cut short, or the instance name was rejected by
	// runner.SafeRunnerInstanceName and therefore never probed. Callers
	// must distinguish this from "clean" — a missing section is a probe
	// failure, not a passing one.
	Reported bool
	// State is the outer container state ("running", "restarting",
	// "exited", "missing", ...). Only meaningful when Reported is true.
	State string
	// Failures carries every failed probe for the instance — AWF inner
	// hygiene first (orphan containers, then orphan networks), then the
	// agentic fanout failures in tag order — with Message/Remediation/
	// DocRef pre-rendered against this instance, matching what the
	// per-instance chain (ValidateAWFHygieneInner +
	// ValidateContainerAgenticFanout) would have produced.
	Failures []PrereqFailure
}

// ValidateAgenticInnerBatch probes every agentic container-mode instance on a
// host in ONE SSH round-trip, regardless of how many instances the host runs.
//
// It replaces the per-instance chain that `gh sr doctor` used to run for every
// agentic runner:
//
//	ContainerStateStatus           -> 1 round-trip (docker inspect)
//	ValidateAWFHygieneInner        -> 2 round-trips (2 hygiene probes)
//	ValidateContainerAgenticFanout -> 1 round-trip (tagged fanout)
//
// That is four round-trips per instance, so an 8-agentic-container host spent
// ~32 of them on this single check — each one a full SSH session setup against
// what is often a WAN link. One host-level round-trip keeps the remote work
// (the probes are unchanged) and drops the per-instance connection cost.
//
// The remote script skips every probe except the state inspect for containers
// whose state is not exactly "running" — the same strict gate the per-instance
// chain applied, stricter than IsContainerAcceptingJobs because the inner
// probes need a fully-running container, not a transient "restarting" one.
//
// Instances whose name fails runner.SafeRunnerInstanceName are never probed
// and stay Reported=false; the caller surfaces them as missing results.
func ValidateAgenticInnerBatch(h *host.Host, targets []AgenticInnerTarget, hostEgressMTU int, cacheEnabled bool) ([]AgenticInnerProbe, error) {
	probes := make([]AgenticInnerProbe, len(targets))
	if len(targets) == 0 {
		return probes, nil
	}
	if h == nil || h.OS != "linux" {
		return nil, fmt.Errorf("batched agentic inner probe requires a Linux host")
	}
	specs := make([]agenticInnerSpecs, len(targets))
	safe := make([]string, len(targets))
	haveProbed := false
	var fanoutInner string
	for i, t := range targets {
		if err := runner.SafeRunnerInstanceName(t.Instance); err != nil {
			continue
		}
		cname := runner.ContainerDockerName(t.Instance)
		safe[i] = t.Instance
		defs := containerCheckDefs(cname, t.RunnerName, hostEgressMTU, cacheEnabled)
		specs[i] = agenticInnerSpecs{
			hygiene: awfInnerHygieneChecks(cname),
			fanout:  containerAgenticFanoutSpecsFromDefs(defs),
		}
		// The fanout inner script embeds no per-instance values (probe bodies
		// and gates are instance-independent; only Message/Remediation carry
		// names), so rendering it once from the first probed target's defs
		// yields the script every instance runs.
		if !haveProbed {
			fanoutInner = containerAgenticFanoutInnerScript(defs)
			haveProbed = true
		}
	}
	if !haveProbed {
		return probes, nil
	}
	script := buildAgenticInnerBatchScript(safe, fanoutInner)
	out, err := h.Run(script)
	if err != nil {
		return nil, err
	}
	parseAgenticInnerBatchOutput(out, specs, probes)
	return probes, nil
}

// agenticInnerSpecs holds the per-instance metadata the batch parser needs to
// turn captured output into PrereqFailures: the two AWF hygiene checks
// pre-rendered for this instance's container, and the fanout spec map keyed by
// check name.
type agenticInnerSpecs struct {
	hygiene []awfHygieneCheck
	fanout  map[string]PrereqFailure
}

// buildAgenticInnerBatchScript renders the single remote command for the whole
// host. Loop indices (not names) key the markers, so instance names containing
// colons, pipes, spaces, or marker-like text cannot corrupt the stream; names
// reach the for-list single-quoted so no metacharacter can reshape the loop.
//
// The hygiene probe commands are name-free (only their failure Message/
// Remediation embed the container name), so awfInnerHygieneChecks("") supplies
// them once for every instance.
//
// Output contract per probed instance:
//
//	@@begin:<n>
//	@@state:<n>|<state>
//	@@hyg:<n>|<k> ... @@hygend:<n>|<k>   (capturing each hygiene probe's stdout)
//	#<check>:<exit>                      (fanout tags)
//	@@end:<n>
//
// Container and network names (the only variable content) cannot contain "@"
// or start lines with "#", and the fanout blocks send everything except their
// echo tags to /dev/null, so the markers delimit exactly what the parser owns.
func buildAgenticInnerBatchScript(safeNames []string, fanoutInner string) string {
	hygiene := awfInnerHygieneChecks("")
	var b strings.Builder
	b.WriteString("n=0\nfor i in")
	for _, name := range safeNames {
		if name == "" {
			continue
		}
		b.WriteString(" " + hostshell.PosixSingleQuote(name))
	}
	b.WriteString("; do\n")
	b.WriteString("n=$((n+1))\n")
	b.WriteString(`echo "@@begin:$n"
st=$(docker inspect --format '{{.State.Status}}' "gh-sr-$i" 2>/dev/null || echo missing)
echo "@@state:$n|$st"
if [ "$st" = running ]; then
`)
	for k, c := range hygiene {
		fmt.Fprintf(&b, "echo \"@@hyg:$n|%d\"\n", k)
		fmt.Fprintf(&b, "docker exec \"gh-sr-$i\" %s\n", c.Cmd)
		fmt.Fprintf(&b, "echo \"@@hygend:$n|%d\"\n", k)
	}
	fmt.Fprintf(&b, "docker exec \"gh-sr-$i\" sh -lc %s\n", hostshell.PosixSingleQuote(fanoutInner))
	b.WriteString(`fi
echo "@@end:$n"
done`)
	return b.String()
}

// parseAgenticInnerBatchOutput walks the tagged stream and fills probes in
// place. Sections that never opened or never closed leave their probe
// Reported=false. A `#<name>:1` tag maps through the instance's fanout specs;
// a hygiene section with non-empty trimmed output maps to that check's
// pre-rendered `-inner` failure. Fanout tags other than `:1` are ignored —
// the same rule parseContainerAgenticFanoutOutput applies to the
// per-instance fanout.
func parseAgenticInnerBatchOutput(out string, specs []agenticInnerSpecs, probes []AgenticInnerProbe) {
	// Markers number instances from 1 (the remote loop's counter); probes and
	// specs are 0-based, so every marker number is used as idx = n-1.
	cur := -1
	hyg := -1
	var hygBuf strings.Builder
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimRight(line, "\r")
		switch {
		case strings.HasPrefix(line, "@@begin:"):
			if n, ok := markerNum(line, "@@begin:"); ok {
				if idx := n - 1; idx >= 0 && idx < len(probes) {
					cur = idx
					hyg = -1
					hygBuf.Reset()
				}
			}
		case strings.HasPrefix(line, "@@state:"):
			if n, rest, ok := strfmt.SplitMarker(line, "@@state:"); ok && cur >= 0 && n-1 == cur {
				probes[cur].State = rest
			}
		case strings.HasPrefix(line, "@@hyg:"):
			if n, rest, ok := strfmt.SplitMarker(line, "@@hyg:"); ok && cur >= 0 && n-1 == cur {
				hyg = strfmt.AtoiOrNeg(rest)
				hygBuf.Reset()
			}
		case strings.HasPrefix(line, "@@hygend:"):
			if n, rest, ok := strfmt.SplitMarker(line, "@@hygend:"); ok && cur >= 0 && n-1 == cur {
				probes[cur].Failures = append(probes[cur].Failures, hygieneFailureFromOutput(specs[cur].hygiene, strfmt.AtoiOrNeg(rest), hygBuf.String())...)
				hyg = -1
			}
		case strings.HasPrefix(line, "@@end:"):
			if n, ok := markerNum(line, "@@end:"); ok && cur >= 0 && n-1 == cur {
				probes[cur].Reported = true
				cur = -1
				hyg = -1
			}
		case cur >= 0 && hyg < 0 && strings.HasPrefix(line, "#"):
			name, status, ok := strings.Cut(strings.TrimPrefix(line, "#"), ":")
			if !ok || strings.TrimSpace(status) != "1" {
				continue
			}
			if spec, ok := specs[cur].fanout[name]; ok {
				probes[cur].Failures = append(probes[cur].Failures, spec)
			}
		case hyg >= 0:
			hygBuf.WriteString(line)
			hygBuf.WriteString("\n")
		}
	}
}

// hygieneFailureFromOutput renders the `-inner` AWF hygiene failure for check
// k when its captured output is non-empty after trimming — the same
// "output means orphans" contract runAWFHygieneChecks applies per probe. An
// out-of-range k (a malformed or future-incompatible marker) yields nothing.
func hygieneFailureFromOutput(checks []awfHygieneCheck, k int, output string) []PrereqFailure {
	if k < 0 || k >= len(checks) {
		return nil
	}
	if strings.TrimSpace(output) == "" {
		return nil
	}
	c := checks[k]
	return []PrereqFailure{{
		Name:        c.Name + "-inner",
		Severity:    SeverityWarning,
		Message:     c.Message,
		Remediation: c.Remediation,
		DocRef:      "agentic-workflows.md §12",
	}}
}

// markerNum parses "<prefix><n>" into n. It stays strict about a trailing
// "|<tail>" (unlike strfmt.SplitMarker, which accepts it) so a malformed
// begin/end marker is ignored rather than partially honoured.
func markerNum(line, prefix string) (int, bool) {
	rest := strings.TrimPrefix(line, prefix)
	if rest == line {
		return -1, false
	}
	n := strfmt.AtoiOrNeg(rest)
	return n, n >= 0
}

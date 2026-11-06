package agentic

import (
	"fmt"
	"strings"

	"github.com/an-lee/gh-sr/internal/host"
	"github.com/an-lee/gh-sr/internal/hostshell"
	"github.com/an-lee/gh-sr/internal/probe"
	"github.com/an-lee/gh-sr/internal/runner"
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
// host: one self-contained block per probed instance, each emitting the probe
// package's framed tagged sections keyed by its literal 1-based index.
// Indices (not names) key the markers, so instance names containing colons,
// pipes, spaces, or marker-like text cannot corrupt the stream; names are
// single-quoted wherever they embed, so no metacharacter can reshape the
// script.
//
// The hygiene probe commands are name-free (only their failure Message/
// Remediation embed the container name), so awfInnerHygieneChecks("") supplies
// them for every instance.
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
	w := probe.NewWriter(&b)
	n := 0
	for _, name := range safeNames {
		if name == "" {
			continue
		}
		n++
		cname := hostshell.PosixSingleQuote(runner.ContainerDockerName(name))
		w.Frame("@@begin", n)
		fmt.Fprintf(&b, "st=$(docker inspect --format '{{.State.Status}}' %s 2>/dev/null || echo missing)\n", cname)
		w.Value("@@state", n, "$st")
		b.WriteString("if [ \"$st\" = running ]; then\n")
		for k, c := range hygiene {
			w.CaptureOpen("@@hyg", n, k)
			fmt.Fprintf(&b, "docker exec %s %s\n", cname, c.Cmd)
			w.CaptureClose("@@hygend", n, k)
		}
		fmt.Fprintf(&b, "docker exec %s sh -lc %s\n", cname, hostshell.PosixSingleQuote(fanoutInner))
		b.WriteString("fi\n")
		w.FrameEnd("@@end", n)
	}
	return b.String()
}

// parseAgenticInnerBatchOutput walks the tagged stream and fills probes in
// place. Sections that never opened or never closed leave their probe
// Reported=false — a block with no @@end never dispatches the probe
// package's FrameEnd, where Reported is set. A `#<name>:1` tag maps through
// the instance's fanout specs; a hygiene section with non-empty trimmed
// output maps to that check's pre-rendered `-inner` failure. Fanout tags
// other than `:1` are ignored — the same rule
// parseContainerAgenticFanoutOutput applies to the per-instance fanout.
func parseAgenticInnerBatchOutput(out string, specs []agenticInnerSpecs, probes []AgenticInnerProbe) {
	parser := probe.Parser{
		Frame:    "@@begin",
		FrameEnd: "@@end",
		Targets:  len(probes),
		Sections: probe.Sections{
			"@@state": "",
			"@@hyg":   "@@hygend",
		},
		Handlers: probe.Handlers{
			FrameEnd: func(n int) {
				probes[n-1].Reported = true
			},
			Value: func(n int, marker, tail string) {
				if marker == "@@state" {
					probes[n-1].State = tail
				}
			},
			Captured: func(n int, marker string, k int, text string) {
				probes[n-1].Failures = append(probes[n-1].Failures,
					hygieneFailureFromOutput(specs[n-1].hygiene, k, text)...)
			},
			Line: func(n int, line string) {
				if !strings.HasPrefix(line, "#") {
					return
				}
				name, status, ok := strings.Cut(strings.TrimPrefix(line, "#"), ":")
				if !ok || strings.TrimSpace(status) != "1" {
					return
				}
				if spec, ok := specs[n-1].fanout[name]; ok {
					probes[n-1].Failures = append(probes[n-1].Failures, spec)
				}
			},
		},
	}
	parser.Parse(out)
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

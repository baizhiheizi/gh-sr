package agentic

import (
	"testing"
)

// BenchmarkContainerCheckDefs measures the per-call cost of building the
// in-scope agentic container probe slice. containerCheckDefs is the workhorse
// shared by ValidateContainerAgenticFanout (the fanout wrapper, called once
// per `gh sr doctor` orchestrator invocation per container-mode runner) and
// containerCheckDefByName (the per-check wrappers, called per individual
// probe). The slice is small (4 base + up to 2 gated defs) but each def runs
// 2-3 fmt.Sprintf calls to pre-render Message/Remediation against the
// outerContainer / runnerName / hostEgressMTU inputs, so per-def cost is
// non-trivial.
//
// Run via: make bench (covers all packages) or directly:
//
//	go test ./internal/agentic/... -bench BenchmarkContainerCheckDefs -benchmem -count=5 -run='^$'
func BenchmarkContainerCheckDefs(b *testing.B) {
	const (
		oc   = "gh-sr-runner"
		name = "agentic-1"
	)
	cases := []struct {
		name          string
		hostEgressMTU int
		cacheEnabled  bool
	}{
		{"always_on", 0, false},
		{"cache_only", 0, true},
		{"mtu_only", 1400, false},
		{"all_gated", 1400, true},
	}
	for _, tc := range cases {
		tc := tc
		b.Run(tc.name, func(b *testing.B) {
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_ = containerCheckDefs(oc, name, tc.hostEgressMTU, tc.cacheEnabled)
			}
		})
	}
}

// BenchmarkValidateContainerAgenticFanout_Setup isolates the pre-Run work
// the fanout wrapper does: building the defs slice, the combined shell
// command, and the parser specs map. The fanout then pays for h.Run and
// parseContainerAgenticFanoutOutput — those are fixed costs we don't touch.
// This bench only measures what the dedup can reclaim: redundant
// fmt.Sprintf allocations in the defs slice + the strings.Builder churn
// that consumes them.
//
// The "split" sub-benchmark calls the legacy (outer, runnerName, mtu,
// cache) helpers separately — what the fanout wrapper used to do, paying
// for two full containerCheckDefs builds. The "shared" sub-benchmark calls
// the FromDefs helpers after one containerCheckDefs build — what the fanout
// wrapper now does. The gap between them is the energy the refactor saves
// on every `gh sr doctor` probe against a container-mode runner.
//
// Run via:
//
//	go test ./internal/agentic/... -bench BenchmarkValidateContainerAgenticFanout_Setup -benchmem -count=5 -run='^$'
func BenchmarkValidateContainerAgenticFanout_Setup(b *testing.B) {
	const (
		oc   = "gh-sr-runner"
		name = "agentic-1"
	)
	cases := []struct {
		name          string
		hostEgressMTU int
		cacheEnabled  bool
	}{
		{"always_on", 0, false},
		{"all_gated", 1400, true},
	}
	for _, tc := range cases {
		tc := tc
		b.Run(tc.name+"/split", func(b *testing.B) {
			// Legacy shape: each helper builds its own defs slice.
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_ = containerAgenticFanoutCheckCommand(oc, name, tc.hostEgressMTU, tc.cacheEnabled)
				_ = containerAgenticFanoutSpecs(oc, name, tc.hostEgressMTU, tc.cacheEnabled)
			}
		})
		b.Run(tc.name+"/shared", func(b *testing.B) {
			// New shape: build the defs slice once and feed both
			// downstream helpers.
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				defs := containerCheckDefs(oc, name, tc.hostEgressMTU, tc.cacheEnabled)
				_ = containerAgenticFanoutCheckCommandFromDefs(oc, defs)
				_ = containerAgenticFanoutSpecsFromDefs(defs)
			}
		})
	}
}

// BenchmarkContainerCheckDefByName measures the per-call cost of looking up
// a single probe definition by name. Switches on name to build only the
// requested def — the per-check wrappers (called from `gh sr doctor`
// container-mode probes and from the standalone *CheckCommand helpers) pay
// for one def's worth of fmt.Sprintf instead of every sibling's. Run this
// after any change to the per-def constructors to confirm the byName
// shape still avoids the slice-of-all-defs fanout.
//
// Run via:
//
//	go test ./internal/agentic/... -bench BenchmarkContainerCheckDefByName -benchmem -count=5 -run='^$'
func BenchmarkContainerCheckDefByName(b *testing.B) {
	const (
		oc   = "gh-sr-runner"
		name = "agentic-1"
	)
	cases := []struct {
		name          string
		probe         string
		hostEgressMTU int
		cacheEnabled  bool
	}{
		{"always_on_inner", "container-inner-host-docker-internal", 0, false},
		{"gated_cache_env", "container-cache-env", 0, true},
		{"gated_mtu", "container-mtu", 1400, false},
	}
	for _, tc := range cases {
		tc := tc
		b.Run(tc.name, func(b *testing.B) {
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_, _ = containerCheckDefByName(tc.probe, oc, name, tc.hostEgressMTU, tc.cacheEnabled)
			}
		})
	}
}

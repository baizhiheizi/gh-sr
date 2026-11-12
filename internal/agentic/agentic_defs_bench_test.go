package agentic

import (
	"testing"
)

// BenchmarkContainerCheckDefs measures the per-call cost of building the
// in-scope agentic container probe slice. containerCheckDefs feeds the
// batched inner probe (ValidateAgenticInnerBatch) once per container-mode
// runner per `gh sr doctor` orchestrator invocation. The slice is small
// (4 base + up to 2 gated defs) but each def runs 2-3 fmt.Sprintf calls to
// pre-render Message/Remediation against the outerContainer / runnerName /
// hostEgressMTU inputs, so per-def cost is non-trivial.
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

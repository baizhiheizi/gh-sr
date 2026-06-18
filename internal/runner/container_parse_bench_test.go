package runner

import "testing"

// parseContainerStatusInspectOutputInputs mirrors the inspect-output lines
// containerLocalStatusFromDocker emits in production. parseContainerStatusInspectOutput
// is called once per host per Status() call, which itself runs on every TUI
// refresh tick (5s by default), so for a 10-host panel this function runs
// ~10x/refresh and is in the per-keypress render hot path indirectly.
var parseContainerStatusInspectOutputInputs = []string{
	"running|gh-sr/agentic-runner:2.320.0|sha256:abc123|deadbeef",
	"running|gh-sr/agentic-runner:2.320.0-xa1b2c3d|sha256:x|",
	"exited|gh-sr/agentic-runner:1.0.0|sha256:1|rev1",
	"restarting|x:y|sha256:4|r",
	"not installed|||",
}

// BenchmarkParseContainerStatusInspectOutput measures the per-call cost of
// parsing the pipe-delimited inspect line. strings.Cut chain is 0-alloc vs
// strings.Split's 4-element slice + padding loop.
func BenchmarkParseContainerStatusInspectOutput(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		for _, in := range parseContainerStatusInspectOutputInputs {
			_, _, _ = parseContainerStatusInspectOutput(in)
		}
	}
}

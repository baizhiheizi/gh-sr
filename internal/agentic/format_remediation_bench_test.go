package agentic

import (
	"testing"
)

// BenchmarkFormatRemediation measures the per-call cost of building a
// formatted remediation string. FormatRemediation is invoked once per
// prereq failure during `gh sr doctor` reporting and once per failure inside
// FormatAllRemediations. The Remediation string is typically a 4-6 line
// command snippet (e.g. "apt-get update && apt-get install ..."), so the
// inner strings.Split previously allocated a 5-6 element slice plus string
// headers per call. Switching the line iterator to strings.SplitSeq removes
// the slice header and backing array allocation per call.
func BenchmarkFormatRemediation(b *testing.B) {
	specs := []struct {
		name string
		f    PrereqFailure
	}{
		{
			name: "single_line",
			f: PrereqFailure{
				Message:     "package missing",
				Remediation: "apt-get install foo",
			},
		},
		{
			name: "multi_line",
			f: PrereqFailure{
				Message:     "service not running",
				Remediation: "systemctl enable foo\nsystemctl start foo\nsystemctl status foo",
			},
		},
	}
	for _, s := range specs {
		b.Run(s.name, func(b *testing.B) {
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_ = FormatRemediation(s.f)
			}
		})
	}
}

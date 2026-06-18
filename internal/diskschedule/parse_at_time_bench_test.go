package diskschedule

import "testing"

// parseAtTimeInputs mirrors the time strings parseAtTime sees in production.
// Install is the only caller, and the contract is HH:MM (well-formed), so
// realistic inputs are well-formed values that a user might pass via
// `--at-time` or that DefaultAtTime ("03:00") exercises on every install.
var parseAtTimeInputs = []string{
	"03:00",
	"00:00",
	"23:59",
	"9:30",
	"12:5",
	"9:00",
}

// BenchmarkParseAtTime measures the per-install cost of validating AtTime.
// Install is called rarely (once per user, when wiring up the schedule), so
// the absolute saving is small — but this is the same fmt.Sscanf anti-pattern
// that PR #191 fixed in extractTrailingPercent, and the fix is one line.
func BenchmarkParseAtTime(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		for _, in := range parseAtTimeInputs {
			_, _, _ = parseAtTime(in)
		}
	}
}

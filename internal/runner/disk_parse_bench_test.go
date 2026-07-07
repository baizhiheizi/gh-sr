package runner

import "testing"

// BenchmarkParseFourInt64s measures the per-call cost of parsing one line of
// four int64 values. parseFourInt64s is called once per host per
// `gh sr disk` listing refresh (via dirSizesWindows and dirSizesPOSIX), so
// dropping the strings.Fields allocation compounds across listings with many
// hosts.
func BenchmarkParseFourInt64s(b *testing.B) {
	raw := "1000000 500000 100000 300000\n"
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _, _, _, _ = parseFourInt64s(raw)
	}
}

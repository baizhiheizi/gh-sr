package autostart

import (
	"strings"
	"testing"
)

// formatLaunchdDetailBefore mirrors the pre-optimisation implementation:
// strings.Split materialises the full launchd output into a []string, then
// re-slices to the first 5 entries. Kept side-by-side with formatLaunchdDetail
// so the BenchmarkFormatLaunchdDetail pairs can produce before/after numbers
// without re-checking out the old source.
func formatLaunchdDetailBefore(out string) string {
	lines := strings.Split(out, "\n")
	if len(lines) > 5 {
		lines = lines[:5]
	}
	return strings.TrimSpace(strings.Join(lines, " "))
}

// BenchmarkFormatLaunchdDetail measures the per-call cost of flattening the
// launchd `print` output into the `installed (launchd): ...` Status row
// detail. Status runs once per native macOS instance per Status tick; the
// launchd `print` output is typically 10-20 lines.
//
// The before variant uses the original strings.Split + slice-then-cap
// implementation. The after variant uses strings.SplitSeq + early stop,
// which avoids materialising the full launchd print slice and stops
// iterating after 5 lines.
func BenchmarkFormatLaunchdDetail(b *testing.B) {
	specs := []struct {
		name  string
		input string
	}{
		{
			name:  "short_3_lines",
			input: "label = com.example.runner\nprogram = /opt/runner/run.sh\nrunAtLoad = true",
		},
		{
			name: "exact_5_lines",
			input: strings.Join([]string{
				"label = com.example.runner",
				"program = /opt/runner/run.sh",
				"runAtLoad = true",
				"keepAlive = true",
				"standardOutPath = /tmp/runner.log",
			}, "\n"),
		},
		{
			name:  "long_20_lines",
			input: strings.Repeat("environment variables = { PATH = /opt/runner/bin }\n", 20),
		},
	}
	for _, s := range specs {
		b.Run("before/"+s.name, func(b *testing.B) {
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_ = formatLaunchdDetailBefore(s.input)
			}
		})
		b.Run("after/"+s.name, func(b *testing.B) {
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_ = formatLaunchdDetail(s.input)
			}
		})
	}
}

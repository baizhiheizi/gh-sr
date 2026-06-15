package tui

import (
	"testing"

	"github.com/an-lee/gh-sr/internal/host"
)

// TestExtractTrailingPercent pins the contract that colorizePercent depends on:
// given the cell strings produced by metricsRow, extract the trailing
// percentage as a float (or 0 when no number is present). This locks the
// behavior before any optimization-driven refactor of the parser.
func TestExtractTrailingPercent(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		in   string
		want float64
	}{
		{"paren_percent", "12.3/45.6 GiB (78.9%)", 78.9},
		{"zero_percent", "0.0/0.0 MiB (0%)", 0.0},
		{"integer_percent", "99.0/100.0 GiB (95.5%)", 95.5},
		{"bare_percent", "3.2%", 3.2},
		{"hundred_percent", "100%", 100.0},
		{"no_number", "err", 0.0},
		{"dash_placeholder", "-", 0.0},
		{"unreachable_text", "unreachable", 0.0},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := extractTrailingPercent(tc.in); got != tc.want {
				t.Errorf("extractTrailingPercent(%q) = %v, want %v", tc.in, got, tc.want)
			}
		})
	}
}

// extractTrailingPercentInputs mirrors the cell formats emitted by metricsRow
// in production (the three colored columns on the host-metrics table).
var extractTrailingPercentInputs = []string{
	"12.3/45.6 GiB (78.9%)",
	"0.0/0.0 MiB (0%)",
	"99.0/100.0 GiB (95.5%)",
	"err",
	"-",
	"unreachable",
	"3.2%",
	"100%",
}

// BenchmarkExtractTrailingPercent measures the per-cell parse cost that runs
// once per colored metrics cell per Bubble Tea View() call (per keypress and
// per refresh tick). With 10 hosts × 3 colored columns, that's 30 calls per
// render; over a long TUI session the cumulative cost is non-trivial.
func BenchmarkExtractTrailingPercent(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		for _, in := range extractTrailingPercentInputs {
			_ = extractTrailingPercent(in)
		}
	}
}

// BenchmarkMetricsRow exercises the full per-host row construction that
// runs once per host per render. Combined with the colorize callback this is
// the hot path the TUI host-metrics panel hammers on every View() call.
func BenchmarkMetricsRow(b *testing.B) {
	samples := []host.HostMetrics{
		{Name: "h1", CPUPercent: 12.3, MemUsedMiB: 1024, MemTotalMiB: 4096, DiskUsedGiB: 50, DiskTotalGiB: 200, Load1: 0.5, Load5: 0.4, Load15: 0.3, Uptime: "5d"},
		{Name: "h2", CPUPercent: 0.0, MemUsedMiB: 0, MemTotalMiB: 0, DiskUsedGiB: 0, DiskTotalGiB: 0, Uptime: "-"},
		{Name: "h3", Err: errSynthetic("ssh: handshake timeout")},
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, m := range samples {
			_ = metricsRow(m)
		}
	}
}

type syntheticError struct{ s string }

func (e *syntheticError) Error() string { return e.s }
func errSynthetic(s string) error       { return &syntheticError{s: s} }

package host

import (
	"testing"
)

// BenchmarkParseUnixMetrics measures parseUnixMetrics on a realistic script
// output shape (8 key=value lines bracketed by sentinels + preamble/trailing
// noise). This is the per-tick hot path called from the TUI dashboard's metric
// refresh goroutine.
func BenchmarkParseUnixMetrics(b *testing.B) {
	raw := `some preamble noise
::GH_SR_METRICS_START::
cpu_idle=85.3
mem_total_mib=16024
mem_used_mib=12300
disk_total_gib=500
disk_used_gib=123
load=1.23 0.89 0.45
uptime=3d 4h 12m
::GH_SR_METRICS_END::
trailing noise`
	var m HostMetrics
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := parseUnixMetrics(raw, &m); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkLoadStr measures the per-host load-avg formatter that runs once
// per host per TUI render. Mirrors the realistic nonzero triple used in
// parseUnixMetrics tests so the strings.Builder path can be compared head-to-
// head with the old fmt.Sprintf path.
func BenchmarkLoadStr(b *testing.B) {
	m := HostMetrics{Load1: 1.23, Load5: 0.89, Load15: 0.45}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = m.LoadStr()
	}
}

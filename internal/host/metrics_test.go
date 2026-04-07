package host

import (
	"testing"
)

func TestParseUnixMetrics_Linux(t *testing.T) {
	raw := `some preamble noise
::GHR_METRICS_START::
cpu_idle=85.3
mem_total_mib=16024
mem_used_mib=12300
disk_total_gib=500
disk_used_gib=123
load=1.23 0.89 0.45
uptime=3d 4h 12m
::GHR_METRICS_END::
trailing noise`

	var m HostMetrics
	if err := parseUnixMetrics(raw, &m); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	assertFloat(t, "CPUPercent", m.CPUPercent, 14.7)
	assertFloat(t, "MemTotalMiB", m.MemTotalMiB, 16024)
	assertFloat(t, "MemUsedMiB", m.MemUsedMiB, 12300)
	assertFloat(t, "DiskTotalGiB", m.DiskTotalGiB, 500)
	assertFloat(t, "DiskUsedGiB", m.DiskUsedGiB, 123)
	assertFloat(t, "Load1", m.Load1, 1.23)
	assertFloat(t, "Load5", m.Load5, 0.89)
	assertFloat(t, "Load15", m.Load15, 0.45)

	if m.Uptime != "3d 4h 12m" {
		t.Errorf("Uptime = %q, want %q", m.Uptime, "3d 4h 12m")
	}
}

func TestParseUnixMetrics_MissingEnd(t *testing.T) {
	raw := `::GHR_METRICS_START::
cpu_idle=50.0
mem_total_mib=8192
mem_used_mib=4096`

	var m HostMetrics
	if err := parseUnixMetrics(raw, &m); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	assertFloat(t, "CPUPercent", m.CPUPercent, 50.0)
	assertFloat(t, "MemTotalMiB", m.MemTotalMiB, 8192)
}

func TestHostMetrics_Percents(t *testing.T) {
	m := HostMetrics{
		MemUsedMiB:   3000,
		MemTotalMiB:  8000,
		DiskUsedGiB:  100,
		DiskTotalGiB: 500,
	}

	assertFloat(t, "MemPercent", m.MemPercent(), 37.5)
	assertFloat(t, "DiskPercent", m.DiskPercent(), 20.0)
}

func TestHostMetrics_ZeroTotal(t *testing.T) {
	m := HostMetrics{}
	if m.MemPercent() != 0 {
		t.Errorf("MemPercent on zero total should be 0, got %f", m.MemPercent())
	}
	if m.DiskPercent() != 0 {
		t.Errorf("DiskPercent on zero total should be 0, got %f", m.DiskPercent())
	}
}

func TestHostMetrics_LoadStr(t *testing.T) {
	m := HostMetrics{Load1: 1.5, Load5: 0.8, Load15: 0.3}
	if got := m.LoadStr(); got != "1.50 0.80 0.30" {
		t.Errorf("LoadStr() = %q, want %q", got, "1.50 0.80 0.30")
	}

	empty := HostMetrics{}
	if got := empty.LoadStr(); got != "-" {
		t.Errorf("LoadStr() on zero load = %q, want %q", got, "-")
	}
}

func assertFloat(t *testing.T, name string, got, want float64) {
	t.Helper()
	diff := got - want
	if diff < -0.1 || diff > 0.1 {
		t.Errorf("%s = %.2f, want %.2f", name, got, want)
	}
}

package host

import (
	"strings"
	"testing"

	"github.com/an-lee/gh-sr/internal/config"
	"github.com/an-lee/gh-sr/internal/testutil"
)

func TestParseUnixMetrics_Linux(t *testing.T) {
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
	raw := `::GH_SR_METRICS_START::
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

// TestParseUnixMetrics_LoadMalformed locks the IndexByte-based load
// parser's tolerance to incomplete load lines. After the zero-alloc refactor
// of parseUnixMetrics' load case (replacing strings.Fields with two
// strings.IndexByte calls + three sub-slice ParseFloats), a load line with
// fewer than three space-separated values must leave the Load fields at
// their zero value rather than panicking on a negative index.
func TestParseUnixMetrics_LoadMalformed(t *testing.T) {
	cases := []struct {
		name string
		line string
	}{
		{"no spaces", "load=0.50"},
		{"one space only", "load=0.50 0.40"},
		{"empty value", "load="},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			raw := "::GH_SR_METRICS_START::\n" + tc.line + "\n::GH_SR_METRICS_END::"
			var m HostMetrics
			if err := parseUnixMetrics(raw, &m); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			// Loads must remain zero on malformed input — the parser
			// silently skips rather than corrupting m with partial data.
			if m.Load1 != 0 || m.Load5 != 0 || m.Load15 != 0 {
				t.Errorf("load fields should stay zero on malformed %q, got %v %v %v",
					tc.line, m.Load1, m.Load5, m.Load15)
			}
		})
	}
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

// TestCollectMetrics_DispatchOS locks the platform-specific script generator
// picked by Host.CollectMetrics. Regression guard: a future refactor that
// silently drops the darwin or windows branch would otherwise pass CI because
// the dispatcher's per-OS path is unit-test-free.
func TestCollectMetrics_DispatchOS(t *testing.T) {
	t.Parallel()

	const sampleOutput = `::GH_SR_METRICS_START::
cpu_idle=50.0
mem_total_mib=8192
mem_used_mib=4096
disk_total_gib=100
disk_used_gib=50
load=0.50 0.40 0.30
uptime=1d 2h 3m
::GH_SR_METRICS_END::`

	cases := []struct {
		name     string
		os       string
		addr     string
		wantSubs []string // substrings that MUST appear in the dispatched script
	}{
		{
			name:     "linux uses /proc/stat",
			os:       "linux",
			addr:     "user@host",
			wantSubs: []string{"/proc/stat", "/proc/meminfo", "/proc/loadavg"},
		},
		{
			name:     "darwin uses top + vm_stat",
			os:       "darwin",
			addr:     "user@host",
			wantSubs: []string{"top -l 2", "vm_stat", "vm.loadavg"},
		},
		{
			name: "windows uses powershell with -EncodedCommand",
			os:   "windows",
			addr: "user@host",
			// For Windows + non-local addr, host.wrapCommand base64-encodes the
			// script via encodePowerShellScript, so the body isn't directly
			// searchable. The presence of `-EncodedCommand` and the exe is the
			// reliable signal that the Windows branch was taken.
			wantSubs: []string{"powershell.exe", "-EncodedCommand"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			mock := &testutil.MockExecutor{Output: sampleOutput}
			h := newMockHost("test", config.HostConfig{OS: tc.os, Addr: tc.addr}, mock)

			got := h.CollectMetrics()

			if got.Err != nil {
				t.Fatalf("CollectMetrics err = %v, want nil", got.Err)
			}
			if len(mock.Calls) != 1 {
				t.Fatalf("expected exactly 1 shell call, got %d: %q", len(mock.Calls), mock.Calls)
			}
			cmd := mock.Calls[0]
			for _, sub := range tc.wantSubs {
				if !strings.Contains(cmd, sub) {
					t.Errorf("dispatched script missing %q\nfull command:\n%s", sub, cmd)
				}
			}
			if got.Name != "test" {
				t.Errorf("Name = %q, want %q", got.Name, "test")
			}
			if got.CPUPercent < 0 || got.CPUPercent > 100 {
				t.Errorf("CPUPercent out of range: %f", got.CPUPercent)
			}
		})
	}
}

// TestUnixMetricsScript_Branches locks both POSIX branches in unixMetricsScript.
// The darwin branch is structurally different from the linux branch (top vs
// /proc/stat, vm_stat vs /proc/meminfo); a future refactor that drops one
// branch should fail this test.
func TestUnixMetricsScript_Branches(t *testing.T) {
	t.Parallel()

	t.Run("linux", func(t *testing.T) {
		script := unixMetricsScript("linux")
		for _, sub := range []string{"/proc/stat", "/proc/meminfo", "/proc/loadavg", "GH_SR_METRICS_START", "GH_SR_METRICS_END"} {
			if !strings.Contains(script, sub) {
				t.Errorf("linux script missing %q", sub)
			}
		}
	})

	t.Run("darwin", func(t *testing.T) {
		script := unixMetricsScript("darwin")
		for _, sub := range []string{"top -l 2", "vm_stat", "vm.loadavg", "GH_SR_METRICS_START", "GH_SR_METRICS_END"} {
			if !strings.Contains(script, sub) {
				t.Errorf("darwin script missing %q", sub)
			}
		}
	})

	t.Run("unknown os falls back to linux", func(t *testing.T) {
		// Anything other than "darwin" should take the linux path. This
		// matches the current behaviour (FreeBSD, illumos, etc. all use
		// the linux script) — if we ever need to dispatch on other
		// values, this test will flag it.
		if got := unixMetricsScript("freebsd"); !strings.Contains(got, "/proc/stat") {
			t.Errorf("unknown-OS branch should fall back to linux script, got:\n%s", got)
		}
	})
}

// TestCollectMetrics_ScriptErrorPropagates covers the wrap-on-shell-error path
// in collectMetricsUnix (returns "running metrics script: %w").
func TestCollectMetrics_ScriptErrorPropagates(t *testing.T) {
	t.Parallel()

	mock := &testutil.MockExecutor{
		RunErr: assertCalledError(),
	}
	h := newMockHost("test", config.HostConfig{OS: "linux", Addr: "user@host"}, mock)

	got := h.CollectMetrics()

	if got.Err == nil {
		t.Fatal("CollectMetrics err = nil, want non-nil")
	}
	if !strings.Contains(got.Err.Error(), "running metrics script") {
		t.Errorf("Err = %q, want it to wrap 'running metrics script'", got.Err)
	}
}

// TestWindowsMetricsScript_Body locks the Windows script body. Pairs with
// TestCollectMetrics_DispatchOS/windows_uses_powershell_with_-EncodedCommand —
// that one confirms dispatch, this one confirms the script content the
// dispatcher is wrapping.
func TestWindowsMetricsScript_Body(t *testing.T) {
	t.Parallel()

	script := windowsMetricsScript()
	for _, sub := range []string{
		"Get-CimInstance",
		"Win32_Processor",
		"Win32_OperatingSystem",
		"Win32_LogicalDisk",
		"GH_SR_METRICS_START",
		"GH_SR_METRICS_END",
	} {
		if !strings.Contains(script, sub) {
			t.Errorf("windows script missing %q", sub)
		}
	}
}

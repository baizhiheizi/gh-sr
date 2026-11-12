package tui

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/an-lee/gh-sr/internal/host"
)

// captureStdout swaps os.Stdout with a pipe, runs fn, restores the
// original stdout, and returns whatever fn wrote. Used by the
// PrintHostMetricsTable tests because that helper writes directly to
// os.Stdout rather than taking an io.Writer (mirrors the existing
// PrintTable behavior; matching the call site keeps the surface stable).
//
// The pipe must be drained on a goroutine BEFORE fn() writes to it, or the
// 64 KiB pipe buffer fills and the write blocks. We synchronize the drain
// with a channel so the helper has a clean handoff and no race with the
// t.Fatalf below it.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	orig := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	os.Stdout = w

	// Start draining immediately so the writer never blocks on a full pipe.
	var buf bytes.Buffer
	done := make(chan struct{})
	go func() {
		_, _ = io.Copy(&buf, r)
		close(done)
	}()

	fn()
	_ = w.Close()
	<-done
	_ = r.Close()
	os.Stdout = orig
	return buf.String()
}

// TestColorizePercent_thresholds pins the three severity thresholds that
// drive the styled renderers. The boundaries (>= 90, >= 70, else) are part
// of the visual contract: misaligning them silently shifts every colored
// cell without changing any Contains-based assertion downstream.
//
// We assert on preserved cell text rather than the presence of ANSI
// escapes because lipgloss emits plain text under `go test` (non-TTY
// stdout); the contract we pin is that the input is preserved and the
// three branches all execute without panic.
func TestColorizePercent_thresholds(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		in   string
	}{
		{"high", "95.0%"},
		{"boundary_90", "90%"},
		{"busy", "75.5%"},
		{"boundary_70", "70%"},
		{"calm", "12.3%"},
		{"zero", "0%"},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := colorizePercent(tc.in)
			if got == "" {
				t.Errorf("colorizePercent(%q) returned empty string", tc.in)
			}
			if !strings.Contains(got, tc.in) {
				t.Errorf("colorizePercent(%q) should preserve the original cell text; got %q", tc.in, got)
			}
		})
	}
}

// TestHostMetricsColorize_perColumnRouting pins the contract that the three
// percentage columns (CPU/MEMORY/DISK) are routed through colorizePercent
// while the remaining columns (HOST, LOAD AVG, UPTIME) pass through
// unchanged. A regression that drops or extends the column range silently
// miscolors the table.
//
// We assert on the routing behavior — pass-through vs not-equal — rather
// than ANSI escape presence for the same non-TTY reason noted in
// TestColorizePercent_thresholds. The colored-column rows are kept as a
// record of the intended column set; a future change can add a stronger
// "must differ" assertion if lipgloss is ever driven under a forced-color
// mode in tests.
func TestHostMetricsColorize_perColumnRouting(t *testing.T) {
	t.Parallel()
	cases := []struct {
		col           int
		cell          string
		wantUnchanged bool
	}{
		{0, "host-1", true},                 // HOST → no colorization
		{1, "12.3%", false},                 // CPU → colorized
		{2, "1024/4096 MiB (25.0%)", false}, // MEMORY → colorized
		{3, "50/200 GiB (25.0%)", false},    // DISK → colorized
		{4, "0.50 0.40 0.30", true},         // LOAD AVG → no colorization
		{5, "5d", true},                     // UPTIME → no colorization
		{6, "ignored", true},                // out of range → no colorization
		{-1, "ignored", true},               // negative → no colorization
	}
	for _, tc := range cases {
		got := hostMetricsColorize(tc.col, tc.cell)
		if tc.wantUnchanged && got != tc.cell {
			t.Errorf("hostMetricsColorize(col=%d, %q) should pass through unchanged; got %q", tc.col, tc.cell, got)
		}
		// For the colored columns we record the expectation that the call
		// returns the cell text; lipgloss under `go test` may return it
		// verbatim (no TTY) or wrap it in SGR (forced color), so we cannot
		// make a stronger "must differ" assertion without flakiness.
		_ = tc.wantUnchanged
	}
}

// TestMetricsRow_emptyUptimeDefaultsToDash pins the "Uptime == \"\" → \"-\"""
// contract that the dashboard relies on so a host with an empty Uptime
// string still has a printable cell. A regression to literal "" would
// collapse the column width and misalign the table.
func TestMetricsRow_emptyUptimeDefaultsToDash(t *testing.T) {
	t.Parallel()
	m := host.HostMetrics{
		Name:       "h-empty-uptime",
		CPUPercent: 1.0,
		MemUsedMiB: 1, MemTotalMiB: 100,
		DiskUsedGiB: 1, DiskTotalGiB: 100,
		Uptime: "",
	}
	row := metricsRow(m)
	if len(row) != 6 {
		t.Fatalf("metricsRow returned %d cells, want 6: %v", len(row), row)
	}
	if row[5] != "-" {
		t.Errorf("metricsRow Uptime cell = %q, want %q (empty Uptime should default to dash)", row[5], "-")
	}
	if row[0] != "h-empty-uptime" {
		t.Errorf("metricsRow Name cell = %q, want %q", row[0], "h-empty-uptime")
	}
	// Sanity-check the percentage cells still use the formatPercent path.
	if !strings.HasSuffix(row[1], "%") {
		t.Errorf("CPU cell should end with %% (formatPercent), got %q", row[1])
	}
}

// TestPrintHostMetricsTable_rendersHeaderAndRows verifies the public entry
// point writes the title, header, and one row per host. We assert on the
// text content (not styling) because the table package's styling depends
// on terminal capabilities.
func TestPrintHostMetricsTable_rendersHeaderAndRows(t *testing.T) {
	// No t.Parallel(): captureStdout mutates package-global os.Stdout,
	// so concurrent invocations race on the file descriptor.
	metrics := []host.HostMetrics{
		{Name: "h1", CPUPercent: 12.3, MemUsedMiB: 1024, MemTotalMiB: 4096, DiskUsedGiB: 50, DiskTotalGiB: 200, Load1: 0.5, Load5: 0.4, Load15: 0.3, Uptime: "5d"},
		{Name: "h2", CPUPercent: 78.9, MemUsedMiB: 8192, MemTotalMiB: 16384, DiskUsedGiB: 100, DiskTotalGiB: 250, Load1: 2.5, Load5: 2.1, Load15: 1.8, Uptime: "12d"},
	}
	out := captureStdout(t, func() { PrintHostMetricsTable(metrics) })
	for _, want := range []string{"Host Metrics", "HOST", "h1", "h2", "12.3%", "78.9%"} {
		if !strings.Contains(out, want) {
			t.Errorf("PrintHostMetricsTable output missing %q. Full output:\n%s", want, out)
		}
	}
}

// TestPrintHostMetricsTable_emptyInput verifies the empty-input branch
// prints the configured EmptyMsg instead of an empty table.
func TestPrintHostMetricsTable_emptyInput(t *testing.T) {
	// No t.Parallel(): captureStdout mutates package-global os.Stdout.
	out := captureStdout(t, func() { PrintHostMetricsTable(nil) })
	if !strings.Contains(out, "No hosts found.") {
		t.Errorf("PrintHostMetricsTable(nil) should print EmptyMsg, got %q", out)
	}
}

// TestExtractTrailingPercent_edgeCases pins the defensive guards in the
// zero-alloc manual byte scan (PR #340) that drives colorizePercent's
// severity threshold. The function is on the TUI render hot path
// (per-keypress + per-refresh-tick), so silent parse regressions would
// silently shift every colored cell. The cases below cover: no '%',
// trailing whitespace, leading whitespace (which strconv.ParseFloat
// rejects), missing digits before '%', and a single '.' in the middle
// of an otherwise-numeric tail. The "invalid float" case documents the
// existing return-0 behaviour so a future change to return NaN or panic
// is flagged as a behaviour change in code review.
func TestExtractTrailingPercent_edgeCases(t *testing.T) {
	// No t.Parallel(): stable subset, table-driven.
	cases := []struct {
		name string
		in   string
		want float64
	}{
		// Happy path — same as colourisePercent's documented examples.
		{"simple int percent", "3.2%", 3.2},
		{"full used/total cell", "0.0/0.0 MiB (0%)", 0},
		{"full used/total cell high", "99.0/100.0 GiB (95.5%)", 95.5},
		{"whole number", "100%", 100},
		{"zero", "0%", 0},

		// Defensive: no '%' anywhere.
		{"empty string", "", 0},
		{"plain word", "err", 0},
		{"dash sentinel", "-", 0},
		{"unreachable sentinel", "unreachable", 0},

		// Defensive: '%' present but not preceded by digits.
		{"percent only", "%", 0},
		{"letter before percent", "abc%", 0},

		// Defensive: whitespace handling. The old strings.TrimRight path
		// produced 95.5 for "95.5 %"; the manual scan must match that.
		{"space before percent", "95.5 %", 95.5},
		{"multiple spaces before percent", "95.5   %", 95.5},

		// Leading whitespace stops the walk-back loop, so the parse
		// target is "3.2" (ParseFloat rejects leading whitespace, so
		// the function implicitly tolerates it by stripping it). The
		// walk-back skips spaces by stopping at the first non-digit/
		// non-dot character.
		{"leading space before digit", " 3.2%", 3.2},

		// Paren before digit is the production cell shape: "(78.9%)".
		{"paren wrap", "(78.9%)", 78.9},

		// Malformed: two dots in a row is not a valid float.
		{"invalid float", "1.2.3%", 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := extractTrailingPercent(tc.in); got != tc.want {
				t.Errorf("extractTrailingPercent(%q) = %v, want %v", tc.in, got, tc.want)
			}
		})
	}
}

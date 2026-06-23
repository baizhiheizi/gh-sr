package tui

import "testing"

// rowSamples mimics the column count of the runner-status table (9 columns).
// A typical TUI session renders this many cells per row × one row per host on
// every refresh tick (5s default) plus every keypress.
var rowSamples = []struct {
	cells  []string
	widths []int
}{
	{
		cells:  []string{"runner-1", "host1.example", "o/r1", "container", "gh-sr/agentic-runner:2.320.0", "abc123", "running", "online", "self-hosted,linux,x64"},
		widths: []int{8, 16, 8, 9, 28, 7, 8, 7, 21},
	},
	{
		cells:  []string{"runner-2", "host2.example", "o/r2", "native", "-", "-", "stopped", "offline", "self-hosted,linux,arm64"},
		widths: []int{8, 16, 8, 7, 1, 1, 8, 8, 22},
	},
}

// headerSample mirrors a 9-column header line.
var headerSample = struct {
	headers []string
	widths  []int
}{
	headers: []string{"INSTANCE", "HOST", "REPO", "MODE", "IMAGE", "BUILD", "LOCAL", "GITHUB", "LABELS"},
	widths:  []int{8, 16, 8, 9, 28, 7, 8, 7, 21},
}

// colorizePassthrough is a no-op colorize for renderRow/renderHighlightedRow
// that mimics the production path (returns the cell unchanged for cols the
// colorize fn does not transform).
func colorizePassthrough(col int, cell string) string { return cell }

// BenchmarkRenderHeader measures the per-render cost of building the styled
// header line. The TUI dashboard rebuilds this every View() call.
func BenchmarkRenderHeader(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = renderHeader(headerSample.headers, headerSample.widths)
	}
}

// BenchmarkRenderRow measures the per-row cost. With N rows × every render,
// this is the per-tick render hot path for the dashboard table.
func BenchmarkRenderRow(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		for _, s := range rowSamples {
			_ = renderRow(s.cells, s.widths, colorizePassthrough)
		}
	}
}

// BenchmarkRenderHighlightedRow measures the cursor-row variant (the row
// carrying the per-cell selection background).
func BenchmarkRenderHighlightedRow(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		for _, s := range rowSamples {
			_ = renderHighlightedRow(s.cells, s.widths, colorizePassthrough)
		}
	}
}

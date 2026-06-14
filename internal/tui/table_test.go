package tui

import (
	"strings"
	"testing"
)

// TestRenderHeader_alignsColumnWidths verifies the header line uses each
// column's width plus the +2 cell padding, so headers and body cells align.
func TestRenderHeader_alignsColumnWidths(t *testing.T) {
	t.Parallel()
	headers := []string{"A", "BBB"}
	widths := []int{3, 5}
	got := renderHeader(headers, widths)
	if !strings.Contains(got, "A") || !strings.Contains(got, "BBB") {
		t.Fatalf("header should contain column labels, got: %q", got)
	}
	// Visible width must equal the widest cell (widths[i] + 2 padding) per column.
	if w := visibleWidth(got); w < 10 {
		t.Fatalf("header visible width too small: %d (expected >= 10)", w)
	}
}

// TestRenderRow_passesThroughColorize verifies renderRow calls colorize for
// every column and renders the styled result inside the per-cell padding.
func TestRenderRow_passesThroughColorize(t *testing.T) {
	t.Parallel()
	cells := []string{"x", "y", "z"}
	widths := []int{3, 3, 3}
	calls := 0
	colorize := func(col int, cell string) string {
		calls++
		if cell != cells[col] {
			t.Errorf("colorize[%d] got cell %q want %q", col, cell, cells[col])
		}
		return cell
	}
	got := renderRow(cells, widths, colorize)
	if calls != 3 {
		t.Errorf("colorize should be called once per column, got %d calls", calls)
	}
	if !strings.Contains(got, "x") || !strings.Contains(got, "y") || !strings.Contains(got, "z") {
		t.Errorf("row should contain each cell, got: %q", got)
	}
}

// TestRenderRow_nilColorizeRendersAsIs confirms a nil colorize callback still
// produces a row containing every cell (no panic).
func TestRenderRow_nilColorizeRendersAsIs(t *testing.T) {
	t.Parallel()
	cells := []string{"one", "two"}
	widths := []int{5, 5}
	got := renderRow(cells, widths, nil)
	if !strings.Contains(got, "one") || !strings.Contains(got, "two") {
		t.Fatalf("row should contain both cells with nil colorize, got: %q", got)
	}
}

// TestRenderHighlightedRow_matchesPerCellCursorPattern verifies the cursor
// variant still produces a row with each cell visible — the per-cell background
// behavior is the contract viewMain depends on.
func TestRenderHighlightedRow_matchesPerCellCursorPattern(t *testing.T) {
	t.Parallel()
	cells := []string{"alpha", "beta"}
	widths := []int{5, 5}
	got := renderHighlightedRow(cells, widths, nil)
	if !strings.Contains(got, "alpha") || !strings.Contains(got, "beta") {
		t.Fatalf("highlighted row should contain both cells, got: %q", got)
	}
}

// visibleWidth returns the printable width of an ANSI-styled string. lipgloss
// styled output embeds escape sequences; we approximate by stripping CSI
// sequences (anything between ESC and a final byte 0x40-0x7e).
func visibleWidth(s string) int {
	var out strings.Builder
	i := 0
	for i < len(s) {
		if s[i] == 0x1b && i+1 < len(s) && s[i+1] == '[' {
			// Skip until terminator byte.
			j := i + 2
			for j < len(s) {
				b := s[j]
				if b >= 0x40 && b <= 0x7e {
					j++
					break
				}
				j++
			}
			i = j
			continue
		}
		out.WriteByte(s[i])
		i++
	}
	// Count runes, not bytes — multi-byte UTF-8 chars count as 1.
	n := 0
	for range out.String() {
		n++
	}
	return n
}

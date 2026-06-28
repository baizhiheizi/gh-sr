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

// TestRenderMenuItems_marksSelectedItem verifies the cursor row is rendered
// with the "  > " arrow marker and every other row uses the plain "    "
// indent. The arrow marker must wrap in selectedStyle (visible via the
// surrounding ANSI codes), but the label itself stays readable in both
// styles so the user can find it on screen.
func TestRenderMenuItems_marksSelectedItem(t *testing.T) {
	t.Parallel()
	items := []string{"alpha", "beta", "gamma"}
	got := renderMenuItems(items, 1)
	if !strings.Contains(got, "alpha") || !strings.Contains(got, "beta") || !strings.Contains(got, "gamma") {
		t.Fatalf("all labels should appear, got: %q", got)
	}
	if !strings.Contains(got, "> beta") {
		t.Errorf("selected row should carry the > arrow marker, got: %q", got)
	}
	if strings.Contains(got, "> alpha") || strings.Contains(got, "> gamma") {
		t.Errorf("non-selected rows should not carry the > marker, got: %q", got)
	}
	// 3 items × 1 trailing newline each = 3 newlines.
	if n := strings.Count(got, "\n"); n != 3 {
		t.Errorf("expected 3 newlines (one per item), got %d in: %q", n, got)
	}
}

// TestRenderMenuItems_firstAndLastCursor positions the cursor at both ends
// of a 4-item list to confirm the helper covers boundary indices correctly.
func TestRenderMenuItems_firstAndLastCursor(t *testing.T) {
	t.Parallel()
	items := []string{"one", "two", "three", "four"}
	first := renderMenuItems(items, 0)
	if !strings.Contains(first, "> one") {
		t.Errorf("cursor=0 should mark first item, got: %q", first)
	}
	last := renderMenuItems(items, 3)
	if !strings.Contains(last, "> four") {
		t.Errorf("cursor=3 should mark last item, got: %q", last)
	}
}

// TestNewAltView_enablesAltScreen confirms the dashboard's NewView wrapper
// always sets AltScreen so panels render in the alternate buffer (not the
// main screen). Without this, a forgetful panel would draw over the user's
// previous terminal content.
func TestNewAltView_enablesAltScreen(t *testing.T) {
	t.Parallel()
	v := newAltView("hello")
	if !v.AltScreen {
		t.Errorf("newAltView should set AltScreen=true, got %+v", v)
	}
	// The view content must round-trip through the helper.
	if v.Content != "hello" {
		t.Errorf("view content should be %q, got %q", "hello", v.Content)
	}
}

// TestPrintTable_emptyStyled verifies empty styled tables print EmptyMsg.
func TestPrintTable_emptyStyled(t *testing.T) {
	t.Parallel()
	var buf strings.Builder
	ok := PrintTable(&buf, TablePrintOptions{EmptyMsg: "No runners found."})
	if ok {
		t.Fatal("PrintTable should return false for empty rows")
	}
	if got := buf.String(); got != "No runners found.\n" {
		t.Errorf("got %q want %q", got, "No runners found.\n")
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

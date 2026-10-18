package tui

import (
	"fmt"
	"io"
	"strings"

	"github.com/an-lee/gh-sr/internal/table"
	"github.com/charmbracelet/lipgloss"
)

// TablePrintOptions configures PrintTable.
type TablePrintOptions struct {
	Title    string
	EmptyMsg string
	Headers  []string
	Rows     [][]string
	Colorize func(col int, cell string) string
}

// PrintTable writes a styled lipgloss table to w. When Rows is empty,
// EmptyMsg is printed and false is returned.
func PrintTable(w io.Writer, opts TablePrintOptions) bool {
	if len(opts.Rows) == 0 {
		if opts.EmptyMsg != "" {
			fmt.Fprintln(w, opts.EmptyMsg)
		}
		return false
	}
	if opts.Title != "" {
		fmt.Fprintln(w, titleStyle.Render(opts.Title))
	}
	widths := table.ColumnWidths(opts.Headers, opts.Rows)
	fmt.Fprintln(w, renderHeader(opts.Headers, widths))
	for _, row := range opts.Rows {
		fmt.Fprintln(w, renderRow(row, widths, opts.Colorize, false))
	}
	return true
}

// runnerStatusHeaders / runnerStatusCells / runnerStatusColorize (the
// CLI-table accessors for the runner-status table) now live in columns.go,
// derived from the single runnerStatusColumns descriptor so the CLI table
// and the responsive dashboard view cannot drift apart.

// renderHeader builds the styled header line. Each header cell is padded to
// widths[i]+2 (matching the per-cell padding in renderRow) so header and body
// columns align visually.
func renderHeader(headers []string, widths []int) string {
	var b strings.Builder
	for i, h := range headers {
		b.WriteString(headerStyle.Width(widths[i] + 2).Render(h))
	}
	return b.String()
}

// renderRow builds one styled row line. colorize(col, cell) may return the
// cell unchanged or a styled string; if nil, cells are rendered as-is.
// highlight applies the cursor-row background to every cell — the per-cell
// background is what produces the visually-distinct "selected row" block (a
// single wrapper around the row would not survive per-cell padding), so it is
// a property of the row renderer rather than a caller-side style. Padding
// matches renderHeader (widths[j]+2).
//
// The path that matters here is the in-memory copy of cells + their styled
// forms: lipgloss.Style.Render builds a string per cell (with internal
// bytes.Buffer growth) and the per-row builder needs one final allocation
// for b.String(). Hot render loops such as viewMain avoid both copy-costs
// by calling renderRowInto directly into the parent's builder.
func renderRow(cells []string, widths []int, colorize func(col int, cell string) string, highlight bool) string {
	var b strings.Builder
	renderRowInto(&b, cells, widths, colorize, highlight)
	return b.String()
}

// renderRowInto appends the styled cells of one row to b. Same per-cell
// semantics as renderRow but writes to a caller-supplied builder so the row
// can be embedded in a larger output (e.g. viewMain) without paying for a
// trailing strings.Builder grow + b.String() copy just to be concatenated
// with "\n" by the caller. The single implementation (with the highlight
// flag) replaces the former renderRow/renderHighlightedRow pair that
// differed only by one chained .Background call and could drift.
func renderRowInto(b *strings.Builder, cells []string, widths []int, colorize func(col int, cell string) string, highlight bool) {
	base := cellStyle
	if highlight {
		base = cellStyle.Background(lipgloss.Color("8"))
	}
	for j, cell := range cells {
		styled := cell
		if colorize != nil {
			styled = colorize(j, cell)
		}
		b.WriteString(base.Width(widths[j] + 2).Render(styled))
	}
}

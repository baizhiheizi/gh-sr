// Package table provides shared column-width and plain-text table printing
// used by internal/tui (styled CLI tables) and internal/ops (disk usage output).
package table

import (
	"fmt"
	"io"
)

// Options configures PrintPlain.
type Options struct {
	EmptyMsg string
	Headers  []string
	Rows     [][]string
}

// ColumnWidths returns the maximum rendered width per column (header vs cells).
func ColumnWidths(headers []string, rows [][]string) []int {
	widths := make([]int, len(headers))
	for i, h := range headers {
		widths[i] = len(h)
	}
	for _, row := range rows {
		for j, cell := range row {
			if j < len(widths) && len(cell) > widths[j] {
				widths[j] = len(cell)
			}
		}
	}
	return widths
}

// PrintPlain writes a plain-text table to w. When Rows is empty, EmptyMsg is
// printed and false is returned.
func PrintPlain(w io.Writer, opts Options) bool {
	if len(opts.Rows) == 0 {
		if opts.EmptyMsg != "" {
			fmt.Fprintln(w, opts.EmptyMsg)
		}
		return false
	}
	widths := ColumnWidths(opts.Headers, opts.Rows)
	printRow(w, opts.Headers, widths)
	for _, row := range opts.Rows {
		printRow(w, row, widths)
	}
	return true
}

func printRow(w io.Writer, cells []string, widths []int) {
	for i, cell := range cells {
		if i >= len(widths) {
			break
		}
		fmt.Fprintf(w, "%-*s  ", widths[i], cell)
	}
	fmt.Fprintln(w)
}

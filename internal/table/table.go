// Package table provides shared column-width and plain-text table printing
// used by internal/tui (styled CLI tables) and internal/ops (disk usage output).
package table

import (
	"fmt"
	"io"
	"strings"
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

// RenderPlain returns the plain-text table as a string. When Rows is empty,
// EmptyMsg is returned as the sole line (without a trailing newline — callers
// append their own terminator as needed).
//
// Unlike PrintPlain, RenderPlain uses a strings.Builder for the per-cell
// padding loop instead of fmt.Fprintf so the per-cell alloc count stays at
// one. This matters on the TUI host-metrics render path, which calls into
// this helper once per refresh tick per View() and whose alloc count
// compounds across long sessions.
func RenderPlain(opts Options) string {
	if len(opts.Rows) == 0 {
		return opts.EmptyMsg
	}
	widths := ColumnWidths(opts.Headers, opts.Rows)

	var b strings.Builder
	// Header row + N data rows, with per-cell padding budget.
	b.Grow((len(opts.Headers) + len(opts.Rows)) * 32)
	appendRowPlain(&b, opts.Headers, widths)
	b.WriteByte('\n')
	for _, row := range opts.Rows {
		appendRowPlain(&b, row, widths)
		b.WriteByte('\n')
	}
	return b.String()
}

// appendRowPlain writes one padded row (without a trailing newline) to b.
// Inlining the right-pad as builder writes is a 1-alloc drop per cell vs
// fmt.Fprintf's format-string parser + reflection.
func appendRowPlain(b *strings.Builder, cells []string, widths []int) {
	for i, cell := range cells {
		if i >= len(widths) {
			break
		}
		b.WriteString(cell)
		if len(cell) < widths[i] {
			b.WriteString(strings.Repeat(" ", widths[i]-len(cell)))
		}
		b.WriteString("  ")
	}
}

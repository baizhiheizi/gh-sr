package tui

import (
	"strings"
	"testing"
)

// BenchmarkWrapLines measures the per-call cost of wrapping multi-line text
// to a fixed column width. wrapLines is invoked by the TUI when rendering
// rows whose cell content is longer than the column width; the most common
// shape is a 2-3 line error message inside a 40-60 character column. The
// inner strings.Split previously allocated the full input-line slice up
// front; strings.SplitSeq yields the same substrings without that
// allocation.
func BenchmarkWrapLines(b *testing.B) {
	specs := []struct {
		name  string
		input string
		width int
	}{
		{
			name:  "single_short",
			input: "runner up",
			width: 40,
		},
		{
			name:  "multi_line_short",
			input: "line one\nline two\nline three",
			width: 40,
		},
		{
			name:  "long_single_line",
			input: strings.Repeat("x", 200),
			width: 40,
		},
		{
			name:  "many_lines",
			input: strings.Repeat("runner started on host h-1 via gh sr up\n", 6),
			width: 60,
		},
	}
	for _, s := range specs {
		b.Run(s.name, func(b *testing.B) {
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_ = wrapLines(s.input, s.width)
			}
		})
	}
}

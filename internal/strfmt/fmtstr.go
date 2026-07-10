// Package strfmt provides allocation-free formatting helpers shared across
// the gh-sr modules. The helpers here are deliberately thin wrappers around
// strconv that own the buffer-sizing assumption (the documented worst-case
// AppendFloat output is 24 bytes for the `'f'` format with prec decimals
// on a float64) so individual call sites stop carrying their own rationale
// comment.
//
// Out of scope: scripts/benchstat is a standalone tool (//go:build ignore on
// its main.go, stdlib only, no module deps) and cannot import internal/
// packages. That file continues to call strconv.AppendFloat directly.
package strfmt

import "strconv"

// FmtFloat appends the 'f'-format representation of v with prec decimals to
// dst and returns the resulting slice. The helper itself never allocates;
// the caller is responsible for sizing dst. 24 bytes is the documented upper
// bound for strconv.AppendFloat(_, _, 'f', prec, 64).
//
// Used by:
//   - internal/tui/metrics.go (formatPercent, formatUsedTotal)
//   - internal/runner/disk.go  (FormatBytesHuman switch arms)
func FmtFloat(dst []byte, v float64, prec int) []byte {
	return strconv.AppendFloat(dst, v, 'f', prec, 64)
}

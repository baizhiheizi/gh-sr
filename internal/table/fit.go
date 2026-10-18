package table

import "slices"

// FitColumns shrinks natural column widths until a table rendered with a
// per-column padding of pad cells fits within maxTotal cells (the
// width+2 budget renderHeader/renderRowInto emit per cell).
//
// Reduction is applied to the widest column still above its floor, one
// column at a time: the widest (typically the least dense text) column
// absorbs the whole remaining deficit before any other column shrinks, so
// narrow status-like columns keep their natural width as long as possible.
// Callers that prefer dropping a column instead of truncating it hard can
// react to the returned total still exceeding maxTotal (see
// tui.planColumns).
//
// natural and mins must have equal length; mins[i] is the floor below
// which column i is never shrunk (pass natural[i] to pin a column). A
// floor above the natural width is clamped down. When even the floors do
// not fit maxTotal, the floors are returned — the caller decides whether
// to drop columns or accept the overflow.
func FitColumns(natural, mins []int, maxTotal, pad int) []int {
	out := slices.Clone(natural)
	if len(mins) != len(natural) {
		// Malformed input cannot be fitted; leave the caller's widths
		// alone rather than guessing floors.
		return out
	}
	total := 0
	for _, w := range out {
		total += w + pad
	}
	if total <= maxTotal {
		return out
	}
	floors := make([]int, len(mins))
	for i, m := range mins {
		floors[i] = min(m, natural[i])
	}
	for total > maxTotal {
		// Find the widest column that can still shrink. Ties resolve to
		// the lowest index, keeping the layout deterministic.
		best := -1
		for i, w := range out {
			if w > floors[i] && (best < 0 || w > out[best]) {
				best = i
			}
		}
		if best < 0 {
			break // every column is at its floor; overflow is unavoidable
		}
		// Take as much as possible from this column in one step: either
		// just enough to fit, or down to its floor.
		target := out[best] - (total - maxTotal)
		if target < floors[best] {
			target = floors[best]
		}
		total -= out[best] - target
		out[best] = target
	}
	return out
}

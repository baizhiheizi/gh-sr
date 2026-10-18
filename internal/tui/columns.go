package tui

import (
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/charmbracelet/lipgloss"

	"github.com/an-lee/gh-sr/internal/runner"
	"github.com/an-lee/gh-sr/internal/table"
)

// This file owns the dashboard's responsive-layout policy: which columns a
// table keeps at a given terminal width, how far each may shrink, and the
// small text helpers (ellipsis truncation, hint packing, prose wrapping)
// the panels use to keep every line inside the terminal.

// cellPad mirrors the per-cell horizontal padding renderHeader and
// renderRowInto add around every cell (widths[i]+2). The layout math here
// must budget the same padding the renderers emit, or a "fitted" table
// would still overflow the terminal by 2 cells per column.
const cellPad = 2

// minTruncWidth is the default truncation floor for flex columns: wide
// enough for "xx…" plus padding, narrow enough that several truncated
// columns still fit a 40-column terminal.
const minTruncWidth = 4

// columnSpec is the responsive-layout policy for one table column.
type columnSpec struct {
	// flex marks the column as truncatable (with an ellipsis) when the
	// table is wider than the terminal. Non-flex columns keep their
	// natural width until a column must be dropped.
	flex bool
	// min is the truncation floor, consulted only when flex is set. Zero
	// means minTruncWidth. Floors above the column's natural width are
	// clamped down by planColumns.
	min int
	// drop is the narrow-terminal drop priority: 0 = never dropped in the
	// normal pass, higher values are dropped first.
	drop int
	// dropLast is the emergency drop priority, consulted only once every
	// column carrying a nonzero drop has already been removed and the
	// floored layout still overflows. Without this second tier a table
	// whose remaining columns are all pinned has nothing left to shed and
	// renders wider than the terminal — hard-wrapping into an unreadable
	// mess, which is the exact failure this layout policy exists to
	// prevent (host metrics overflowed a 60-column terminal at 64 columns
	// wide; the runner table overflowed anything under ~52 columns).
	// Dropping is safe where truncating is not: a removed column takes
	// its status color with it intact, while a truncated status cell
	// loses the color because the colorizers match exact cell text.
	dropLast int
}

// statusColumn binds the responsive policy to the runner-status table's
// data path: how a cell is extracted from a RunnerStatus and how it is
// colored. The header/cell/color triple replaces the three parallel
// slices (runnerStatusHeaders / runnerStatusCells / runnerStatusColorize)
// that previously had to be kept in sync by hand — those CLI accessors
// are now derived from this table, so a column edit lands in one place.
type statusColumn struct {
	columnSpec
	header string
	cell   func(runner.RunnerStatus) string
	color  func(string) string
}

// runnerStatusColumns is the canonical column ordering for the
// runner-status table shared by PrintStatusTable (CLI) and the dashboard.
//
// Responsive policy: columns first truncate (with an ellipsis) down to
// their per-column floor, then drop in two tiers — LABELS first (widest,
// least operational), then IMAGE, then BUILD — and, only once those tiers
// are exhausted and the table still does not fit, the dropLast tier sheds
// REPO, MODE, HOST and INSTANCE, leaving LOCAL and GITHUB (the two state
// signals) standing last. Floors sit at the "still readable" threshold
// rather than the bare minimum, so an 80-column terminal keeps eight
// readable columns instead of nine crushed ones. MODE/LOCAL/GITHUB are
// never truncated (pinned) because truncating a status cell would also
// strip its color — the colorizers match on exact cell text — but they can
// still be dropped, which keeps the color intact.
var runnerStatusColumns = []statusColumn{
	{header: "INSTANCE", cell: func(s runner.RunnerStatus) string { return s.Instance }, columnSpec: columnSpec{flex: true, min: 6, dropLast: 2}},
	{header: "HOST", cell: func(s runner.RunnerStatus) string { return s.Host }, columnSpec: columnSpec{flex: true, min: 6, dropLast: 3}},
	{header: "REPO", cell: func(s runner.RunnerStatus) string { return s.Repo }, columnSpec: columnSpec{flex: true, min: 6, dropLast: 5}},
	{header: "MODE", cell: func(s runner.RunnerStatus) string { return s.Mode }, columnSpec: columnSpec{dropLast: 4}},
	{header: "IMAGE", cell: imageCell, columnSpec: columnSpec{flex: true, min: 10, drop: 2}},
	{header: "BUILD", cell: buildCell, color: colorizeImageBuild, columnSpec: columnSpec{drop: 1}},
	{header: "LOCAL", cell: func(s runner.RunnerStatus) string { return s.Local }, color: colorizeLocalStatus},
	{header: "GITHUB", cell: func(s runner.RunnerStatus) string { return formatGitHubStatus(s) }, color: colorizeGitHubStatus, columnSpec: columnSpec{dropLast: 1}},
	{header: "LABELS", cell: func(s runner.RunnerStatus) string { return s.Labels }, columnSpec: columnSpec{flex: true, min: 8, drop: 3}},
}

func imageCell(s runner.RunnerStatus) string {
	if s.ContainerImage == "" {
		return "-"
	}
	return s.ContainerImage
}

func buildCell(s runner.RunnerStatus) string {
	if s.ContainerImageBuild == "" {
		return "-"
	}
	return s.ContainerImageBuild
}

// runnerStatusHeaders is the full header list for the CLI status table,
// derived from runnerStatusColumns.
var runnerStatusHeaders = headersOf(runnerStatusColumns)

// runnerStatusCells builds the nine CLI status cells, derived from
// runnerStatusColumns so the CLI table and the dashboard cannot drift.
func runnerStatusCells(s runner.RunnerStatus) []string {
	cells := make([]string, len(runnerStatusColumns))
	for i, c := range runnerStatusColumns {
		cells[i] = c.cell(s)
	}
	return cells
}

// runnerStatusColorize applies the per-column color on the CLI table
// path, derived from runnerStatusColumns.
func runnerStatusColorize(col int, cell string) string {
	if col < 0 || col >= len(runnerStatusColumns) {
		return cell
	}
	if c := runnerStatusColumns[col]; c.color != nil {
		return c.color(cell)
	}
	return cell
}

func headersOf(cols []statusColumn) []string {
	out := make([]string, len(cols))
	for i, c := range cols {
		out[i] = c.header
	}
	return out
}

// planColumns decides which columns to render and at what widths so the
// table (cells + per-column padding) fits avail. Columns are dropped by
// descending drop priority while the floored widths overflow; if that pass
// exhausts every droppable column the dropLast tier sheds the rest so the
// rendered row always fits (one column is always kept — a table with no
// columns conveys nothing). The surviving widths come from
// table.FitColumns (widest-flex-first truncation). kept indexes into the
// specs/natural slices and is parallel to widths. avail <= 0 keeps every
// column at natural width — the pre-WindowSizeMsg frame falls back to a
// fixed layout instead of truncating against an unknown terminal.
func planColumns(specs []columnSpec, natural []int, avail int) (kept, widths []int) {
	kept = make([]int, len(specs))
	for i := range specs {
		kept[i] = i
	}
	if avail <= 0 || len(specs) == 0 {
		return kept, slices.Clone(natural)
	}
	mins := make([]int, len(specs))
	for i, sp := range specs {
		switch {
		case !sp.flex:
			mins[i] = natural[i]
		case sp.min > 0:
			mins[i] = min(sp.min, natural[i])
		default:
			mins[i] = min(minTruncWidth, natural[i])
		}
	}
	widths = fitKept(natural, mins, kept, avail)
	for tableTotal(widths) > avail && len(kept) > 1 {
		drop := widestPriority(kept, specs, true)
		if drop < 0 {
			// Normal tiers are exhausted; fall back to the emergency
			// tier so a pinned-only remainder still fits.
			drop = widestPriority(kept, specs, false)
		}
		if drop < 0 {
			break
		}
		kept = append(kept[:drop], kept[drop+1:]...)
		widths = fitKept(natural, mins, kept, avail)
	}
	return kept, widths
}

// widestPriority returns the position within kept of the column with the
// highest priority for the requested tier (drop, or dropLast when normal is
// false), or -1 when no kept column has a nonzero priority in that tier.
// Ties resolve to the lowest index, keeping the layout deterministic.
func widestPriority(kept []int, specs []columnSpec, normal bool) int {
	priority := func(sp columnSpec) int {
		if normal {
			return sp.drop
		}
		return sp.dropLast
	}
	best := -1
	for k, i := range kept {
		p := priority(specs[i])
		if p <= 0 {
			continue
		}
		if best < 0 || p > priority(specs[kept[best]]) {
			best = k
		}
	}
	return best
}

// fitKept runs table.FitColumns over the kept subset of natural/mins.
func fitKept(natural, mins []int, kept []int, avail int) []int {
	nat := make([]int, len(kept))
	floor := make([]int, len(kept))
	for k, i := range kept {
		nat[k] = natural[i]
		floor[k] = mins[i]
	}
	return table.FitColumns(nat, floor, avail, cellPad)
}

func tableTotal(widths []int) int {
	t := 0
	for _, w := range widths {
		t += w + cellPad
	}
	return t
}

// truncateCell clips s to at most w columns, replacing the cut with a
// single ellipsis so a truncated cell is visibly truncated. Truncation
// walks runes (never splits a multi-byte character) but counts bytes,
// matching the byte-based column widths table.ColumnWidths produces: an
// ASCII cell truncates to exactly w columns, a non-ASCII cell to at most
// w bytes. w <= 1 collapses to the ellipsis alone.
func truncateCell(s string, w int) string {
	if w <= 0 {
		return ""
	}
	if len(s) <= w {
		return s
	}
	limit := w - 1 // reserve one column for the ellipsis
	end := 0
	for end < len(s) {
		_, size := utf8.DecodeRuneInString(s[end:])
		if end+size > limit {
			break
		}
		end += size
	}
	return s[:end] + "…"
}

// packSegments greedily packs hint segments ("j/k: move", …) into lines
// of at most width columns, joined by two spaces. A single segment wider
// than width stays on its own line (overflows are visible rather than
// silently hidden) — callers pass short segments.
func packSegments(segs []string, width int) []string {
	if len(segs) == 0 {
		return []string{""}
	}
	lines := make([]string, 0, 1)
	cur := ""
	for _, s := range segs {
		switch {
		case cur == "":
			cur = s
		case len(cur)+cellPad+len(s) <= width:
			cur += "  " + s
		default:
			lines = append(lines, cur)
			cur = s
		}
	}
	return append(lines, cur)
}

// wrapWordsToWidth wraps prose to width columns, breaking only on spaces
// and preserving blank lines. Unlike wrapLines (a hard character wrap for
// logs, where whitespace matters) this collapses runs of spaces — it is
// for help text and error messages, not log output. Widths below 8 fall
// back to 8 so a degenerate resize can never produce one-word-per-line
// output.
func wrapWordsToWidth(s string, width int) []string {
	if width < 8 {
		width = 8
	}
	var out []string
	for line := range strings.SplitSeq(s, "\n") {
		words := strings.Fields(line)
		if len(words) == 0 {
			out = append(out, "")
			continue
		}
		// A word longer than width (a repo slug, hostname, URL or file
		// path — all common in filter lines and error text) has no break
		// opportunity, so greedy packing would emit it verbatim and the
		// line would run past the terminal edge no matter how narrow the
		// terminal is. Split those up front.
		words = splitOverlongWords(words, width)
		cur := ""
		for _, w := range words {
			if cur == "" {
				cur = w
			} else if len(cur)+1+len(w) <= width {
				cur += " " + w
			} else {
				out = append(out, cur)
				cur = w
			}
		}
		out = append(out, cur)
	}
	return out
}

// splitOverlongWords breaks any word wider than width into rune-safe
// chunks of at most width bytes, preserving word order. Chunking is
// byte-based (like truncateCell and table.ColumnWidths) and byte length is
// always an upper bound on rendered columns for UTF-8, so a chunk can
// never overflow.
func splitOverlongWords(words []string, width int) []string {
	var out []string
	for _, w := range words {
		for len(w) > width {
			cut := width
			for cut > 0 && !utf8.RuneStart(w[cut]) {
				cut--
			}
			if cut == 0 {
				// width is smaller than the leading rune; emit the whole
				// rune so the loop always makes progress.
				_, size := utf8.DecodeRuneInString(w)
				cut = size
			}
			out = append(out, w[:cut])
			w = w[cut:]
		}
		out = append(out, w)
	}
	return out
}

// rstripLines removes trailing spaces from every line of s. lipgloss pads
// the lines of a multi-line styled block to a common width; the padding is
// invisible, but on a terminal exactly as wide as the block it wraps into
// a phantom blank line.
func rstripLines(s string) string {
	lines := strings.Split(s, "\n")
	for i, l := range lines {
		lines[i] = strings.TrimRight(l, " ")
	}
	return strings.Join(lines, "\n")
}

// renderStyledBlock renders lines through style as a single styled block
// (one MarginTop, consistent styling across lines — per-line renders
// would repeat the style's margins between every line) with the block
// padding stripped. Only used for text styles: never for styles carrying
// a background, where the trailing padding is part of the highlight bar.
func renderStyledBlock(lines []string, style lipgloss.Style) string {
	return rstripLines(style.Render(strings.Join(lines, "\n")))
}

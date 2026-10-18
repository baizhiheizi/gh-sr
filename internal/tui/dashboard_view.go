package tui

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/lipgloss"

	"github.com/an-lee/gh-sr/internal/table"
)

func (m *dashboardModel) View() tea.View {
	switch m.panel {
	case panelScroll:
		return m.viewScroll()
	case panelConfirmCleanup:
		return m.viewConfirmCleanup()
	case panelConfirmRemove:
		return m.viewConfirmRemove()
	case panelActionMenu:
		return m.viewActionMenu()
	case panelGlobalMenu:
		return m.viewGlobalMenu()
	case panelFilterMenu:
		return m.viewFilterMenu()
	case panelFilterHost:
		return m.viewFilterList(m.filterHostChoices, "Select host filter (esc cancel)")
	case panelFilterRepo:
		return m.viewFilterList(m.filterRepoChoices, "Select repo filter (esc cancel)")
	case panelHostMetrics:
		return m.viewHostMetrics()
	default:
		return m.viewMain()
	}
}

// newAltView wraps tea.NewView with the AltScreen flag the dashboard enables
// on every panel. Centralising it keeps every View() return path consistent
// (one of the 11 panels forgetting the flag used to render in the main screen
// buffer instead of the alternate buffer — easy to miss in a code review).
func newAltView(s string) tea.View {
	v := tea.NewView(s)
	v.AltScreen = true
	return v
}

// renderMenuItems builds the per-item list shared by every cursor-driven menu
// panel (action / global / filter / dynamic filter list). The selected item is
// prefixed with the "  > " marker through selectedStyle; the remaining items
// use the plain "    " indent so the column of arrow markers stays aligned.
// The trailing newline is part of the returned string so callers can append
// it directly to a strings.Builder.
//
// The string-returning shape is preserved for test use; menu renderers that
// already have a strings.Builder to write into should call renderMenuItemsInto
// to skip the final b.String() copy and the per-item "+ "\n"" concat allocs.
func renderMenuItems(items []string, cursor int) string {
	var b strings.Builder
	renderMenuItemsInto(&b, items, cursor)
	return b.String()
}

// renderMenuItemsInto appends the per-item lines to b. Same per-line
// semantics as renderMenuItems but writes to a caller-supplied builder so the
// lines can be embedded in a larger panel output without paying for an
// intermediate strings.Builder grow + b.String() copy just to be concatenated
// by the caller. Mirrors the renderRowInto / renderRow split used by the
// table render path.
func renderMenuItemsInto(b *strings.Builder, items []string, cursor int) {
	for i, label := range items {
		if i == cursor {
			b.WriteString(selectedStyle.Render("  > " + label))
		} else {
			b.WriteString("    ")
			b.WriteString(label)
		}
		b.WriteByte('\n')
	}
}

func (m *dashboardModel) viewMain() tea.View {
	var b strings.Builder
	avail := m.termWidth() - cellPad

	b.WriteString(m.renderTitleLine("gh sr dashboard", strconv.Itoa(len(m.statuses))+" runners"))
	b.WriteString("\n\n")

	filterParts := []string{}
	if m.tuiHostFilter != "" {
		filterParts = append(filterParts, "host="+m.tuiHostFilter)
	}
	if m.tuiRepoFilter != "" {
		filterParts = append(filterParts, "repo="+m.tuiRepoFilter)
	}
	if len(filterParts) == 0 {
		b.WriteString(helpStyle.Render("  Filters: (none)  — press f to change"))
		b.WriteByte('\n')
	} else {
		b.WriteString(renderIndentedWrapped("  Filters: "+strings.Join(filterParts, "  ")+"  — press f to change", avail, helpStyle))
	}

	if m.busy && m.busyOp != "" {
		b.WriteString(statusUnknown.Render("  … " + m.busyOp + " in progress"))
		b.WriteByte('\n')
	}
	if m.toast != "" {
		b.WriteString(statusOnline.Render("  " + truncateCell(m.toast, max(1, avail))))
		b.WriteByte('\n')
	}
	b.WriteString("\n")

	if m.loading && len(m.statuses) == 0 {
		b.WriteString("  Loading...\n")
		return newAltView(b.String())
	}

	if m.lastErr != "" {
		b.WriteString(renderIndentedWrapped("Error: "+m.lastErr, avail, statusStopped))
		b.WriteString("\n\n")
	}

	if len(m.statuses) == 0 {
		b.WriteString("  No runners in view (check filters or config).\n")
		b.WriteString(m.footerMain())
		if m.showHelp {
			b.WriteByte('\n')
			b.WriteString(m.helpOverlay())
		}
		return newAltView(b.String())
	}

	// Responsive table: measure natural widths, then let planColumns drop
	// / truncate columns so the row line fits the terminal. Cells are
	// truncated as plain text (before colorizing) so ANSI sequences are
	// never cut mid-escape.
	specs := make([]columnSpec, len(runnerStatusColumns))
	natural := make([]int, len(runnerStatusColumns))
	for i, c := range runnerStatusColumns {
		specs[i] = c.columnSpec
		natural[i] = len(c.header)
		for _, s := range m.statuses {
			if l := len(c.cell(s)); l > natural[i] {
				natural[i] = l
			}
		}
	}
	kept, widths := planColumns(specs, natural, avail)

	headers := make([]string, len(kept))
	for k, i := range kept {
		headers[k] = truncateCell(runnerStatusColumns[i].header, widths[k])
	}
	rows := make([][]string, len(m.statuses))
	for r, s := range m.statuses {
		row := make([]string, len(kept))
		for k, i := range kept {
			row[k] = truncateCell(runnerStatusColumns[i].cell(s), widths[k])
		}
		rows[r] = row
	}
	colorize := func(col int, cell string) string {
		c := runnerStatusColumns[kept[col]]
		if c.color == nil {
			return cell
		}
		return c.color(cell)
	}

	b.WriteString(renderHeader(headers, widths))
	b.WriteByte('\n')

	for i, cells := range rows {
		renderRowInto(&b, cells, widths, colorize, i == m.cursor)
		b.WriteByte('\n')
	}

	b.WriteString(m.footerMain())
	if m.showHelp {
		b.WriteByte('\n')
		b.WriteString(m.helpOverlay())
	}

	return newAltView(b.String())
}

// renderTitleLine renders a panel title with suffix right-aligned on the
// same line, padded with plain spaces to the terminal width. When the two
// cannot share a line (narrow terminal, long suffix) the suffix is
// dropped rather than wrapping over the title. The title is rendered
// through a margin-free copy of titleStyle — MarginBottom on the styled
// string would insert a newline and push the suffix onto its own line;
// the caller's following "\n\n" provides the spacing instead.
func (m *dashboardModel) renderTitleLine(title, suffix string) string {
	gap := m.termWidth() - len(title) - len(suffix) - 3
	if suffix == "" || gap < 1 {
		return titleStyle.MarginBottom(0).Render(title)
	}
	return titleStyle.MarginBottom(0).Render(title) + strings.Repeat(" ", gap) + helpStyle.Render(suffix)
}

// renderIndentedWrapped writes s word-wrapped to width with a two-space
// indent on every line, all through style as a single block. Used for
// prose lines (filters, errors) that would otherwise overflow narrow
// terminals.
func renderIndentedWrapped(s string, width int, style lipgloss.Style) string {
	width = max(20, width)
	var lines []string
	for _, line := range wrapWordsToWidth(s, width-cellPad) {
		lines = append(lines, "  "+line)
	}
	return renderStyledBlock(lines, style) + "\n"
}

// footerMainSegments are the main-panel key hints in display order.
// footerMain packs them into lines that fit the terminal width, so a
// narrow terminal gets wrapped hints instead of one overflowing line
// (the previous fixed footer was ~103 columns — already too wide for an
// 80-column terminal).
var footerMainSegments = []string{
	"j/k: move",
	"enter: runner actions",
	"g: global menu",
	"h: host metrics",
	"f: filters",
	"r: refresh",
	"?: help",
	"q: quit",
}

// footerRefreshing is appended as the last segment while a refresh is in
// flight. It must stay the only difference between the idle and loading
// footers so the shortcut hints themselves never move between states.
const footerRefreshing = "(refreshing…)"

// footerMain renders the bottom help line, wrapping segments onto as many
// lines as the terminal width requires. The leading newline (outside the
// helpStyle render, so lipgloss treats the hints as one styled block and
// applies MarginTop once) keeps the blank line between the table and the
// footer that the previous pre-rendered constants produced. The result is
// memoized per (width, loading) — see the dashboardModel field comment.
func (m *dashboardModel) footerMain() string {
	if m.footerText != "" && m.footerWidth == m.termWidth() && m.footerLoading == m.loading {
		return m.footerText
	}
	segs := footerMainSegments
	if m.loading {
		segs = append(slices.Clone(segs), footerRefreshing)
	}
	lines := packSegments(segs, m.termWidth()-cellPad)
	block := make([]string, len(lines))
	for i, line := range lines {
		block[i] = "  " + line
	}
	m.footerWidth = m.termWidth()
	m.footerLoading = m.loading
	m.footerText = "\n" + renderStyledBlock(block, helpStyle)
	return m.footerText
}

// helpOverlayText is the expanded help block; the leading two-space indent
// is added per line by helpOverlay so wrapping stays indent-aware.
const helpOverlayText = `Main: j/k navigate rows, enter opens actions for the selected instance.
Actions: setup, up, down, restart, update, logs (esc back).
Host metrics (h): CPU, memory, disk, load average, uptime per host.
Global (g): doctor, host metrics, cleanup, show/validate config, edit yaml/env, filters.
Filters (f): narrow by host or repo; clear restores full list.
Scroll views (logs, doctor, config): j/k line, ctrl+u/ctrl+d page, home/end, esc back.
Cleanup asks for y/n confirmation. Run gh sr init from a shell (not in the TUI).`

// helpOverlay renders the expanded help wrapped to the terminal width.
func (m *dashboardModel) helpOverlay() string {
	lines := []string{"  — Help —"}
	for _, line := range wrapWordsToWidth(helpOverlayText, max(20, m.termWidth()-cellPad)-cellPad) {
		lines = append(lines, "  "+line)
	}
	return renderStyledBlock(lines, helpStyle) + "\n"
}

func (m *dashboardModel) viewActionMenu() tea.View {
	var b strings.Builder
	inst, _ := m.selectedInstance()
	b.WriteString(titleStyle.Render("Runner actions"))
	b.WriteString("\n\n")
	inst = truncateCell(inst, max(10, m.termWidth()-14))
	b.WriteString(fmt.Sprintf("  Instance: %s\n\n", configVal.Render(inst)))
	renderMenuItemsInto(&b, actionMenuLabels, m.menuCursor)
	b.WriteString(helpStyle.Render("\n  enter: run  esc: back"))
	b.WriteByte('\n')
	return newAltView(b.String())
}

func (m *dashboardModel) viewGlobalMenu() tea.View {
	var b strings.Builder
	b.WriteString(titleStyle.Render("Global menu"))
	b.WriteString("\n\n")
	renderMenuItemsInto(&b, globalMenuLabels, m.menuCursor)
	b.WriteString(helpStyle.Render("\n  enter: choose  esc: back"))
	b.WriteByte('\n')
	return newAltView(b.String())
}

func (m *dashboardModel) viewFilterMenu() tea.View {
	var b strings.Builder
	b.WriteString(titleStyle.Render("Filters"))
	b.WriteString("\n\n")
	renderMenuItemsInto(&b, filterMenuLabels, m.menuCursor)
	b.WriteString(helpStyle.Render("\n  enter: choose  esc: back"))
	b.WriteByte('\n')
	return newAltView(b.String())
}

func (m *dashboardModel) viewFilterList(choices []string, subtitle string) tea.View {
	var b strings.Builder
	b.WriteString(titleStyle.Render("Pick filter"))
	b.WriteString("\n\n")
	b.WriteString(helpStyle.Render("  " + subtitle))
	b.WriteString("\n\n")
	if len(choices) == 0 {
		b.WriteString("  (no values)\n")
	} else {
		renderMenuItemsInto(&b, choices, m.menuCursor)
	}
	b.WriteString(helpStyle.Render("\n  enter: apply  esc: back"))
	b.WriteByte('\n')
	return newAltView(b.String())
}

func (m *dashboardModel) viewConfirmCleanup() tea.View {
	s := titleStyle.Render("Confirm cleanup") + "\n\n" +
		"  Remove offline self-hosted runners via the GitHub API?\n\n" +
		helpStyle.Render("  y: confirm   n / esc: cancel") + "\n"
	return newAltView(s)
}

func (m *dashboardModel) viewConfirmRemove() tea.View {
	s := titleStyle.Render("Confirm remove") + "\n\n" +
		fmt.Sprintf("  Remove runner %s? This will deregister it from GitHub,\n  remove it from the host, and delete it from config.\n\n",
			configVal.Render(m.confirmRemoveInst)) +
		helpStyle.Render("  y: confirm   n / esc: cancel") + "\n"
	return newAltView(s)
}

func (m *dashboardModel) viewScroll() tea.View {
	h := m.height
	if h < 8 {
		h = 24
	}
	visible := max(3, h-6)
	end := min(m.scrollOff+visible, len(m.scrollLines))

	var b strings.Builder
	b.WriteString(titleStyle.Render(m.scrollTitle))
	b.WriteString("\n\n")
	if m.scrollOff > 0 || end < len(m.scrollLines) {
		b.WriteString(helpStyle.Render(fmt.Sprintf(
			"  lines %d–%d of %d  (j/k scroll, ctrl+u/ctrl+d page, esc back)\n\n",
			m.scrollOff+1, end, len(m.scrollLines),
		)))
	} else {
		b.WriteString(helpStyle.Render("  j/k scroll · ctrl+u/ctrl+d page · esc back\n\n"))
	}
	for i := m.scrollOff; i < end; i++ {
		b.WriteString("  ")
		b.WriteString(m.scrollLines[i])
		b.WriteByte('\n')
	}
	return newAltView(b.String())
}

func (m *dashboardModel) viewHostMetrics() tea.View {
	var b strings.Builder
	b.WriteString(m.renderTitleLine("Host Metrics", strconv.Itoa(len(m.hostMetrics))+" hosts"))
	b.WriteString("\n\n")

	if m.metricsLoading && len(m.hostMetrics) == 0 {
		b.WriteString("  Loading metrics...\n")
		return newAltView(b.String())
	}

	if len(m.hostMetrics) == 0 {
		b.WriteString("  No hosts found.\n")
	} else {
		rows := buildHostMetricsRows(m.hostMetrics)
		avail := m.termWidth() - cellPad

		// Same responsive layout as the runner table: HOST / LOAD AVG /
		// UPTIME truncate with an ellipsis (floors chosen so a truncated
		// cell still reads as a name/number), CPU/MEMORY/DISK keep their
		// natural width so the trailing percentage — and with it the
		// colorizePercent severity color — survives. UPTIME is dropped
		// entirely when even the floors do not fit.
		natural := table.ColumnWidths(hostMetricsHeaders, rows)
		kept, widths := planColumns(hostMetricsSpecs, natural, avail)

		headers := make([]string, len(kept))
		for k, i := range kept {
			headers[k] = truncateCell(hostMetricsHeaders[i], widths[k])
		}
		colorize := func(col int, cell string) string {
			if i := kept[col]; i >= 1 && i <= 3 {
				return colorizePercent(cell)
			}
			return cell
		}

		b.WriteString(renderHeader(headers, widths))
		b.WriteByte('\n')

		for _, row := range rows {
			cells := make([]string, len(kept))
			for k, i := range kept {
				cells[k] = truncateCell(row[i], widths[k])
			}
			renderRowInto(&b, cells, widths, colorize, false)
			b.WriteByte('\n')
		}
	}

	b.WriteString(helpStyle.Render("\n  r: refresh  esc: back  q: back"))
	return newAltView(b.String())
}

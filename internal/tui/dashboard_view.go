package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
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

func (m *dashboardModel) viewMain() tea.View {
	var b strings.Builder

	b.WriteString(titleStyle.Render("gh sr dashboard"))
	b.WriteString("\n\n")

	filterParts := []string{}
	if m.tuiHostFilter != "" {
		filterParts = append(filterParts, "host="+m.tuiHostFilter)
	}
	if m.tuiRepoFilter != "" {
		filterParts = append(filterParts, "repo="+m.tuiRepoFilter)
	}
	if len(filterParts) == 0 {
		b.WriteString(helpStyle.Render("  Filters: (none)  — press f to change") + "\n")
	} else {
		b.WriteString(helpStyle.Render("  Filters: "+strings.Join(filterParts, "  ")) + "\n")
	}

	if m.busy && m.busyOp != "" {
		b.WriteString(statusUnknown.Render("  … "+m.busyOp+" in progress") + "\n")
	}
	if m.toast != "" {
		b.WriteString(statusOnline.Render("  "+m.toast) + "\n")
	}
	b.WriteString("\n")

	if m.loading && len(m.statuses) == 0 {
		b.WriteString("  Loading...\n")
		v := tea.NewView(b.String())
		v.AltScreen = true
		return v
	}

	if m.lastErr != "" {
		b.WriteString(statusStopped.Render("  Error: "+m.lastErr) + "\n\n")
	}

	if len(m.statuses) == 0 {
		b.WriteString("  No runners in view (check filters or config).\n")
		b.WriteString(m.footerMain())
		if m.showHelp {
			b.WriteString("\n" + helpOverlay())
		}
		v := tea.NewView(b.String())
		v.AltScreen = true
		return v
	}

	headers := []string{"INSTANCE", "HOST", "REPO", "MODE", "IMAGE", "BUILD", "LOCAL", "GITHUB", "LABELS"}
	rows := make([][]string, len(m.statuses))
	for i, s := range m.statuses {
		rows[i] = runnerStatusCells(s)
	}
	widths := computeColumnWidths(headers, rows)

	colorize := func(col int, cell string) string {
		switch col {
		case 5:
			return colorizeImageBuild(cell)
		case 6:
			return colorizeLocalStatus(cell)
		case 7:
			return colorizeGitHubStatus(cell)
		default:
			return cell
		}
	}

	b.WriteString(renderHeader(headers, widths) + "\n")

	for i, cells := range rows {
		if i == m.cursor {
			b.WriteString(renderHighlightedRow(cells, widths, colorize) + "\n")
		} else {
			b.WriteString(renderRow(cells, widths, colorize) + "\n")
		}
	}

	b.WriteString(m.footerMain())
	if m.showHelp {
		b.WriteString("\n" + helpOverlay())
	}

	v := tea.NewView(b.String())
	v.AltScreen = true
	return v
}

func (m *dashboardModel) footerMain() string {
	loadingIndicator := ""
	if m.loading {
		loadingIndicator = "  (refreshing…)"
	}
	return helpStyle.Render(fmt.Sprintf(
		"\n  j/k: move  enter: runner actions  g: global menu  h: host metrics  f: filters  r: refresh  ?: help  q: quit%s",
		loadingIndicator,
	))
}

func helpOverlay() string {
	return helpStyle.Render(`  — Help —
  Main: j/k navigate rows, enter opens actions for the selected instance.
  Actions: setup, up, down, restart, update, logs (esc back).
  Host metrics (h): CPU, memory, disk, load average, uptime per host.
  Global (g): doctor, host metrics, cleanup, show/validate config, edit yaml/env, filters.
  Filters (f): narrow by host or repo; clear restores full list.
  Scroll views (logs, doctor, config): j/k line, ctrl+u/ctrl+d page, home/end, esc back.
  Cleanup asks for y/n confirmation. Run gh sr init from a shell (not in the TUI).`)
}

func (m *dashboardModel) viewActionMenu() tea.View {
	var b strings.Builder
	inst, _ := m.selectedInstance()
	b.WriteString(titleStyle.Render("Runner actions"))
	b.WriteString("\n\n")
	b.WriteString(fmt.Sprintf("  Instance: %s\n\n", configVal.Render(inst)))
	for i, label := range actionMenuLabels {
		line := "    " + label
		if i == m.menuCursor {
			line = selectedStyle.Render("  > " + label)
		}
		b.WriteString(line + "\n")
	}
	b.WriteString(helpStyle.Render("\n  enter: run  esc: back") + "\n")
	v := tea.NewView(b.String())
	v.AltScreen = true
	return v
}

func (m *dashboardModel) viewGlobalMenu() tea.View {
	var b strings.Builder
	b.WriteString(titleStyle.Render("Global menu"))
	b.WriteString("\n\n")
	for i, label := range globalMenuLabels {
		line := "    " + label
		if i == m.menuCursor {
			line = selectedStyle.Render("  > " + label)
		}
		b.WriteString(line + "\n")
	}
	b.WriteString(helpStyle.Render("\n  enter: choose  esc: back") + "\n")
	v := tea.NewView(b.String())
	v.AltScreen = true
	return v
}

func (m *dashboardModel) viewFilterMenu() tea.View {
	var b strings.Builder
	b.WriteString(titleStyle.Render("Filters"))
	b.WriteString("\n\n")
	for i, label := range filterMenuLabels {
		line := "    " + label
		if i == m.menuCursor {
			line = selectedStyle.Render("  > " + label)
		}
		b.WriteString(line + "\n")
	}
	b.WriteString(helpStyle.Render("\n  enter: choose  esc: back") + "\n")
	v := tea.NewView(b.String())
	v.AltScreen = true
	return v
}

func (m *dashboardModel) viewFilterList(choices []string, subtitle string) tea.View {
	var b strings.Builder
	b.WriteString(titleStyle.Render("Pick filter"))
	b.WriteString("\n\n")
	b.WriteString(helpStyle.Render("  "+subtitle) + "\n\n")
	if len(choices) == 0 {
		b.WriteString("  (no values)\n")
	} else {
		for i, c := range choices {
			line := "    " + c
			if i == m.menuCursor {
				line = selectedStyle.Render("  > " + c)
			}
			b.WriteString(line + "\n")
		}
	}
	b.WriteString(helpStyle.Render("\n  enter: apply  esc: back") + "\n")
	v := tea.NewView(b.String())
	v.AltScreen = true
	return v
}

func (m *dashboardModel) viewConfirmCleanup() tea.View {
	s := titleStyle.Render("Confirm cleanup") + "\n\n" +
		"  Remove offline self-hosted runners via the GitHub API?\n\n" +
		helpStyle.Render("  y: confirm   n / esc: cancel") + "\n"
	v := tea.NewView(s)
	v.AltScreen = true
	return v
}

func (m *dashboardModel) viewConfirmRemove() tea.View {
	s := titleStyle.Render("Confirm remove") + "\n\n" +
		fmt.Sprintf("  Remove runner %s? This will deregister it from GitHub,\n  remove it from the host, and delete it from config.\n\n",
			configVal.Render(m.confirmRemoveInst)) +
		helpStyle.Render("  y: confirm   n / esc: cancel") + "\n"
	v := tea.NewView(s)
	v.AltScreen = true
	return v
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
		b.WriteString("  " + m.scrollLines[i] + "\n")
	}
	v := tea.NewView(b.String())
	v.AltScreen = true
	return v
}

func (m *dashboardModel) viewHostMetrics() tea.View {
	var b strings.Builder
	b.WriteString(titleStyle.Render("Host Metrics"))
	b.WriteString("\n\n")

	if m.metricsLoading && len(m.hostMetrics) == 0 {
		b.WriteString("  Loading metrics...\n")
		v := tea.NewView(b.String())
		v.AltScreen = true
		return v
	}

	if len(m.hostMetrics) == 0 {
		b.WriteString("  No hosts found.\n")
	} else {
		headers := []string{"HOST", "CPU", "MEMORY", "DISK", "LOAD AVG", "UPTIME"}
		rows := make([][]string, len(m.hostMetrics))
		for i, met := range m.hostMetrics {
			rows[i] = metricsRow(met)
		}

		widths := computeColumnWidths(headers, rows)

		colorize := func(col int, cell string) string {
			if col >= 1 && col <= 3 {
				return colorizePercent(cell)
			}
			return cell
		}

		b.WriteString(renderHeader(headers, widths) + "\n")

		for _, row := range rows {
			b.WriteString(renderRow(row, widths, colorize) + "\n")
		}
	}

	b.WriteString(helpStyle.Render("\n  r: refresh  esc: back  q: back"))
	v := tea.NewView(b.String())
	v.AltScreen = true
	return v
}

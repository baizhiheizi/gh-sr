package tui

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/an-lee/gh-sr/internal/host"
)

// PrintHostMetricsTable prints a tabular summary of host resource usage to stdout.
func PrintHostMetricsTable(metrics []host.HostMetrics) {
	if len(metrics) == 0 {
		fmt.Println("No hosts found.")
		return
	}

	fmt.Println(titleStyle.Render("Host Metrics"))

	headers := []string{"HOST", "CPU", "MEMORY", "DISK", "LOAD AVG", "UPTIME"}
	rows := make([][]string, len(metrics))
	for i, m := range metrics {
		rows[i] = metricsRow(m)
	}

	widths := computeColumnWidths(headers, rows)

	colorize := func(col int, cell string) string {
		if col >= 1 && col <= 3 {
			return colorizePercent(cell)
		}
		return cell
	}

	fmt.Println(renderHeader(headers, widths))
	for _, row := range rows {
		fmt.Println(renderRow(row, widths, colorize))
	}
}

// FormatHostMetrics returns a styled multiline string suitable for the TUI scroll panel.
func FormatHostMetrics(metrics []host.HostMetrics) string {
	if len(metrics) == 0 {
		return "  No hosts found."
	}

	headers := []string{"HOST", "CPU", "MEMORY", "DISK", "LOAD AVG", "UPTIME"}
	rows := make([][]string, len(metrics))
	for i, m := range metrics {
		rows[i] = metricsRow(m)
	}

	widths := computeColumnWidths(headers, rows)

	var b strings.Builder
	// Header row + N data rows, with per-cell padding budget.
	b.Grow((len(headers) + len(rows)) * 32)
	for i, h := range headers {
		appendHostCell(&b, h, widths[i])
	}
	b.WriteByte('\n')

	for _, row := range rows {
		for j, cell := range row {
			appendHostCell(&b, cell, widths[j])
		}
		b.WriteByte('\n')
	}
	return b.String()
}

// appendHostCell appends `cell` padded on the right to `width` plus the 2-space
// separator that host-metrics tables use. Inlining this in place of
// fmt.Sprintf("%-*s  ", w, cell) is a 1-alloc drop per cell, which compounds
// across N hosts × M columns × R render ticks.
func appendHostCell(b *strings.Builder, cell string, width int) {
	b.WriteString(cell)
	if len(cell) < width {
		b.WriteString(strings.Repeat(" ", width-len(cell)))
	}
	b.WriteString("  ")
}

func metricsRow(m host.HostMetrics) []string {
	if m.Err != nil {
		return []string{m.Name, "err", "err", "err", "-", "unreachable"}
	}
	// strconv.FormatFloat + strings.Builder avoids the per-call
	// reflection/format-string machinery that fmt.Sprintf drags in. metricsRow
	// is on the TUI metrics render path (once per host per View()), so reducing
	// its alloc count compounds across long dashboard sessions.
	cpu := formatPercent(m.CPUPercent, 1)
	mem := formatUsedTotal(m.MemUsedMiB, m.MemTotalMiB, m.MemPercent(), "MiB")
	disk := formatUsedTotal(m.DiskUsedGiB, m.DiskTotalGiB, m.DiskPercent(), "GiB")
	load := m.LoadStr()
	uptime := m.Uptime
	if uptime == "" {
		uptime = "-"
	}
	return []string{m.Name, cpu, mem, disk, load, uptime}
}

// formatPercent formats v with `prec` decimals followed by '%'.
func formatPercent(v float64, prec int) string {
	var b strings.Builder
	b.Grow(8)
	b.WriteString(strconv.FormatFloat(v, 'f', prec, 64))
	b.WriteByte('%')
	return b.String()
}

// formatUsedTotal formats "used/total UNIT (pct%)".
func formatUsedTotal(used, total, pct float64, unit string) string {
	var b strings.Builder
	b.Grow(24)
	b.WriteString(strconv.FormatFloat(used, 'f', 0, 64))
	b.WriteByte('/')
	b.WriteString(strconv.FormatFloat(total, 'f', 0, 64))
	b.WriteByte(' ')
	b.WriteString(unit)
	b.WriteString(" (")
	b.WriteString(strconv.FormatFloat(pct, 'f', 0, 64))
	b.WriteString("%)")
	return b.String()
}

// colorizePercent highlights a cell that ends with a percentage based on severity.
func colorizePercent(cell string) string {
	pct := extractTrailingPercent(cell)
	switch {
	case pct >= 90:
		return statusStopped.Render(cell)
	case pct >= 70:
		return statusBusy.Render(cell)
	default:
		return statusOnline.Render(cell)
	}
}

func extractTrailingPercent(s string) float64 {
	s = strings.TrimRight(s, ")")
	idx := strings.LastIndex(s, "(")
	if idx >= 0 {
		s = s[idx+1:]
	}
	s = strings.TrimSpace(s)
	s = strings.TrimSuffix(s, "%")
	s = strings.TrimSpace(s)
	// strconv.ParseFloat is ~7x faster than fmt.Sscanf for a single float
	// (Sscanf goes through the format-string parser + reflection). On the
	// TUI metrics render path this is called once per colored cell per host
	// on every Bubble Tea View() call (per keypress and per refresh tick).
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0
	}
	return v
}

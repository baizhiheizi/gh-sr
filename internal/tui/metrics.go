package tui

import (
	"fmt"
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

	var headerLine string
	for i, h := range headers {
		headerLine += headerStyle.Width(widths[i] + 2).Render(h)
	}
	fmt.Println(headerLine)

	for _, row := range rows {
		var line string
		for j, cell := range row {
			styled := cell
			switch j {
			case 1:
				styled = colorizePercent(cell)
			case 2:
				styled = colorizePercent(cell)
			case 3:
				styled = colorizePercent(cell)
			}
			line += cellStyle.Width(widths[j] + 2).Render(styled)
		}
		fmt.Println(line)
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
	for i, h := range headers {
		b.WriteString(fmt.Sprintf("%-*s  ", widths[i], h))
	}
	b.WriteString("\n")

	for _, row := range rows {
		for j, cell := range row {
			b.WriteString(fmt.Sprintf("%-*s  ", widths[j], cell))
		}
		b.WriteString("\n")
	}
	return b.String()
}

func metricsRow(m host.HostMetrics) []string {
	if m.Err != nil {
		return []string{m.Name, "err", "err", "err", "-", "unreachable"}
	}
	cpu := fmt.Sprintf("%.1f%%", m.CPUPercent)
	mem := fmt.Sprintf("%.0f/%.0f MiB (%.0f%%)", m.MemUsedMiB, m.MemTotalMiB, m.MemPercent())
	disk := fmt.Sprintf("%.0f/%.0f GiB (%.0f%%)", m.DiskUsedGiB, m.DiskTotalGiB, m.DiskPercent())
	load := m.LoadStr()
	uptime := m.Uptime
	if uptime == "" {
		uptime = "-"
	}
	return []string{m.Name, cpu, mem, disk, load, uptime}
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
	var v float64
	if _, err := fmt.Sscanf(s, "%f", &v); err != nil {
		return 0
	}
	return v
}

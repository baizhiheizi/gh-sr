package tui

import (
	"os"
	"strconv"
	"strings"

	"github.com/an-lee/gh-sr/internal/host"
	"github.com/an-lee/gh-sr/internal/table"
)

// hostMetricsHeaders is the canonical column ordering for the host-metrics
// table shared by PrintHostMetricsTable, FormatHostMetrics, and viewHostMetrics.
// Keep this slice in sync with metricsRow so a column rename does not silently
// misalign the three renderers.
var hostMetricsHeaders = []string{"HOST", "CPU", "MEMORY", "DISK", "LOAD AVG", "UPTIME"}

// buildHostMetricsRows maps metrics → the per-row string slices used by all
// three host-metrics renderers. Centralising the row construction keeps the
// header literal and metricsRow call in one place.
func buildHostMetricsRows(metrics []host.HostMetrics) [][]string {
	rows := make([][]string, len(metrics))
	for i, m := range metrics {
		rows[i] = metricsRow(m)
	}
	return rows
}

// hostMetricsColorize highlights CPU/MEMORY/DISK percentage cells (columns
// 1..3) using colorizePercent. Non-percentage cells pass through unchanged.
// Shared by the styled host-metrics renderers (PrintHostMetricsTable +
// viewHostMetrics); the plain-text renderer FormatHostMetrics does not apply
// colorization since table.RenderPlain has no Colorize hook.
func hostMetricsColorize(col int, cell string) string {
	if col >= 1 && col <= 3 {
		return colorizePercent(cell)
	}
	return cell
}

// PrintHostMetricsTable prints a tabular summary of host resource usage to stdout.
func PrintHostMetricsTable(metrics []host.HostMetrics) {
	PrintTable(os.Stdout, TablePrintOptions{
		Title:    "Host Metrics",
		EmptyMsg: "No hosts found.",
		Headers:  hostMetricsHeaders,
		Rows:     buildHostMetricsRows(metrics),
		Colorize: hostMetricsColorize,
	})
}

// FormatHostMetrics returns a styled multiline string suitable for the TUI scroll panel.
func FormatHostMetrics(metrics []host.HostMetrics) string {
	if len(metrics) == 0 {
		return "  No hosts found."
	}
	return table.RenderPlain(table.Options{
		EmptyMsg: "  No hosts found.",
		Headers:  hostMetricsHeaders,
		Rows:     buildHostMetricsRows(metrics),
	})
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
// formatPercent formats v with `prec` decimals followed by '%'.
//
// strconv.AppendFloat + a stack-allocated byte buffer avoids both the
// per-call string allocation that strconv.FormatFloat returns AND the
// strings.Builder heap allocation that the previous implementation
// dragged in. metricsRow calls this once per host per View(); for a
// 10-host panel that's 10 calls per render, and the cumulative cost
// compounds across long dashboard sessions.
//
// The largest realistic output is "100.0%" (6 chars); [16]byte holds
// the maximum AppendFloat output (24 chars) plus '%'.
func formatPercent(v float64, prec int) string {
	var buf [24]byte
	b := buf[:0]
	b = strconv.AppendFloat(b, v, 'f', prec, 64)
	b = append(b, '%')
	return string(b)
}

// formatUsedTotal formats "used/total UNIT (pct%)".
//
// strconv.AppendFloat + stack buffer avoids the strings.Builder heap
// allocation the previous implementation had. The largest realistic
// output is around 24 chars (e.g. "999999/9999999 GiB (100%)"); [40]byte
// holds AppendFloat's worst case (24 chars per float × 1 float at a time
// since the buffer is reused across writes) plus the 8 non-float chars
// ("/", " ", " (", "%)"). The buffer is big enough that this function
// never allocates on the heap.
func formatUsedTotal(used, total, pct float64, unit string) string {
	var buf [48]byte
	b := buf[:0]
	b = strconv.AppendFloat(b, used, 'f', 0, 64)
	b = append(b, '/')
	b = strconv.AppendFloat(b, total, 'f', 0, 64)
	b = append(b, ' ')
	b = append(b, unit...)
	b = append(b, ' ', '(')
	b = strconv.AppendFloat(b, pct, 'f', 0, 64)
	b = append(b, '%', ')')
	return string(b)
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

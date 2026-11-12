package tui

import (
	"os"
	"strconv"
	"strings"

	"github.com/an-lee/gh-sr/internal/host"
)

// hostMetricsHeaders is the canonical column ordering for the host-metrics
// table shared by PrintHostMetricsTable and viewHostMetrics.
// Keep this slice in sync with metricsRow so a column rename does not silently
// misalign the two renderers.
var hostMetricsHeaders = []string{"HOST", "CPU", "MEMORY", "DISK", "LOAD AVG", "UPTIME"}

// hostMetricsSpecs is the responsive-layout policy for the host-metrics
// table (parallel to hostMetricsHeaders). HOST / LOAD AVG / UPTIME are
// flex with floors wide enough that a truncated cell still reads as a
// name or number; CPU / MEMORY / DISK keep their natural width because
// truncating "used/total UNIT (pct%)" would drop the trailing percentage
// and with it the colorizePercent severity color. UPTIME is the one
// droppable column — the least actionable metric when space runs out.
// dropLast is the emergency tier for terminals too narrow for the floored
// table: LOAD AVG goes first, then DISK, then MEMORY, then HOST, so CPU
// (the single number people scan for) and one row label always survive.
// Without it the table overflowed a 60-column terminal at 64 columns wide.
var hostMetricsSpecs = []columnSpec{
	{flex: true, min: 6, dropLast: 1},
	{},
	{dropLast: 2},
	{flex: false, dropLast: 3},
	{flex: true, min: 5, dropLast: 4},
	{flex: true, min: 4, drop: 1},
}

// buildHostMetricsRows maps metrics → the per-row string slices used by both
// host-metrics renderers. Centralising the row construction keeps the header
// literal and metricsRow call in one place.
func buildHostMetricsRows(metrics []host.HostMetrics) [][]string {
	rows := make([][]string, len(metrics))
	for i, m := range metrics {
		rows[i] = metricsRow(m)
	}
	return rows
}

// hostMetricsColorize highlights CPU/MEMORY/DISK percentage cells (columns
// 1..3) using colorizePercent. Non-percentage cells pass through unchanged.
// Shared by both host-metrics renderers (PrintHostMetricsTable +
// viewHostMetrics).
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

// appendFormatPercent appends "12.3%" (v with prec decimals + '%') to dst
// and returns the extended slice. strconv.AppendFloat does not allocate, and the
// trailing '%' is a single byte, so the function stays allocation-free for
// any dst the caller passes with spare capacity.
func appendFormatPercent(dst []byte, v float64, prec int) []byte {
	dst = strconv.AppendFloat(dst, v, 'f', prec, 64)
	return append(dst, '%')
}

// appendFormatUsedTotal appends "used/total UNIT (pct%)" (used/total/pct
// formatted with 0 decimals) to dst. The largest realistic output is around
// 24 chars ("999999/9999999 GiB (100%)"); a small stack scratch buffer
// covers this with room to spare.
func appendFormatUsedTotal(dst []byte, used, total, pct float64, unit string) []byte {
	dst = strconv.AppendFloat(dst, used, 'f', 0, 64)
	dst = append(dst, '/')
	dst = strconv.AppendFloat(dst, total, 'f', 0, 64)
	dst = append(dst, ' ')
	dst = append(dst, unit...)
	dst = append(dst, ' ', '(')
	dst = strconv.AppendFloat(dst, pct, 'f', 0, 64)
	return append(dst, '%', ')')
}

// metricsRow builds the per-host row that PrintHostMetricsTable and
// viewHostMetrics both render. The error branch produces a recognizable
// placeholder row so a single unreachable host does not blank the table.
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
//
// Thin wrapper over appendFormatPercent — the string-returning form exists
// only for callers that need a standalone string (metricsRow); the byte
// layout, buffer sizing, and allocation profile live in the append variant
// so the two cannot drift. metricsRow calls this once per host per View();
// for a 10-host panel that's 10 calls per render, and the cumulative cost
// compounds across long dashboard sessions.
//
// The largest realistic output is "100.0%" (6 chars); [24]byte holds
// the maximum AppendFloat output (24 chars) plus '%'.
func formatPercent(v float64, prec int) string {
	var buf [24]byte
	return string(appendFormatPercent(buf[:0], v, prec))
}

// formatUsedTotal formats "used/total UNIT (pct%)".
//
// Thin wrapper over appendFormatUsedTotal — same single-source-of-truth
// rationale as formatPercent. The largest realistic output is around 24
// chars (e.g. "999999/9999999 GiB (100%)"); [48]byte holds AppendFloat's
// worst case (24 chars per float × 1 float at a time since the buffer is
// reused across writes) plus the 8 non-float chars ("/", " ", " (", "%)").
// The buffer is big enough that this function never allocates on the heap.
func formatUsedTotal(used, total, pct float64, unit string) string {
	var buf [48]byte
	return string(appendFormatUsedTotal(buf[:0], used, total, pct, unit))
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
	// Manual scan to find the trailing percentage without allocating
	// intermediate trimmed strings. The cell formats colorizePercent sees:
	//
	//   "12.3/45.6 GiB (78.9%)"  → 78.9
	//   "0.0/0.0 MiB (0%)"       → 0
	//   "99.0/100.0 GiB (95.5%)" → 95.5
	//   "3.2%"                   → 3.2
	//   "100%"                   → 100
	//   "err" / "-" / "unreachable" → 0 (no '%' anywhere)
	//
	// The original implementation chained strings.TrimRight +
	// strings.LastIndex + two strings.TrimSpace + strings.TrimSuffix,
	// allocating up to 4 intermediate strings. This version finds the last
	// '%', walks back to skip trailing whitespace, and parses digits + a
	// single optional '.', allocating only the float64 return value (and
	// the strconv error's wrapping, which we drop via the `_` on the
	// pre-1.21 ParseFloat path that returned an error). On the TUI metrics
	// render path this is called once per colored cell per host on every
	// Bubble Tea View() call (per keypress and per refresh tick).
	i := strings.LastIndexByte(s, '%')
	if i < 0 {
		return 0
	}
	// Walk back over trailing whitespace (defensive — production cells
	// from formatUsedTotal don't have trailing whitespace, but
	// "95.5 %" with a space before % is plausible and we want to handle
	// it the same way the old TrimSpace + TrimSuffix did).
	j := i
	for j > 0 && s[j-1] == ' ' {
		j--
	}
	// Walk back over digits + at most one '.'. Stop at the first non-digit
	// so leading whitespace (or '(') is excluded from the ParseFloat slice —
	// strconv.ParseFloat rejects leading spaces.
	start := j
	for start > 0 {
		c := s[start-1]
		if c >= '0' && c <= '9' {
			start--
			continue
		}
		if c == '.' {
			start--
			continue
		}
		break
	}
	v, err := strconv.ParseFloat(s[start:j], 64)
	if err != nil {
		return 0
	}
	return v
}

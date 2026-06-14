package tui

import (
	"github.com/an-lee/gh-sr/internal/runner"
	"github.com/charmbracelet/lipgloss"
)

func computeColumnWidths(headers []string, rows [][]string) []int {
	widths := make([]int, len(headers))
	for i, h := range headers {
		widths[i] = len(h)
	}
	for _, row := range rows {
		for j, cell := range row {
			if len(cell) > widths[j] {
				widths[j] = len(cell)
			}
		}
	}
	return widths
}

func runnerStatusCells(s runner.RunnerStatus) []string {
	ghStatus := formatGitHubStatus(s)
	img := s.ContainerImage
	if img == "" {
		img = "-"
	}
	build := s.ContainerImageBuild
	if build == "" {
		build = "-"
	}
	return []string{s.Instance, s.Host, s.Repo, s.Mode, img, build, s.Local, ghStatus, s.Labels}
}

// renderHeader builds the styled header line. Each header cell is padded to
// widths[i]+2 (matching the per-cell padding in renderRow) so header and body
// columns align visually.
func renderHeader(headers []string, widths []int) string {
	var line string
	for i, h := range headers {
		line += headerStyle.Width(widths[i] + 2).Render(h)
	}
	return line
}

// renderRow builds one styled row line. colorize(col, cell) may return the
// cell unchanged or a styled string; if nil, cells are rendered as-is.
// Padding matches renderHeader (widths[j]+2).
func renderRow(cells []string, widths []int, colorize func(col int, cell string) string) string {
	var line string
	for j, cell := range cells {
		styled := cell
		if colorize != nil {
			styled = colorize(j, cell)
		}
		line += cellStyle.Width(widths[j] + 2).Render(styled)
	}
	return line
}

// renderHighlightedRow builds a styled row with the cursor-row background
// applied to every cell — the per-cell background is what produces the
// visually-distinct "selected row" block, so we keep it per-cell (not a single
// wrapper) to match the original viewMain behavior. colorize behaves as in
// renderRow.
func renderHighlightedRow(cells []string, widths []int, colorize func(col int, cell string) string) string {
	var line string
	for j, cell := range cells {
		styled := cell
		if colorize != nil {
			styled = colorize(j, cell)
		}
		line += cellStyle.
			Width(widths[j] + 2).
			Background(lipgloss.Color("8")).
			Render(styled)
	}
	return line
}

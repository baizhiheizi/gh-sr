package tui

import "github.com/an-lee/gh-sr/internal/runner"

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

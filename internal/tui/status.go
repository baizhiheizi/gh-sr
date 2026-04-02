package tui

import (
	"fmt"
	"strings"

	"github.com/an-lee/gh-runners/internal/config"
	"github.com/an-lee/gh-runners/internal/runner"
)

func PrintStatusTable(statuses []runner.RunnerStatus) {
	if len(statuses) == 0 {
		fmt.Println("No runners found.")
		return
	}

	fmt.Println(titleStyle.Render("Runner Status"))

	headers := []string{"INSTANCE", "HOST", "REPO", "MODE", "LOCAL", "GITHUB", "LABELS"}
	widths := make([]int, len(headers))
	for i, h := range headers {
		widths[i] = len(h)
	}

	rows := make([][]string, len(statuses))
	for i, s := range statuses {
		githubStatus := formatGitHubStatus(s)
		rows[i] = []string{s.Instance, s.Host, s.Repo, s.Mode, s.Local, githubStatus, s.Labels}
		for j, cell := range rows[i] {
			if len(cell) > widths[j] {
				widths[j] = len(cell)
			}
		}
	}

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
			case 4: // LOCAL
				styled = colorizeLocalStatus(cell)
			case 5: // GITHUB
				styled = colorizeGitHubStatus(cell)
			}
			line += cellStyle.Width(widths[j] + 2).Render(styled)
		}
		fmt.Println(line)
	}
}

func PrintConfig(cfg *config.Config) {
	fmt.Println(titleStyle.Render("Resolved Configuration"))

	patDisplay := cfg.GitHub.PAT
	if len(patDisplay) > 8 {
		patDisplay = patDisplay[:8] + "..." + patDisplay[len(patDisplay)-4:]
	}
	fmt.Printf("  %s %s\n\n", configKey.Render("PAT:"), configVal.Render(patDisplay))

	fmt.Println(configKey.Render("Hosts:"))
	for name, h := range cfg.Hosts {
		fmt.Printf("  %s  addr=%s  os=%s  arch=%s\n",
			configVal.Render(name), h.Addr, h.OS, h.Arch)
	}

	fmt.Println()
	fmt.Println(configKey.Render("Runners:"))
	for _, r := range cfg.Runners {
		hcfg := cfg.Hosts[r.Host]
		mode := r.EffectiveMode(hcfg.OS)
		fmt.Printf("  %s  repo=%s  host=%s  count=%d  mode=%s  labels=[%s]\n",
			configVal.Render(r.Name), r.Repo, r.Host, r.Count, mode, strings.Join(r.Labels, ", "))
	}
}

func formatGitHubStatus(s runner.RunnerStatus) string {
	if s.Remote == "" {
		return "-"
	}
	if s.Busy {
		return "busy"
	}
	return s.Remote
}

func colorizeLocalStatus(status string) string {
	switch status {
	case "running":
		return statusRunning.Render(status)
	case "stopped":
		return statusStopped.Render(status)
	case "not installed":
		return statusUnknown.Render(status)
	case "unreachable":
		return statusStopped.Render(status)
	default:
		return statusUnknown.Render(status)
	}
}

func colorizeGitHubStatus(status string) string {
	switch status {
	case "online":
		return statusOnline.Render(status)
	case "offline":
		return statusOffline.Render(status)
	case "busy":
		return statusBusy.Render(status)
	default:
		return statusUnknown.Render(status)
	}
}

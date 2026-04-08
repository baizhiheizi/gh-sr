package tui

import (
	"fmt"
	"sort"
	"strings"

	"github.com/an-lee/ghr/internal/config"
	"github.com/an-lee/ghr/internal/runner"
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

// FormatConfig returns a styled, redacted snapshot of the resolved configuration (stable host order).
func FormatConfig(cfg *config.Config) string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("Resolved Configuration"))
	b.WriteString("\n")

	tok, tokErr := config.ResolveToken(cfg)
	var patDisplay string
	if tokErr != nil {
		patDisplay = "(none)"
	} else if tok.Source == config.TokenSourceGH {
		patDisplay = "(from gh CLI)"
	} else if len(tok.Token) > 8 {
		patDisplay = tok.Token[:8] + "..." + tok.Token[len(tok.Token)-4:]
	} else {
		patDisplay = "(set)"
	}
	b.WriteString(fmt.Sprintf("  %s %s\n\n", configKey.Render("Token:"), configVal.Render(patDisplay)))

	b.WriteString(configKey.Render("Hosts:"))
	b.WriteString("\n")
	hostNames := make([]string, 0, len(cfg.Hosts))
	for name := range cfg.Hosts {
		hostNames = append(hostNames, name)
	}
	sort.Strings(hostNames)
	for _, name := range hostNames {
		h := cfg.Hosts[name]
		b.WriteString(fmt.Sprintf("  %s  addr=%s  os=%s  arch=%s\n",
			configVal.Render(name), h.Addr, h.OS, h.Arch))
	}

	b.WriteString("\n")
	b.WriteString(configKey.Render("Runners:"))
	b.WriteString("\n")
	for _, r := range cfg.Runners {
		hcfg := cfg.Hosts[r.Host]
		mode := r.EffectiveMode(hcfg.OS)
		b.WriteString(fmt.Sprintf("  %s  repo=%s  host=%s  count=%d  mode=%s  labels=[%s]\n",
			configVal.Render(r.Name), r.Repo, r.Host, r.Count, mode, strings.Join(r.Labels, ", ")))
	}
	return b.String()
}

func PrintConfig(cfg *config.Config) {
	fmt.Print(FormatConfig(cfg))
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

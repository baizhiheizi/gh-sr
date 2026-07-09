package tui

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/an-lee/gh-sr/internal/config"
	"github.com/an-lee/gh-sr/internal/runner"
)

func PrintStatusTable(statuses []runner.RunnerStatus) {
	rows := make([][]string, len(statuses))
	for i, s := range statuses {
		rows[i] = runnerStatusCells(s)
	}
	PrintTable(os.Stdout, TablePrintOptions{
		Title:    "Runner Status",
		EmptyMsg: "No runners found.",
		Headers:  runnerStatusHeaders,
		Rows:     rows,
		Colorize: runnerStatusColorize,
	})
}

// FormatConfig returns a styled, redacted snapshot of the resolved configuration (stable host order).
func FormatConfig(cfg *config.Config) string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("Resolved Configuration"))
	b.WriteString("\n")

	_, tokErr := config.ResolveToken(cfg)
	tokenDisplay := "(none)"
	if tokErr == nil {
		tokenDisplay = "(from gh CLI)"
	}
	b.WriteString(fmt.Sprintf("  %s %s\n\n", configKey.Render("Token:"), configVal.Render(tokenDisplay)))

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
		extra := ""
		if r.Ephemeral {
			extra += "  ephemeral"
		}
		b.WriteString(fmt.Sprintf("  %s  target=%s  host=%s  count=%d  mode=%s  labels=[%s]%s\n",
			configVal.Render(r.Name), r.DisplayTarget(), r.Host, r.Count, r.EffectiveRunnerMode(), strings.Join(r.Labels, ", "), extra))
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

func colorizeImageBuild(cell string) string {
	switch {
	case strings.HasPrefix(cell, "ok"):
		return statusOnline.Render(cell)
	case strings.HasPrefix(cell, "stale"):
		return statusStopped.Render(cell)
	case cell == "?":
		return statusUnknown.Render(cell)
	default:
		return cell
	}
}

func colorizeLocalStatus(status string) string {
	switch status {
	case "running":
		return statusRunning.Render(status)
	case "stopped":
		return statusStopped.Render(status)
	case "failed":
		return statusStopped.Render(status)
	case "restarting":
		return statusBusy.Render(status)
	case "service error":
		return statusBusy.Render(status)
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

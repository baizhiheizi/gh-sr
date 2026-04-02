package tui

import (
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/lipgloss"

	"github.com/an-lee/ghr/internal/config"
	"github.com/an-lee/ghr/internal/host"
	"github.com/an-lee/ghr/internal/runner"
)

type dashboardModel struct {
	cfg      *config.Config
	mgr      *runner.Manager
	statuses []runner.RunnerStatus
	cursor   int
	width    int
	height   int
	loading  bool
	lastErr  string
}

type statusRefreshedMsg struct {
	statuses []runner.RunnerStatus
	err      error
}

type tickMsg time.Time

func RunDashboard(cfg *config.Config) error {
	mgr := runner.NewManager(cfg.GitHub.PAT)
	m := dashboardModel{
		cfg:     cfg,
		mgr:     mgr,
		loading: true,
	}

	p := tea.NewProgram(m)
	_, err := p.Run()
	return err
}

func (m dashboardModel) Init() tea.Cmd {
	return tea.Batch(
		refreshStatus(m.cfg, m.mgr),
		tickEvery(5*time.Second),
	)
}

func (m dashboardModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "j", "down":
			if m.cursor < len(m.statuses)-1 {
				m.cursor++
			}
		case "k", "up":
			if m.cursor > 0 {
				m.cursor--
			}
		case "r":
			m.loading = true
			return m, refreshStatus(m.cfg, m.mgr)
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case statusRefreshedMsg:
		m.loading = false
		if msg.err != nil {
			m.lastErr = msg.err.Error()
		} else {
			m.lastErr = ""
			m.statuses = msg.statuses
		}

	case tickMsg:
		return m, tea.Batch(
			refreshStatus(m.cfg, m.mgr),
			tickEvery(5*time.Second),
		)
	}

	return m, nil
}

func (m dashboardModel) View() tea.View {
	var b strings.Builder

	title := titleStyle.Render("ghr dashboard")
	b.WriteString(title + "\n\n")

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
		b.WriteString("  No runners configured.\n")
		v := tea.NewView(b.String())
		v.AltScreen = true
		return v
	}

	headers := []string{"INSTANCE", "HOST", "REPO", "MODE", "LOCAL", "GITHUB", "LABELS"}
	widths := computeWidths(headers, m.statuses)

	var headerLine string
	for i, h := range headers {
		headerLine += headerStyle.Width(widths[i] + 2).Render(h)
	}
	b.WriteString(headerLine + "\n")

	for i, s := range m.statuses {
		ghStatus := formatGitHubStatus(s)
		cells := []string{s.Instance, s.Host, s.Repo, s.Mode, s.Local, ghStatus, s.Labels}

		var line string
		for j, cell := range cells {
			styled := cell
			switch j {
			case 4:
				styled = colorizeLocalStatus(cell)
			case 5:
				styled = colorizeGitHubStatus(cell)
			}
			style := cellStyle.Width(widths[j] + 2)
			if i == m.cursor {
				style = style.Background(lipgloss.Color("8"))
			}
			line += style.Render(styled)
		}
		b.WriteString(line + "\n")
	}

	loadingIndicator := ""
	if m.loading {
		loadingIndicator = " (refreshing...)"
	}

	help := helpStyle.Render(fmt.Sprintf(
		"  j/k: navigate  r: refresh  q: quit%s",
		loadingIndicator,
	))
	b.WriteString("\n" + help + "\n")

	v := tea.NewView(b.String())
	v.AltScreen = true
	return v
}

func computeWidths(headers []string, statuses []runner.RunnerStatus) []int {
	widths := make([]int, len(headers))
	for i, h := range headers {
		widths[i] = len(h)
	}
	for _, s := range statuses {
		ghStatus := formatGitHubStatus(s)
		cells := []string{s.Instance, s.Host, s.Repo, s.Mode, s.Local, ghStatus, s.Labels}
		for j, cell := range cells {
			if len(cell) > widths[j] {
				widths[j] = len(cell)
			}
		}
	}
	return widths
}

func refreshStatus(cfg *config.Config, mgr *runner.Manager) tea.Cmd {
	return func() tea.Msg {
		var allStatuses []runner.RunnerStatus

		for _, rc := range cfg.Runners {
			hcfg := cfg.Hosts[rc.Host]
			h := host.NewHost(rc.Host, hcfg)
			if err := h.Connect(); err != nil {
				for _, name := range rc.InstanceNames() {
					allStatuses = append(allStatuses, runner.RunnerStatus{
						Instance: name,
						Host:     rc.Host,
						Repo:     rc.Repo,
						Mode:     rc.EffectiveMode(hcfg.OS),
						Local:    "unreachable",
					})
				}
				continue
			}
			defer h.Close()

			statuses, err := mgr.Status(h, rc)
			if err != nil {
				continue
			}
			allStatuses = append(allStatuses, statuses...)
		}

		mgr.EnrichWithGitHubStatus(allStatuses, cfg)

		return statusRefreshedMsg{statuses: allStatuses}
	}
}

func tickEvery(d time.Duration) tea.Cmd {
	return func() tea.Msg {
		time.Sleep(d)
		return tickMsg(time.Now())
	}
}

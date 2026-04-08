package runner

import (
	"fmt"
	"io"
	"os"
	"strings"
	"sync"

	"github.com/an-lee/ghr/internal/autostart"
	"github.com/an-lee/ghr/internal/config"
	"github.com/an-lee/ghr/internal/host"
)

type Manager struct {
	GitHub *GitHubClient
	// Out receives progress messages from runner operations. If nil, os.Stdout is used.
	Out io.Writer
}

func (m *Manager) out() io.Writer {
	if m != nil && m.Out != nil {
		return m.Out
	}
	return os.Stdout
}

func NewManager(pat string) *Manager {
	return &Manager{
		GitHub: NewGitHubClient(pat),
	}
}

type RunnerStatus struct {
	Instance string
	Host     string
	Repo     string
	Labels   string
	Mode     string
	Local    string // "running", "stopped", "not installed"
	Remote   string // from GitHub API: "online", "offline", ""
	Busy     bool
}

// ResolveModeOnHost returns the effective runner mode for this host, probing Docker availability
// when mode is not explicitly set. If Docker is available and working, returns "docker";
// otherwise falls back to "native". When mode is explicitly set in config, returns it as-is.
func ResolveModeOnHost(h *host.Host, rc config.RunnerConfig) string {
	if rc.Mode != "" {
		return rc.Mode
	}
	if host.DetectDockerAvailable(h) {
		return "docker"
	}
	return "native"
}

func (m *Manager) Setup(h *host.Host, rc config.RunnerConfig) error {
	mode := rc.EffectiveMode(h.OS)

	switch mode {
	case "docker":
		return m.setupDocker(h)
	case "native":
		return m.setupNative(h, rc)
	default:
		return fmt.Errorf("unknown mode %q", mode)
	}
}

// NeedsSetup checks whether a runner requires setup before it can start.
// For docker mode: checks if the image is available locally.
// For native mode: checks if any instance is missing the .runner config file.
func (m *Manager) NeedsSetup(h *host.Host, rc config.RunnerConfig) bool {
	mode := rc.EffectiveMode(h.OS)
	switch mode {
	case "docker":
		cmd := fmt.Sprintf("docker images -q %s 2>/dev/null", RunnerDockerImage)
		if h.OS == "windows" {
			cmd = fmt.Sprintf(`docker images -q %s 2>$null`, RunnerDockerImage)
		}
		out, err := dockerRun(h, cmd)
		return err != nil || strings.TrimSpace(out) == ""
	case "native":
		for _, name := range rc.InstanceNames() {
			ok, _ := NativeRunnerConfigPresent(h, name)
			if !ok {
				return true
			}
		}
		return false
	}
	return true
}

// EnsureSetup runs setup only if the runner is not already installed.
func (m *Manager) EnsureSetup(h *host.Host, rc config.RunnerConfig) error {
	if !m.NeedsSetup(h, rc) {
		return nil
	}
	fmt.Fprintf(m.out(), "  %s: not yet set up, running setup...\n", rc.Name)
	return m.Setup(h, rc)
}

func (m *Manager) Start(h *host.Host, rc config.RunnerConfig) error {
	mode := rc.EffectiveMode(h.OS)

	for _, name := range rc.InstanceNames() {
		var err error
		switch mode {
		case "docker":
			err = m.startDocker(h, rc, name)
		case "native":
			kind, derr := autostart.Detect(h, name)
			if derr != nil {
				return fmt.Errorf("starting %s: %w", name, derr)
			}
			if kind != autostart.KindNone {
				err = autostart.Start(h, name)
			} else {
				err = m.startNative(h, rc, name)
			}
		}
		if err != nil {
			return fmt.Errorf("starting %s: %w", name, err)
		}
	}
	return nil
}

func (m *Manager) Stop(h *host.Host, rc config.RunnerConfig) error {
	mode := rc.EffectiveMode(h.OS)

	for _, name := range rc.InstanceNames() {
		var err error
		switch mode {
		case "docker":
			err = m.stopDocker(h, name)
		case "native":
			kind, derr := autostart.Detect(h, name)
			if derr != nil {
				return fmt.Errorf("stopping %s: %w", name, derr)
			}
			if kind != autostart.KindNone {
				err = autostart.Stop(h, name)
			} else {
				err = m.stopNative(h, name)
			}
		}
		if err != nil {
			return fmt.Errorf("stopping %s: %w", name, err)
		}
	}
	return nil
}

func (m *Manager) Remove(h *host.Host, rc config.RunnerConfig) error {
	mode := rc.EffectiveMode(h.OS)

	for _, name := range rc.InstanceNames() {
		var err error
		switch mode {
		case "docker":
			err = m.removeDocker(h, name)
		case "native":
			err = m.removeNative(h, rc, name)
		}
		if err != nil {
			return fmt.Errorf("removing %s: %w", name, err)
		}
	}
	return nil
}

func (m *Manager) Status(h *host.Host, rc config.RunnerConfig) ([]RunnerStatus, error) {
	mode := rc.EffectiveMode(h.OS)
	var statuses []RunnerStatus

	for _, name := range rc.InstanceNames() {
		s := RunnerStatus{
			Instance: name,
			Host:     rc.Host,
			Repo:     rc.Repo,
			Labels:   strings.Join(rc.EffectiveLabels(h.OS, h.Arch), ", "),
			Mode:     mode,
		}

		switch mode {
		case "docker":
			s.Local = m.statusDocker(h, name)
		case "native":
			s.Local = m.statusNative(h, name)
		}

		statuses = append(statuses, s)
	}

	return statuses, nil
}

func (m *Manager) Logs(h *host.Host, rc config.RunnerConfig, instanceName string) (string, error) {
	mode := rc.EffectiveMode(h.OS)

	switch mode {
	case "docker":
		return m.logsDocker(h, instanceName)
	case "native":
		return m.logsNative(h, instanceName)
	default:
		return "", fmt.Errorf("unknown mode %q", mode)
	}
}

// expectedGitHubRunnerOS is the self-hosted runner "os" field from the GitHub API for this ghr row (mode + host OS).
func expectedGitHubRunnerOS(mode, hostOS string) string {
	effective := mode
	if effective == "" {
		if hostOS == "linux" {
			effective = "docker"
		} else {
			effective = "native"
		}
	}
	if effective == "docker" {
		return "Linux"
	}
	switch hostOS {
	case "windows":
		return "Windows"
	case "linux":
		return "Linux"
	case "darwin":
		return "macOS"
	default:
		return ""
	}
}

func (m *Manager) EnrichWithGitHubStatus(statuses []RunnerStatus, cfg *config.Config) {
	repos := cfg.UniqueRepos()
	repoRunners := make(map[string][]GitHubRunner, len(repos))

	var mu sync.Mutex
	var wg sync.WaitGroup
	for _, repo := range repos {
		wg.Add(1)
		go func(repo string) {
			defer wg.Done()
			runners, err := m.GitHub.ListRunners(repo)
			if err != nil {
				return
			}
			mu.Lock()
			repoRunners[repo] = runners
			mu.Unlock()
		}(repo)
	}
	wg.Wait()

	for i := range statuses {
		hcfg, ok := cfg.Hosts[statuses[i].Host]
		if !ok {
			continue
		}
		exp := expectedGitHubRunnerOS(statuses[i].Mode, hcfg.OS)
		ghRunners := repoRunners[statuses[i].Repo]
		for _, gr := range ghRunners {
			if gr.Name != statuses[i].Instance {
				continue
			}
			if exp != "" && gr.OS != "" && !strings.EqualFold(gr.OS, exp) {
				continue
			}
			statuses[i].Remote = gr.Status
			statuses[i].Busy = gr.Busy
			break
		}
	}
}

func (m *Manager) CleanupOffline(cfg *config.Config) (int, error) {
	removed := 0
	for _, repo := range cfg.UniqueRepos() {
		runners, err := m.GitHub.ListRunners(repo)
		if err != nil {
			return removed, fmt.Errorf("listing runners for %s: %w", repo, err)
		}
		for _, r := range runners {
			if r.Status == "offline" {
				if err := m.GitHub.DeleteRunner(repo, r.ID); err != nil {
					return removed, fmt.Errorf("deleting runner %s (id=%d): %w", r.Name, r.ID, err)
				}
				removed++
			}
		}
	}
	return removed, nil
}

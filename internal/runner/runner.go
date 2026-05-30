package runner

import (
	"fmt"
	"io"
	"os"
	"strings"
	"sync"

	"github.com/an-lee/gh-sr/internal/autostart"
	"github.com/an-lee/gh-sr/internal/config"
	"github.com/an-lee/gh-sr/internal/host"
)

type Manager struct {
	GitHub *GitHubClient
	// Out receives progress messages from runner operations. If nil, os.Stdout is used.
	Out io.Writer
	// GhSrVersion is the gh-sr CLI version (e.g. Makefile -X main.version). Used for
	// container image layout revision and Docker image labels.
	GhSrVersion string
	// ContainerImageExtraApt is global extra apt packages for the gh-sr/agentic-runner
	// image (from runners.yml container_runner_image). Set by ops before container setup.
	ContainerImageExtraApt []string
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
	Repo     string // owner/repo for repo-scoped, "org:name" for org-scoped
	Labels   string
	Mode     string
	// ContainerImage is the Docker image ref from the container config (runner_mode: container only).
	// Empty for native runners or when the container does not exist.
	ContainerImage string
	// ContainerImageRevision is the gh-sr.image-revision label on the running image (empty if missing).
	ContainerImageRevision string
	// ContainerImageBuild is a short match hint: "-", "?", "ok (hexprefix)", "stale (hexprefix)".
	ContainerImageBuild string
	Local               string // "running", "stopped", "not installed"
	Remote              string // from GitHub API: "online", "offline", ""
	Busy                bool
}

func (m *Manager) Setup(h *host.Host, rc config.RunnerConfig) error {
	if rc.IsContainerMode() {
		return m.setupContainer(h, rc)
	}
	return m.setupNative(h, rc)
}

func (m *Manager) containerImageExtraApt() []string {
	if m == nil {
		return nil
	}
	return m.ContainerImageExtraApt
}

// NeedsSetup checks whether a runner requires setup before it can start.
// Checks if any instance is missing the .runner config file (native) or container (container mode).
func (m *Manager) NeedsSetup(h *host.Host, rc config.RunnerConfig) bool {
	if rc.IsContainerMode() {
		return m.needsSetupContainer(h, rc)
	}
	for _, name := range rc.InstanceNames() {
		ok, _ := NativeRunnerConfigPresent(h, name)
		if !ok {
			return true
		}
	}
	return false
}

// EnsureSetup runs setup only if the runner is not already installed.
func (m *Manager) EnsureSetup(h *host.Host, rc config.RunnerConfig) error {
	if !m.NeedsSetup(h, rc) {
		return nil
	}
	fmt.Fprintf(m.out(), "  %s: not yet set up, running setup...\n", rc.Name)
	return m.Setup(h, rc)
}

// RebuildImage rebuilds the container runner Docker image for container-mode
// runners (agentic or not), recreates each container instance (preserving runner
// state), and starts them. Native-mode runners are a no-op (return nil).
func (m *Manager) RebuildImage(h *host.Host, rc config.RunnerConfig) error {
	if !rc.IsContainerMode() {
		fmt.Fprintf(m.out(), "  %s: skipping rebuild (not runner_mode: container)\n", rc.Name)
		return nil
	}
	return m.rebuildContainerImage(h, rc)
}

func (m *Manager) startAutostartWithDarwinFallback(h *host.Host, rc config.RunnerConfig, name string) error {
	err := autostart.Start(h, name)
	if err != nil && h.OS == "darwin" {
		fmt.Fprintf(m.out(), "  %s: warning: launchd start failed (%v); falling back to direct start (log in at the Mac GUI for launchd autostart on boot)\n", name, err)
		return m.startNative(h, rc, name)
	}
	return err
}

func (m *Manager) Start(h *host.Host, rc config.RunnerConfig) error {
	if rc.IsContainerMode() {
		for i, name := range rc.InstanceNames() {
			env := m.NewContainerEnvironment(h, rc, i, name)
			if err := env.Start(); err != nil {
				return err
			}
			fmt.Fprintf(m.out(), "  %s: container started\n", name)
			// Health-gate: confirm the slot is actually ready (inner dockerd up,
			// runner registered) before treating it as available. Non-fatal so a
			// slow first boot does not fail `gh sr up`; doctor surfaces persistent issues.
			if err := env.AwaitHealthy(defaultContainerHealthTimeout); err != nil {
				fmt.Fprintf(m.out(), "  %s: warning: not ready yet: %v\n", name, err)
			} else {
				fmt.Fprintf(m.out(), "  %s: ready (inner dockerd up, runner registered)\n", name)
			}
		}
		return nil
	}

	for _, name := range rc.InstanceNames() {
		// Prefer svc.sh for Linux if it's deployed
		if h.OS == "linux" && svcShPresent(h, name) {
			dir := h.RunnerDir(name)
			// Install the systemd unit first if not already installed (.service file is the marker).
			cmd := fmt.Sprintf("cd %s && %s\nif [ ! -f .service ]; then $SUDO ./svc.sh install; fi\n$SUDO ./svc.sh start", dir, strings.TrimSpace(linuxElevatePrelude))
			out, err := h.Run(cmd)
			if err != nil {
				return fmt.Errorf("starting %s via svc.sh: %w", name, err)
			}
			fmt.Fprintf(m.out(), "  %s: %s\n", name, strings.TrimSpace(out))
			continue
		}

		kind, derr := autostart.Detect(h, name)
		if derr != nil {
			return fmt.Errorf("starting %s: %w", name, derr)
		}
		var err error
		if kind != autostart.KindNone {
			err = m.startAutostartWithDarwinFallback(h, rc, name)
		} else {
			// Auto-install autostart for non-ephemeral runners so the runner
			// auto-restarts on crash and starts on boot.
			if !rc.Ephemeral {
				fmt.Fprintf(m.out(), "  %s: installing autostart for always-on...\n", name)
				if ierr := autostart.Install(h, name, autostart.InstallOpts{}); ierr != nil {
					fmt.Fprintf(m.out(), "  %s: warning: failed to install autostart: %v\n", name, ierr)
					// Fall back to direct start; autostart install failure is non-fatal.
				}
				// Re-detect after install attempt.
				kind, _ = autostart.Detect(h, name)
			}
			if kind != autostart.KindNone {
				err = m.startAutostartWithDarwinFallback(h, rc, name)
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
	if rc.IsContainerMode() {
		for _, name := range rc.InstanceNames() {
			if err := m.stopContainer(h, name); err != nil {
				return err
			}
			fmt.Fprintf(m.out(), "  %s: container stopped\n", name)
		}
		return nil
	}

	for _, name := range rc.InstanceNames() {
		// Prefer svc.sh for Linux if it's deployed
		if h.OS == "linux" && svcShPresent(h, name) {
			dir := h.RunnerDir(name)
			cmd := fmt.Sprintf("cd %s && %s\n$SUDO ./svc.sh stop", dir, strings.TrimSpace(linuxElevatePrelude))
			out, err := h.Run(cmd)
			if err != nil {
				return fmt.Errorf("stopping %s via svc.sh: %w", name, err)
			}
			fmt.Fprintf(m.out(), "  %s: %s\n", name, strings.TrimSpace(out))
			continue
		}

		kind, derr := autostart.Detect(h, name)
		if derr != nil {
			return fmt.Errorf("stopping %s: %w", name, derr)
		}
		var err error
		if kind != autostart.KindNone {
			err = autostart.Stop(h, name)
			// LaunchAgent may be installed but the runner was started directly
			// (e.g. macOS SSH with no GUI login). Stop that process too.
			if h.OS == "darwin" {
				if nerr := m.stopNative(h, name); nerr != nil && err == nil {
					err = nerr
				}
			}
		} else {
			err = m.stopNative(h, name)
		}
		if err != nil {
			return fmt.Errorf("stopping %s: %w", name, err)
		}
	}
	return nil
}

func (m *Manager) Remove(h *host.Host, rc config.RunnerConfig) error {
	for _, name := range rc.InstanceNames() {
		if rc.IsContainerMode() {
			if err := m.removeContainer(h, rc, name); err != nil {
				return fmt.Errorf("removing %s: %w", name, err)
			}
		} else {
			if err := m.removeNative(h, rc, name); err != nil {
				return fmt.Errorf("removing %s: %w", name, err)
			}
		}
	}
	return nil
}

func (m *Manager) Status(h *host.Host, rc config.RunnerConfig) ([]RunnerStatus, error) {
	var statuses []RunnerStatus

	for i, name := range rc.InstanceNames() {
		repoDisplay := rc.Repo
		if rc.Org != "" {
			repoDisplay = "org:" + rc.Org
		}
		mode := rc.EffectiveRunnerMode()
		s := RunnerStatus{
			Instance: name,
			Host:     rc.Host,
			Repo:     repoDisplay,
			Labels:   strings.Join(rc.EffectiveLabelsForInstance(h.OS, h.Arch, i), ", "),
			Mode:     mode,
		}

		if rc.IsContainerMode() {
			s.Local, s.ContainerImage, s.ContainerImageRevision = m.containerLocalStatusImageAndRevision(h, name)
			expected := ContainerImageLayoutRevision(m.GhSrVersion, m.containerImageExtraApt())
			s.ContainerImageBuild = formatContainerImageBuild(s.Local, expected, s.ContainerImageRevision)
		} else {
			s.Local = m.statusNative(h, name)
			s.ContainerImageBuild = nativeRunnerVersion(h, name)
		}
		statuses = append(statuses, s)
	}

	return statuses, nil
}

func (m *Manager) Logs(h *host.Host, rc config.RunnerConfig, instanceName string) (string, error) {
	if rc.IsContainerMode() {
		return m.logsContainer(h, instanceName)
	}
	return m.logsNative(h, instanceName)
}

// expectedGitHubRunnerOS returns the expected OS label for GitHub API based on host OS.
func expectedGitHubRunnerOS(hostOS string) string {
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
	type scopeKey struct{ scope, target string }
	keys := make(map[scopeKey]bool)
	for _, r := range cfg.Runners {
		keys[scopeKey{r.Scope(), r.ScopeTarget()}] = true
	}

	scopeRunners := make(map[scopeKey][]GitHubRunner, len(keys))
	var mu sync.Mutex
	var wg sync.WaitGroup
	for key := range keys {
		wg.Add(1)
		go func(k scopeKey) {
			defer wg.Done()
			runners, err := m.GitHub.ListRunnersScoped(k.scope, k.target)
			if err != nil {
				return
			}
			mu.Lock()
			scopeRunners[k] = runners
			mu.Unlock()
		}(key)
	}
	wg.Wait()

	rcByInstance := make(map[string]*config.RunnerConfig)
	for i := range cfg.Runners {
		rc := &cfg.Runners[i]
		for _, inst := range rc.InstanceNames() {
			rcByInstance[inst] = rc
		}
	}

	for i := range statuses {
		hcfg, ok := cfg.Hosts[statuses[i].Host]
		if !ok {
			continue
		}
		rc := rcByInstance[statuses[i].Instance]
		if rc == nil {
			continue
		}
		exp := expectedGitHubRunnerOS(hcfg.OS)
		key := scopeKey{rc.Scope(), rc.ScopeTarget()}
		for _, gr := range scopeRunners[key] {
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
	type scopeKey struct{ scope, target string }
	seen := make(map[scopeKey]bool)
	removed := 0

	for _, rc := range cfg.Runners {
		key := scopeKey{rc.Scope(), rc.ScopeTarget()}
		if seen[key] {
			continue
		}
		seen[key] = true
		runners, err := m.GitHub.ListRunnersScoped(key.scope, key.target)
		if err != nil {
			return removed, fmt.Errorf("listing runners for %s: %w", key.target, err)
		}
		for _, r := range runners {
			if r.Status == "offline" {
				if err := m.GitHub.DeleteRunnerScoped(key.scope, key.target, r.ID); err != nil {
					return removed, fmt.Errorf("deleting runner %s (id=%d): %w", r.Name, r.ID, err)
				}
				removed++
			}
		}
	}
	return removed, nil
}

func formatContainerImageBuild(local, expected, actual string) string {
	if local == "not installed" {
		return "-"
	}
	if actual == "" {
		return "?"
	}
	short := func(s string) string {
		if len(s) <= 8 {
			return s
		}
		return s[:8]
	}
	if actual == expected {
		return "ok (" + short(expected) + ")"
	}
	return "stale (" + short(actual) + ")"
}

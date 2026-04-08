package config

import (
	"fmt"
	"os"
	"runtime"
	"strings"

	"gopkg.in/yaml.v3"
)

const LocalAddr = "local"

func IsLocalAddr(addr string) bool {
	return strings.EqualFold(strings.TrimSpace(addr), LocalAddr)
}

type Config struct {
	GitHub  GitHubConfig          `yaml:"github"`
	Hosts   map[string]HostConfig `yaml:"hosts"`
	Runners []RunnerConfig        `yaml:"runners"`
}

type GitHubConfig struct {
	PAT string `yaml:"pat"`
}

type HostConfig struct {
	Addr         string `yaml:"addr"`
	OS           string `yaml:"os"`
	Arch         string `yaml:"arch"`
	WindowsPS    string `yaml:"windows_ps"`    // powershell (default) or pwsh — which exe runs encoded remote scripts on Windows
	DockerSocket string `yaml:"docker_socket"` // override Docker socket path (Linux/macOS; if empty, gh wm probes default path, docker context, then Colima default on macOS)
}

type RunnerConfig struct {
	Name              string   `yaml:"name"`
	Repo              string   `yaml:"repo"`
	Org               string   `yaml:"org"`
	Group             string   `yaml:"group"`
	Host              string   `yaml:"host"`
	Count             int      `yaml:"count"`
	Labels            []string `yaml:"labels"`
	Mode              string   `yaml:"mode"`
	Profile           string   `yaml:"profile"`
	Ephemeral         bool     `yaml:"ephemeral"`
	DockerNetworkMode string   `yaml:"docker_network_mode"` // bridge (default) or host; only for docker-mode Linux runners
	// DockerCapAdd lists Linux capability names passed to docker run --cap-add (e.g. NET_ADMIN for gh-aw AWF iptables).
	DockerCapAdd []string `yaml:"docker_cap_add"`
}

// Scope returns "repo" or "org" depending on how the runner is registered.
func (rc *RunnerConfig) Scope() string {
	if rc.Org != "" {
		return "org"
	}
	return "repo"
}

// ScopeTarget returns the repo (owner/repo) or org name used for GitHub API calls.
func (rc *RunnerConfig) ScopeTarget() string {
	if rc.Org != "" {
		return rc.Org
	}
	return rc.Repo
}

// IsAgentic reports whether the runner uses the agentic workflow profile.
func (rc *RunnerConfig) IsAgentic() bool {
	return strings.EqualFold(strings.TrimSpace(rc.Profile), "agentic")
}

func (rc *RunnerConfig) EffectiveMode(hostOS string) string {
	if rc.Mode != "" {
		return rc.Mode
	}
	if hostOS == "linux" {
		return "docker"
	}
	return "native"
}

// EffectiveDockerNetworkMode returns bridge or host for docker run --network.
// Only docker-mode runners may use host; everything else resolves to bridge.
func (rc *RunnerConfig) EffectiveDockerNetworkMode(hostOS string) string {
	if rc.EffectiveMode(hostOS) != "docker" {
		return "bridge"
	}
	switch strings.ToLower(strings.TrimSpace(rc.DockerNetworkMode)) {
	case "host":
		return "host"
	default:
		return "bridge"
	}
}

// DefaultLabels generates standard GitHub Actions labels based on mode, host OS, and arch.
// Docker-mode runners always report as Linux regardless of host OS.
func DefaultLabels(mode, hostOS, arch string) []string {
	labels := []string{"self-hosted"}

	osLabel := ""
	switch mode {
	case "docker":
		osLabel = "Linux"
	default:
		switch hostOS {
		case "linux":
			osLabel = "Linux"
		case "darwin":
			osLabel = "macOS"
		case "windows":
			osLabel = "Windows"
		}
	}
	if osLabel != "" {
		labels = append(labels, osLabel)
	}

	archLabel := ""
	switch arch {
	case "amd64":
		archLabel = "X64"
	case "arm64":
		archLabel = "ARM64"
	}
	if archLabel != "" {
		labels = append(labels, archLabel)
	}

	return labels
}

// EffectiveLabels returns the runner's labels, auto-generating them from host info if empty.
// Agentic-profile runners automatically get a "gh-aw" label appended.
func (rc *RunnerConfig) EffectiveLabels(hostOS, arch string) []string {
	var labels []string
	if len(rc.Labels) > 0 {
		labels = rc.Labels
	} else {
		mode := rc.EffectiveMode(hostOS)
		labels = DefaultLabels(mode, hostOS, arch)
	}
	if rc.IsAgentic() && !hasLabel(labels, "gh-aw") {
		labels = append(labels, "gh-aw")
	}
	return labels
}

func hasLabel(labels []string, target string) bool {
	for _, l := range labels {
		if strings.EqualFold(l, target) {
			return true
		}
	}
	return false
}

func (rc *RunnerConfig) InstanceNames() []string {
	count := rc.Count
	if count < 1 {
		count = 1
	}
	names := make([]string, count)
	for i := range count {
		names[i] = fmt.Sprintf("%s-%d", rc.Name, i+1)
	}
	return names
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing config: %w", err)
	}

	cfg.resolveEnvRefs()
	cfg.applyDefaults()

	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return &cfg, nil
}

func (c *Config) resolveEnvRefs() {
	c.GitHub.PAT = resolveEnv(c.GitHub.PAT)
}

func resolveEnv(val string) string {
	if strings.HasPrefix(val, "env:") {
		envVar := strings.TrimPrefix(val, "env:")
		return os.Getenv(envVar)
	}
	return val
}

func normalizeArch(goarch string) string {
	switch goarch {
	case "amd64", "arm64":
		return goarch
	default:
		return goarch
	}
}

func (c *Config) applyDefaults() {
	for name, h := range c.Hosts {
		if IsLocalAddr(h.Addr) {
			if h.OS == "" {
				h.OS = runtime.GOOS
			}
			if h.Arch == "" {
				h.Arch = normalizeArch(runtime.GOARCH)
			}
			c.Hosts[name] = h
		}
	}
	for i := range c.Runners {
		if c.Runners[i].Count < 1 {
			c.Runners[i].Count = 1
		}
		c.Runners[i].applyAgenticDefaults()
	}
}

// applyAgenticDefaults fills in the Docker configuration implied by profile: agentic.
func (rc *RunnerConfig) applyAgenticDefaults() {
	if !rc.IsAgentic() {
		return
	}
	if rc.Mode == "" {
		rc.Mode = "docker"
	}
	if rc.DockerNetworkMode == "" {
		rc.DockerNetworkMode = "host"
	}
	if !hasCapability(rc.DockerCapAdd, "NET_ADMIN") {
		rc.DockerCapAdd = append(rc.DockerCapAdd, "NET_ADMIN")
	}
}

func hasCapability(caps []string, target string) bool {
	for _, c := range caps {
		if strings.EqualFold(c, target) {
			return true
		}
	}
	return false
}

func (c *Config) Validate() error {
	if len(c.Hosts) == 0 {
		return fmt.Errorf("at least one host must be defined")
	}

	for name, h := range c.Hosts {
		if h.Addr == "" {
			return fmt.Errorf("host %q: addr is required (use \"local\" for the current machine)", name)
		}
		// os and arch may be empty for remote hosts -- they are auto-detected over SSH before operations.
		if h.OS != "" {
			switch h.OS {
			case "linux", "darwin", "windows":
			default:
				return fmt.Errorf("host %q: os must be linux, darwin, or windows (got %q)", name, h.OS)
			}
		}
		if h.Arch != "" {
			switch h.Arch {
			case "amd64", "arm64":
			default:
				return fmt.Errorf("host %q: arch must be amd64 or arm64 (got %q)", name, h.Arch)
			}
		}
		if h.WindowsPS != "" {
			if h.OS != "" && h.OS != "windows" {
				return fmt.Errorf("host %q: windows_ps is only valid when os is windows", name)
			}
			switch strings.ToLower(strings.TrimSpace(h.WindowsPS)) {
			case "powershell", "pwsh":
			default:
				return fmt.Errorf("host %q: windows_ps must be powershell or pwsh (got %q)", name, h.WindowsPS)
			}
		}
		if h.DockerSocket != "" {
			if h.OS != "" && h.OS != "linux" && h.OS != "darwin" {
				return fmt.Errorf("host %q: docker_socket is only supported on Linux and macOS hosts", name)
			}
			if !strings.HasPrefix(h.DockerSocket, "/") {
				return fmt.Errorf("host %q: docker_socket must be an absolute path (got %q)", name, h.DockerSocket)
			}
		}
	}

	if len(c.Runners) == 0 {
		return fmt.Errorf("at least one runner must be defined")
	}

	for _, r := range c.Runners {
		if r.Name == "" {
			return fmt.Errorf("runner name is required")
		}
		if r.Repo == "" && r.Org == "" {
			return fmt.Errorf("runner %q: repo or org is required", r.Name)
		}
		if r.Repo != "" && r.Org != "" {
			return fmt.Errorf("runner %q: specify repo or org, not both", r.Name)
		}
		if r.Group != "" && r.Org == "" {
			return fmt.Errorf("runner %q: group requires org (runner groups are organization-level)", r.Name)
		}
		if r.Host == "" {
			return fmt.Errorf("runner %q: host is required", r.Name)
		}
		hcfg, ok := c.Hosts[r.Host]
		if !ok {
			return fmt.Errorf("runner %q: host %q not found in hosts", r.Name, r.Host)
		}
		if r.Profile != "" && !strings.EqualFold(r.Profile, "agentic") {
			return fmt.Errorf("runner %q: profile must be 'agentic' (got %q)", r.Name, r.Profile)
		}
		if r.IsAgentic() && r.Mode == "native" {
			return fmt.Errorf("runner %q: profile 'agentic' requires docker mode (AWF sandbox needs Docker)", r.Name)
		}
		if r.Mode != "" && r.Mode != "docker" && r.Mode != "native" {
			return fmt.Errorf("runner %q: mode must be 'docker' or 'native' (got %q)", r.Name, r.Mode)
		}
		netMode := strings.ToLower(strings.TrimSpace(r.DockerNetworkMode))
		if netMode != "" && netMode != "bridge" && netMode != "host" {
			return fmt.Errorf("runner %q: docker_network_mode must be 'bridge' or 'host' (got %q)", r.Name, r.DockerNetworkMode)
		}
		if netMode != "" {
			if r.EffectiveMode(hcfg.OS) != "docker" {
				return fmt.Errorf("runner %q: docker_network_mode applies only when mode is docker", r.Name)
			}
		}
		if err := validateDockerCapAdd(&r, hcfg.OS); err != nil {
			return err
		}
	}

	return nil
}

func validateDockerCapAdd(r *RunnerConfig, hostOS string) error {
	if len(r.DockerCapAdd) == 0 {
		return nil
	}
	if r.EffectiveMode(hostOS) != "docker" {
		return fmt.Errorf("runner %q: docker_cap_add applies only when mode is docker", r.Name)
	}
	for _, cap := range r.DockerCapAdd {
		c := strings.TrimSpace(cap)
		if c == "" {
			return fmt.Errorf("runner %q: docker_cap_add contains an empty entry", r.Name)
		}
		if c != cap {
			return fmt.Errorf("runner %q: docker_cap_add entries must not have leading or trailing spaces (got %q)", r.Name, cap)
		}
		for _, ch := range c {
			isLetter := (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z')
			isDigit := ch >= '0' && ch <= '9'
			if !isLetter && !isDigit && ch != '_' {
				return fmt.Errorf("runner %q: docker_cap_add invalid capability name %q (use letters, digits, underscore only)", r.Name, c)
			}
		}
	}
	return nil
}

// NeedsDetection reports whether any host is missing os or arch (and is not local).
func (c *Config) NeedsDetection() bool {
	for _, h := range c.Hosts {
		if !IsLocalAddr(h.Addr) && (h.OS == "" || h.Arch == "") {
			return true
		}
	}
	return false
}

func (c *Config) RunnersForHost(hostName string) []RunnerConfig {
	var result []RunnerConfig
	for _, r := range c.Runners {
		if r.Host == hostName {
			result = append(result, r)
		}
	}
	return result
}

func (c *Config) RunnersForRepo(repo string) []RunnerConfig {
	var result []RunnerConfig
	for _, r := range c.Runners {
		if r.Repo == repo {
			result = append(result, r)
		}
	}
	return result
}

func (c *Config) UniqueRepos() []string {
	seen := map[string]bool{}
	var repos []string
	for _, r := range c.Runners {
		if r.Repo == "" {
			continue
		}
		if !seen[r.Repo] {
			seen[r.Repo] = true
			repos = append(repos, r.Repo)
		}
	}
	return repos
}

func (c *Config) UniqueOrgs() []string {
	seen := map[string]bool{}
	var orgs []string
	for _, r := range c.Runners {
		if r.Org == "" {
			continue
		}
		if !seen[r.Org] {
			seen[r.Org] = true
			orgs = append(orgs, r.Org)
		}
	}
	return orgs
}

func (c *Config) FindRunner(name string) (*RunnerConfig, bool) {
	for i := range c.Runners {
		if c.Runners[i].Name == name {
			return &c.Runners[i], true
		}
		for _, inst := range c.Runners[i].InstanceNames() {
			if inst == name {
				return &c.Runners[i], true
			}
		}
	}
	return nil, false
}

// ResolveRunnerInstance maps a CLI argument (base name or instance) to the instance directory name (e.g. myapp-1).
func (rc *RunnerConfig) ResolveRunnerInstance(nameArg string) (string, error) {
	for _, inst := range rc.InstanceNames() {
		if inst == nameArg {
			return inst, nil
		}
	}
	if rc.Name == nameArg {
		names := rc.InstanceNames()
		if len(names) != 1 {
			return "", fmt.Errorf("runner %q has %d instances; specify one of: %s", rc.Name, len(names), strings.Join(names, ", "))
		}
		return names[0], nil
	}
	return "", fmt.Errorf("runner %q: %q is not a valid name or instance", rc.Name, nameArg)
}

// FindRunnerForLogs resolves a runner by base or instance name. If hostFilter is non-empty, only that host's runner block matches.
// Returns an error when nothing matches or when multiple hosts match the same name without a host filter.
func (c *Config) FindRunnerForLogs(nameArg, hostFilter string) (*RunnerConfig, error) {
	var matches []*RunnerConfig
	seen := map[*RunnerConfig]bool{}
	for i := range c.Runners {
		r := &c.Runners[i]
		if hostFilter != "" && r.Host != hostFilter {
			continue
		}
		matched := false
		if r.Name == nameArg {
			matched = true
		} else {
			for _, inst := range r.InstanceNames() {
				if inst == nameArg {
					matched = true
					break
				}
			}
		}
		if matched && !seen[r] {
			seen[r] = true
			matches = append(matches, r)
		}
	}
	if len(matches) == 0 {
		if hostFilter != "" {
			return nil, fmt.Errorf("runner %q not found for host %q", nameArg, hostFilter)
		}
		return nil, fmt.Errorf("runner %q not found in config", nameArg)
	}
	if len(matches) > 1 {
		hosts := make([]string, 0, len(matches))
		for _, r := range matches {
			hosts = append(hosts, r.Host)
		}
		return nil, fmt.Errorf("runner %q matches multiple hosts %v; specify --host", nameArg, hosts)
	}
	return matches[0], nil
}

// FilterRunners returns runners matching optional host, repo, and/or explicit runner/instance names.
func FilterRunners(cfg *Config, hostFilter, repoFilter string, nameArgs []string) []RunnerConfig {
	runners := cfg.Runners

	if hostFilter != "" {
		var filtered []RunnerConfig
		for _, r := range runners {
			if r.Host == hostFilter {
				filtered = append(filtered, r)
			}
		}
		runners = filtered
	}

	if repoFilter != "" {
		var filtered []RunnerConfig
		for _, r := range runners {
			if r.Repo == repoFilter {
				filtered = append(filtered, r)
			}
		}
		runners = filtered
	}

	if len(nameArgs) > 0 {
		nameSet := map[string]bool{}
		for _, a := range nameArgs {
			nameSet[a] = true
		}
		var filtered []RunnerConfig
		for _, r := range runners {
			if nameSet[r.Name] {
				filtered = append(filtered, r)
				continue
			}
			for _, inst := range r.InstanceNames() {
				if nameSet[inst] {
					filtered = append(filtered, r)
					break
				}
			}
		}
		runners = filtered
	}

	return runners
}

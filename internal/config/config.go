package config

import (
	"fmt"
	"os"
	"regexp"
	"runtime"
	"strings"

	"gopkg.in/yaml.v3"
)

const LocalAddr = "local"

func IsLocalAddr(addr string) bool {
	return strings.EqualFold(strings.TrimSpace(addr), LocalAddr)
}

type Config struct {
	GitHub               GitHubConfig               `yaml:"github"`
	Hosts                map[string]HostConfig      `yaml:"hosts"`
	Runners              []RunnerConfig             `yaml:"runners"`
	ContainerRunnerImage ContainerRunnerImageConfig `yaml:"container_runner_image,omitempty"`
}

// ContainerRunnerImageConfig controls optional customization of the locally built
// gh-sr/agentic-runner Docker image (runner_mode: container).
type ContainerRunnerImageConfig struct {
	// ExtraAptPackages lists additional Debian package names to install in the
	// image at build time (Ubuntu main archive only in v1).
	ExtraAptPackages []string `yaml:"extra_apt_packages,omitempty"`
}

const (
	maxContainerRunnerExtraAptPackages = 256
	maxContainerRunnerAptPkgNameLen    = 200
)

var debianPackageNamePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9+.-]*$`)

func validateContainerRunnerImage(img *ContainerRunnerImageConfig) error {
	if img == nil || len(img.ExtraAptPackages) == 0 {
		return nil
	}
	if len(img.ExtraAptPackages) > maxContainerRunnerExtraAptPackages {
		return fmt.Errorf("container_runner_image.extra_apt_packages: at most %d entries allowed (got %d)",
			maxContainerRunnerExtraAptPackages, len(img.ExtraAptPackages))
	}
	for i, raw := range img.ExtraAptPackages {
		p := strings.TrimSpace(raw)
		if p == "" {
			return fmt.Errorf("container_runner_image.extra_apt_packages[%d]: empty package name", i)
		}
		if len(p) > maxContainerRunnerAptPkgNameLen {
			return fmt.Errorf("container_runner_image.extra_apt_packages[%d]: package name too long (max %d characters)",
				i, maxContainerRunnerAptPkgNameLen)
		}
		if !debianPackageNamePattern.MatchString(p) {
			return fmt.Errorf("container_runner_image.extra_apt_packages[%d]: invalid package name %q (use lowercase Debian package tokens: [a-z0-9+.-])", i, p)
		}
	}
	return nil
}

// ContainerRunnerImageExtraAptPackages returns a copy of extra apt package names
// for the container runner image build.
func (c *Config) ContainerRunnerImageExtraAptPackages() []string {
	if c == nil {
		return nil
	}
	out := make([]string, len(c.ContainerRunnerImage.ExtraAptPackages))
	copy(out, c.ContainerRunnerImage.ExtraAptPackages)
	return out
}

type GitHubConfig struct {
	PAT string `yaml:"pat"`
}

type HostConfig struct {
	Addr      string `yaml:"addr"`
	OS        string `yaml:"os"`
	Arch      string `yaml:"arch"`
	WindowsPS string `yaml:"windows_ps"` // powershell (default) or pwsh — which exe runs encoded remote scripts on Windows
}

// AgenticMCPLabelPrefix is prepended to the TCP port in runner labels (e.g. gh-sr-mcp-9080).
const AgenticMCPLabelPrefix = "gh-sr-mcp-"

// RunnerModeNative runs the actions runner process directly on the host OS.
const RunnerModeNative = "native"

// RunnerModeContainer runs each runner instance inside its own privileged Docker
// container (DinD), providing full filesystem and network isolation between
// concurrent jobs on the same host. Used for agentic workflows (isolated /tmp/gh-aw,
// MCP on port 80) and for any self-hosted runner that wants container isolation.
const RunnerModeContainer = "container"

type RunnerConfig struct {
	Name       string   `yaml:"name"`
	Repo       string   `yaml:"repo"`
	Org        string   `yaml:"org"`
	Group      string   `yaml:"group"`
	Host       string   `yaml:"host"`
	Count      int      `yaml:"count"`
	Labels     []string `yaml:"labels"`
	Ephemeral  bool     `yaml:"ephemeral"`
	Profile    string   `yaml:"profile"`     // "agentic" for GitHub Agentic Workflows
	RunnerMode string   `yaml:"runner_mode"` // "native" (default) or "container"
	// AgenticMCPPorts assigns one MCP gateway port per runner instance (length must equal count).
	// Mutually exclusive with AgenticMCPPortBase.
	AgenticMCPPorts []int `yaml:"agentic_mcp_ports,omitempty"`
	// AgenticMCPPortBase assigns ports base, base+1, … for instances 0..count-1 (mutually exclusive with AgenticMCPPorts).
	AgenticMCPPortBase *int `yaml:"agentic_mcp_port_base,omitempty"`
}

// IsAgentic returns true if the runner uses the agentic profile.
func (rc *RunnerConfig) IsAgentic() bool {
	return rc.Profile == "agentic"
}

// IsContainerMode returns true if the runner uses container-isolated DinD mode
// (same image for agentic and non-agentic runners).
func (rc *RunnerConfig) IsContainerMode() bool {
	return rc.RunnerMode == RunnerModeContainer
}

// EffectiveRunnerMode returns the resolved runner mode (defaults to "native").
func (rc *RunnerConfig) EffectiveRunnerMode() string {
	if rc.RunnerMode == RunnerModeContainer {
		return RunnerModeContainer
	}
	return RunnerModeNative
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

// EffectiveMode always returns "native" (docker mode has been removed).
func (rc *RunnerConfig) EffectiveMode(_ string) string {
	return "native"
}

// DefaultLabels generates standard GitHub Actions labels based on host OS and arch.
func DefaultLabels(hostOS, arch string) []string {
	labels := []string{"self-hosted"}

	osLabel := ""
	switch hostOS {
	case "linux":
		osLabel = "Linux"
	case "darwin":
		osLabel = "macOS"
	case "windows":
		osLabel = "Windows"
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

// InstanceCount returns the number of runner instances (at least 1).
func (rc *RunnerConfig) InstanceCount() int {
	c := rc.Count
	if c < 1 {
		return 1
	}
	return c
}

// AgenticMCPPortsResolved returns one TCP port per instance when agentic MCP port strategy is configured.
func (rc *RunnerConfig) AgenticMCPPortsResolved() ([]int, bool) {
	if !rc.IsAgentic() {
		return nil, false
	}
	n := rc.InstanceCount()
	if len(rc.AgenticMCPPorts) > 0 {
		return rc.AgenticMCPPorts, true
	}
	if rc.AgenticMCPPortBase != nil {
		base := *rc.AgenticMCPPortBase
		if base < 1 {
			return nil, false
		}
		ports := make([]int, n)
		for i := range n {
			ports[i] = base + i
		}
		return ports, true
	}
	return nil, false
}

func (rc *RunnerConfig) effectiveLabelsCore(hostOS, arch string) []string {
	var labels []string
	if len(rc.Labels) > 0 {
		labels = append([]string(nil), rc.Labels...)
	} else {
		labels = DefaultLabels(hostOS, arch)
	}
	if rc.IsAgentic() && !hasLabel(labels, "agentic") {
		labels = append(labels, "agentic")
	}
	return labels
}

// EffectiveLabels returns labels for the first instance (index 0). For identical labels across
// instances, this matches every instance; when agentic_mcp_ports / agentic_mcp_port_base is set,
// use EffectiveLabelsForInstance for each registration.
func (rc *RunnerConfig) EffectiveLabels(hostOS, arch string) []string {
	return rc.EffectiveLabelsForInstance(hostOS, arch, 0)
}

// EffectiveLabelsForInstance returns the runner's labels for instance instanceIndex (0-based).
// When agentic MCP ports are configured, appends gh-sr-mcp-<port> for that instance.
func (rc *RunnerConfig) EffectiveLabelsForInstance(hostOS, arch string, instanceIndex int) []string {
	labels := rc.effectiveLabelsCore(hostOS, arch)
	ports, ok := rc.AgenticMCPPortsResolved()
	if !ok || len(ports) == 0 {
		return labels
	}
	if instanceIndex < 0 || instanceIndex >= len(ports) {
		return labels
	}
	tag := fmt.Sprintf("%s%d", AgenticMCPLabelPrefix, ports[instanceIndex])
	if !hasLabel(labels, tag) {
		labels = append(labels, tag)
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

func (rc *RunnerConfig) applyAgenticDefaults() {
	if !rc.IsAgentic() {
		return
	}
	// Agentic runners get "agentic" label auto-added if not present.
	if !hasLabel(rc.Labels, "agentic") {
		rc.Labels = append(rc.Labels, "agentic")
	}
}

func (c *Config) Validate() error {
	if c.GitHub.PAT != "" {
		return fmt.Errorf("github.pat is no longer supported; remove it from runners.yml and authenticate with `gh auth login`")
	}

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
	}

	if len(c.Runners) == 0 {
		return fmt.Errorf("at least one runner must be defined")
	}

	instanceOwners := make(map[string]string)
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
		if _, ok := c.Hosts[r.Host]; !ok {
			return fmt.Errorf("runner %q: host %q not found in hosts", r.Name, r.Host)
		}
		if r.Profile != "" && r.Profile != "agentic" {
			return fmt.Errorf("runner %q: profile must be empty or \"agentic\" (got %q)", r.Name, r.Profile)
		}
		if r.RunnerMode != "" && r.RunnerMode != RunnerModeNative && r.RunnerMode != RunnerModeContainer {
			return fmt.Errorf("runner %q: runner_mode must be %q or %q (got %q)", r.Name, RunnerModeNative, RunnerModeContainer, r.RunnerMode)
		}
		if r.IsContainerMode() && (len(r.AgenticMCPPorts) > 0 || r.AgenticMCPPortBase != nil) {
			return fmt.Errorf("runner %q: agentic_mcp_ports / agentic_mcp_port_base are not needed with runner_mode: container (each container has its own isolated port 80)", r.Name)
		}
		if err := validateAgenticMCPPorts(&r); err != nil {
			return err
		}
		for _, instance := range r.InstanceNames() {
			key := r.Host + "\x00" + instance
			if prev, ok := instanceOwners[key]; ok {
				return fmt.Errorf("runner instance %q is defined more than once on host %q (runners %q and %q); runner names must be unique per host", instance, r.Host, prev, r.Name)
			}
			instanceOwners[key] = r.Name
		}
	}

	if err := validateContainerRunnerImage(&c.ContainerRunnerImage); err != nil {
		return err
	}

	return nil
}

func validateAgenticMCPPorts(r *RunnerConfig) error {
	hasPorts := len(r.AgenticMCPPorts) > 0
	hasBase := r.AgenticMCPPortBase != nil
	if !hasPorts && !hasBase {
		return nil
	}
	if !r.IsAgentic() {
		return fmt.Errorf("runner %q: agentic_mcp_ports / agentic_mcp_port_base require profile: agentic", r.Name)
	}
	if hasPorts && hasBase {
		return fmt.Errorf("runner %q: set only one of agentic_mcp_ports or agentic_mcp_port_base", r.Name)
	}
	n := r.InstanceCount()
	if hasPorts {
		if len(r.AgenticMCPPorts) != n {
			return fmt.Errorf("runner %q: agentic_mcp_ports must have length %d (count), got %d", r.Name, n, len(r.AgenticMCPPorts))
		}
		seen := make(map[int]bool, n)
		for _, p := range r.AgenticMCPPorts {
			if p < 1 || p > 65535 {
				return fmt.Errorf("runner %q: agentic_mcp_ports values must be between 1 and 65535 (got %d)", r.Name, p)
			}
			if seen[p] {
				return fmt.Errorf("runner %q: agentic_mcp_ports must be unique (duplicate %d)", r.Name, p)
			}
			seen[p] = true
		}
		return nil
	}
	base := *r.AgenticMCPPortBase
	if base < 1 || base > 65535 {
		return fmt.Errorf("runner %q: agentic_mcp_port_base must be between 1 and 65535 (got %d)", r.Name, base)
	}
	if base+n-1 > 65535 {
		return fmt.Errorf("runner %q: agentic_mcp_port_base %d with count %d exceeds port 65535", r.Name, base, n)
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
// All active filters are evaluated in a single pass to avoid intermediate allocations.
func FilterRunners(cfg *Config, hostFilter, repoFilter string, nameArgs []string) []RunnerConfig {
	if hostFilter == "" && repoFilter == "" && len(nameArgs) == 0 {
		return cfg.Runners
	}

	var nameSet map[string]bool
	if len(nameArgs) > 0 {
		nameSet = make(map[string]bool, len(nameArgs))
		for _, a := range nameArgs {
			nameSet[a] = true
		}
	}

	var filtered []RunnerConfig
	for _, r := range cfg.Runners {
		if hostFilter != "" && r.Host != hostFilter {
			continue
		}
		if repoFilter != "" && r.Repo != repoFilter {
			continue
		}
		if nameSet != nil {
			if nameSet[r.Name] {
				filtered = append(filtered, r)
				continue
			}
			matched := false
			for _, inst := range r.InstanceNames() {
				if nameSet[inst] {
					matched = true
					break
				}
			}
			if !matched {
				continue
			}
		}
		filtered = append(filtered, r)
	}
	return filtered
}

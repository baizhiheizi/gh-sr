package config

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

type Config struct {
	GitHub  GitHubConfig          `yaml:"github"`
	Hosts   map[string]HostConfig `yaml:"hosts"`
	Runners []RunnerConfig        `yaml:"runners"`
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

type RunnerConfig struct {
	Name   string   `yaml:"name"`
	Repo   string   `yaml:"repo"`
	Host   string   `yaml:"host"`
	Count  int      `yaml:"count"`
	Labels []string `yaml:"labels"`
	Mode   string   `yaml:"mode"`
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

func (c *Config) applyDefaults() {
	for i := range c.Runners {
		if c.Runners[i].Count < 1 {
			c.Runners[i].Count = 1
		}
	}
}

func (c *Config) Validate() error {
	if c.GitHub.PAT == "" {
		return fmt.Errorf("github.pat is required (use 'env:VAR_NAME' to read from environment)")
	}

	if len(c.Hosts) == 0 {
		return fmt.Errorf("at least one host must be defined")
	}

	for name, h := range c.Hosts {
		if h.Addr == "" {
			return fmt.Errorf("host %q: addr is required", name)
		}
		switch h.OS {
		case "linux", "darwin", "windows":
		default:
			return fmt.Errorf("host %q: os must be linux, darwin, or windows (got %q)", name, h.OS)
		}
		switch h.Arch {
		case "amd64", "arm64":
		default:
			return fmt.Errorf("host %q: arch must be amd64 or arm64 (got %q)", name, h.Arch)
		}
		if h.WindowsPS != "" {
			if h.OS != "windows" {
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

	for _, r := range c.Runners {
		if r.Name == "" {
			return fmt.Errorf("runner name is required")
		}
		if r.Repo == "" {
			return fmt.Errorf("runner %q: repo is required", r.Name)
		}
		if r.Host == "" {
			return fmt.Errorf("runner %q: host is required", r.Name)
		}
		if _, ok := c.Hosts[r.Host]; !ok {
			return fmt.Errorf("runner %q: host %q not found in hosts", r.Name, r.Host)
		}
		if r.Mode != "" && r.Mode != "docker" && r.Mode != "native" {
			return fmt.Errorf("runner %q: mode must be 'docker' or 'native' (got %q)", r.Name, r.Mode)
		}
	}

	return nil
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
		if !seen[r.Repo] {
			seen[r.Repo] = true
			repos = append(repos, r.Repo)
		}
	}
	return repos
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

package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// makeRunnerCfg builds a Config with n runners spread across numHosts hosts.
func makeRunnerCfg(n, numHosts int) *Config {
	hosts := make(map[string]HostConfig, numHosts)
	hostNames := make([]string, numHosts)
	for i := 0; i < numHosts; i++ {
		name := string(rune('a'+i%26)) + "host"
		hostNames[i] = name
		hosts[name] = HostConfig{Addr: "192.168.1." + string(rune('0'+i%10)), OS: "linux", Arch: "amd64"}
	}
	runners := make([]RunnerConfig, n)
	for i := 0; i < n; i++ {
		runners[i] = RunnerConfig{
			Name:  "runner-" + string(rune('a'+i%26)),
			Repo:  "org/repo-" + string(rune('0'+i%10)),
			Host:  hostNames[i%numHosts],
			Count: 2,
		}
	}
	return &Config{Hosts: hosts, Runners: runners}
}

func BenchmarkFilterRunners_NoFilter(b *testing.B) {
	cfg := makeRunnerCfg(100, 10)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		FilterRunners(cfg, "", "", nil)
	}
}

func BenchmarkFilterRunners_ByHost(b *testing.B) {
	cfg := makeRunnerCfg(100, 10)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		FilterRunners(cfg, "ahost", "", nil)
	}
}

func BenchmarkFilterRunners_ByName(b *testing.B) {
	cfg := makeRunnerCfg(100, 10)
	// name-based filter exercises InstanceNames per runner
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		FilterRunners(cfg, "", "", []string{"runner-a-2"})
	}
}

func BenchmarkFilterRunners_AllFilters(b *testing.B) {
	cfg := makeRunnerCfg(100, 10)
	// all three filters active: single-pass is most beneficial here
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		FilterRunners(cfg, "ahost", "org/repo-0", []string{"runner-a-2"})
	}
}

func BenchmarkInstanceNames(b *testing.B) {
	rc := RunnerConfig{Name: "my-runner", Count: 10}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rc.InstanceNames()
	}
}

func BenchmarkDefaultLabels(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		DefaultLabels("linux", "amd64")
	}
}

func BenchmarkEffectiveLabels_Generated(b *testing.B) {
	rc := RunnerConfig{}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rc.EffectiveLabels("linux", "amd64")
	}
}

func BenchmarkEffectiveLabels_Explicit(b *testing.B) {
	rc := RunnerConfig{Labels: []string{"self-hosted", "Linux", "X64", "fast"}}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rc.EffectiveLabels("linux", "amd64")
	}
}

// makeConfigYAML generates a minimal valid YAML config with n runners across numHosts hosts.
func makeConfigYAML(n, numHosts int) string {
	var sb strings.Builder
	sb.WriteString("hosts:\n")
	for i := 0; i < numHosts; i++ {
		name := fmt.Sprintf("host%02d", i)
		sb.WriteString(fmt.Sprintf("  %s:\n    addr: local\n    os: linux\n    arch: amd64\n", name))
	}
	sb.WriteString("runners:\n")
	for i := 0; i < n; i++ {
		host := fmt.Sprintf("host%02d", i%numHosts)
		sb.WriteString(fmt.Sprintf("  - name: runner-%02d\n    repo: org/repo-%02d\n    host: %s\n    count: 2\n", i, i%10, host))
	}
	return sb.String()
}

// BenchmarkLoad measures end-to-end config loading (file read + YAML parse + validate).
// Config loading is on the hot path — it's called for every CLI invocation.
func BenchmarkLoad_Small(b *testing.B) {
	dir := b.TempDir()
	path := filepath.Join(dir, "runners.yml")
	if err := os.WriteFile(path, []byte(makeConfigYAML(5, 2)), 0o600); err != nil {
		b.Fatal(err)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := Load(path); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkLoad_Large(b *testing.B) {
	dir := b.TempDir()
	path := filepath.Join(dir, "runners.yml")
	if err := os.WriteFile(path, []byte(makeConfigYAML(50, 10)), 0o600); err != nil {
		b.Fatal(err)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := Load(path); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkValidate measures just the validation pass, isolating it from YAML parsing.
func BenchmarkValidate_Small(b *testing.B) {
	cfg := makeRunnerCfg(5, 2)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = cfg.Validate()
	}
}

func BenchmarkValidate_Large(b *testing.B) {
	cfg := makeRunnerCfg(100, 10)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = cfg.Validate()
	}
}

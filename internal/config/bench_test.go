package config

import (
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

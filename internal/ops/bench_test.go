package ops

import (
	"testing"

	"github.com/an-lee/gh-sr/internal/config"
)

func makeCfgWithHosts(n int) *config.Config {
	hosts := make(map[string]config.HostConfig, n)
	for i := 0; i < n; i++ {
		// spread across a–z names for realistic sort cost
		name := string([]byte{byte('a' + i%26), byte('a' + (i/26)%26)}) + "-host"
		hosts[name] = config.HostConfig{}
	}
	return &config.Config{Hosts: hosts}
}

func BenchmarkSortedHostNames_10(b *testing.B) {
	cfg := makeCfgWithHosts(10)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sortedHostNames(cfg, "")
	}
}

func BenchmarkSortedHostNames_100(b *testing.B) {
	cfg := makeCfgWithHosts(100)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sortedHostNames(cfg, "")
	}
}

func BenchmarkSortedHostNames_WithFilter(b *testing.B) {
	cfg := makeCfgWithHosts(100)
	// pick a host that exists
	var target string
	for k := range cfg.Hosts {
		target = k
		break
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sortedHostNames(cfg, target)
	}
}

package runner

import (
	"strconv"
	"testing"

	"github.com/an-lee/gh-sr/internal/config"
)

// makeEnrichCfg builds a synthetic config + scopeRunners fixture for benchmarking
// EnrichWithGitHubStatus without hitting the GitHub API. The fixture mirrors the
// production pattern of one scope per repo with a few runners per scope.
func makeEnrichCfg(numRepos, countPerRepo int) (*config.Config, map[scopeKey][]GitHubRunner) {
	cfg := &config.Config{
		GitHub: config.GitHubConfig{},
		Hosts: map[string]config.HostConfig{
			"h": {Addr: "a@b", OS: "linux", Arch: "amd64"},
		},
	}
	scopeRunners := make(map[scopeKey][]GitHubRunner)
	statuses := make([]RunnerStatus, 0, numRepos*countPerRepo)
	for i := 0; i < numRepos; i++ {
		name := "runner-" + strconv.Itoa(i)
		repo := "o/r" + strconv.Itoa(i)
		cfg.Runners = append(cfg.Runners, config.RunnerConfig{
			Name:  name,
			Repo:  repo,
			Host:  "h",
			Count: countPerRepo,
		})
		key := scopeKey{"repo", repo}
		gh := make([]GitHubRunner, countPerRepo)
		for j := 1; j <= countPerRepo; j++ {
			inst := name + "-" + strconv.Itoa(j)
			gh[j-1] = GitHubRunner{Name: inst, Status: "online", OS: "Linux"}
			statuses = append(statuses, RunnerStatus{
				Instance: inst,
				Host:     "h",
				Repo:     repo,
				Mode:     "docker",
			})
		}
		scopeRunners[key] = gh
	}
	return cfg, scopeRunners
}

// BenchmarkEnrichFromScopeRunners measures the inner work of EnrichWithGitHubStatus
// (rcByInstance build + status-to-GitHub-runner matching) without the GitHub API
// round trip. This is the alloc hotspot targeted by the inline-rc-by-instance
// refactor: the per-TUI-refresh path runs this once every 5 seconds while the
// dashboard is open.
func BenchmarkEnrichFromScopeRunners(b *testing.B) {
	cfg, scopeRunners := makeEnrichCfg(20, 10) // 20 repos × 10 instances = 200 statuses
	m := &Manager{}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Copy statuses so the in-place mutation doesn't compound across iterations.
		statuses := make([]RunnerStatus, 0, 200)
		for _, s := range makeEnrichStatuses(cfg) {
			statuses = append(statuses, s)
		}
		m.enrichFromScopeRunners(statuses, cfg, scopeRunners)
	}
}

// BenchmarkEnrichFromScopeRunners_Small: 5 repos × 2 instances. Mirrors a small
// personal config where the per-refresh alloc cost is most noticeable.
func BenchmarkEnrichFromScopeRunners_Small(b *testing.B) {
	cfg, scopeRunners := makeEnrichCfg(5, 2)
	m := &Manager{}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		statuses := make([]RunnerStatus, 0, 10)
		for _, s := range makeEnrichStatuses(cfg) {
			statuses = append(statuses, s)
		}
		m.enrichFromScopeRunners(statuses, cfg, scopeRunners)
	}
}

// makeEnrichStatuses builds a fresh []RunnerStatus slice mirroring the entries
// produced by makeEnrichCfg. Kept separate from makeEnrichCfg so the benchmark
// can copy statuses per iteration without re-running the full fixture build.
func makeEnrichStatuses(cfg *config.Config) []RunnerStatus {
	var out []RunnerStatus
	for _, rc := range cfg.Runners {
		repoDisplay := rc.Repo
		if rc.Org != "" {
			repoDisplay = "org:" + rc.Org
		}
		count := rc.Count
		if count < 1 {
			count = 1
		}
		for j := 1; j <= count; j++ {
			out = append(out, RunnerStatus{
				Instance: rc.Name + "-" + strconv.Itoa(j),
				Host:     rc.Host,
				Repo:     repoDisplay,
				Mode:     "docker",
			})
		}
	}
	return out
}

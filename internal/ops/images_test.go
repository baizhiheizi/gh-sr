package ops

import (
	"strings"
	"testing"

	"github.com/an-lee/gh-sr/internal/config"
)

// TestContainerRunnerContainers pins the per-host runner-container
// enumeration used by the image-sync fan-out: container-mode instances only,
// deterministic order.
func TestContainerRunnerContainers(t *testing.T) {
	t.Parallel()
	cfg := &config.Config{Runners: []config.RunnerConfig{
		{Name: "native-1", Repo: "o/r", Host: "h1"},
		{Name: "ci", Repo: "o/r", Host: "h1", Count: 2, RunnerMode: config.RunnerModeContainer},
		{Name: "aw", Repo: "o/r", Host: "h1", Profile: "agentic"},
		{Name: "other-host", Repo: "o/r", Host: "h2", RunnerMode: config.RunnerModeContainer},
	}}
	got := containerRunnerContainers(cfg, "h1")
	want := []string{"gh-sr-aw-1", "gh-sr-ci-1", "gh-sr-ci-2"}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v, want %v", got, want)
		}
	}
	if got := containerRunnerContainers(cfg, "h2"); len(got) != 1 || got[0] != "gh-sr-other-host-1" {
		t.Errorf("h2: got %v", got)
	}
}

// TestImagesSyncRequiresPrefetch pins the empty-prefetch contract.
func TestImagesSyncRequiresPrefetch(t *testing.T) {
	t.Parallel()
	cfg := &config.Config{Hosts: map[string]config.HostConfig{
		"h1": {Addr: "local", OS: "linux"},
	}}
	err := ImagesSync(&strings.Builder{}, cfg, "")
	if err == nil || !strings.Contains(err.Error(), "prefetch") {
		t.Fatalf("expected prefetch error, got %v", err)
	}
}

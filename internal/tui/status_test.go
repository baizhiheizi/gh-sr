package tui

import (
	"strings"
	"testing"

	"github.com/an-lee/ghr/internal/config"
	"github.com/an-lee/ghr/internal/runner"
)

func Test_formatGitHubStatus(t *testing.T) {
	t.Parallel()
	if got := formatGitHubStatus(runner.RunnerStatus{}); got != "-" {
		t.Errorf("empty remote: got %q", got)
	}
	if got := formatGitHubStatus(runner.RunnerStatus{Remote: "online", Busy: false}); got != "online" {
		t.Errorf("online: got %q", got)
	}
	if got := formatGitHubStatus(runner.RunnerStatus{Remote: "online", Busy: true}); got != "busy" {
		t.Errorf("busy: got %q", got)
	}
}

func TestFormatConfig_containsHostsAndRunners(t *testing.T) {
	t.Parallel()
	cfg := &config.Config{
		GitHub: config.GitHubConfig{PAT: "github_pat_abcdefghijklmnop"},
		Hosts: map[string]config.HostConfig{
			"h1": {Addr: "local", OS: "linux", Arch: "amd64"},
		},
		Runners: []config.RunnerConfig{
			{Name: "r1", Repo: "o/r", Host: "h1", Count: 1, Labels: []string{"self-hosted"}},
		},
	}
	out := FormatConfig(cfg)
	if !strings.Contains(out, "h1") || !strings.Contains(out, "r1") || !strings.Contains(out, "o/r") {
		t.Fatalf("unexpected FormatConfig output:\n%s", out)
	}
	if strings.Contains(out, "github_pat_abcdefghijklmnop") {
		t.Fatal("PAT should be redacted in FormatConfig")
	}
}

func TestClampCursor(t *testing.T) {
	t.Parallel()
	if got := clampCursor(3, 2); got != 1 {
		t.Fatalf("clamp 3 with n=2: got %d", got)
	}
	if got := clampCursor(0, 0); got != 0 {
		t.Fatalf("empty: got %d", got)
	}
}

func TestNonTTYHint_nonempty(t *testing.T) {
	t.Parallel()
	if NonTTYHint == "" {
		t.Fatal("NonTTYHint empty")
	}
}

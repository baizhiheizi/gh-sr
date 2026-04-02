package tui

import (
	"testing"

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

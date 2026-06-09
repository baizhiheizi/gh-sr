package ops

import (
	"errors"
	"strings"
	"testing"

	"github.com/an-lee/gh-sr/internal/config"
	"github.com/an-lee/gh-sr/internal/runner"
)

func TestDiskStatusMapsFrom_scopedByHost(t *testing.T) {
	t.Parallel()
	statuses := []runner.RunnerStatus{
		{Host: "h1", Instance: "r1", Busy: true, Remote: "online"},
		{Host: "h2", Instance: "r1", Busy: false, Remote: "online"},
	}
	m := diskStatusMapsFrom(statuses)
	if !m.busy[diskHostInstanceKey("h1", "r1")] {
		t.Fatal("expected h1/r1 busy")
	}
	if m.busy[diskHostInstanceKey("h2", "r1")] {
		t.Fatal("expected h2/r1 not busy")
	}
	if !m.githubKnown[diskHostInstanceKey("h1", "r1")] {
		t.Fatal("expected github known for h1/r1")
	}
}

func TestPruneResultsError(t *testing.T) {
	t.Parallel()
	if err := pruneResultsError(nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	err := pruneResultsError([]runner.PruneResult{
		{Instance: "ci-1", Host: "linux", Err: errors.New("clear failed")},
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "ci-1 on linux") {
		t.Fatalf("got %v", err)
	}
}

func TestConfiguredInstancesOnHost(t *testing.T) {
	t.Parallel()
	runners := []config.RunnerConfig{
		{Name: "ci", Host: "h1", Count: 2},
	}
	got := configuredInstancesOnHost(runners)
	if len(got) != 2 {
		t.Fatalf("got %d instances", len(got))
	}
	if _, ok := got["ci-1"]; !ok {
		t.Fatal("missing ci-1")
	}
	if _, ok := got["ci-2"]; !ok {
		t.Fatal("missing ci-2")
	}
}

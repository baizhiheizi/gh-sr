package runner

import (
	"errors"
	"strings"
	"testing"

	"github.com/an-lee/gh-sr/internal/config"
	"github.com/an-lee/gh-sr/internal/host"
	"github.com/an-lee/gh-sr/internal/testutil"
)

// TestHostInventoryFor pins the one inventory contract all three consumers
// (doctor disk rows, ops CollectDiskUsage, ops PruneDisk) now share: the
// configured instance set for one host merged with the on-disk directories,
// sorted, with orphans carrying a nil config and a failed listing surfaced
// as ListErr instead of silently dropped.
func TestHostInventoryFor(t *testing.T) {
	t.Parallel()
	lsDirs := func(out string, err error) *testutil.MockExecutor {
		return &testutil.MockExecutor{RunFn: func(cmd string) (string, error) {
			if strings.Contains(cmd, `ls -1 "$HOME/.gh-sr/runners"`) {
				return out, err
			}
			return "", nil
		}}
	}
	runners := []config.RunnerConfig{
		{Name: "ci", Host: "h1", Count: 2},
		{Name: "build", Host: "h1", Count: 1},
		{Name: "elsewhere", Host: "h2", Count: 2},
	}

	t.Run("merges configured and on-disk instances, sorted, orphans nil", func(t *testing.T) {
		t.Parallel()
		h := host.NewHost("h1", config.HostConfig{OS: "linux"})
		h.SetConn(lsDirs("ci-2\nzz-orphan\nbuild-1\nzz-orphan\n", nil))
		inv := HostInventoryFor(h, runners, "h1", true)
		if inv.ListErr != nil {
			t.Fatalf("unexpected ListErr: %v", inv.ListErr)
		}
		wantNames := []string{"build-1", "ci-1", "ci-2", "zz-orphan"}
		if got := inv.Names(); strings.Join(got, ",") != strings.Join(wantNames, ",") {
			t.Errorf("Names = %v, want %v (sorted, deduped)", got, wantNames)
		}
		cfg := inv.ConfigByInstance()
		if len(cfg) != 3 {
			t.Fatalf("ConfigByInstance = %d entries, want 3 (orphan absent)", len(cfg))
		}
		if cfg["ci-1"].Name != "ci" || cfg["build-1"].Name != "build" {
			t.Errorf("configs miswired: %+v", cfg)
		}
		for _, in := range inv.Instances {
			if in.Name == "zz-orphan" && in.Config != nil {
				t.Error("orphan must carry nil config")
			}
		}
	})

	t.Run("host filter drops other hosts", func(t *testing.T) {
		t.Parallel()
		h := host.NewHost("h1", config.HostConfig{OS: "linux"})
		h.SetConn(lsDirs("", nil))
		inv := HostInventoryFor(h, runners, "h1", false)
		for _, in := range inv.Instances {
			if strings.HasPrefix(in.Name, "elsewhere") {
				t.Errorf("instance %s belongs to h2", in.Name)
			}
		}
		if len(inv.Instances) != 3 {
			t.Errorf("got %d instances, want 3", len(inv.Instances))
		}
	})

	t.Run("listing error keeps the configured half and surfaces ListErr", func(t *testing.T) {
		t.Parallel()
		h := host.NewHost("h1", config.HostConfig{OS: "linux"})
		h.SetConn(lsDirs("", errors.New("ssh down")))
		inv := HostInventoryFor(h, runners, "h1", true)
		if inv.ListErr == nil {
			t.Fatal("ListErr must be surfaced, not swallowed")
		}
		got := inv.Names()
		if len(got) != 3 || got[0] != "build-1" || got[1] != "ci-1" || got[2] != "ci-2" {
			t.Errorf("configured half = %v, want [build-1 ci-1 ci-2] sorted", got)
		}
	})

	t.Run("includeOnDisk=false skips the SSH listing", func(t *testing.T) {
		t.Parallel()
		calls := 0
		mock := &testutil.MockExecutor{RunFn: func(cmd string) (string, error) {
			calls++
			return "", nil
		}}
		h := host.NewHost("h1", config.HostConfig{OS: "linux"})
		h.SetConn(mock)
		inv := HostInventoryFor(h, runners, "h1", false)
		if calls != 0 {
			t.Errorf("listing ran %d times, want 0 (prune-without-orphans must not pay the round trip)", calls)
		}
		if len(inv.Instances) != 3 {
			t.Errorf("got %d configured instances, want 3", len(inv.Instances))
		}
	})
}

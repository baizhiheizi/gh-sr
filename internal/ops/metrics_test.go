package ops

import (
	"testing"

	"github.com/an-lee/gh-sr/internal/config"
)

func TestSortedHostNames(t *testing.T) {
	t.Parallel()

	makeConfig := func(hosts ...string) *config.Config {
		cfg := &config.Config{Hosts: make(map[string]config.HostConfig)}
		for _, h := range hosts {
			cfg.Hosts[h] = config.HostConfig{}
		}
		return cfg
	}

	t.Run("no filter returns all hosts sorted", func(t *testing.T) {
		t.Parallel()
		cfg := makeConfig("zebra", "alpha", "mango")
		got := sortedHostNames(cfg, "")
		want := []string{"alpha", "mango", "zebra"}
		if len(got) != len(want) {
			t.Fatalf("len = %d; want %d", len(got), len(want))
		}
		for i := range want {
			if got[i] != want[i] {
				t.Errorf("[%d] got %q; want %q", i, got[i], want[i])
			}
		}
	})

	t.Run("empty config returns empty slice", func(t *testing.T) {
		t.Parallel()
		got := sortedHostNames(makeConfig(), "")
		if len(got) != 0 {
			t.Fatalf("expected empty slice, got %v", got)
		}
	})

	t.Run("filter on existing host returns single entry", func(t *testing.T) {
		t.Parallel()
		cfg := makeConfig("a", "b", "c")
		got := sortedHostNames(cfg, "b")
		if len(got) != 1 || got[0] != "b" {
			t.Fatalf("got %v; want [b]", got)
		}
	})

	t.Run("filter on missing host returns nil", func(t *testing.T) {
		t.Parallel()
		cfg := makeConfig("a", "b")
		got := sortedHostNames(cfg, "x")
		if got != nil {
			t.Fatalf("got %v; want nil", got)
		}
	})

	t.Run("single host no filter", func(t *testing.T) {
		t.Parallel()
		cfg := makeConfig("only")
		got := sortedHostNames(cfg, "")
		if len(got) != 1 || got[0] != "only" {
			t.Fatalf("got %v; want [only]", got)
		}
	})
}

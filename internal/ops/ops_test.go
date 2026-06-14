package ops

import (
	"bytes"
	"sync"
	"testing"

	"github.com/an-lee/gh-sr/internal/config"
	"github.com/an-lee/gh-sr/internal/runner"
)

func TestGroupRunnersByHost(t *testing.T) {
	t.Parallel()

	t.Run("empty input returns nil", func(t *testing.T) {
		t.Parallel()
		groups := groupRunnersByHost(nil)
		if len(groups) != 0 {
			t.Errorf("len(groups) = %d, want 0", len(groups))
		}
	})

	t.Run("single host preserves runner order", func(t *testing.T) {
		t.Parallel()
		runners := []config.RunnerConfig{
			{Name: "r1", Host: "h1"},
			{Name: "r2", Host: "h1"},
			{Name: "r3", Host: "h1"},
		}
		groups := groupRunnersByHost(runners)
		if len(groups) != 1 {
			t.Fatalf("got %d groups, want 1", len(groups))
		}
		if groups[0].name != "h1" {
			t.Errorf("group[0].name = %q, want %q", groups[0].name, "h1")
		}
		got := []string{
			groups[0].runners[0].Name,
			groups[0].runners[1].Name,
			groups[0].runners[2].Name,
		}
		want := []string{"r1", "r2", "r3"}
		for i := range want {
			if got[i] != want[i] {
				t.Errorf("runners[%d] = %q, want %q", i, got[i], want[i])
			}
		}
	})

	t.Run("multiple hosts preserve discovery order", func(t *testing.T) {
		t.Parallel()
		// Interleaved hosts: verify that the first occurrence of each host
		// determines the group's position, not later ones.
		runners := []config.RunnerConfig{
			{Name: "r1", Host: "h1"},
			{Name: "r2", Host: "h2"},
			{Name: "r3", Host: "h1"},
			{Name: "r4", Host: "h3"},
			{Name: "r5", Host: "h2"},
		}
		groups := groupRunnersByHost(runners)
		if len(groups) != 3 {
			t.Fatalf("got %d groups, want 3", len(groups))
		}
		wantOrder := []string{"h1", "h2", "h3"}
		for i, g := range groups {
			if g.name != wantOrder[i] {
				t.Errorf("groups[%d].name = %q, want %q", i, g.name, wantOrder[i])
			}
		}
		if len(groups[0].runners) != 2 {
			t.Errorf("h1 group size = %d, want 2", len(groups[0].runners))
		}
		if len(groups[1].runners) != 2 {
			t.Errorf("h2 group size = %d, want 2", len(groups[1].runners))
		}
		if len(groups[2].runners) != 1 {
			t.Errorf("h3 group size = %d, want 1", len(groups[2].runners))
		}
	})
}

func TestApplyContainerImageExtras(t *testing.T) {
	t.Parallel()

	t.Run("nil manager is no-op", func(t *testing.T) {
		t.Parallel()
		// Must not panic.
		applyContainerImageExtras(nil, &config.Config{})
	})

	t.Run("nil config clears extras", func(t *testing.T) {
		t.Parallel()
		mgr := &runner.Manager{ContainerImageExtraApt: []string{"curl", "git"}}
		applyContainerImageExtras(mgr, nil)
		if mgr.ContainerImageExtraApt != nil {
			t.Errorf("expected nil, got %v", mgr.ContainerImageExtraApt)
		}
	})

	t.Run("config with no extras clears manager", func(t *testing.T) {
		t.Parallel()
		mgr := &runner.Manager{ContainerImageExtraApt: []string{"curl"}}
		cfg := &config.Config{}
		applyContainerImageExtras(mgr, cfg)
		if len(mgr.ContainerImageExtraApt) != 0 {
			t.Errorf("expected empty, got %v", mgr.ContainerImageExtraApt)
		}
	})

	t.Run("config with extras populates manager", func(t *testing.T) {
		t.Parallel()
		mgr := &runner.Manager{}
		cfg := &config.Config{
			ContainerRunnerImage: config.ContainerRunnerImageConfig{
				ExtraAptPackages: []string{"sqlite3", "ffmpeg"},
			},
		}
		applyContainerImageExtras(mgr, cfg)
		if len(mgr.ContainerImageExtraApt) != 2 {
			t.Fatalf("got %d items, want 2", len(mgr.ContainerImageExtraApt))
		}
		if mgr.ContainerImageExtraApt[0] != "sqlite3" || mgr.ContainerImageExtraApt[1] != "ffmpeg" {
			t.Errorf("got %v", mgr.ContainerImageExtraApt)
		}
	})
}

func TestLockedWriter_Sequential(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	lw := &lockedWriter{w: &buf}
	n, err := lw.Write([]byte("hello"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if n != 5 {
		t.Errorf("wrote %d bytes, want 5", n)
	}
	if buf.String() != "hello" {
		t.Errorf("buf = %q, want %q", buf.String(), "hello")
	}
}

func TestLockedWriter_Concurrent(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	lw := &lockedWriter{w: &buf}
	const workers = 10
	const writesPerWorker = 100
	expected := workers * writesPerWorker

	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < writesPerWorker; j++ {
				lw.Write([]byte("x"))
			}
		}()
	}
	wg.Wait()

	if buf.Len() != expected {
		t.Errorf("buf.Len() = %d, want %d", buf.Len(), expected)
	}
}

func TestResolveAndFilter(t *testing.T) {
	t.Parallel()

	// fully resolved local host → ResolveHostInfo short-circuits
	resolvedCfg := func() *config.Config {
		return &config.Config{
			Hosts: map[string]config.HostConfig{
				"h1": {Addr: "local", OS: "linux", Arch: "amd64"},
			},
			Runners: []config.RunnerConfig{
				{Name: "r1", Host: "h1", Repo: "org/repo"},
				{Name: "r2", Host: "h1", Repo: "org/repo"},
			},
		}
	}

	t.Run("returns all runners when no filters", func(t *testing.T) {
		t.Parallel()
		runners, err := resolveAndFilter(nil, resolvedCfg(), "", "", nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(runners) != 2 {
			t.Errorf("len(runners) = %d, want 2", len(runners))
		}
	})

	t.Run("filters by host", func(t *testing.T) {
		t.Parallel()
		cfg := &config.Config{
			Hosts: map[string]config.HostConfig{
				"h1": {Addr: "local", OS: "linux", Arch: "amd64"},
				"h2": {Addr: "local", OS: "linux", Arch: "amd64"},
			},
			Runners: []config.RunnerConfig{
				{Name: "r1", Host: "h1", Repo: "org/repo"},
				{Name: "r2", Host: "h2", Repo: "org/repo"},
			},
		}
		runners, err := resolveAndFilter(nil, cfg, "h1", "", nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(runners) != 1 {
			t.Fatalf("len(runners) = %d, want 1", len(runners))
		}
		if runners[0].Host != "h1" {
			t.Errorf("runners[0].Host = %q, want h1", runners[0].Host)
		}
	})

	t.Run("filters by repo", func(t *testing.T) {
		t.Parallel()
		cfg := &config.Config{
			Hosts: map[string]config.HostConfig{
				"h1": {Addr: "local", OS: "linux", Arch: "amd64"},
			},
			Runners: []config.RunnerConfig{
				{Name: "r1", Host: "h1", Repo: "org/repo1"},
				{Name: "r2", Host: "h1", Repo: "org/repo2"},
			},
		}
		runners, err := resolveAndFilter(nil, cfg, "", "org/repo1", nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(runners) != 1 {
			t.Fatalf("len(runners) = %d, want 1", len(runners))
		}
		if runners[0].Repo != "org/repo1" {
			t.Errorf("runners[0].Repo = %q, want org/repo1", runners[0].Repo)
		}
	})

	t.Run("filters by name args", func(t *testing.T) {
		t.Parallel()
		cfg := &config.Config{
			Hosts: map[string]config.HostConfig{
				"h1": {Addr: "local", OS: "linux", Arch: "amd64"},
			},
			Runners: []config.RunnerConfig{
				{Name: "r1", Host: "h1", Repo: "org/repo"},
				{Name: "r2", Host: "h1", Repo: "org/repo"},
				{Name: "r3", Host: "h1", Repo: "org/repo"},
			},
		}
		runners, err := resolveAndFilter(nil, cfg, "", "", []string{"r1", "r3"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(runners) != 2 {
			t.Fatalf("len(runners) = %d, want 2", len(runners))
		}
		got := []string{runners[0].Name, runners[1].Name}
		if got[0] != "r1" || got[1] != "r3" {
			t.Errorf("got names %v, want [r1 r3]", got)
		}
	})

	t.Run("returns empty slice when no match", func(t *testing.T) {
		t.Parallel()
		runners, err := resolveAndFilter(nil, resolvedCfg(), "", "nonexistent", nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(runners) != 0 {
			t.Errorf("len(runners) = %d, want 0", len(runners))
		}
	})
}

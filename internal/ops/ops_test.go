package ops

import (
	"bytes"
	"sync"
	"testing"

	"github.com/an-lee/gh-sr/internal/config"
	"github.com/an-lee/gh-sr/internal/runner"
)

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
		if mgr.ContainerImageExtraApt != nil {
			t.Errorf("expected nil, got %v", mgr.ContainerImageExtraApt)
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

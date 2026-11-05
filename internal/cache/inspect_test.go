package cache

import (
	"errors"
	"strings"
	"testing"

	"github.com/an-lee/gh-sr/internal/testutil"
)

// TestInspect covers the Inspect orchestrator that reports the per-host cache
// server's docker state, image, runner URL, storage path, health, and storage
// size. Inspect issues ONE SSH round trip (the one-shot script), so the mocks
// answer with the composed tag-prefixed output via oneShotOutput. It pins
// every branch:
//   - one-shot script error → wrapped error
//   - absent container (empty state) → State="", Healthy=false, no du bytes
//   - state only (no image in output) → State set, Image ""
//   - state+image → both populated
//   - healthy probe → Healthy=true
//   - unhealthy probe (non-"healthy" body) → Healthy=false
//   - empty $HOME → resolving-home-dir error
//   - du returns garbage or empty → StorageBytes=0
func TestInspect(t *testing.T) {
	t.Parallel()

	t.Run("script error wraps cleanly", func(t *testing.T) {
		t.Parallel()
		sentinel := errors.New("ssh gone")
		mock := &testutil.MockExecutor{RunFn: func(cmd string) (string, error) {
			return "", sentinel
		}}
		_, err := Inspect(newCacheHost(t, mock), Settings{Enabled: true})
		if err == nil || !strings.Contains(err.Error(), "inspecting cache container") {
			t.Fatalf("got %v; want wrapped inspect error", err)
		}
		if !errors.Is(err, sentinel) {
			t.Errorf("error chain should wrap sentinel, got %v", err)
		}
	})

	t.Run("absent container leaves State empty and skips probes in one round trip", func(t *testing.T) {
		t.Parallel()
		mock := &testutil.MockExecutor{RunFn: func(cmd string) (string, error) {
			return oneShotOutput("", "/root", gwOutput, "", ""), nil
		}}
		info, err := Inspect(newCacheHost(t, mock), Settings{Enabled: true})
		if err != nil {
			t.Fatalf("Inspect: %v", err)
		}
		if info.State != "" || info.Image != "" {
			t.Errorf("absent container: got state=%q image=%q, want both empty", info.State, info.Image)
		}
		if info.Healthy {
			t.Errorf("absent container must not be marked healthy")
		}
		if info.StorageBytes != 0 {
			t.Errorf("absent container must not record storage size, got %d", info.StorageBytes)
		}
		if len(mock.Calls) != 1 {
			t.Errorf("Inspect must cost exactly one round trip, got %d: %v", len(mock.Calls), mock.Calls)
		}
	})

	t.Run("state only output yields State without Image", func(t *testing.T) {
		t.Parallel()
		mock := &testutil.MockExecutor{RunFn: func(cmd string) (string, error) {
			return oneShotOutput("running", "/root", gwOutput, "healthy", "1024"), nil
		}}
		info, err := Inspect(newCacheHost(t, mock), Settings{Enabled: true})
		if err != nil {
			t.Fatalf("Inspect: %v", err)
		}
		if info.State != "running" {
			t.Errorf("state: got %q, want running", info.State)
		}
		if info.Image != "" {
			t.Errorf("image: got %q, want empty when not in output", info.Image)
		}
	})

	t.Run("state and image captured from pipe-delimited output", func(t *testing.T) {
		t.Parallel()
		mock := &testutil.MockExecutor{RunFn: func(cmd string) (string, error) {
			return oneShotOutput("running|ghcr.io/falcondev-oss/github-actions-cache-server:latest",
				"/root", gwOutput, "healthy", "2048"), nil
		}}
		info, err := Inspect(newCacheHost(t, mock), Settings{Enabled: true})
		if err != nil {
			t.Fatalf("Inspect: %v", err)
		}
		if info.State != "running" {
			t.Errorf("state: got %q, want running", info.State)
		}
		if info.Image != "ghcr.io/falcondev-oss/github-actions-cache-server:latest" {
			t.Errorf("image: got %q", info.Image)
		}
		if !info.Healthy {
			t.Errorf("expected Healthy=true")
		}
		if info.StorageBytes != 2048 {
			t.Errorf("storage bytes: got %d, want 2048", info.StorageBytes)
		}
	})

	t.Run("unhealthy body marks Healthy false", func(t *testing.T) {
		t.Parallel()
		mock := &testutil.MockExecutor{RunFn: func(cmd string) (string, error) {
			return oneShotOutput("running|example/image:1", "/root", gwOutput, "starting", ""), nil
		}}
		info, err := Inspect(newCacheHost(t, mock), Settings{Enabled: true})
		if err != nil {
			t.Fatalf("Inspect: %v", err)
		}
		if info.Healthy {
			t.Errorf("non-'healthy' body must not mark Healthy")
		}
	})

	t.Run("failed health probe leaves Healthy false", func(t *testing.T) {
		t.Parallel()
		mock := &testutil.MockExecutor{RunFn: func(cmd string) (string, error) {
			return oneShotOutput("running|example/image:1", "/root", gwOutput, "", ""), nil
		}}
		info, err := Inspect(newCacheHost(t, mock), Settings{Enabled: true})
		if err != nil {
			t.Fatalf("Inspect: %v", err)
		}
		if info.Healthy {
			t.Errorf("probe error must leave Healthy=false")
		}
	})

	t.Run("du garbage leaves StorageBytes at 0", func(t *testing.T) {
		t.Parallel()
		mock := &testutil.MockExecutor{RunFn: func(cmd string) (string, error) {
			return oneShotOutput("running|example/image:1", "/root", gwOutput, "healthy", "not-a-number"), nil
		}}
		info, err := Inspect(newCacheHost(t, mock), Settings{Enabled: true})
		if err != nil {
			t.Fatalf("Inspect: %v", err)
		}
		if info.StorageBytes != 0 {
			t.Errorf("du garbage: got %d, want 0", info.StorageBytes)
		}
	})

	t.Run("empty du leaves StorageBytes at 0", func(t *testing.T) {
		t.Parallel()
		mock := &testutil.MockExecutor{RunFn: func(cmd string) (string, error) {
			return oneShotOutput("running|example/image:1", "/root", gwOutput, "healthy", ""), nil
		}}
		info, err := Inspect(newCacheHost(t, mock), Settings{Enabled: true})
		if err != nil {
			t.Fatalf("Inspect should tolerate du failure, got: %v", err)
		}
		if info.StorageBytes != 0 {
			t.Errorf("du error: got %d, want 0", info.StorageBytes)
		}
	})

	t.Run("empty $HOME surfaces resolving error", func(t *testing.T) {
		t.Parallel()
		mock := &testutil.MockExecutor{RunFn: func(cmd string) (string, error) {
			return oneShotOutput("", "", gwOutput, "", ""), nil
		}}
		_, err := Inspect(newCacheHost(t, mock), Settings{
			Enabled:     true,
			StoragePath: "$HOME/cache",
		})
		if err == nil || !strings.Contains(err.Error(), "resolving home dir") {
			t.Fatalf("got %v; want resolving-home-dir error", err)
		}
	})

	t.Run("URL populated from URLOverride when present", func(t *testing.T) {
		t.Parallel()
		mock := &testutil.MockExecutor{RunFn: func(cmd string) (string, error) {
			return oneShotOutput("", "/root", "", "", ""), nil
		}}
		info, err := Inspect(newCacheHost(t, mock), Settings{
			Enabled:     true,
			URLOverride: "http://10.0.0.5:3000/",
		})
		if err != nil {
			t.Fatalf("Inspect: %v", err)
		}
		if info.URL != "http://10.0.0.5:3000/" {
			t.Errorf("URL from override: got %q, want trailing-slash form", info.URL)
		}
	})
}

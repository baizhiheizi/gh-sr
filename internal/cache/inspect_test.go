package cache

import (
	"errors"
	"strings"
	"testing"

	"github.com/an-lee/gh-sr/internal/testutil"
)

// TestInspect covers the Inspect orchestrator that reports the per-host cache
// server's docker state, image, runner URL, storage path, health, and storage
// size. It pins every branch:
//   - docker inspect error → wrapped error
//   - absent container (empty output) → State="", no health probe, no du
//   - state only (no image in output) → State set, Image ""
//   - state+image → both populated
//   - healthy probe → Healthy=true
//   - unhealthy probe (non-"healthy" body) → Healthy=false
//   - health probe fails (non-zero curl) → Healthy=false
//   - du returns valid int → StorageBytes set
//   - du returns garbage → StorageBytes=0
//   - du command errors → StorageBytes=0
//   - resolvedStoragePath fails → wrapped error
func TestInspect(t *testing.T) {
	t.Parallel()

	t.Run("docker inspect error wraps cleanly", func(t *testing.T) {
		t.Parallel()
		sentinel := errors.New("ssh gone")
		mock := &testutil.MockExecutor{RunFn: func(cmd string) (string, error) {
			if strings.Contains(cmd, "docker inspect") {
				return "", sentinel
			}
			return "", nil
		}}
		_, err := Inspect(newCacheHost(t, mock), Settings{Enabled: true})
		if err == nil || !strings.Contains(err.Error(), "inspecting cache container") {
			t.Fatalf("got %v; want wrapped inspect error", err)
		}
		if !errors.Is(err, sentinel) {
			t.Errorf("error chain should wrap sentinel, got %v", err)
		}
	})

	t.Run("absent container leaves State empty and skips probes", func(t *testing.T) {
		t.Parallel()
		mock := &testutil.MockExecutor{RunFn: func(cmd string) (string, error) {
			switch {
			case strings.Contains(cmd, "docker inspect --format="):
				return "", nil
			case strings.Contains(cmd, "echo $HOME"):
				return "/root\n", nil
			default:
				return "", nil
			}
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
		for _, c := range mock.Calls {
			if strings.Contains(c, "/health") || strings.Contains(c, "du -sb") {
				t.Errorf("absent container must not probe health/du, call=%q", c)
			}
		}
	})

	t.Run("state only output yields State without Image", func(t *testing.T) {
		t.Parallel()
		mock := &testutil.MockExecutor{RunFn: func(cmd string) (string, error) {
			switch {
			case strings.Contains(cmd, "docker inspect --format="):
				return "running\n", nil
			case strings.Contains(cmd, "ip -4 -o addr show docker0"):
				return gwOutput, nil
			case strings.Contains(cmd, "curl -fsS"):
				return "healthy", nil
			case strings.Contains(cmd, "echo $HOME"):
				return "/root\n", nil
			case strings.Contains(cmd, "du -sb"):
				return "1024\n", nil
			default:
				return "", nil
			}
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
			switch {
			case strings.Contains(cmd, "docker inspect --format="):
				return "running|ghcr.io/falcondev-oss/github-actions-cache-server:latest\n", nil
			case strings.Contains(cmd, "ip -4 -o addr show docker0"):
				return gwOutput, nil
			case strings.Contains(cmd, "curl -fsS"):
				return "healthy", nil
			case strings.Contains(cmd, "echo $HOME"):
				return "/root\n", nil
			case strings.Contains(cmd, "du -sb"):
				return "2048\n", nil
			default:
				return "", nil
			}
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
			switch {
			case strings.Contains(cmd, "docker inspect --format="):
				return "running|example/image:1\n", nil
			case strings.Contains(cmd, "ip -4 -o addr show docker0"):
				return gwOutput, nil
			case strings.Contains(cmd, "curl -fsS"):
				return "starting", nil
			case strings.Contains(cmd, "echo $HOME"):
				return "/root\n", nil
			default:
				return "", nil
			}
		}}
		info, err := Inspect(newCacheHost(t, mock), Settings{Enabled: true})
		if err != nil {
			t.Fatalf("Inspect: %v", err)
		}
		if info.Healthy {
			t.Errorf("non-'healthy' body must not mark Healthy")
		}
	})

	t.Run("health probe failure leaves Healthy false", func(t *testing.T) {
		t.Parallel()
		mock := &testutil.MockExecutor{RunFn: func(cmd string) (string, error) {
			switch {
			case strings.Contains(cmd, "docker inspect --format="):
				return "running|example/image:1\n", nil
			case strings.Contains(cmd, "ip -4 -o addr show docker0"):
				return gwOutput, nil
			case strings.Contains(cmd, "curl -fsS"):
				return "", errors.New("connection refused")
			case strings.Contains(cmd, "echo $HOME"):
				return "/root\n", nil
			default:
				return "", nil
			}
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
			switch {
			case strings.Contains(cmd, "docker inspect --format="):
				return "running|example/image:1\n", nil
			case strings.Contains(cmd, "ip -4 -o addr show docker0"):
				return gwOutput, nil
			case strings.Contains(cmd, "curl -fsS"):
				return "healthy", nil
			case strings.Contains(cmd, "echo $HOME"):
				return "/root\n", nil
			case strings.Contains(cmd, "du -sb"):
				return "not-a-number\n", nil
			default:
				return "", nil
			}
		}}
		info, err := Inspect(newCacheHost(t, mock), Settings{Enabled: true})
		if err != nil {
			t.Fatalf("Inspect: %v", err)
		}
		if info.StorageBytes != 0 {
			t.Errorf("du garbage: got %d, want 0", info.StorageBytes)
		}
	})

	t.Run("du command error leaves StorageBytes at 0", func(t *testing.T) {
		t.Parallel()
		mock := &testutil.MockExecutor{RunFn: func(cmd string) (string, error) {
			switch {
			case strings.Contains(cmd, "docker inspect --format="):
				return "running|example/image:1\n", nil
			case strings.Contains(cmd, "ip -4 -o addr show docker0"):
				return gwOutput, nil
			case strings.Contains(cmd, "curl -fsS"):
				return "healthy", nil
			case strings.Contains(cmd, "echo $HOME"):
				return "/root\n", nil
			case strings.Contains(cmd, "du -sb"):
				return "", errors.New("storage gone")
			default:
				return "", nil
			}
		}}
		info, err := Inspect(newCacheHost(t, mock), Settings{Enabled: true})
		if err != nil {
			t.Fatalf("Inspect should tolerate du failure, got: %v", err)
		}
		if info.StorageBytes != 0 {
			t.Errorf("du error: got %d, want 0", info.StorageBytes)
		}
	})

	t.Run("resolvedStoragePath error surfaces", func(t *testing.T) {
		t.Parallel()
		mock := &testutil.MockExecutor{RunFn: func(cmd string) (string, error) {
			if strings.Contains(cmd, "echo $HOME") {
				return "", errors.New("shell gone")
			}
			return "", nil
		}}
		_, err := Inspect(newCacheHost(t, mock), Settings{
			Enabled:     true,
			StoragePath: "$HOME/cache",
		})
		if err == nil || !strings.Contains(err.Error(), "resolving home dir") {
			t.Fatalf("got %v; want wrapped resolvedStoragePath error", err)
		}
	})

	t.Run("URL populated from URLOverride when present", func(t *testing.T) {
		t.Parallel()
		mock := &testutil.MockExecutor{RunFn: func(cmd string) (string, error) {
			switch {
			case strings.Contains(cmd, "docker inspect --format="):
				return "", nil
			case strings.Contains(cmd, "echo $HOME"):
				return "/root\n", nil
			default:
				return "", nil
			}
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

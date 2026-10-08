package cache

import (
	"io"
	"strings"
	"testing"

	"github.com/an-lee/gh-sr/internal/testutil"
)

// gatewayCallCount returns the number of `ip -4 -o addr show docker0` SSH
// calls the mock recorded — the proxy metric for the duplicate-lookup fix.
func gatewayCallCount(calls []string) int {
	n := 0
	for _, c := range calls {
		if strings.Contains(c, "ip -4 -o addr show docker0") {
			n++
		}
	}
	return n
}

func TestInspect_gatewayResolvedOnce(t *testing.T) {
	t.Parallel()

	t.Run("auto bind resolves gateway exactly once for URL + health probe", func(t *testing.T) {
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
				return "1024\n", nil
			default:
				return "", nil
			}
		}}
		info, err := Inspect(newCacheHost(t, mock), Settings{Enabled: true})
		if err != nil {
			t.Fatalf("Inspect: %v", err)
		}
		if info.URL != "http://172.17.0.1:27420/" {
			t.Errorf("URL: got %q, want http://172.17.0.1:27420/", info.URL)
		}
		if got := gatewayCallCount(mock.Calls); got != 1 {
			t.Errorf("gateway resolves: got %d, want 1 (pre-fix paid 2)", got)
		}
	})

	t.Run("explicit BindAddr skips the gateway resolve entirely", func(t *testing.T) {
		t.Parallel()
		mock := &testutil.MockExecutor{RunFn: func(cmd string) (string, error) {
			switch {
			case strings.Contains(cmd, "docker inspect --format="):
				return "running|example/image:1\n", nil
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
		info, err := Inspect(newCacheHost(t, mock), Settings{Enabled: true, BindAddr: "10.0.0.5"})
		if err != nil {
			t.Fatalf("Inspect: %v", err)
		}
		if info.URL != "http://10.0.0.5:27420/" {
			t.Errorf("URL: got %q, want http://10.0.0.5:27420/", info.URL)
		}
		if got := gatewayCallCount(mock.Calls); got != 0 {
			t.Errorf("gateway resolves: got %d, want 0 (pre-fix paid 1)", got)
		}
	})

	t.Run("URLOverride skips the gateway resolve entirely", func(t *testing.T) {
		t.Parallel()
		mock := &testutil.MockExecutor{RunFn: func(cmd string) (string, error) {
			switch {
			case strings.Contains(cmd, "docker inspect --format="):
				return "running|example/image:1\n", nil
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
		info, err := Inspect(newCacheHost(t, mock), Settings{Enabled: true, URLOverride: "http://override:9999/"})
		if err != nil {
			t.Fatalf("Inspect: %v", err)
		}
		if info.URL != "http://override:9999/" {
			t.Errorf("URL: got %q, want http://override:9999/", info.URL)
		}
		if got := gatewayCallCount(mock.Calls); got != 0 {
			t.Errorf("gateway resolves: got %d, want 0 (pre-fix paid 1)", got)
		}
	})

	t.Run("absent container skips gateway resolve just like pre-fix", func(t *testing.T) {
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
		if _, err := Inspect(newCacheHost(t, mock), Settings{Enabled: true}); err != nil {
			t.Fatalf("Inspect: %v", err)
		}
		if got := gatewayCallCount(mock.Calls); got != 1 {
			t.Errorf("absent container gateway resolves: got %d, want 1 (RunnerURL still needs it for info.URL)", got)
		}
	})
}

// TestDeploy_gatewayResolvedOnce documents that Ensure (deploy) now reuses
// the gateway IP it already resolved for the bind address, instead of
// triggering a second `ip -4 -o addr show docker0` when it builds the
// API_BASE_URL env.
func TestDeploy_gatewayResolvedOnce(t *testing.T) {
	t.Parallel()

	t.Run("auto bind resolves gateway once for bind + API_BASE_URL", func(t *testing.T) {
		t.Parallel()
		mock := &testutil.MockExecutor{RunFn: func(cmd string) (string, error) {
			switch {
			case strings.Contains(cmd, "docker inspect"):
				return "", nil
			case strings.Contains(cmd, "ip -4 -o addr show docker0"):
				return gwOutput, nil
			case strings.Contains(cmd, "echo $HOME"):
				return "/root\n", nil
			default:
				return "", nil
			}
		}}
		if err := Ensure(io.Discard, newCacheHost(t, mock), Settings{Enabled: true}); err != nil {
			t.Fatalf("Ensure: %v", err)
		}
		if got := gatewayCallCount(mock.Calls); got != 1 {
			t.Errorf("gateway resolves: got %d, want 1 (pre-fix paid 2)", got)
		}
		var apiBase string
		for _, c := range mock.Calls {
			if strings.Contains(c, "API_BASE_URL=") {
				apiBase = c
			}
		}
		if !strings.Contains(apiBase, "http://172.17.0.1:") {
			t.Errorf("API_BASE_URL should embed docker0 gateway, got: %q", apiBase)
		}
	})

	t.Run("missing gateway still resolves once and warns", func(t *testing.T) {
		t.Parallel()
		mock := &testutil.MockExecutor{RunFn: func(cmd string) (string, error) {
			switch {
			case strings.Contains(cmd, "docker inspect"):
				return "", nil
			case strings.Contains(cmd, "ip -4 -o addr show docker0"):
				return "", io.EOF
			case strings.Contains(cmd, "echo $HOME"):
				return "/root\n", nil
			default:
				return "", nil
			}
		}}
		if err := Ensure(io.Discard, newCacheHost(t, mock), Settings{Enabled: true}); err != nil {
			t.Fatalf("Ensure: %v", err)
		}
		if got := gatewayCallCount(mock.Calls); got != 1 {
			t.Errorf("gateway resolves: got %d, want 1 (pre-fix paid 2 even when empty)", got)
		}
	})
}

// BenchmarkInspect measures wall-clock cost of one Inspect call under the
// auto-bind configuration — the hot path the fix targets. The mock executor
// returns instantly, so the result is dominated by the resolvedStoragePath
// / health / du SSH-round-trip overhead rather than real network latency.
// Run with `make bench-save` and compare to a pre-fix baseline.
func BenchmarkInspect(b *testing.B) {
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
			return "1024\n", nil
		default:
			return "", nil
		}
	}}
	h := newCacheHost(&testing.T{}, mock)
	s := Settings{Enabled: true}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := Inspect(h, s); err != nil {
			b.Fatal(err)
		}
	}
}

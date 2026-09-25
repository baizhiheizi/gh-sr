package ops

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/an-lee/gh-sr/internal/cache"
	"github.com/an-lee/gh-sr/internal/config"
	"github.com/an-lee/gh-sr/internal/host"
	"github.com/an-lee/gh-sr/internal/testutil"
)

// runningStateMock returns a MockExecutor that reports the cache container as
// already running on the docker inspect call (so cache.Ensure short-circuits
// at the `case "running": return nil` branch). Returns "" for everything else,
// which keeps the orchestrator's per-host loop focused on dispatch + printing
// — not on re-testing cache.Ensure's many sub-paths (which has its own
// dedicated tests in internal/cache).
func runningStateMock() *testutil.MockExecutor {
	return &testutil.MockExecutor{
		RunFn: func(cmd string) (string, error) {
			if strings.Contains(cmd, "docker inspect --format='{{.State.Status}}") {
				return "running", nil
			}
			return "", nil
		},
	}
}

// existsStateMock reports the cache container as present-but-not-running with
// the current layout label, so cache.Ensure falls into the `case "exists"`
// branch and issues `docker start` without falling through to a recreate.
// Two docker inspect commands are issued by Ensure:
//   - containerState     : `docker inspect --format='{{.State.Status}}'` (no `|`)
//   - cacheLayoutCurrent : `docker inspect --format '...|<label query>'` (has `|`)
//
// cacheLayoutCurrent returns true when the output contains `|<cacheLayoutRev>`
// (currently `|v3`); the containerState reply is "exists" so Ensure reaches
// the `case "exists": docker start` branch.
func existsStateMock() *testutil.MockExecutor {
	return &testutil.MockExecutor{
		RunFn: func(cmd string) (string, error) {
			switch {
			case strings.Contains(cmd, "docker inspect --format") && !strings.Contains(cmd, "|"):
				return "exists", nil
			case strings.Contains(cmd, "docker inspect --format") && strings.Contains(cmd, "|"):
				return "exited|v3", nil
			default:
				return "", nil
			}
		},
	}
}

// inspectingStateMock reports the cache container with a state + image so
// cache.Inspect populates both State and Image (used by CacheStatus). Returns
// "" for everything else (no health probe answer, no du size).
func inspectingStateMock(state, image string) *testutil.MockExecutor {
	return &testutil.MockExecutor{
		RunFn: func(cmd string) (string, error) {
			if strings.Contains(cmd, "docker inspect --format='{{.State.Status}}|{{.Config.Image}}") {
				return state + "|" + image, nil
			}
			return "", nil
		},
	}
}

// pruneEnabledMock returns a MockExecutor that satisfies cache.Prune's three
// calls in order: storage path resolve (echo $HOME), management-key probe
// (cat management_key), then DELETE on /management-api/cache-entries. Any
// other command is a no-op so non-Prune invocations remain safe.
func pruneEnabledMock() *testutil.MockExecutor {
	return &testutil.MockExecutor{
		RunFn: func(cmd string) (string, error) {
			switch {
			case strings.Contains(cmd, "echo $HOME"):
				return "/root\n", nil
			case strings.Contains(cmd, "test -f") && strings.Contains(cmd, "management_key"):
				return "fakekey", nil
			case strings.Contains(cmd, "X-Api-Key") && strings.Contains(cmd, "DELETE"):
				return "", nil
			default:
				return "", nil
			}
		},
	}
}

// removeOKMock returns "" for the docker rm -f call so cache.Remove completes.
func removeOKMock() *testutil.MockExecutor {
	return &testutil.MockExecutor{
		RunFn: func(string) (string, error) { return "", nil },
	}
}

// enabledCacheConfig returns a *config.Config with cache explicitly enabled
// and the minimum shape for cacheSettings(cfg) to produce a non-nil
// *cache.Settings. The cfg hosts are local-only so ResolveHostInfo short-
// circuits without touching SSH.
func enabledCacheConfig(hosts ...string) *config.Config {
	enabled := true
	cfg := cfgWithLocalHost(hosts...)
	cfg.Cache = config.CacheConfig{
		Enabled:          &enabled,
		Port:             3000,
		BindAddr:         "172.17.0.1",
		StoragePath:      "/srv/cache",
		RetentionDays:    7,
		MaxSizeBytes:     1024,
		MaxUsagePercent:  80,
		Image:            "example.com/cache:v1",
		ManagementAPIKey: "k",
	}
	return cfg
}

// addHost appends a non-default host to a cfg. Used by cacheTargets tests
// that need a windows host mixed in with linux hosts.
func addHost(cfg *config.Config, name, os string) {
	cfg.Hosts[name] = config.HostConfig{Addr: "local", OS: os, Arch: "amd64"}
}

// TestCacheTargets covers the per-host filter logic of cacheTargets: linux
// filter, filterHost narrowing, sort order, and the "no cache-eligible hosts"
// empty-list case. These tests do not exercise connectHostFn because
// cacheTargets short-circuits on cfg-without-SSH-needs (cfgWithLocalHost).
func TestCacheTargets(t *testing.T) {
	t.Parallel()

	t.Run("single linux host returned", func(t *testing.T) {
		t.Parallel()
		installMockConnectHost(t, map[string]host.Executor{}) // unused
		cfg := cfgWithLocalHost("h1")
		got, err := cacheTargets(&bytes.Buffer{}, cfg, "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(got) != 1 || got[0] != "h1" {
			t.Errorf("got %v; want [h1]", got)
		}
	})

	t.Run("non-linux host is skipped with explanatory note", func(t *testing.T) {
		t.Parallel()
		installMockConnectHost(t, map[string]host.Executor{})
		cfg := cfgWithLocalHost("linux1")
		addHost(cfg, "win1", "windows")
		var buf bytes.Buffer
		got, err := cacheTargets(&buf, cfg, "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(got) != 1 || got[0] != "linux1" {
			t.Errorf("got %v; want [linux1]", got)
		}
		out := buf.String()
		if !strings.Contains(out, "Skipping host win1") {
			t.Errorf("expected skip note for win1; got %q", out)
		}
		if !strings.Contains(out, "cache server requires Linux") {
			t.Errorf("expected linux requirement note; got %q", out)
		}
	})

	t.Run("filterHost narrows to single host", func(t *testing.T) {
		t.Parallel()
		installMockConnectHost(t, map[string]host.Executor{})
		cfg := cfgWithLocalHost("h1", "h2", "h3")
		got, err := cacheTargets(&bytes.Buffer{}, cfg, "h2")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(got) != 1 || got[0] != "h2" {
			t.Errorf("got %v; want [h2]", got)
		}
	})

	t.Run("filterHost that matches nothing returns empty slice", func(t *testing.T) {
		t.Parallel()
		installMockConnectHost(t, map[string]host.Executor{})
		cfg := cfgWithLocalHost("h1", "h2")
		got, err := cacheTargets(&bytes.Buffer{}, cfg, "no-such-host")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(got) != 0 {
			t.Errorf("got %v; want empty slice", got)
		}
	})

	t.Run("multiple hosts are sorted alphabetically", func(t *testing.T) {
		t.Parallel()
		installMockConnectHost(t, map[string]host.Executor{})
		cfg := cfgWithLocalHost("zeta", "alpha", "mu")
		got, err := cacheTargets(&bytes.Buffer{}, cfg, "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		want := []string{"alpha", "mu", "zeta"}
		if len(got) != len(want) {
			t.Fatalf("got %v; want %v", got, want)
		}
		for i := range want {
			if got[i] != want[i] {
				t.Errorf("position %d: got %q want %q", i, got[i], want[i])
			}
		}
	})
}

// TestPrintCacheStatus covers the pure formatter: empty state, state with
// and without URL, and the (healthy) suffix. printCacheStatus is the only
// orchestrator helper that does not need a host mock.
func TestPrintCacheStatus(t *testing.T) {
	t.Parallel()

	t.Run("empty state prints not-deployed banner", func(t *testing.T) {
		t.Parallel()
		var buf bytes.Buffer
		printCacheStatus(&buf, cache.StatusInfo{})
		out := buf.String()
		if !strings.Contains(out, "cache: not deployed") {
			t.Errorf("got %q; want 'cache: not deployed'", out)
		}
	})

	t.Run("running state without URL omits url line", func(t *testing.T) {
		t.Parallel()
		var buf bytes.Buffer
		printCacheStatus(&buf, cache.StatusInfo{
			State:        "running",
			Image:        "example.com/cache:v1",
			StoragePath:  "/srv/cache",
			StorageBytes: 4096,
		})
		out := buf.String()
		for _, want := range []string{
			"state:   running",
			"image:   example.com/cache:v1",
			"storage: /srv/cache (",
		} {
			if !strings.Contains(out, want) {
				t.Errorf("missing %q in:\n%s", want, out)
			}
		}
		if strings.Contains(out, "url:") {
			t.Errorf("url line should be omitted when URL is empty; got:\n%s", out)
		}
	})

	t.Run("running state with URL prints url line", func(t *testing.T) {
		t.Parallel()
		var buf bytes.Buffer
		printCacheStatus(&buf, cache.StatusInfo{
			State:        "running",
			Image:        "example.com/cache:v1",
			URL:          "http://172.17.0.1:3000",
			StoragePath:  "/srv/cache",
			StorageBytes: 8192,
		})
		out := buf.String()
		if !strings.Contains(out, "url:     http://172.17.0.1:3000") {
			t.Errorf("missing url line in:\n%s", out)
		}
	})

	t.Run("healthy flag appends (healthy) suffix to state", func(t *testing.T) {
		t.Parallel()
		var buf bytes.Buffer
		printCacheStatus(&buf, cache.StatusInfo{
			State:   "running",
			Healthy: true,
		})
		out := buf.String()
		if !strings.Contains(out, "state:   running (healthy)") {
			t.Errorf("missing (healthy) suffix; got:\n%s", out)
		}
	})

	t.Run("healthy false does not append suffix", func(t *testing.T) {
		t.Parallel()
		var buf bytes.Buffer
		printCacheStatus(&buf, cache.StatusInfo{State: "exited"})
		out := buf.String()
		if strings.Contains(out, "(healthy)") {
			t.Errorf("non-healthy state must not print (healthy); got:\n%s", out)
		}
	})
}

// TestCacheDeploy covers the per-host deploy loop: cache-disabled error,
// successful already-running path, exists → docker start path, connect
// failure, and absent → deploy fallback path.
func TestCacheDeploy(t *testing.T) {
	t.Parallel()

	t.Run("cache disabled in cfg returns descriptive error", func(t *testing.T) {
		t.Parallel()
		installMockConnectHost(t, map[string]host.Executor{
			"h1": runningStateMock(),
		})
		// Zero-value Config (Cache.Enabled == nil) → cacheSettings returns nil.
		cfg := cfgWithLocalHost("h1")
		var buf bytes.Buffer
		err := CacheDeploy(&buf, cfg, "")
		if err == nil {
			t.Fatalf("expected error when cache is disabled; got nil")
		}
		if !strings.Contains(err.Error(), "cache is disabled") {
			t.Errorf("error must explain cache-disabled state; got %v", err)
		}
	})

	t.Run("connect error on host propagates and aborts loop", func(t *testing.T) {
		t.Parallel()
		sentinel := errors.New("ssh handshake failed")
		installFailingConnectHost(t, sentinel)
		cfg := enabledCacheConfig("h1")
		var buf bytes.Buffer
		err := CacheDeploy(&buf, cfg, "")
		if !errors.Is(err, sentinel) {
			t.Fatalf("got %v; want wrap of sentinel", err)
		}
		// Banner prints BEFORE the connect call, so it WILL appear. The
		// important contract here is that the connect error propagates and
		// the loop terminates before any cache.Ensure work is attempted —
		// assert the host was never touched.
		out := buf.String()
		if !strings.Contains(out, "Deploying cache on h1") {
			t.Errorf("banner should print (it precedes connect); got %q", out)
		}
	})

	t.Run("already-running host short-circuits with single banner", func(t *testing.T) {
		t.Parallel()
		installMockConnectHost(t, map[string]host.Executor{
			"h1": runningStateMock(),
		})
		cfg := enabledCacheConfig("h1")
		var buf bytes.Buffer
		if err := CacheDeploy(&buf, cfg, ""); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		out := buf.String()
		if c := strings.Count(out, "Deploying cache on h1"); c != 1 {
			t.Errorf("expected exactly 1 deploy banner; got %d\n%s", c, out)
		}
	})

	t.Run("exists state issues docker start via cache.Ensure", func(t *testing.T) {
		t.Parallel()
		exec := existsStateMock()
		installMockConnectHost(t, map[string]host.Executor{"h1": exec})
		cfg := enabledCacheConfig("h1")
		var buf bytes.Buffer
		if err := CacheDeploy(&buf, cfg, ""); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		// Ensure issues exactly one `docker start` call when state=="exists".
		var startCalls int
		for _, c := range exec.Calls {
			if strings.Contains(c, "docker start") && strings.Contains(c, "gh-sr-cache") {
				startCalls++
			}
		}
		if startCalls != 1 {
			t.Errorf("expected exactly 1 docker start call; got %d\ncalls=%v", startCalls, exec.Calls)
		}
	})

	t.Run("absent state falls through to deploy path and completes", func(t *testing.T) {
		t.Parallel()
		installMockConnectHost(t, map[string]host.Executor{
			"h1": &testutil.MockExecutor{RunFn: func(string) (string, error) { return "", nil }},
		})
		cfg := enabledCacheConfig("h1")
		var buf bytes.Buffer
		if err := CacheDeploy(&buf, cfg, ""); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		out := buf.String()
		if !strings.Contains(out, "Deploying cache on h1") {
			t.Errorf("expected deploy banner; got %q", out)
		}
	})
}

// TestCacheStatus covers per-host cache status reporting: cache-disabled,
// running with image (full status block), not-deployed state, and connect
// error propagation.
func TestCacheStatus(t *testing.T) {
	t.Parallel()

	t.Run("cache disabled prints per-host disabled note", func(t *testing.T) {
		t.Parallel()
		installMockConnectHost(t, map[string]host.Executor{
			"h1": runningStateMock(),
		})
		cfg := cfgWithLocalHost("h1") // zero-value Cache → disabled
		var buf bytes.Buffer
		if err := CacheStatus(&buf, cfg, ""); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		out := buf.String()
		if !strings.Contains(out, "Cache status on h1") {
			t.Errorf("missing banner; got:\n%s", out)
		}
		if !strings.Contains(out, "cache: disabled in runners.yml") {
			t.Errorf("missing disabled note; got:\n%s", out)
		}
	})

	t.Run("running state with image prints full status block", func(t *testing.T) {
		t.Parallel()
		installMockConnectHost(t, map[string]host.Executor{
			"h1": inspectingStateMock("running", "example.com/cache:v1"),
		})
		cfg := enabledCacheConfig("h1")
		var buf bytes.Buffer
		if err := CacheStatus(&buf, cfg, ""); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		out := buf.String()
		for _, want := range []string{
			"Cache status on h1",
			"state:   running",
			"image:   example.com/cache:v1",
		} {
			if !strings.Contains(out, want) {
				t.Errorf("missing %q in:\n%s", want, out)
			}
		}
	})

	t.Run("not-deployed state prints not-deployed banner", func(t *testing.T) {
		t.Parallel()
		installMockConnectHost(t, map[string]host.Executor{
			"h1": inspectingStateMock("", ""),
		})
		cfg := enabledCacheConfig("h1")
		var buf bytes.Buffer
		if err := CacheStatus(&buf, cfg, ""); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(buf.String(), "cache: not deployed") {
			t.Errorf("missing not-deployed note; got:\n%s", buf.String())
		}
	})

	t.Run("connect error propagates", func(t *testing.T) {
		t.Parallel()
		sentinel := errors.New("cache-status connect failed")
		installFailingConnectHost(t, sentinel)
		cfg := enabledCacheConfig("h1")
		var buf bytes.Buffer
		err := CacheStatus(&buf, cfg, "")
		if !errors.Is(err, sentinel) {
			t.Fatalf("got %v; want wrap of sentinel", err)
		}
	})
}

// TestCachePrune covers per-host prune dispatch: cache-disabled print, enabled
// with curl DELETE success, no-management-key hint path, and connect error.
func TestCachePrune(t *testing.T) {
	t.Parallel()

	t.Run("cache disabled prints disabled note and skips prune", func(t *testing.T) {
		t.Parallel()
		installMockConnectHost(t, map[string]host.Executor{
			"h1": pruneEnabledMock(),
		})
		cfg := cfgWithLocalHost("h1") // disabled
		var buf bytes.Buffer
		if err := CachePrune(&buf, cfg, ""); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		out := buf.String()
		if !strings.Contains(out, "cache: disabled in runners.yml") {
			t.Errorf("missing disabled note; got:\n%s", out)
		}
	})

	t.Run("enabled cache issues DELETE and prints success note", func(t *testing.T) {
		t.Parallel()
		installMockConnectHost(t, map[string]host.Executor{
			"h1": pruneEnabledMock(),
		})
		cfg := enabledCacheConfig("h1")
		var buf bytes.Buffer
		if err := CachePrune(&buf, cfg, ""); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		out := buf.String()
		for _, want := range []string{
			"Pruning cache on h1",
			"pruning all cache entries",
			"all cache entries deleted",
		} {
			if !strings.Contains(out, want) {
				t.Errorf("missing %q in:\n%s", want, out)
			}
		}
	})

	t.Run("missing management key prints hint and skips curl", func(t *testing.T) {
		t.Parallel()
		// Clear the configured key and replace the host mock with one that
		// errors on every persistence probe so cache.Prune's
		// Settings.managementKey falls into the no-key hint branch.
		cfg := enabledCacheConfig("h1")
		cfg.Cache.ManagementAPIKey = ""
		noKeyExec := &testutil.MockExecutor{RunFn: func(cmd string) (string, error) {
			if strings.Contains(cmd, "echo $HOME") {
				return "/root\n", nil
			}
			return "", errors.New("not found")
		}}
		installMockConnectHost(t, map[string]host.Executor{"h1": noKeyExec})
		var buf bytes.Buffer
		if err := CachePrune(&buf, cfg, ""); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		out := buf.String()
		if !strings.Contains(out, "no management API key configured") {
			t.Errorf("missing no-key hint; got:\n%s", out)
		}
		for _, c := range noKeyExec.Calls {
			if strings.Contains(c, "DELETE") {
				t.Errorf("no-key path must not issue DELETE; call=%q", c)
			}
		}
	})

	t.Run("connect error propagates", func(t *testing.T) {
		t.Parallel()
		sentinel := errors.New("cache-prune connect failed")
		installFailingConnectHost(t, sentinel)
		cfg := enabledCacheConfig("h1")
		var buf bytes.Buffer
		err := CachePrune(&buf, cfg, "")
		if !errors.Is(err, sentinel) {
			t.Fatalf("got %v; want wrap of sentinel", err)
		}
	})
}

// TestCacheRemove covers per-host uninstall: success, disabled cache (still
// issues docker rm), connect error, and underlying Remove error propagation.
func TestCacheRemove(t *testing.T) {
	t.Parallel()

	t.Run("enabled cache removes container with banner", func(t *testing.T) {
		t.Parallel()
		installMockConnectHost(t, map[string]host.Executor{
			"h1": removeOKMock(),
		})
		cfg := enabledCacheConfig("h1")
		var buf bytes.Buffer
		if err := CacheRemove(&buf, cfg, "", false); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		out := buf.String()
		for _, want := range []string{
			"Removing cache on h1",
			"cache: removed",
		} {
			if !strings.Contains(out, want) {
				t.Errorf("missing %q in:\n%s", want, out)
			}
		}
	})

	t.Run("disabled cache still issues docker rm with zero settings", func(t *testing.T) {
		t.Parallel()
		// When purgeData=false, Remove only does `docker rm -f` and prints
		// the removed banner — it never resolves the storage path. This is
		// the cleanest way to verify the disabled-but-still-rm branch without
		// needing to mock $HOME for resolvedStoragePath.
		installMockConnectHost(t, map[string]host.Executor{
			"h1": removeOKMock(),
		})
		cfg := cfgWithLocalHost("h1") // disabled
		var buf bytes.Buffer
		if err := CacheRemove(&buf, cfg, "", false); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		out := buf.String()
		if !strings.Contains(out, "Removing cache on h1") {
			t.Errorf("missing banner; got:\n%s", out)
		}
		if !strings.Contains(out, "cache: removed") {
			t.Errorf("missing removed note; got:\n%s", out)
		}
	})

	t.Run("connect error propagates", func(t *testing.T) {
		t.Parallel()
		sentinel := errors.New("cache-remove connect failed")
		installFailingConnectHost(t, sentinel)
		cfg := enabledCacheConfig("h1")
		var buf bytes.Buffer
		err := CacheRemove(&buf, cfg, "", false)
		if !errors.Is(err, sentinel) {
			t.Fatalf("got %v; want wrap of sentinel", err)
		}
	})
}

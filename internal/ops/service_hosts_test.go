package ops

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/an-lee/gh-sr/internal/cache"
	"github.com/an-lee/gh-sr/internal/config"
	"github.com/an-lee/gh-sr/internal/host"
	"github.com/an-lee/gh-sr/internal/registry"
	"github.com/an-lee/gh-sr/internal/testutil"
)

// serviceEnsureMock reports every managed service container as already running
// with the current layout label, so each Ensure converges at its no-op branch:
// two `docker inspect` commands per container, no deploy. The layout replies
// are the services' current layout revisions (cache v3, registry v2) — a
// revision bump would only add a recreate, it would not change what the tests
// below assert.
func serviceEnsureMock() *testutil.MockExecutor {
	return &testutil.MockExecutor{
		RunFn: func(cmd string) (string, error) {
			switch {
			case strings.Contains(cmd, "Config.Labels"):
				if strings.Contains(cmd, cache.ContainerName) {
					return "v3", nil
				}
				return "v2", nil
			case strings.Contains(cmd, ".State.Status"):
				return "running", nil
			}
			return "", nil
		},
	}
}

// newServiceHost returns a host wired to mock, bypassing SSH.
func newServiceHost(mock *testutil.MockExecutor) *host.Host {
	h := host.NewHost("h1", config.HostConfig{Addr: "runner@box", OS: "linux", Arch: "amd64"})
	h.SetConn(mock)
	return h
}

// inspected reports whether the recorded remote commands included a
// `docker inspect` of the named container.
func inspected(calls []string, container string) bool {
	for _, cmd := range calls {
		if strings.Contains(cmd, "docker inspect") && strings.Contains(cmd, container) {
			return true
		}
	}
	return false
}

// TestEnsureContainerServices pins the container-mode policy that
// ensureContainerServices owns: native-mode runners never touch the managed
// per-host services, container-mode runners get the enabled ones converged.
//
// The assertions are made on the remote commands that reached the host, so
// the guard cannot be dropped without a test failing: without it a native
// runner's host would inspect — and on a cold host pull and start — cache and
// registry containers it never needs.
func TestEnsureContainerServices(t *testing.T) {
	t.Parallel()

	native := config.RunnerConfig{Name: "n", Host: "h1", Repo: "o/r", RunnerMode: config.RunnerModeNative}
	defaultMode := config.RunnerConfig{Name: "d", Host: "h1", Repo: "o/r"} // defaults to native
	container := config.RunnerConfig{Name: "c", Host: "h1", Repo: "o/r", RunnerMode: config.RunnerModeContainer}

	enabledCache := &cache.Settings{Enabled: true}
	enabledRegistry := &registry.Settings{Enabled: true}

	t.Run("native runner issues no service command", func(t *testing.T) {
		t.Parallel()

		for _, rc := range []config.RunnerConfig{native, defaultMode} {
			mock := serviceEnsureMock()
			h := newServiceHost(mock)
			var buf bytes.Buffer
			if err := ensureContainerServices(&buf, h, rc, enabledCache, enabledRegistry); err != nil {
				t.Fatalf("ensureContainerServices(%q): %v", rc.Name, err)
			}
			if len(mock.Calls) != 0 {
				t.Errorf("runner %q (%s): issued %d remote command(s), want none: %v",
					rc.Name, rc.EffectiveRunnerMode(), len(mock.Calls), mock.Calls)
			}
			for _, cmd := range mock.Calls {
				if strings.Contains(cmd, cache.ContainerName) || strings.Contains(cmd, registry.ContainerName) {
					t.Errorf("runner %q issued a service command: %q", rc.Name, cmd)
				}
			}
		}
	})

	t.Run("container runner converges the cache server only", func(t *testing.T) {
		t.Parallel()

		mock := serviceEnsureMock()
		h := newServiceHost(mock)
		var buf bytes.Buffer
		if err := ensureContainerServices(&buf, h, container, enabledCache, nil); err != nil {
			t.Fatalf("ensureContainerServices: %v", err)
		}
		if !inspected(mock.Calls, cache.ContainerName) {
			t.Errorf("no `docker inspect %s` was issued; calls: %v", cache.ContainerName, mock.Calls)
		}
		if inspected(mock.Calls, registry.ContainerName) {
			t.Errorf("registry mirror was inspected although it is disabled; calls: %v", mock.Calls)
		}
	})

	t.Run("container runner converges the registry mirror only", func(t *testing.T) {
		t.Parallel()

		mock := serviceEnsureMock()
		h := newServiceHost(mock)
		var buf bytes.Buffer
		if err := ensureContainerServices(&buf, h, container, nil, enabledRegistry); err != nil {
			t.Fatalf("ensureContainerServices: %v", err)
		}
		if !inspected(mock.Calls, registry.ContainerName) {
			t.Errorf("no `docker inspect %s` was issued; calls: %v", registry.ContainerName, mock.Calls)
		}
		if inspected(mock.Calls, cache.ContainerName) {
			t.Errorf("cache server was inspected although it is disabled; calls: %v", mock.Calls)
		}
	})

	t.Run("container runner converges both services", func(t *testing.T) {
		t.Parallel()

		mock := serviceEnsureMock()
		h := newServiceHost(mock)
		var buf bytes.Buffer
		if err := ensureContainerServices(&buf, h, container, enabledCache, enabledRegistry); err != nil {
			t.Fatalf("ensureContainerServices: %v", err)
		}
		if !inspected(mock.Calls, cache.ContainerName) {
			t.Errorf("no `docker inspect %s` was issued; calls: %v", cache.ContainerName, mock.Calls)
		}
		if !inspected(mock.Calls, registry.ContainerName) {
			t.Errorf("no `docker inspect %s` was issued; calls: %v", registry.ContainerName, mock.Calls)
		}
	})

	t.Run("container runner with both services disabled issues no command", func(t *testing.T) {
		t.Parallel()

		mock := serviceEnsureMock()
		h := newServiceHost(mock)
		var buf bytes.Buffer
		if err := ensureContainerServices(&buf, h, container, nil, nil); err != nil {
			t.Fatalf("ensureContainerServices: %v", err)
		}
		if len(mock.Calls) != 0 {
			t.Errorf("issued %d remote command(s), want none: %v", len(mock.Calls), mock.Calls)
		}
	})

	t.Run("cache failure stops before the registry mirror", func(t *testing.T) {
		t.Parallel()

		sentinel := errors.New("ssh transport died")
		mock := &testutil.MockExecutor{
			RunFn: func(cmd string) (string, error) {
				if strings.Contains(cmd, cache.ContainerName) {
					return "", sentinel
				}
				return "", nil
			},
		}
		h := newServiceHost(mock)
		var buf bytes.Buffer
		err := ensureContainerServices(&buf, h, container, enabledCache, enabledRegistry)
		if !errors.Is(err, sentinel) {
			t.Fatalf("error = %v; want it to wrap %v", err, sentinel)
		}
		if inspected(mock.Calls, registry.ContainerName) {
			t.Errorf("registry mirror was inspected after the cache failure; calls: %v", mock.Calls)
		}
	})
}

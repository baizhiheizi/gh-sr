package ops

import (
	"bytes"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/an-lee/gh-sr/internal/config"
	"github.com/an-lee/gh-sr/internal/host"
	"github.com/an-lee/gh-sr/internal/testutil"
)

// TestContainerRunnerContainers pins the per-host runner-container
// enumeration used by the image-sync fan-out: container-mode instances only,
// deterministic order.
func TestContainerRunnerContainers(t *testing.T) {
	t.Parallel()
	cfg := &config.Config{Runners: []config.RunnerConfig{
		{Name: "native-1", Repo: "o/r", Host: "h1"},
		{Name: "ci", Repo: "o/r", Host: "h1", Count: 2, RunnerMode: config.RunnerModeContainer},
		{Name: "aw", Repo: "o/r", Host: "h1", Profile: "agentic"},
		{Name: "other-host", Repo: "o/r", Host: "h2", RunnerMode: config.RunnerModeContainer},
	}}
	got := containerRunnerContainers(cfg, "h1")
	want := []string{"gh-sr-aw-1", "gh-sr-ci-1", "gh-sr-ci-2"}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v, want %v", got, want)
		}
	}
	if got := containerRunnerContainers(cfg, "h2"); len(got) != 1 || got[0] != "gh-sr-other-host-1" {
		t.Errorf("h2: got %v", got)
	}
}

// TestImagesSyncRequiresPrefetch pins the empty-prefetch contract.
func TestImagesSyncRequiresPrefetch(t *testing.T) {
	t.Parallel()
	cfg := &config.Config{Hosts: map[string]config.HostConfig{
		"h1": {Addr: "local", OS: "linux"},
	}}
	err := ImagesSync(&strings.Builder{}, cfg, "")
	if err == nil || !strings.Contains(err.Error(), "prefetch") {
		t.Fatalf("expected prefetch error, got %v", err)
	}
}

// multiHostSyncCfg returns a config with one container-mode runner per named
// host, the mirror enabled, and one prefetch ref — the setup ImagesSync and
// syncPrefetchAfterStart fan out over.
func multiHostSyncCfg(hosts ...string) *config.Config {
	cfg := cfgWithLocalHost(hosts...)
	enabled := true
	cfg.RegistryMirror = config.RegistryMirrorConfig{
		Enabled:     &enabled,
		BindAddr:    "127.0.0.1",
		StoragePath: "/srv/registry",
		Prefetch:    []string{"postgres:16"},
	}
	for _, h := range hosts {
		cfg.Runners = append(cfg.Runners, config.RunnerConfig{
			Name:       "ci",
			Repo:       "o/r",
			Host:       h,
			Count:      1,
			RunnerMode: config.RunnerModeContainer,
		})
	}
	return cfg
}

// barrierSyncMock answers registry.SyncPrefetch's command shapes, with the
// host-side prefetch `docker pull` gated on a barrier: it records the host's
// arrival and then blocks until every host has arrived. Only the prefetch
// pull (the postgres:16 ref) hits the barrier — registry.Ensure's own
// registry-image pull must pass through untouched, or the arrival count is
// polluted and the barrier deadlocks. A sequential per-host loop can never
// open all N pulls at once, so its pulls time out and error — the barrier
// turns "hosts ran concurrently" into an observable pass/fail.
func barrierSyncMock(name string, arrival chan<- string, release <-chan struct{}) *testutil.MockExecutor {
	return &testutil.MockExecutor{RunFn: func(cmd string) (string, error) {
		switch {
		case strings.Contains(cmd, "docker pull") && strings.Contains(cmd, "postgres:16"):
			arrival <- name
			select {
			case <-release:
			case <-time.After(5 * time.Second):
				return "", errors.New("barrier timeout: host " + name + " arrived alone")
			}
		case strings.Contains(cmd, "&& echo yes || echo no"):
			return "no\n", nil
		case strings.Contains(cmd, "{{.Id}}"):
			return "sha256:abc\n", nil
		}
		return "", nil
	}}
}

// TestImagesSyncSyncsHostsConcurrently proves the fan-out: all hosts' syncs
// must be in flight at once (barrier completes), and every host reports its
// own banner and summary.
func TestImagesSyncSyncsHostsConcurrently(t *testing.T) {
	t.Parallel()
	const N = 3
	names := []string{"h1", "h2", "h3"}
	arrival := make(chan string, N)
	release := make(chan struct{})
	mocks := make(map[string]host.Executor, N)
	for _, name := range names {
		mocks[name] = barrierSyncMock(name, arrival, release)
	}
	installMockConnectHost(t, mocks)
	go func() {
		for range N {
			<-arrival
		}
		close(release)
	}()
	var buf bytes.Buffer
	if err := ImagesSync(&buf, multiHostSyncCfg(names...), ""); err != nil {
		t.Fatalf("hosts did not sync concurrently (barrier timed out): %v", err)
	}
	out := buf.String()
	for _, name := range names {
		if !strings.Contains(out, "Syncing images on "+name) {
			t.Errorf("missing banner for %s in:\n%s", name, out)
		}
	}
	if c := strings.Count(out, "images: 1 pulled, 1 loaded into runners (0 already cached there)"); c != N {
		t.Errorf("expected %d summary lines, got %d in:\n%s", N, c, out)
	}
}

// TestImagesSyncConnectFailureDoesNotCancelOtherHosts pins the error policy
// under the fan-out: an unreachable host still fails the command (reported
// after every host had its turn), but the other hosts' syncs are no longer
// cancelled the way the old sequential early-return cancelled them.
func TestImagesSyncConnectFailureDoesNotCancelOtherHosts(t *testing.T) {
	t.Parallel()
	// h1 has no mock: installMockConnectHost makes its connect fail.
	release := make(chan struct{})
	close(release)
	installMockConnectHost(t, map[string]host.Executor{
		"h2": barrierSyncMock("h2", make(chan string, 1), release),
	})
	var buf bytes.Buffer
	err := ImagesSync(&buf, multiHostSyncCfg("h1", "h2"), "")
	if err == nil || !strings.Contains(err.Error(), "no mock registered for host h1") {
		t.Fatalf("connect failure must fail the command; got %v", err)
	}
	if out := buf.String(); !strings.Contains(out, "Syncing images on h2") {
		t.Errorf("h2's sync must still run despite h1's connect failure; got:\n%s", out)
	}
}

// TestSyncPrefetchAfterStartMultiHostBestEffort pins the multi-host
// best-effort policy: a connect failure and an Ensure failure each warn with
// the skip wording, a healthy host still syncs and reports, and the caller
// never sees an error.
func TestSyncPrefetchAfterStartMultiHostBestEffort(t *testing.T) {
	t.Parallel()
	ensureFails := &testutil.MockExecutor{RunFn: func(cmd string) (string, error) {
		if strings.Contains(cmd, ".State.Status") && !strings.Contains(cmd, "|{{.Config.Image}}") {
			return "", errors.New("inspect failed")
		}
		return "", nil
	}}
	healthy := &testutil.MockExecutor{RunFn: func(cmd string) (string, error) {
		switch {
		case strings.Contains(cmd, "Config.Labels"):
			return "v2", nil
		case strings.Contains(cmd, "&& echo yes || echo no"):
			return "no\n", nil
		case strings.Contains(cmd, "{{.Id}}"):
			return "sha256:abc\n", nil
		}
		return "", nil
	}}
	// h1 has no mock: connect fails. h2: Ensure fails. h3: syncs.
	installMockConnectHost(t, map[string]host.Executor{"h2": ensureFails, "h3": healthy})
	runner := config.RunnerConfig{Name: "ci", Repo: "o/r", Host: "h1", Count: 1, RunnerMode: config.RunnerModeContainer}
	runners := []config.RunnerConfig{runner,
		{Name: "ci", Repo: "o/r", Host: "h2", Count: 1, RunnerMode: config.RunnerModeContainer},
		{Name: "ci", Repo: "o/r", Host: "h3", Count: 1, RunnerMode: config.RunnerModeContainer},
	}
	var buf bytes.Buffer
	if err := syncPrefetchAfterStart(&buf, multiHostSyncCfg("h1", "h2", "h3"), runners); err != nil {
		t.Fatalf("sync must stay best-effort across hosts; got %v", err)
	}
	out := buf.String()
	for _, want := range []string{
		"Warning: image sync skipped on h1",
		"Warning: image sync skipped on h2",
		"Prefetching images on h3",
		"images: 1 pulled, 1 loaded into runners (0 already cached there)",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
}

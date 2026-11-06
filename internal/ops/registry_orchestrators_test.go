package ops

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/an-lee/gh-sr/internal/config"
	"github.com/an-lee/gh-sr/internal/host"
	"github.com/an-lee/gh-sr/internal/registry"
	"github.com/an-lee/gh-sr/internal/testutil"
)

// The mocks below keep the orchestrator tests on the dispatch + printing
// path, mirroring cache_orchestrators_test.go: registry.Ensure/Inspect/Remove
// and SyncPrefetch have dedicated sub-path coverage in internal/registry, so
// each mock only replies enough for the service call to succeed or fail at
// the branch under test.

// registryRunningMock reports the mirror container as running with the
// current layout label (internal/registry registryLayoutRev = "v2"), so
// registry.Ensure short-circuits at the converged no-op branch.
func registryRunningMock() *testutil.MockExecutor {
	return &testutil.MockExecutor{
		RunFn: func(cmd string) (string, error) {
			switch {
			case strings.Contains(cmd, "Config.Labels"):
				return "v2", nil
			case strings.Contains(cmd, ".State.Status"):
				return "running", nil
			}
			return "", nil
		},
	}
}

// registryInspectingMock answers registry.Inspect's one-shot batch probe
// (buildInspectBatchScript, identifiable by the
// `{{.State.Status}}|{{.Config.Image}}` inspect format) with the
// @@m:/@@st:/@@health:/@@du: stream parseInspectBatchOutput expects. An empty
// state reply omits the health/du sections, matching the script's
// `if [ -n "$out" ]` guard for an absent container.
func registryInspectingMock(state, image string, healthy bool, storageBytes string) *testutil.MockExecutor {
	const batchFmt = "{{.State.Status}}|{{.Config.Image}}"
	health := "fail"
	if healthy {
		health = "ok"
	}
	return &testutil.MockExecutor{
		RunFn: func(cmd string) (string, error) {
			if strings.Contains(cmd, batchFmt) {
				out := "@@m:1\n@@st:1|" + state + "|" + image + "\n"
				if state != "" {
					out += "@@health:1|" + health + "\n@@du:1|" + storageBytes + "\n"
				}
				return out, nil
			}
			return "", nil
		},
	}
}

// registryRemoveOKMock returns "" for every command so registry.Remove's
// docker rm -f (and rm -rf when purging) succeed.
func registryRemoveOKMock() *testutil.MockExecutor {
	return &testutil.MockExecutor{RunFn: func(string) (string, error) { return "", nil }}
}

// registrySyncMock satisfies the SSH command shapes registry.SyncPrefetch
// issues for one prefetch ref pulled through the Docker Hub mirror:
//
//   - `docker pull '<mirror-ref>'`                       → ok
//   - `docker tag '<mirror-ref>' '<ref>'`                → ok (retag, pure metadata)
//   - `docker image inspect --format='{{.Id}}' '<ref>'`  → imageID
//   - per-runner presence probe (`&& echo yes || echo no`) → "yes" for the
//     container names in cached, "no" otherwise (drives save|load + tag)
func registrySyncMock(imageID string, cached map[string]bool) *testutil.MockExecutor {
	return &testutil.MockExecutor{
		RunFn: func(cmd string) (string, error) {
			switch {
			case strings.Contains(cmd, "&& echo yes || echo no"):
				for cname := range cached {
					if strings.Contains(cmd, "'"+cname+"'") {
						return "yes\n", nil
					}
				}
				return "no\n", nil
			case strings.Contains(cmd, "{{.Id}}"):
				return imageID + "\n", nil
			}
			return "", nil
		},
	}
}

// enabledRegistryConfig enables the registry mirror with an absolute
// StoragePath and an explicit BindAddr, so neither mirrorStoragePaths ($HOME
// expansion) nor Inspect/Deploy (docker0 gateway probe) issue extra SSH
// probes. With no mirrors list, SettingsFromConfig synthesizes the legacy
// Docker Hub mirror (container gh-sr-registry).
func enabledRegistryConfig(hosts ...string) *config.Config {
	enabled := true
	cfg := cfgWithLocalHost(hosts...)
	cfg.RegistryMirror = config.RegistryMirrorConfig{
		Enabled:     &enabled,
		BindAddr:    "127.0.0.1",
		StoragePath: "/srv/registry",
	}
	return cfg
}

// containerRunnerCfg returns cfg with count container-mode runner instances
// named prefix-1..count on hostName.
func containerRunnerCfg(hostName string, count int) *config.Config {
	cfg := cfgWithLocalHost(hostName)
	cfg.Runners = []config.RunnerConfig{{
		Name:       "ci",
		Repo:       "o/r",
		Host:       hostName,
		Count:      count,
		RunnerMode: config.RunnerModeContainer,
	}}
	return cfg
}

// TestRegistryTargets covers the registry-specific target selection: the
// linux filter note names the registry mirror workload, and --host narrowing
// applies (serviceTargets itself is fully covered via the cache tests).
func TestRegistryTargets(t *testing.T) {
	t.Parallel()

	t.Run("non-linux skip note names the registry mirror", func(t *testing.T) {
		t.Parallel()
		installMockConnectHost(t, map[string]host.Executor{})
		cfg := cfgWithLocalHost("h1")
		addHost(cfg, "win1", "windows")
		var buf bytes.Buffer
		got, err := registryTargets(&buf, cfg, "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(got) != 1 || got[0] != "h1" {
			t.Errorf("got %v; want [h1]", got)
		}
		if out := buf.String(); !strings.Contains(out, "the registry mirror requires Linux") {
			t.Errorf("missing registry workload note; got %q", out)
		}
	})

	t.Run("filterHost narrows targets", func(t *testing.T) {
		t.Parallel()
		installMockConnectHost(t, map[string]host.Executor{})
		cfg := cfgWithLocalHost("h1", "h2")
		got, err := registryTargets(&bytes.Buffer{}, cfg, "h2")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(got) != 1 || got[0] != "h2" {
			t.Errorf("got %v; want [h2]", got)
		}
	})
}

// TestPrintRegistryStatus covers the pure status formatter: not-deployed
// banner, the [name] prefix rule (hidden for the default dockerhub mirror,
// shown for named ones), the (healthy) suffix, and the conditional url and
// store lines.
func TestPrintRegistryStatus(t *testing.T) {
	t.Parallel()

	t.Run("empty state prints not deployed", func(t *testing.T) {
		t.Parallel()
		var buf bytes.Buffer
		printRegistryStatus(&buf, registry.MirrorStatus{})
		if out := buf.String(); !strings.Contains(out, "registry: not deployed") {
			t.Errorf("got %q; want 'registry: not deployed'", out)
		}
	})

	t.Run("default mirror name has no [name] prefix", func(t *testing.T) {
		t.Parallel()
		var buf bytes.Buffer
		printRegistryStatus(&buf, registry.MirrorStatus{
			Name: config.DefaultRegistryMirrorName,
			Info: registry.StatusInfo{State: "running", Healthy: true},
		})
		out := buf.String()
		if !strings.Contains(out, "registry: state: running (healthy)") {
			t.Errorf("missing healthy state line; got %q", out)
		}
		if strings.Contains(out, "[") {
			t.Errorf("default mirror must not print a [name] prefix; got %q", out)
		}
	})

	t.Run("named mirror prints [name] prefix", func(t *testing.T) {
		t.Parallel()
		var buf bytes.Buffer
		printRegistryStatus(&buf, registry.MirrorStatus{
			Name: "ghcr",
			Info: registry.StatusInfo{State: "exited"},
		})
		if out := buf.String(); !strings.Contains(out, "registry[ghcr]: state: exited") {
			t.Errorf("got %q; want 'registry[ghcr]: state: exited'", out)
		}
	})

	t.Run("url line only when url is set", func(t *testing.T) {
		t.Parallel()
		var without bytes.Buffer
		printRegistryStatus(&without, registry.MirrorStatus{
			Info: registry.StatusInfo{State: "running"},
		})
		if strings.Contains(without.String(), "url:") {
			t.Errorf("empty URL must omit the url line; got %q", without.String())
		}
		var with bytes.Buffer
		printRegistryStatus(&with, registry.MirrorStatus{
			Info: registry.StatusInfo{State: "running", URL: "http://172.17.0.1:5577"},
		})
		if out := with.String(); !strings.Contains(out, "url:   http://172.17.0.1:5577") {
			t.Errorf("missing url line; got %q", out)
		}
	})

	t.Run("store line reports path and human size", func(t *testing.T) {
		t.Parallel()
		var buf bytes.Buffer
		printRegistryStatus(&buf, registry.MirrorStatus{
			Info: registry.StatusInfo{State: "running", StoragePath: "/srv/registry", StorageBytes: 4096},
		})
		out := buf.String()
		if !strings.Contains(out, "store: /srv/registry (") {
			t.Errorf("missing store line; got %q", out)
		}
		if !strings.Contains(out, "image: ") {
			t.Errorf("missing image line; got %q", out)
		}
	})
}

// TestRegistryDeploy covers the deploy orchestrator: the disabled-mirror
// error, the converged no-op success path, and connect-failure propagation.
func TestRegistryDeploy(t *testing.T) {
	t.Parallel()

	t.Run("disabled mirror returns descriptive error", func(t *testing.T) {
		t.Parallel()
		installMockConnectHost(t, map[string]host.Executor{
			"h1": registryRunningMock(),
		})
		cfg := cfgWithLocalHost("h1") // zero-value RegistryMirror → disabled
		var buf bytes.Buffer
		err := RegistryDeploy(&buf, cfg, "")
		if err == nil || !strings.Contains(err.Error(), "registry_mirror is disabled") {
			t.Fatalf("got %v; want registry_mirror disabled error", err)
		}
	})

	t.Run("already-running mirror is a no-op with one banner", func(t *testing.T) {
		t.Parallel()
		installMockConnectHost(t, map[string]host.Executor{
			"h1": registryRunningMock(),
		})
		var buf bytes.Buffer
		if err := RegistryDeploy(&buf, enabledRegistryConfig("h1"), ""); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if c := strings.Count(buf.String(), "Deploying registry on h1"); c != 1 {
			t.Errorf("expected exactly 1 deploy banner; got %d\n%s", c, buf.String())
		}
	})

	t.Run("connect error propagates", func(t *testing.T) {
		t.Parallel()
		sentinel := errors.New("registry-deploy connect failed")
		installFailingConnectHost(t, sentinel)
		var buf bytes.Buffer
		if err := RegistryDeploy(&buf, enabledRegistryConfig("h1"), ""); !errors.Is(err, sentinel) {
			t.Fatalf("got %v; want wrap of sentinel", err)
		}
	})
}

// TestRegistryStatus covers the status orchestrator: the per-host disabled
// note, full status block from a batched Inspect, the not-deployed branch,
// Inspect failure propagation, and connect-failure propagation.
func TestRegistryStatus(t *testing.T) {
	t.Parallel()

	t.Run("disabled mirror prints per-host note", func(t *testing.T) {
		t.Parallel()
		installMockConnectHost(t, map[string]host.Executor{
			"h1": registryInspectingMock("running", "registry:2", true, "4096"),
		})
		cfg := cfgWithLocalHost("h1") // disabled
		var buf bytes.Buffer
		if err := RegistryStatus(&buf, cfg, ""); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		out := buf.String()
		for _, want := range []string{
			"Registry status on h1",
			"registry: disabled in runners.yml",
		} {
			if !strings.Contains(out, want) {
				t.Errorf("missing %q in:\n%s", want, out)
			}
		}
	})

	t.Run("running mirror prints full status block", func(t *testing.T) {
		t.Parallel()
		installMockConnectHost(t, map[string]host.Executor{
			"h1": registryInspectingMock("running", "example.com/registry:v2", true, "4096"),
		})
		var buf bytes.Buffer
		if err := RegistryStatus(&buf, enabledRegistryConfig("h1"), ""); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		out := buf.String()
		for _, want := range []string{
			"state: running (healthy)",
			"image: example.com/registry:v2",
			"store: /srv/registry (",
		} {
			if !strings.Contains(out, want) {
				t.Errorf("missing %q in:\n%s", want, out)
			}
		}
	})

	t.Run("absent container prints not deployed", func(t *testing.T) {
		t.Parallel()
		installMockConnectHost(t, map[string]host.Executor{
			"h1": registryInspectingMock("", "", false, "0"),
		})
		var buf bytes.Buffer
		if err := RegistryStatus(&buf, enabledRegistryConfig("h1"), ""); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if out := buf.String(); !strings.Contains(out, "registry: not deployed") {
			t.Errorf("got %q; want 'registry: not deployed'", out)
		}
	})

	t.Run("inspect failure propagates", func(t *testing.T) {
		t.Parallel()
		installMockConnectHost(t, map[string]host.Executor{
			"h1": &testutil.MockExecutor{RunFn: func(cmd string) (string, error) {
				if strings.Contains(cmd, "{{.State.Status}}|{{.Config.Image}}") {
					return "", errors.New("batch script failed")
				}
				return "", nil
			}},
		})
		var buf bytes.Buffer
		err := RegistryStatus(&buf, enabledRegistryConfig("h1"), "")
		if err == nil || !strings.Contains(err.Error(), "inspecting registry mirror container") {
			t.Fatalf("got %v; want inspect failure", err)
		}
	})

	t.Run("connect error propagates", func(t *testing.T) {
		t.Parallel()
		sentinel := errors.New("registry-status connect failed")
		installFailingConnectHost(t, sentinel)
		var buf bytes.Buffer
		if err := RegistryStatus(&buf, enabledRegistryConfig("h1"), ""); !errors.Is(err, sentinel) {
			t.Fatalf("got %v; want wrap of sentinel", err)
		}
	})
}

// TestRegistryRemove covers the uninstall orchestrator: docker rm without
// purging, the additional storage purge with purgeData, and connect-failure
// propagation.
func TestRegistryRemove(t *testing.T) {
	t.Parallel()

	t.Run("without purgeData only the container is removed", func(t *testing.T) {
		t.Parallel()
		exec := registryRemoveOKMock()
		installMockConnectHost(t, map[string]host.Executor{"h1": exec})
		var buf bytes.Buffer
		if err := RegistryRemove(&buf, enabledRegistryConfig("h1"), "", false); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		out := buf.String()
		if !strings.Contains(out, "registry: removed gh-sr-registry") {
			t.Errorf("missing removed note; got %q", out)
		}
		if strings.Contains(out, "purged") {
			t.Errorf("purgeData=false must not purge storage; got %q", out)
		}
		for _, c := range exec.Calls {
			if strings.Contains(c, "rm -rf") {
				t.Errorf("purgeData=false must not issue rm -rf; call=%q", c)
			}
		}
	})

	t.Run("purgeData also deletes the storage dir", func(t *testing.T) {
		t.Parallel()
		exec := registryRemoveOKMock()
		installMockConnectHost(t, map[string]host.Executor{"h1": exec})
		var buf bytes.Buffer
		if err := RegistryRemove(&buf, enabledRegistryConfig("h1"), "", true); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		out := buf.String()
		for _, want := range []string{
			"registry: removed gh-sr-registry",
			"registry: purged /srv/registry",
		} {
			if !strings.Contains(out, want) {
				t.Errorf("missing %q in:\n%s", want, out)
			}
		}
		var rmRF int
		for _, c := range exec.Calls {
			if strings.Contains(c, "rm -rf") && strings.Contains(c, "/srv/registry") {
				rmRF++
			}
		}
		if rmRF != 1 {
			t.Errorf("expected exactly 1 rm -rf of the storage dir; got %d\ncalls=%v", rmRF, exec.Calls)
		}
	})

	t.Run("connect error propagates", func(t *testing.T) {
		t.Parallel()
		sentinel := errors.New("registry-remove connect failed")
		installFailingConnectHost(t, sentinel)
		var buf bytes.Buffer
		if err := RegistryRemove(&buf, enabledRegistryConfig("h1"), "", false); !errors.Is(err, sentinel) {
			t.Fatalf("got %v; want wrap of sentinel", err)
		}
	})
}

// TestImagesSync covers the `gh sr images sync` orchestrator: mirror-pulled
// prefetch loaded into runner containers, hosts without container runners
// skipped before connecting, the disabled-mirror direct-pull path, Ensure
// failure propagation, and connect-failure propagation. The empty-prefetch
// error is pinned by TestImagesSyncRequiresPrefetch.
func TestImagesSync(t *testing.T) {
	t.Parallel()

	t.Run("syncs prefetch via mirror and loads into runners", func(t *testing.T) {
		t.Parallel()
		// gh-sr-ci-1 already has the image (cached); gh-sr-ci-2 gets it
		// loaded. One pull on the host, one load, one cached skip.
		exec := registrySyncMock("sha256:abc123", map[string]bool{"gh-sr-ci-1": true})
		installMockConnectHost(t, map[string]host.Executor{"h1": exec})
		cfg := containerRunnerCfg("h1", 2)
		cfg.RegistryMirror = config.RegistryMirrorConfig{
			Enabled:     &[]bool{true}[0],
			BindAddr:    "127.0.0.1",
			StoragePath: "/srv/registry",
			Prefetch:    []string{"postgres:16"},
		}
		var buf bytes.Buffer
		if err := ImagesSync(&buf, cfg, ""); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		out := buf.String()
		if !strings.Contains(out, "pulling postgres:16 via dockerhub mirror") {
			t.Errorf("expected mirror-pulled note; got:\n%s", out)
		}
		if !strings.Contains(out, "images: 1 pulled, 1 loaded into runners (1 already cached there)") {
			t.Errorf("expected 1 pulled / 1 loaded / 1 cached; got:\n%s", out)
		}
		var tags int
		for _, c := range exec.Calls {
			if strings.Contains(c, "docker tag ") && !strings.Contains(c, "docker exec") {
				tags++
			}
		}
		if tags == 0 {
			t.Errorf("mirror-pulled image must be retagged to the canonical ref\ncalls=%v", exec.Calls)
		}
	})

	t.Run("skips hosts without container runners before connecting", func(t *testing.T) {
		t.Parallel()
		// Only h1 has a mock registered: if the orchestrator tried to connect
		// to h2, installMockConnectHost would error out the test.
		installMockConnectHost(t, map[string]host.Executor{
			"h1": registrySyncMock("sha256:abc", map[string]bool{"gh-sr-ci-1": true}),
		})
		cfg := containerRunnerCfg("h1", 1)
		addHost(cfg, "h2", "linux")
		cfg.RegistryMirror.Prefetch = []string{"postgres:16"}
		var buf bytes.Buffer
		if err := ImagesSync(&buf, cfg, ""); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if out := buf.String(); !strings.Contains(out, "Skipping host h2 (no container-mode runners)") {
			t.Errorf("missing skip note for h2; got:\n%s", out)
		}
	})

	t.Run("disabled mirror pulls directly without retag", func(t *testing.T) {
		t.Parallel()
		exec := registrySyncMock("sha256:abc", map[string]bool{"gh-sr-ci-1": true})
		installMockConnectHost(t, map[string]host.Executor{"h1": exec})
		cfg := containerRunnerCfg("h1", 1)
		cfg.RegistryMirror.Prefetch = []string{"postgres:16"}
		var buf bytes.Buffer
		if err := ImagesSync(&buf, cfg, ""); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		out := buf.String()
		if strings.Contains(out, "via dockerhub mirror") {
			t.Errorf("disabled mirror must pull directly; got:\n%s", out)
		}
		if !strings.Contains(out, "images: 1 pulled, 0 loaded into runners (1 already cached there)") {
			t.Errorf("expected direct-pull counts; got:\n%s", out)
		}
		for _, c := range exec.Calls {
			if strings.Contains(c, "docker tag ") && !strings.Contains(c, "docker exec") {
				t.Errorf("direct pull must not retag; call=%q", c)
			}
		}
	})

	t.Run("Ensure failure surfaces as the sync error", func(t *testing.T) {
		t.Parallel()
		installMockConnectHost(t, map[string]host.Executor{
			"h1": &testutil.MockExecutor{RunFn: func(cmd string) (string, error) {
				if strings.Contains(cmd, ".State.Status") && !strings.Contains(cmd, "|{{.Config.Image}}") {
					return "", errors.New("inspect failed")
				}
				return "", nil
			}},
		})
		cfg := containerRunnerCfg("h1", 1)
		enabled := true
		cfg.RegistryMirror.Enabled = &enabled
		cfg.RegistryMirror.BindAddr = "127.0.0.1"
		cfg.RegistryMirror.StoragePath = "/srv/registry"
		cfg.RegistryMirror.Prefetch = []string{"postgres:16"}
		var buf bytes.Buffer
		err := ImagesSync(&buf, cfg, "")
		if err == nil || !strings.Contains(err.Error(), "inspecting container gh-sr-registry") {
			t.Fatalf("got %v; want Ensure failure", err)
		}
	})

	t.Run("connect error propagates", func(t *testing.T) {
		t.Parallel()
		sentinel := errors.New("images-sync connect failed")
		installFailingConnectHost(t, sentinel)
		cfg := containerRunnerCfg("h1", 1)
		cfg.RegistryMirror.Prefetch = []string{"postgres:16"}
		var buf bytes.Buffer
		if err := ImagesSync(&buf, cfg, ""); !errors.Is(err, sentinel) {
			t.Fatalf("got %v; want wrap of sentinel", err)
		}
	})
}

// TestSyncPrefetchAfterStart covers the best-effort post-up/rebuild sync:
// no-op without prefetch, silent non-linux skip, and the three warning
// variants (connect, Ensure, SyncPrefetch) that never fail the caller.
func TestSyncPrefetchAfterStart(t *testing.T) {
	// prefetchCfg enables the mirror and registers one container-mode runner
	// on each named host — syncPrefetchAfterStart silently skips hosts whose
	// cfg.Runners carry no container-mode instances.
	prefetchCfg := func(hosts ...string) *config.Config {
		cfg := containerRunnerCfg(hosts[0], 1)
		for _, n := range hosts[1:] {
			addHost(cfg, n, "linux")
		}
		cfg.RegistryMirror.Prefetch = []string{"postgres:16"}
		return cfg
	}
	runner := config.RunnerConfig{
		Name:       "ci",
		Repo:       "o/r",
		Host:       "h1",
		Count:      1,
		RunnerMode: config.RunnerModeContainer,
	}

	t.Run("no prefetch is a silent no-op", func(t *testing.T) {
		t.Parallel()
		var buf bytes.Buffer
		if err := syncPrefetchAfterStart(&buf, cfgWithLocalHost("h1"), []config.RunnerConfig{runner}); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if buf.Len() != 0 {
			t.Errorf("expected no output; got %q", buf.String())
		}
	})

	t.Run("syncs and reports counts", func(t *testing.T) {
		t.Parallel()
		installMockConnectHost(t, map[string]host.Executor{
			"h1": registrySyncMock("sha256:abc", map[string]bool{"gh-sr-ci-1": true}),
		})
		var buf bytes.Buffer
		if err := syncPrefetchAfterStart(&buf, prefetchCfg("h1"), []config.RunnerConfig{runner}); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		out := buf.String()
		for _, want := range []string{
			"Prefetching images on h1...",
			"images: 1 pulled, 0 loaded into runners (1 already cached there)",
		} {
			if !strings.Contains(out, want) {
				t.Errorf("missing %q in:\n%s", want, out)
			}
		}
	})

	t.Run("non-linux host is skipped silently", func(t *testing.T) {
		t.Parallel()
		// No mock registered: a connect attempt would fail the test. The
		// runner must be container-mode so the empty-containers check passes
		// and the OS guard is what actually skips the host.
		installMockConnectHost(t, map[string]host.Executor{})
		cfg := containerRunnerCfg("h1", 1)
		cfg.Hosts["h1"] = config.HostConfig{Addr: "local", OS: "windows", Arch: "amd64"}
		cfg.RegistryMirror.Prefetch = []string{"postgres:16"}
		var buf bytes.Buffer
		if err := syncPrefetchAfterStart(&buf, cfg, []config.RunnerConfig{runner}); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if buf.Len() != 0 {
			t.Errorf("expected no output for non-linux host; got %q", buf.String())
		}
	})

	t.Run("connect failure warns without failing", func(t *testing.T) {
		t.Parallel()
		installFailingConnectHost(t, errors.New("no route"))
		var buf bytes.Buffer
		if err := syncPrefetchAfterStart(&buf, prefetchCfg("h1"), []config.RunnerConfig{runner}); err != nil {
			t.Fatalf("sync must stay best-effort; got %v", err)
		}
		if out := buf.String(); !strings.Contains(out, "Warning: image sync skipped on h1") {
			t.Errorf("missing skip warning; got %q", out)
		}
	})

	t.Run("Ensure failure warns with skipped wording", func(t *testing.T) {
		t.Parallel()
		installMockConnectHost(t, map[string]host.Executor{
			"h1": &testutil.MockExecutor{RunFn: func(cmd string) (string, error) {
				if strings.Contains(cmd, ".State.Status") && !strings.Contains(cmd, "|{{.Config.Image}}") {
					return "", errors.New("inspect failed")
				}
				return "", nil
			}},
		})
		cfg := prefetchCfg("h1")
		enabled := true
		cfg.RegistryMirror.Enabled = &enabled
		cfg.RegistryMirror.BindAddr = "127.0.0.1"
		var buf bytes.Buffer
		if err := syncPrefetchAfterStart(&buf, cfg, []config.RunnerConfig{runner}); err != nil {
			t.Fatalf("sync must stay best-effort; got %v", err)
		}
		if out := buf.String(); !strings.Contains(out, "Warning: image sync skipped on h1") {
			t.Errorf("missing skip warning; got %q", out)
		}
	})

	t.Run("pull failure warns with sync wording", func(t *testing.T) {
		t.Parallel()
		installMockConnectHost(t, map[string]host.Executor{
			"h1": &testutil.MockExecutor{RunFn: func(cmd string) (string, error) {
				switch {
				case strings.Contains(cmd, "docker pull"):
					return "", errors.New("network unreachable")
				case strings.Contains(cmd, "Config.Labels"):
					return "v2", nil
				case strings.Contains(cmd, ".State.Status"):
					return "running", nil
				}
				return "", nil
			}},
		})
		var buf bytes.Buffer
		if err := syncPrefetchAfterStart(&buf, prefetchCfg("h1"), []config.RunnerConfig{runner}); err != nil {
			t.Fatalf("sync must stay best-effort; got %v", err)
		}
		out := buf.String()
		if !strings.Contains(out, "Warning: image sync on h1") {
			t.Errorf("missing sync warning; got %q", out)
		}
		if strings.Contains(out, "Warning: image sync skipped on h1") {
			t.Errorf("pull failure must use the sync wording, not the skip wording; got %q", out)
		}
	})
}

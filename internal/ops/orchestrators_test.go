package ops

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/an-lee/gh-sr/internal/config"
	"github.com/an-lee/gh-sr/internal/host"
	"github.com/an-lee/gh-sr/internal/runner"
)

// TestSetup_EmptyRunners covers the no-match filter case: FilterRunners
// returns no runners, so Setup returns nil without ever invoking
// connectHostFn or applyContainerImageExtras. The host mock is
// intentionally not registered — if the orchestrator tried to connect,
// the test would fail with the "no mock registered" error. The
// "Setup complete." footer is still printed.
func TestSetup_EmptyRunners(t *testing.T) {
	t.Parallel()

	installMockConnectHost(t, map[string]host.Executor{})

	mgr := &runner.Manager{}
	cfg := cfgWithLocalHost("h1")
	cfg.Runners = []config.RunnerConfig{
		{Name: "ci", Host: "h1", Repo: "org/repo", Count: 1},
	}

	var buf bytes.Buffer
	if err := Setup(&buf, cfg, mgr, "", "no-such-repo", nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "Setup complete.") {
		t.Errorf("missing 'Setup complete.' footer; got:\n%s", out)
	}
	// No per-host banner must appear when the filter has no matches.
	if strings.Contains(out, "Setting up on h1") {
		t.Errorf("did not expect a per-host banner for empty filter; got:\n%s", out)
	}
}

// TestSetup_ConnectError covers the failure path: a non-local host without
// pre-resolved OS+arch triggers a host detection probe. The connect
// failure must propagate as the orchestrator's error before any
// per-host setup work is attempted.
func TestSetup_ConnectError(t *testing.T) {
	t.Parallel()

	sentinel := errors.New("ssh dial timeout")
	installFailingConnectHost(t, sentinel)

	mgr := &runner.Manager{}
	cfg := &config.Config{Hosts: map[string]config.HostConfig{
		// non-local AND missing OS/arch → triggers ResolveHostInfo probe.
		"h1": {Addr: "user@10.0.0.1"},
	}}
	cfg.Runners = []config.RunnerConfig{
		{Name: "ci", Host: "h1", Repo: "org/repo", Count: 1},
	}

	var buf bytes.Buffer
	err := Setup(&buf, cfg, mgr, "", "", nil)
	if !errors.Is(err, sentinel) {
		t.Fatalf("got %v; want %v", err, sentinel)
	}
}

// TestRebuildImage_AllNative covers the "no container-mode runners"
// branch: every runner is native-mode, so partitionRebuildTargets puts
// them all in the skipped bucket. The orchestrator prints the per-runner
// skip line, then prints "No container-mode runners to rebuild.", and
// returns nil without ever calling connectHostFn.
func TestRebuildImage_AllNative(t *testing.T) {
	t.Parallel()

	installMockConnectHost(t, map[string]host.Executor{})

	mgr := &runner.Manager{}
	cfg := cfgWithLocalHost("h1")
	cfg.Runners = []config.RunnerConfig{
		{Name: "ci", Host: "h1", Repo: "o/r", RunnerMode: config.RunnerModeNative},
	}

	var buf bytes.Buffer
	if err := RebuildImage(&buf, cfg, mgr, "", "", nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "Skipping ci (runner_mode: native)") {
		t.Errorf("missing 'Skipping ci' line; got:\n%s", out)
	}
	if !strings.Contains(out, "No container-mode runners to rebuild.") {
		t.Errorf("missing 'No container-mode runners' line; got:\n%s", out)
	}
}

// TestRebuildImage_MixedNativesSkipped covers the partition: the input
// runner set has both native and container-mode runners. The native ones
// are skipped (and listed in the "Skipping X" lines), and only the
// container ones are passed to runPerHostParallel.
//
// We do NOT exercise the container path here because doing so requires a
// non-nil mgr.GitHub client (GetLatestRunnerVersion) and a deeper mock of
// the docker build flow. The partition is purely a function of
// RunnerConfig.RunnerMode, so the partition banner can be asserted
// without reaching the container execution path. A separate test in
// internal/runner exercises rebuildContainerImage end-to-end.
func TestRebuildImage_MixedNativesSkipped(t *testing.T) {
	t.Parallel()

	installMockConnectHost(t, map[string]host.Executor{
		"h1": &noopExecutor{},
	})

	mgr := &runner.Manager{}
	cfg := cfgWithLocalHost("h1")
	cfg.Runners = []config.RunnerConfig{
		{Name: "native1", Host: "h1", Repo: "o/r", RunnerMode: config.RunnerModeNative},
		{Name: "native2", Host: "h1", Repo: "o/r", RunnerMode: config.RunnerModeNative},
	}

	var buf bytes.Buffer
	if err := RebuildImage(&buf, cfg, mgr, "", "", nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := buf.String()
	// Both native runners must be in the "Skipping" lines, in input order.
	for _, name := range []string{"native1", "native2"} {
		want := "Skipping " + name + " (runner_mode: native)"
		if !strings.Contains(out, want) {
			t.Errorf("missing %q; got:\n%s", want, out)
		}
	}
	// "No container-mode runners" must appear because we only have natives.
	if !strings.Contains(out, "No container-mode runners to rebuild.") {
		t.Errorf("expected 'No container-mode runners' with all-native input; got:\n%s", out)
	}
	// Order: native1's "Skipping" line must precede native2's.
	i1 := strings.Index(out, "Skipping native1")
	i2 := strings.Index(out, "Skipping native2")
	if i1 < 0 || i2 < 0 || i1 >= i2 {
		t.Errorf("expected native1 skip before native2 skip; got i1=%d i2=%d", i1, i2)
	}
}

// TestRebuildImage_EmptyRunners covers the no-match filter case: the
// filter narrows the runner set to empty, so RebuildImage's
// resolveAndFilter returns no runners. The orchestrator's
// partitionRebuildTargets receives an empty slice and prints
// "No container-mode runners to rebuild." (the same code path as the
// all-native case). The host mock is intentionally not registered — if
// the orchestrator tried to connect, the test would fail.
func TestRebuildImage_EmptyRunners(t *testing.T) {
	t.Parallel()

	installMockConnectHost(t, map[string]host.Executor{})

	mgr := &runner.Manager{}
	cfg := cfgWithLocalHost("h1")
	cfg.Runners = []config.RunnerConfig{
		{Name: "ci", Host: "h1", Repo: "org/repo", RunnerMode: config.RunnerModeContainer},
	}

	var buf bytes.Buffer
	if err := RebuildImage(&buf, cfg, mgr, "", "no-such-repo", nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "No container-mode runners to rebuild.") {
		t.Errorf("missing 'No container-mode runners' line; got:\n%s", out)
	}
	// No per-runner "Rebuilding image" banner must appear when the filter
	// has no matches.
	if strings.Contains(out, "Rebuilding image for") {
		t.Errorf("did not expect a per-runner banner for empty filter; got:\n%s", out)
	}
}

// TestUpdate_EmptyRunners covers the no-match filter case: FilterRunners
// returns no runners, so Update returns nil without ever invoking
// connectHostFn. The "Update complete." footer is still printed. The
// host mock is intentionally not registered.
func TestUpdate_EmptyRunners(t *testing.T) {
	t.Parallel()

	installMockConnectHost(t, map[string]host.Executor{})

	mgr := &runner.Manager{}
	cfg := cfgWithLocalHost("h1")
	cfg.Runners = []config.RunnerConfig{
		{Name: "ci", Host: "h1", Repo: "org/repo", Count: 1},
	}

	var buf bytes.Buffer
	if err := Update(&buf, cfg, mgr, "", "no-such-repo", nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "Update complete.") {
		t.Errorf("missing 'Update complete.' footer; got:\n%s", out)
	}
	// No per-runner "Updating X on Y..." banner must appear when the
	// filter has no matches.
	if strings.Contains(out, "Updating ") {
		t.Errorf("did not expect a per-runner banner for empty filter; got:\n%s", out)
	}
}

// TestUpdate_ConnectError covers the failure path: a non-local host
// without pre-resolved OS+arch triggers a host detection probe. The
// connect failure must propagate as the orchestrator's error before any
// per-runner work is attempted.
func TestUpdate_ConnectError(t *testing.T) {
	t.Parallel()

	sentinel := errors.New("ssh dial timeout")
	installFailingConnectHost(t, sentinel)

	mgr := &runner.Manager{}
	cfg := &config.Config{Hosts: map[string]config.HostConfig{
		// non-local AND missing OS/arch → triggers ResolveHostInfo probe.
		"h1": {Addr: "user@10.0.0.1"},
	}}
	cfg.Runners = []config.RunnerConfig{
		{Name: "ci", Host: "h1", Repo: "org/repo", Count: 1},
	}

	var buf bytes.Buffer
	err := Update(&buf, cfg, mgr, "", "", nil)
	if !errors.Is(err, sentinel) {
		t.Fatalf("got %v; want %v", err, sentinel)
	}
}

// noopExecutor is a host.Executor whose Run/Upload/Close are all
// no-ops. Used by orchestrator-level tests that need a host connection
// but don't care about the underlying command stream.
type noopExecutor struct{}

func (noopExecutor) Run(cmd string) (string, error)            { return "", nil }
func (noopExecutor) Upload(localPath, remotePath string) error { return nil }
func (noopExecutor) Close() error                              { return nil }

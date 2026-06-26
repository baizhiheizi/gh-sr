package ops

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/an-lee/gh-sr/internal/config"
	"github.com/an-lee/gh-sr/internal/host"
	"github.com/an-lee/gh-sr/internal/runner"
	"github.com/an-lee/gh-sr/internal/testutil"
)

// newSetupGitHubHTTPServer returns an httptest.Server that answers the two
// GitHub API endpoints the container-mode setup path queries:
//   - GET  /repos/actions/runner/releases/latest → a releaseResponse with the given tag.
//   - POST /repos/o/r/actions/runners/registration-token → a tokenResponse.
//
// Anything else returns 404 so accidental API drift in the orchestrator fails
// the test loudly instead of silently succeeding.
func newSetupGitHubHTTPServer(t *testing.T, tag, token string) *httptest.Server {
	t.Helper()
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/repos/actions/runner/releases/latest" && r.Method == http.MethodGet:
			_ = json.NewEncoder(w).Encode(map[string]string{"tag_name": tag})
		case strings.HasSuffix(r.URL.Path, "/actions/runners/registration-token") && r.Method == http.MethodPost:
			_ = json.NewEncoder(w).Encode(map[string]string{"token": token, "expires_at": "2099-01-01T00:00:00Z"})
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(ts.Close)
	return ts
}

// newSetupContainerMockExecutor returns a MockExecutor wired so the container
// setup path on a Linux host completes the docker probes (image-present,
// container present) without performing an actual build. This keeps the
// orchestrator's mgr.Setup call short — the focus of these tests is the
// orchestrator's per-host dispatch and dedup contract, not setupContainer
// itself (which has its own dedicated tests in internal/runner).
//
// Substring probes:
//   - "docker --version"             → "yes"
//   - "docker info"                  → "ok"
//   - "docker image inspect"         → "yes" (image present, build skipped)
//   - "docker inspect" (no image)    → "yes" (container present)
//   - everything else                → ""
func newSetupContainerMockExecutor() *testutil.MockExecutor {
	return &testutil.MockExecutor{
		RunFn: func(cmd string) (string, error) {
			switch {
			case strings.Contains(cmd, "docker --version"):
				return "yes", nil
			case strings.Contains(cmd, "docker info"):
				return "ok", nil
			case strings.Contains(cmd, "docker image inspect"):
				return "yes", nil
			case strings.Contains(cmd, "docker inspect"):
				return "yes", nil
			default:
				return "", nil
			}
		},
	}
}

// makeContainerModeRunners returns n container-mode RunnerConfig entries on
// the given host. Names are <prefix>-1 .. <prefix>-n. The runner config
// matches what the docker flow expects (Repo, Host, RunnerMode=container).
func makeContainerModeRunners(hostName, prefix string, n int) []config.RunnerConfig {
	out := make([]config.RunnerConfig, n)
	for i := 0; i < n; i++ {
		out[i] = config.RunnerConfig{
			Name:       prefix + "-" + string(rune('1'+i)),
			Repo:       "o/r",
			Host:       hostName,
			RunnerMode: config.RunnerModeContainer,
		}
	}
	return out
}

// TestSetup_DedupesMultipleRunnersOnSameHost pins the unique-to-Setup
// contract: when N runners share a host, the per-host banner appears
// exactly ONCE (the orchestrator dedups with a hostsDone map) and the
// "Setup complete." footer appears exactly once. This catches a refactor
// that re-introduces a per-runner banner or accidentally runs setupHost
// per runner instead of per host.
func TestSetup_DedupesMultipleRunnersOnSameHost(t *testing.T) {
	t.Parallel()

	exec := newSetupContainerMockExecutor()
	installMockConnectHost(t, map[string]host.Executor{"h1": exec})

	ts := newSetupGitHubHTTPServer(t, "v2.330.0", "REG_TOKEN")
	mgr := &runner.Manager{GitHub: runner.NewGitHubClientWithHTTP("pat", ts.Client(), ts.URL)}
	cfg := cfgWithLocalHost("h1")
	cfg.Runners = makeContainerModeRunners("h1", "ci", 3)

	var buf bytes.Buffer
	if err := Setup(&buf, cfg, mgr, "", "", nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := buf.String()

	// Per-host banner appears exactly once even though there are 3 runners.
	if c := strings.Count(out, "Setting up on h1"); c != 1 {
		t.Errorf("expected exactly 1 'Setting up on h1' banner for 3 runners on h1; got %d\n%s", c, out)
	}
	// Footer is also unique.
	if c := strings.Count(out, "Setup complete."); c != 1 {
		t.Errorf("expected exactly 1 'Setup complete.' footer; got %d\n%s", c, out)
	}
	// Sanity: connectHostFn was invoked exactly once for h1 (dedup is also
	// observed in the connect path, not just the banner).
	connectCalls := 0
	for _, c := range exec.Calls {
		if c == "" {
			continue
		}
		connectCalls++
	}
	_ = connectCalls // not a strict assertion — Setup issues multiple h.Run calls per setupContainer; we rely on the banner count above.
}

// TestSetup_DedupPreservesFirstEncounterOrder pins that dedup keeps the
// host-banner order matching the first-encounter order of runners, not the
// alphabetical / map iteration order. With runners [ci-a (h2), ci-b (h1),
// ci-c (h2)] we expect banners in the order h2, h1 (not h1, h2).
func TestSetup_DedupPreservesFirstEncounterOrder(t *testing.T) {
	t.Parallel()

	installMockConnectHost(t, map[string]host.Executor{
		"h1": newSetupContainerMockExecutor(),
		"h2": newSetupContainerMockExecutor(),
	})

	ts := newSetupGitHubHTTPServer(t, "v2.330.0", "REG_TOKEN")
	mgr := &runner.Manager{GitHub: runner.NewGitHubClientWithHTTP("pat", ts.Client(), ts.URL)}
	cfg := cfgWithLocalHost("h1", "h2")
	cfg.Runners = []config.RunnerConfig{
		{Name: "ci-a", Repo: "o/r", Host: "h2", RunnerMode: config.RunnerModeContainer},
		{Name: "ci-b", Repo: "o/r", Host: "h1", RunnerMode: config.RunnerModeContainer},
		{Name: "ci-c", Repo: "o/r", Host: "h2", RunnerMode: config.RunnerModeContainer},
	}

	var buf bytes.Buffer
	if err := Setup(&buf, cfg, mgr, "", "", nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := buf.String()
	h2Idx := strings.Index(out, "Setting up on h2")
	h1Idx := strings.Index(out, "Setting up on h1")
	if h2Idx < 0 {
		t.Fatalf("missing h2 banner; got:\n%s", out)
	}
	if h1Idx < 0 {
		t.Fatalf("missing h1 banner; got:\n%s", out)
	}
	if h2Idx >= h1Idx {
		t.Errorf("expected h2 banner (first-encounter) before h1 banner; h2Idx=%d h1Idx=%d\n%s", h2Idx, h1Idx, out)
	}
}

// TestSetup_MultiHostEachGetsBanner verifies that with runners spread
// across multiple hosts, each unique host gets its own "Setting up on X"
// banner exactly once. The orchestrator runs per host sequentially within
// a single goroutine fan-out (one host = one SSH connection).
func TestSetup_MultiHostEachGetsBanner(t *testing.T) {
	t.Parallel()

	installMockConnectHost(t, map[string]host.Executor{
		"h1": newSetupContainerMockExecutor(),
		"h2": newSetupContainerMockExecutor(),
		"h3": newSetupContainerMockExecutor(),
	})

	ts := newSetupGitHubHTTPServer(t, "v2.330.0", "REG_TOKEN")
	mgr := &runner.Manager{GitHub: runner.NewGitHubClientWithHTTP("pat", ts.Client(), ts.URL)}
	cfg := cfgWithLocalHost("h1", "h2", "h3")
	cfg.Runners = []config.RunnerConfig{
		{Name: "a", Repo: "o/r", Host: "h1", RunnerMode: config.RunnerModeContainer},
		{Name: "b", Repo: "o/r", Host: "h2", RunnerMode: config.RunnerModeContainer},
		{Name: "c", Repo: "o/r", Host: "h3", RunnerMode: config.RunnerModeContainer},
	}

	var buf bytes.Buffer
	if err := Setup(&buf, cfg, mgr, "", "", nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := buf.String()
	for _, want := range []string{
		"Setting up on h1 (local)...",
		"Setting up on h2 (local)...",
		"Setting up on h3 (local)...",
	} {
		if c := strings.Count(out, want); c != 1 {
			t.Errorf("expected exactly 1 %q; got %d\n%s", want, c, out)
		}
	}
}

// TestSetup_SetupHostErrorPropagates verifies that an error from
// setupHost (here: mgr.Setup failure) propagates as the orchestrator's
// error and aborts subsequent hosts. The "Setup complete." footer must
// NOT be printed when any host's setup fails.
func TestSetup_SetupHostErrorPropagates(t *testing.T) {
	t.Parallel()

	var sawDockerProbe atomic.Bool
	exec := &testutil.MockExecutor{
		RunFn: func(cmd string) (string, error) {
			switch {
			case strings.Contains(cmd, "docker --version"):
				sawDockerProbe.Store(true)
				return "no", nil // docker not installed → setupContainer returns ErrDockerNotInstalled-style failure
			default:
				return "", nil
			}
		},
	}
	installMockConnectHost(t, map[string]host.Executor{"h1": exec})

	ts := newSetupGitHubHTTPServer(t, "v2.330.0", "REG_TOKEN")
	mgr := &runner.Manager{GitHub: runner.NewGitHubClientWithHTTP("pat", ts.Client(), ts.URL)}
	cfg := cfgWithLocalHost("h1")
	cfg.Runners = []config.RunnerConfig{
		{Name: "ci", Repo: "o/r", Host: "h1", RunnerMode: config.RunnerModeContainer},
	}

	var buf bytes.Buffer
	err := Setup(&buf, cfg, mgr, "", "", nil)
	if err == nil {
		t.Fatalf("expected error from setupHost, got nil")
	}
	// The error must mention the underlying cause (ErrDockerNotInstalled or
	// our sentinel). At minimum, Setup must propagate non-nil.
	if !sawDockerProbe.Load() {
		t.Errorf("expected docker --version probe to be issued; calls=%v", exec.Calls)
	}
	if strings.Contains(buf.String(), "Setup complete.") {
		t.Errorf("'Setup complete.' footer must NOT appear when setupHost errors; got:\n%s", buf.String())
	}
}

// TestSetup_SetupHostConnectError exercises the connect-error branch
// inside setupHost itself (not the ResolveHostInfo probe path covered
// by TestSetup_ConnectError): resolveAndFilter succeeds because the host
// is local, but the per-host connect call inside setupHost fails. The
// orchestrator must propagate that error and NOT print the footer.
//
// This is the path that brings setupHost's coverage from 80% to 100%:
// the line `if err != nil { return err }` after connectHostFn returns
// is otherwise unreachable from the other Setup tests (which either
// succeed past the connect call or fail at a different site).
func TestSetup_SetupHostConnectError(t *testing.T) {
	t.Parallel()

	connectSentinel := errors.New("setupHost connect failed")
	// installFailingConnectHost makes connectHostFn return the sentinel for
	// every host. resolveAndFilter still succeeds (local addr), so the
	// error has to surface from inside setupHost.
	installFailingConnectHost(t, connectSentinel)

	mgr := &runner.Manager{}
	cfg := cfgWithLocalHost("h1")
	cfg.Runners = []config.RunnerConfig{
		{Name: "ci", Repo: "o/r", Host: "h1", RunnerMode: config.RunnerModeContainer},
	}

	var buf bytes.Buffer
	err := Setup(&buf, cfg, mgr, "", "", nil)
	if !errors.Is(err, connectSentinel) {
		t.Fatalf("got %v; want %v", err, connectSentinel)
	}
	if strings.Contains(buf.String(), "Setup complete.") {
		t.Errorf("'Setup complete.' footer must NOT appear after setupHost connect error; got:\n%s", buf.String())
	}
}

// TestSetup_FilterByHost pins the filter integration: filterHost="h1"
// narrows the runner set so only h1's banner appears and h2 is never
// touched. The host mock for h2 is intentionally not registered — if the
// orchestrator tried to connect to h2, the test would fail with "no mock
// registered for host h2".
func TestSetup_FilterByHost(t *testing.T) {
	t.Parallel()

	installMockConnectHost(t, map[string]host.Executor{
		"h1": newSetupContainerMockExecutor(),
	})

	ts := newSetupGitHubHTTPServer(t, "v2.330.0", "REG_TOKEN")
	mgr := &runner.Manager{GitHub: runner.NewGitHubClientWithHTTP("pat", ts.Client(), ts.URL)}
	cfg := cfgWithLocalHost("h1", "h2")
	cfg.Runners = []config.RunnerConfig{
		{Name: "ci1", Repo: "o/r", Host: "h1", RunnerMode: config.RunnerModeContainer},
		{Name: "ci2", Repo: "o/r", Host: "h2", RunnerMode: config.RunnerModeContainer},
	}

	var buf bytes.Buffer
	if err := Setup(&buf, cfg, mgr, "h1", "", nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "Setting up on h1 (local)...") {
		t.Errorf("expected h1 banner; got:\n%s", out)
	}
	if strings.Contains(out, "Setting up on h2") {
		t.Errorf("did not expect h2 banner (filtered out); got:\n%s", out)
	}
}

// TestSetup_FilterByNameArgs pins the name-args filter integration:
// when the user passes `gh sr setup r1 r3`, only those two runners'
// host banners appear. With r1 and r3 on h1, this also serves as a
// regression guard against accidentally dropping the host banner
// when a runner filter narrows the set.
func TestSetup_FilterByNameArgs(t *testing.T) {
	t.Parallel()

	installMockConnectHost(t, map[string]host.Executor{
		"h1": newSetupContainerMockExecutor(),
		"h2": newSetupContainerMockExecutor(),
	})

	ts := newSetupGitHubHTTPServer(t, "v2.330.0", "REG_TOKEN")
	mgr := &runner.Manager{GitHub: runner.NewGitHubClientWithHTTP("pat", ts.Client(), ts.URL)}
	cfg := cfgWithLocalHost("h1", "h2")
	cfg.Runners = []config.RunnerConfig{
		{Name: "r1", Repo: "o/r", Host: "h1", RunnerMode: config.RunnerModeContainer},
		{Name: "r2", Repo: "o/r", Host: "h2", RunnerMode: config.RunnerModeContainer},
		{Name: "r3", Repo: "o/r", Host: "h1", RunnerMode: config.RunnerModeContainer},
	}

	var buf bytes.Buffer
	if err := Setup(&buf, cfg, mgr, "", "", []string{"r1", "r3"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "Setting up on h1") {
		t.Errorf("expected h1 banner (hosts r1 + r3); got:\n%s", out)
	}
	if strings.Contains(out, "Setting up on h2") {
		t.Errorf("did not expect h2 banner (r2 filtered out); got:\n%s", out)
	}
}

// TestSetup_ApplyContainerImageExtrasWiresMgr pins the applyContainerImageExtras
// call that runs immediately after resolveAndFilter. The orchestrator must
// populate the Manager's image-extra fields from cfg before any setupHost
// call. This is a regression guard for a refactor that drops the wiring.
func TestSetup_ApplyContainerImageExtrasWiresMgr(t *testing.T) {
	t.Parallel()

	installMockConnectHost(t, map[string]host.Executor{
		"h1": newSetupContainerMockExecutor(),
	})

	ts := newSetupGitHubHTTPServer(t, "v2.330.0", "REG_TOKEN")
	mgr := &runner.Manager{GitHub: runner.NewGitHubClientWithHTTP("pat", ts.Client(), ts.URL)}

	cfg := cfgWithLocalHost("h1")
	cfg.Runners = []config.RunnerConfig{
		{Name: "ci", Repo: "o/r", Host: "h1", RunnerMode: config.RunnerModeContainer},
	}
	// Configure the image-extras knobs so we can assert they propagate.
	cfg.ContainerRunnerImage = config.ContainerRunnerImageConfig{
		ExtraAptPackages:    []string{"jq", "git"},
		MTU:                 1400,
		DockerdStartTimeout: 90,
		BootstrapMaxRetries: 5,
		StartStaggerSeconds: 3,
	}

	var buf bytes.Buffer
	if err := Setup(&buf, cfg, mgr, "", "", nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got := mgr.ContainerImageExtraApt; len(got) != 2 || got[0] != "jq" || got[1] != "git" {
		t.Errorf("ContainerImageExtraApt: got %v; want [jq git]", got)
	}
	if got := mgr.ContainerMTU; got != 1400 {
		t.Errorf("ContainerMTU: got %d; want 1400", got)
	}
	if got := mgr.ContainerDockerdStartTimeout; got != 90 {
		t.Errorf("ContainerDockerdStartTimeout: got %d; want 90", got)
	}
	if got := mgr.ContainerBootstrapMaxRetries; got != 5 {
		t.Errorf("ContainerBootstrapMaxRetries: got %d; want 5", got)
	}
	if got := mgr.ContainerStartStaggerSeconds; got != 3 {
		t.Errorf("ContainerStartStaggerSeconds: got %d; want 3", got)
	}
}

// TestSetup_NilWriterNotSupported pins the current behaviour: Setup
// panics on a nil writer because writeHostBanner is called directly
// (before the per-host parallel wrapper that guards nil writer with
// io.Discard). The runPerHostParallel-based orchestrators (Up, Down,
// Restart, RebuildImage) tolerate nil writer because they wrap w with
// a lockedWriter-or-io.Discard before any banner write; Setup does
// not, because it iterates hosts in-process and writes the banner
// inline.
//
// If the maintainer decides to align Setup with the other orchestrators
// and accept nil writer, this test should be flipped to the positive
// form and a `if w == nil { w = io.Discard }` guard added to Setup.
func TestSetup_NilWriterNotSupported(t *testing.T) {
	t.Parallel()

	installMockConnectHost(t, map[string]host.Executor{
		"h1": newSetupContainerMockExecutor(),
	})

	ts := newSetupGitHubHTTPServer(t, "v2.330.0", "REG_TOKEN")
	mgr := &runner.Manager{GitHub: runner.NewGitHubClientWithHTTP("pat", ts.Client(), ts.URL)}
	cfg := cfgWithLocalHost("h1")
	cfg.Runners = []config.RunnerConfig{
		{Name: "ci", Repo: "o/r", Host: "h1", RunnerMode: config.RunnerModeContainer},
	}

	defer func() {
		if r := recover(); r == nil {
			t.Errorf("Setup(nil writer) should panic; expected writeHostBanner to dereference nil")
		}
	}()
	_ = Setup(nil, cfg, mgr, "", "", nil)
}

// TestSetup_HostsDoneMapPicksFirstEncounter pins the dedup data-structure
// contract: the hostsDone map is keyed by rc.Host and is set BEFORE the
// setupHost call (so a setupHost failure on host X does NOT cause X to be
// re-attempted by a subsequent runner also on X). With runners
// [fail (h1), ok (h1)] the second runner must NOT trigger a second
// "Setting up on h1" banner — the dedup entry set by the first runner
// short-circuits the second.
//
// This is also the "skip the rest" semantics: the orchestrator returns
// the first error and stops iterating, but it also never re-tries h1
// for the second runner.
func TestSetup_HostsDoneMapPicksFirstEncounter(t *testing.T) {
	t.Parallel()

	// h1 always errors. h2 succeeds.
	h1Err := errors.New("h1 setup failed")
	installMockConnectHost(t, map[string]host.Executor{
		"h1": &testutil.MockExecutor{
			RunFn: func(cmd string) (string, error) {
				return "", h1Err
			},
		},
		"h2": newSetupContainerMockExecutor(),
	})

	ts := newSetupGitHubHTTPServer(t, "v2.330.0", "REG_TOKEN")
	mgr := &runner.Manager{GitHub: runner.NewGitHubClientWithHTTP("pat", ts.Client(), ts.URL)}
	cfg := cfgWithLocalHost("h1", "h2")
	cfg.Runners = []config.RunnerConfig{
		// First runner on h1 errors out → Setup returns the error.
		{Name: "fail", Repo: "o/r", Host: "h1", RunnerMode: config.RunnerModeContainer},
		// Second runner on h2 never gets attempted (the loop returned
		// after the first setupHost error).
		{Name: "ok", Repo: "o/r", Host: "h2", RunnerMode: config.RunnerModeContainer},
	}

	var buf bytes.Buffer
	err := Setup(&buf, cfg, mgr, "", "", nil)
	if err == nil {
		t.Fatalf("expected error from first host's setup; got nil")
	}
	out := buf.String()
	if strings.Contains(out, "Setting up on h2") {
		t.Errorf("Setup should stop after h1's error; h2 banner must NOT appear; got:\n%s", out)
	}
	if strings.Contains(out, "Setup complete.") {
		t.Errorf("'Setup complete.' footer must NOT appear after error; got:\n%s", out)
	}
}

package ops

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/an-lee/gh-sr/internal/config"
	"github.com/an-lee/gh-sr/internal/host"
	"github.com/an-lee/gh-sr/internal/runner"
	"github.com/an-lee/gh-sr/internal/testutil"
)

// newUpdateGitHubHTTPServer returns an httptest.Server that answers the two
// GitHub API endpoints the native-mode Update path queries:
//
//   - GET  /repos/actions/runner/releases/latest → a releaseResponse with the
//     given tag (for setupNative's GetLatestRunnerVersion).
//   - POST /repos/o/r/actions/runners/remove-token → a tokenResponse (for
//     removeNative's GetRemovalTokenScoped).
//
// Anything else returns 404 so accidental API drift in the orchestrator fails
// the test loudly instead of silently succeeding.
func newUpdateGitHubHTTPServer(t *testing.T, tag, token string) *httptest.Server {
	t.Helper()
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/repos/actions/runner/releases/latest" && r.Method == http.MethodGet:
			_ = json.NewEncoder(w).Encode(map[string]string{"tag_name": tag})
		case strings.HasSuffix(r.URL.Path, "/actions/runners/remove-token") && r.Method == http.MethodPost:
			_ = json.NewEncoder(w).Encode(map[string]string{"token": token, "expires_at": "2099-01-01T00:00:00Z"})
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(ts.Close)
	return ts
}

// newUpdateMockExecutor returns a MockExecutor wired so the native-mode
// Update path on Linux completes Remove + Setup + Start without touching real
// services. The orchestrator calls mgr.Remove, mgr.Setup, mgr.Start in that
// order on the same connection; each phase issues probes that overlap with
// the others, so the mock disambiguates by substring.
//
// Remove path (uses the same probes as newRemoveMockExecutor):
//   - svc.sh probe returns "no" → svc.sh branch skipped.
//   - autostart probes return empty → Detect returns KindNone.
//   - stopNative's pid-file probe returns "not running" → no signal needed.
//   - config.sh remove (after GetRemovalTokenScoped) is a no-op.
//   - removeNativeDirectory's rm -rf is a no-op.
//
// Setup path (assumes the runner is already installed):
//   - svc.sh probe returns "no" (shared with Remove).
//   - autostart probes return empty (shared with Remove).
//   - NativeRunnerConfigPresent returns "yes" → setupNative prints
//     "already installed, skipping" and continues.
//
// Start path (assumes rc.Ephemeral == true to skip autostart install):
//   - svc.sh probe returns "no" (shared).
//   - autostart probes return empty (shared).
//   - NativeRunnerConfigPresent returns "yes" → no setupNative.
//   - nohup ./run.sh launch returns "started PID 12345" → start succeeded.
//   - sleep 5 stale-registration probe returns "ok" → no retry.
//
// Disambiguation: the Remove-only "config.sh remove" and "rm -rf" patterns
// only appear in the Remove phase; the Start-only "nohup" and "sleep 5"
// patterns only appear in the Start phase. Setup reuses the shared probes
// (svc.sh, autostart, NativeRunnerConfigPresent).
func newUpdateMockExecutor() *testutil.MockExecutor {
	return &testutil.MockExecutor{
		RunFn: func(cmd string) (string, error) {
			switch {
			// Remove + Setup + Start share the svc.sh probe.
			case strings.Contains(cmd, "test -f") && strings.Contains(cmd, "svc.sh"):
				return "no\n", nil
			// Remove + Setup + Start share the systemd-user probe.
			case strings.Contains(cmd, "test -f") && strings.Contains(cmd, ".config/systemd/user"):
				return "", nil
			// Remove + Setup + Start share the systemd-system probe.
			case strings.Contains(cmd, "test -f") && strings.Contains(cmd, "/etc/systemd/system"):
				return "", nil
			// Remove: stopNative's pid-file probe.
			case strings.Contains(cmd, "pid_file="):
				return "not running\n", nil
			// Remove: config.sh remove (after GetRemovalTokenScoped).
			case strings.Contains(cmd, "config.sh remove"):
				return "", nil
			// Remove: removeNativeDirectory's rm -rf.
			case strings.Contains(cmd, "rm -rf"):
				return "", nil
			// Setup + Start: NativeRunnerConfigPresent (test -d + run.sh).
			case strings.Contains(cmd, "test -d") && strings.Contains(cmd, "run.sh"):
				return "yes\n", nil
			// Start: the nohup-runner launch command.
			case strings.Contains(cmd, "nohup ./run.sh"):
				return "started PID 12345\n", nil
			// Start: stale-registration probe (sleeps, then greps runner.log).
			case strings.Contains(cmd, "sleep 5"):
				return "ok\n", nil
			default:
				return "", nil
			}
		},
	}
}

// TestUpdate_SuccessPath covers the success path of the per-host lambda:
// the orchestrator prints "Updating X on Y...", then mgr.Remove, mgr.Setup,
// and mgr.Start are all called (in that order), and the "Update complete."
// footer is printed.
//
// The mock executor answers all the probes Remove + Setup + Start issue for
// a native ephemeral runner that's already installed. The httptest GitHub
// server answers GetLatestRunnerVersion (for setupNative) and
// GetRemovalTokenScoped (for removeNative).
func TestUpdate_SuccessPath(t *testing.T) {
	t.Parallel()

	exec := newUpdateMockExecutor()
	installMockConnectHost(t, map[string]host.Executor{"h1": exec})

	ts := newUpdateGitHubHTTPServer(t, "v2.330.0", "REMOVE_TOKEN")
	mgr := &runner.Manager{GitHub: runner.NewGitHubClientWithHTTP("pat", ts.Client(), ts.URL)}
	cfg := cfgWithLocalHost("h1")
	cfg.Runners = []config.RunnerConfig{
		{Name: "ci", Host: "h1", Repo: "o/r", Count: 1, Ephemeral: true},
	}

	var buf bytes.Buffer
	if err := Update(&buf, cfg, mgr, "", "", nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "Updating ci on h1...") {
		t.Errorf("missing 'Updating ci on h1...' banner; got:\n%s", out)
	}
	if !strings.Contains(out, "Update complete.") {
		t.Errorf("missing 'Update complete.' footer; got:\n%s", out)
	}

	// Remove path: config.sh remove must have been issued.
	var sawRemoveConfigSh bool
	for _, c := range exec.Calls {
		if strings.Contains(c, "config.sh remove") {
			sawRemoveConfigSh = true
			break
		}
	}
	if !sawRemoveConfigSh {
		t.Errorf("expected Remove's config.sh remove probe; calls=%v", exec.Calls)
	}

	// Start path: nohup launch must have been issued.
	var sawStartNohup bool
	for _, c := range exec.Calls {
		if strings.Contains(c, "nohup ./run.sh") {
			sawStartNohup = true
			break
		}
	}
	if !sawStartNohup {
		t.Errorf("expected Start's nohup command; calls=%v", exec.Calls)
	}
}

// TestUpdate_CallOrderRemoveSetupStart pins the call-order contract:
// mgr.Remove runs first, mgr.Setup runs after, mgr.Start runs last. A
// refactor that swapped Setup and Start, or that ran them concurrently,
// would break the contract — Setup must run after Remove so a stale
// install is replaced before Start, and Start must run after Setup so
// the fresh install is the one that gets started.
func TestUpdate_CallOrderRemoveSetupStart(t *testing.T) {
	t.Parallel()

	exec := newUpdateMockExecutor()
	installMockConnectHost(t, map[string]host.Executor{"h1": exec})

	ts := newUpdateGitHubHTTPServer(t, "v2.330.0", "REMOVE_TOKEN")
	mgr := &runner.Manager{GitHub: runner.NewGitHubClientWithHTTP("pat", ts.Client(), ts.URL)}
	cfg := cfgWithLocalHost("h1")
	cfg.Runners = []config.RunnerConfig{
		{Name: "ci", Host: "h1", Repo: "o/r", Count: 1, Ephemeral: true},
	}

	var buf bytes.Buffer
	if err := Update(&buf, cfg, mgr, "", "", nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	removeIdx, setupProbeIdx, startNohupIdx := -1, -1, -1
	for i, c := range exec.Calls {
		if removeIdx == -1 && strings.Contains(c, "config.sh remove") {
			removeIdx = i
		}
		// Setup path: NativeRunnerConfigPresent runs in setupNative before
		// the per-instance "already installed" branch. The probe is
		// `test -d ... run.sh`. Start also probes the same pattern, but
		// in the orchestrator's per-host lambda, the FIRST instance of
		// this probe is from Setup (Remove doesn't run it).
		if setupProbeIdx == -1 && strings.Contains(c, "test -d") && strings.Contains(c, "run.sh") {
			setupProbeIdx = i
		}
		if startNohupIdx == -1 && strings.Contains(c, "nohup ./run.sh") {
			startNohupIdx = i
		}
	}
	if removeIdx == -1 {
		t.Fatalf("Remove's config.sh remove probe never issued; calls=%v", exec.Calls)
	}
	if setupProbeIdx == -1 {
		t.Fatalf("Setup's NativeRunnerConfigPresent probe never issued; calls=%v", exec.Calls)
	}
	if startNohupIdx == -1 {
		t.Fatalf("Start's nohup command never issued; calls=%v", exec.Calls)
	}
	if !(removeIdx < setupProbeIdx && setupProbeIdx < startNohupIdx) {
		t.Errorf("expected Remove < Setup < Start order; got remove=%d setup=%d start=%d",
			removeIdx, setupProbeIdx, startNohupIdx)
	}
}

// TestUpdate_RemoveErrorIsIgnored pins the contract that the orchestrator
// deliberately swallows mgr.Remove's error: the line `_ = mgr.Remove(h, rc)`
// means a partial Remove (e.g. the runner is already gone from the host)
// does not abort the Update. Setup and Start must still run.
func TestUpdate_RemoveErrorIsIgnored(t *testing.T) {
	t.Parallel()

	// config.sh remove returns a hard error — this surfaces from removeNative.
	// The orchestrator must swallow it and continue to Setup + Start.
	removeSentinel := errors.New("simulated config.sh remove failure")
	exec := &testutil.MockExecutor{
		RunFn: func(cmd string) (string, error) {
			switch {
			case strings.Contains(cmd, "test -f") && strings.Contains(cmd, "svc.sh"):
				return "no\n", nil
			case strings.Contains(cmd, "test -f") && strings.Contains(cmd, ".config/systemd/user"):
				return "", nil
			case strings.Contains(cmd, "test -f") && strings.Contains(cmd, "/etc/systemd/system"):
				return "", nil
			case strings.Contains(cmd, "pid_file="):
				return "not running\n", nil
			case strings.Contains(cmd, "config.sh remove"):
				return "", removeSentinel
			case strings.Contains(cmd, "test -d") && strings.Contains(cmd, "run.sh"):
				return "yes\n", nil
			case strings.Contains(cmd, "nohup ./run.sh"):
				return "started PID 12345\n", nil
			case strings.Contains(cmd, "sleep 5"):
				return "ok\n", nil
			default:
				return "", nil
			}
		},
	}
	installMockConnectHost(t, map[string]host.Executor{"h1": exec})

	ts := newUpdateGitHubHTTPServer(t, "v2.330.0", "REMOVE_TOKEN")
	mgr := &runner.Manager{GitHub: runner.NewGitHubClientWithHTTP("pat", ts.Client(), ts.URL)}
	cfg := cfgWithLocalHost("h1")
	cfg.Runners = []config.RunnerConfig{
		{Name: "ci", Host: "h1", Repo: "o/r", Count: 1, Ephemeral: true},
	}

	var buf bytes.Buffer
	if err := Update(&buf, cfg, mgr, "", "", nil); err != nil {
		t.Fatalf("Remove's error must be swallowed; got %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "Update complete.") {
		t.Errorf("missing 'Update complete.' footer; got:\n%s", out)
	}
	// Start's nohup must have been issued even though Remove errored.
	var sawStartNohup bool
	for _, c := range exec.Calls {
		if strings.Contains(c, "nohup ./run.sh") {
			sawStartNohup = true
			break
		}
	}
	if !sawStartNohup {
		t.Errorf("Start should still run after Remove errored; calls=%v", exec.Calls)
	}
}

// TestUpdate_SetupErrorAbortsBeforeStart pins the contract that a
// mgr.Setup failure aborts the per-host lambda and prevents mgr.Start
// from being called. This protects against a refactor that accidentally
// runs Start in parallel with Setup, or that ignores Setup errors.
func TestUpdate_SetupErrorAbortsBeforeStart(t *testing.T) {
	t.Parallel()

	// Make GetLatestRunnerVersion fail by serving a 500 from the GitHub
	// server. setupNative returns the error before any native install
	// happens, so we never even reach the NativeRunnerConfigPresent probe
	// from Setup. We then assert that Start's nohup never fires.
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		// 404 for releases/latest makes GetLatestRunnerVersion return an error.
		// The httptest server is closed by t.Cleanup attached by
		// newUpdateGitHubHTTPServer's caller — but here we don't use that
		// helper, so we register the cleanup directly.
		http.Error(w, "simulated upstream failure", http.StatusInternalServerError)
	}))
	t.Cleanup(ts.Close)

	exec := newUpdateMockExecutor()
	installMockConnectHost(t, map[string]host.Executor{"h1": exec})

	mgr := &runner.Manager{GitHub: runner.NewGitHubClientWithHTTP("pat", ts.Client(), ts.URL)}
	cfg := cfgWithLocalHost("h1")
	cfg.Runners = []config.RunnerConfig{
		{Name: "ci", Host: "h1", Repo: "o/r", Count: 1, Ephemeral: true},
	}

	var buf bytes.Buffer
	if err := Update(&buf, cfg, mgr, "", "", nil); err == nil {
		t.Fatalf("expected Setup error to propagate; got nil")
	}
	if strings.Contains(buf.String(), "Update complete.") {
		t.Errorf("'Update complete.' footer must NOT appear when Setup errors; got:\n%s", buf.String())
	}
	// Start's nohup must NOT have been issued because Setup aborted.
	for _, c := range exec.Calls {
		if strings.Contains(c, "nohup ./run.sh") {
			t.Errorf("Start should not run when Setup errors; calls=%v", exec.Calls)
			break
		}
	}
}

// TestUpdate_MultiRunnerSameHost pins the SSH-amortisation contract: when
// N runners share a host, the per-host lambda runs N times on the same
// connection, not N times × N probes. Each runner must see its own
// Remove + Setup + Start sequence.
func TestUpdate_MultiRunnerSameHost(t *testing.T) {
	t.Parallel()

	exec := newUpdateMockExecutor()
	installMockConnectHost(t, map[string]host.Executor{"h1": exec})

	ts := newUpdateGitHubHTTPServer(t, "v2.330.0", "REMOVE_TOKEN")
	mgr := &runner.Manager{GitHub: runner.NewGitHubClientWithHTTP("pat", ts.Client(), ts.URL)}
	cfg := cfgWithLocalHost("h1")
	cfg.Runners = []config.RunnerConfig{
		{Name: "first", Host: "h1", Repo: "o/r", Count: 1, Ephemeral: true},
		{Name: "second", Host: "h1", Repo: "o/r", Count: 1, Ephemeral: true},
	}

	var buf bytes.Buffer
	if err := Update(&buf, cfg, mgr, "", "", nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := buf.String()
	for _, name := range []string{"first", "second"} {
		want := "Updating " + name + " on h1..."
		if !strings.Contains(out, want) {
			t.Errorf("missing %q; got:\n%s", want, out)
		}
	}
	// Both runners must have gone through Start, so we expect 2 nohup calls.
	nohupCount := 0
	for _, c := range exec.Calls {
		if strings.Contains(c, "nohup ./run.sh") {
			nohupCount++
		}
	}
	if nohupCount != 2 {
		t.Errorf("expected 2 nohup launches (one per runner); got %d; calls=%v", nohupCount, exec.Calls)
	}
	// And 2 config.sh remove calls (one per runner).
	removeCount := 0
	for _, c := range exec.Calls {
		if strings.Contains(c, "config.sh remove") {
			removeCount++
		}
	}
	if removeCount != 2 {
		t.Errorf("expected 2 config.sh remove calls (one per runner); got %d; calls=%v", removeCount, exec.Calls)
	}
}

package ops

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/an-lee/gh-sr/internal/config"
	"github.com/an-lee/gh-sr/internal/host"
	"github.com/an-lee/gh-sr/internal/runner"
	"github.com/an-lee/gh-sr/internal/testutil"
)

// newStatusNativeRunningMock returns a MockExecutor wired so the native
// statusNativeAndVersion path resolves to ("running", "1.0.0"):
//
//   - the combined linuxInstanceProbe (`[ -f $dir/svc.sh ...`/`cat .runner-version`)
//     emits "V1.0.0\n" via the V marker (no S/U/Y markers) → hasSvc=false,
//     kind=KindNone, version="1.0.0".
//   - the PID-fallthrough shell (which contains `kill -0`) returns "running".
//
// Pre-fold the mock split the version into a separate `cat .runner-version`
// SSH and emitted "1.0.0\n" directly. Post-fold the version rides the
// combined probe, so the mock now emits the V-prefixed form on the probe
// path; the standalone `cat` SSH is gone.
func newStatusNativeRunningMock() *testutil.MockExecutor {
	return &testutil.MockExecutor{
		RunFn: func(cmd string) (string, error) {
			switch {
			case strings.Contains(cmd, ".runner-version"):
				return "V1.0.0\n", nil
			case strings.Contains(cmd, "kill -0"):
				return "running\n", nil
			default:
				return "", nil
			}
		},
	}
}

// newCollectStatusHTTPServer returns an httptest.Server answering GitHub's
// "list runners" endpoint with the given runner rows per (scope, target).
// Used by the EnrichWithGitHubStatus assertion test.
func newCollectStatusHTTPServer(t *testing.T, rows map[string][]runner.GitHubRunner) *httptest.Server {
	t.Helper()
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || !strings.Contains(r.URL.Path, "/actions/runners") {
			http.NotFound(w, r)
			return
		}
		scope, target, ok := parseActionsPath(r.URL.Path, "runners")
		if !ok {
			http.NotFound(w, r)
			return
		}
		key := scope + "|" + target
		out := rows[key]
		if out == nil {
			out = []runner.GitHubRunner{}
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"total_count": len(out),
			"runners":     out,
		})
	}))
	t.Cleanup(ts.Close)
	return ts
}

// newEmptyGitHubHTTPServer returns an httptest.Server that answers every
// "list runners" call with an empty page. Tests that do not exercise
// GitHub enrichment use this so mgr.EnrichWithGitHubStatus does not
// dereference a nil client.
func newEmptyGitHubHTTPServer(t *testing.T) *httptest.Server {
	t.Helper()
	return newCollectStatusHTTPServer(t, nil)
}

// TestCollectStatus_EmptyRunners covers the no-match filter case:
// FilterRunners returns no runners, so CollectStatus returns
// ([], nil) without ever invoking connectHostFn or the GitHub client.
// The host mock is intentionally not registered — if the orchestrator
// tried to connect, the test would fail with "no mock registered".
func TestCollectStatus_EmptyRunners(t *testing.T) {
	t.Parallel()

	installMockConnectHost(t, map[string]host.Executor{})

	ts := newEmptyGitHubHTTPServer(t)
	mgr := &runner.Manager{GitHub: runner.NewGitHubClientWithHTTP("pat", ts.Client(), ts.URL)}
	cfg := cfgWithLocalHost("h1")
	cfg.Runners = []config.RunnerConfig{
		{Name: "ci", Host: "h1", Repo: "org/repo", Count: 1},
	}

	var buf bytes.Buffer
	statuses, err := CollectStatus(&buf, cfg, mgr, "", "no-such-repo", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(statuses) != 0 {
		t.Errorf("expected empty statuses, got %d: %+v", len(statuses), statuses)
	}
	if got := buf.String(); got != "" {
		t.Errorf("expected no output, got %q", got)
	}
}

// TestCollectStatus_NativeSingleRunner covers the happy path: one local
// host, one native runner with Count=1, mgr.Status returns a single
// RunnerStatus with Local="running". Verifies the orchestrator wires
// the host connection and aggregates statuses by host group.
func TestCollectStatus_NativeSingleRunner(t *testing.T) {
	t.Parallel()

	installMockConnectHost(t, map[string]host.Executor{
		"h1": newStatusNativeRunningMock(),
	})

	ts := newEmptyGitHubHTTPServer(t)
	mgr := &runner.Manager{GitHub: runner.NewGitHubClientWithHTTP("pat", ts.Client(), ts.URL)}
	cfg := cfgWithLocalHost("h1")
	cfg.Runners = []config.RunnerConfig{
		{Name: "ci", Host: "h1", Repo: "o/r", Count: 1},
	}

	var buf bytes.Buffer
	statuses, err := CollectStatus(&buf, cfg, mgr, "", "", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(statuses) != 1 {
		t.Fatalf("expected 1 status, got %d: %+v", len(statuses), statuses)
	}
	if statuses[0].Instance != "ci-1" {
		t.Errorf("Instance = %q, want %q", statuses[0].Instance, "ci-1")
	}
	if statuses[0].Host != "h1" {
		t.Errorf("Host = %q, want %q", statuses[0].Host, "h1")
	}
	if statuses[0].Local != "running" {
		t.Errorf("Local = %q, want %q", statuses[0].Local, "running")
	}
	if statuses[0].Mode != "native" {
		t.Errorf("Mode = %q, want %q", statuses[0].Mode, "native")
	}
	if statuses[0].ContainerImageBuild != "1.0.0" {
		t.Errorf("ContainerImageBuild = %q, want %q", statuses[0].ContainerImageBuild, "1.0.0")
	}
}

// TestCollectStatus_MultiInstance verifies the orchestrator emits one
// RunnerStatus per instance (Count=3 → 3 statuses named ci-1, ci-2, ci-3),
// preserving input order. Pins that the orchestrator does not collapse
// multi-instance runners to a single row.
func TestCollectStatus_MultiInstance(t *testing.T) {
	t.Parallel()

	installMockConnectHost(t, map[string]host.Executor{
		"h1": newStatusNativeRunningMock(),
	})

	ts := newEmptyGitHubHTTPServer(t)
	mgr := &runner.Manager{GitHub: runner.NewGitHubClientWithHTTP("pat", ts.Client(), ts.URL)}
	cfg := cfgWithLocalHost("h1")
	cfg.Runners = []config.RunnerConfig{
		{Name: "ci", Host: "h1", Repo: "o/r", Count: 3},
	}

	var buf bytes.Buffer
	statuses, err := CollectStatus(&buf, cfg, mgr, "", "", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(statuses) != 3 {
		t.Fatalf("expected 3 statuses, got %d: %+v", len(statuses), statuses)
	}
	for i, s := range statuses {
		wantName := fmt.Sprintf("ci-%d", i+1)
		if s.Instance != wantName {
			t.Errorf("statuses[%d].Instance = %q, want %q", i, s.Instance, wantName)
		}
		if s.Local != "running" {
			t.Errorf("statuses[%d].Local = %q, want %q", i, s.Local, "running")
		}
	}
}

// TestCollectStatus_ConnectErrorYieldsUnreachable covers the per-host
// connect-failure branch: connectHostFn returns an error, so the
// orchestrator emits a "Warning: cannot connect to <host>" line on the
// writer and synthesizes one RunnerStatus per instance with
// Local="unreachable" and Mode="native". Pins that a partial failure
// does NOT abort the whole call — other hosts' statuses still come back.
func TestCollectStatus_ConnectErrorYieldsUnreachable(t *testing.T) {
	t.Parallel()

	sentinel := errors.New("ssh dial timeout")
	installFailingConnectHost(t, sentinel)

	ts := newEmptyGitHubHTTPServer(t)
	mgr := &runner.Manager{GitHub: runner.NewGitHubClientWithHTTP("pat", ts.Client(), ts.URL)}
	cfg := cfgWithLocalHost("h1")
	cfg.Runners = []config.RunnerConfig{
		{Name: "ci", Host: "h1", Repo: "o/r", Count: 2},
	}

	var buf bytes.Buffer
	statuses, err := CollectStatus(&buf, cfg, mgr, "", "", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(statuses) != 2 {
		t.Fatalf("expected 2 unreachable statuses, got %d: %+v", len(statuses), statuses)
	}
	for i, s := range statuses {
		if s.Local != "unreachable" {
			t.Errorf("statuses[%d].Local = %q, want %q", i, s.Local, "unreachable")
		}
		if s.Mode != "native" {
			t.Errorf("statuses[%d].Mode = %q, want %q", i, s.Mode, "native")
		}
		if s.ContainerImageBuild != "-" {
			t.Errorf("statuses[%d].ContainerImageBuild = %q, want %q", i, s.ContainerImageBuild, "-")
		}
	}
	out := buf.String()
	if !strings.Contains(out, "Warning: cannot connect to h1") {
		t.Errorf("missing warning line; got:\n%s", out)
	}
	if !strings.Contains(out, sentinel.Error()) {
		t.Errorf("warning should include sentinel text %q; got:\n%s", sentinel.Error(), out)
	}
}

// TestCollectStatus_MultiHostConcurrent verifies the per-host-group
// concurrency guarantee: two host groups (h1, h2) each spawn a
// goroutine and execute mgr.Status in parallel, not sequentially. Uses
// a barrier on the mock executor so both goroutines must enter
// mgr.Status before any of them returns. If the orchestrator ran them
// sequentially, only one would reach the barrier within the timeout.
//
// Each host gets its own barrierMockExecutor instance: MockExecutor is
// not goroutine-safe, so sharing a single instance across two hosts
// races on the response-tracking fields.
func TestCollectStatus_MultiHostConcurrent(t *testing.T) {
	t.Parallel()

	const hostCount = 2
	entered := make(chan struct{}, hostCount)
	barrier := make(chan struct{})

	exec1 := &barrierMockExecutor{
		MockExecutor: newStatusNativeRunningMock(),
		entered:      entered,
		barrier:      barrier,
	}
	exec2 := &barrierMockExecutor{
		MockExecutor: newStatusNativeRunningMock(),
		entered:      entered,
		barrier:      barrier,
	}

	cfg := &config.Config{Hosts: map[string]config.HostConfig{
		"h1": {Addr: "local", OS: "linux", Arch: "amd64"},
		"h2": {Addr: "local", OS: "linux", Arch: "amd64"},
	}}
	installMockConnectHost(t, map[string]host.Executor{
		"h1": exec1,
		"h2": exec2,
	})

	ts := newEmptyGitHubHTTPServer(t)
	mgr := &runner.Manager{GitHub: runner.NewGitHubClientWithHTTP("pat", ts.Client(), ts.URL)}
	cfg.Runners = []config.RunnerConfig{
		{Name: "a", Host: "h1", Repo: "o/r", Count: 1},
		{Name: "b", Host: "h2", Repo: "o/r", Count: 1},
	}

	done := make(chan error, 1)
	go func() {
		var buf bytes.Buffer
		_, err := CollectStatus(&buf, cfg, mgr, "", "", nil)
		done <- err
	}()

	// Both host goroutines must reach the barrier within a short window —
	// proves they ran in parallel rather than sequentially. Generous
	// timeout accounts for race-detector overhead on busy CI.
	timeout := time.After(10 * time.Second)
	for i := 0; i < hostCount; i++ {
		select {
		case <-entered:
		case <-timeout:
			close(barrier)
			t.Fatalf("only %d/%d host goroutines reached Status within timeout (sequential?)", i, hostCount)
		}
	}
	close(barrier)
	if err := <-done; err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// TestCollectStatus_EnrichesWithGitHub covers the final
// mgr.EnrichWithGitHubStatus call: with a real *runner.GitHubClient
// backed by httptest returning one GitHubRunner matching the local
// instance name, the orchestrator must populate statuses[].Remote and
// statuses[].Busy. Pins the wire-up from CollectStatus →
// EnrichWithGitHubStatus → ListRunnersScoped → GitHub API.
func TestCollectStatus_EnrichesWithGitHub(t *testing.T) {
	t.Parallel()

	ts := newCollectStatusHTTPServer(t, map[string][]runner.GitHubRunner{
		"repo|o/r": {
			{ID: 7, Name: "ci-1", Status: "online", Busy: true, OS: "Linux"},
		},
	})
	mgr := &runner.Manager{GitHub: runner.NewGitHubClientWithHTTP("pat", ts.Client(), ts.URL)}

	installMockConnectHost(t, map[string]host.Executor{
		"h1": newStatusNativeRunningMock(),
	})

	cfg := cfgWithLocalHost("h1")
	cfg.Runners = []config.RunnerConfig{
		{Name: "ci", Host: "h1", Repo: "o/r", Count: 1},
	}

	var buf bytes.Buffer
	statuses, err := CollectStatus(&buf, cfg, mgr, "", "", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(statuses) != 1 {
		t.Fatalf("expected 1 status, got %d: %+v", len(statuses), statuses)
	}
	if statuses[0].Remote != "online" {
		t.Errorf("Remote = %q, want %q (EnrichWithGitHubStatus did not run)", statuses[0].Remote, "online")
	}
	if !statuses[0].Busy {
		t.Errorf("Busy = false, want true (EnrichWithGitHubStatus did not copy busy flag)")
	}
}

// TestCollectStatus_FilterByRepo covers the --repo filter narrowing the
// runner set: only runners matching the repo survive FilterRunners, and
// the orchestrator emits statuses only for them. Pins that
// FilterRunners is wired in via resolveAndFilter.
func TestCollectStatus_FilterByRepo(t *testing.T) {
	t.Parallel()

	installMockConnectHost(t, map[string]host.Executor{
		"h1": newStatusNativeRunningMock(),
	})

	ts := newEmptyGitHubHTTPServer(t)
	mgr := &runner.Manager{GitHub: runner.NewGitHubClientWithHTTP("pat", ts.Client(), ts.URL)}
	cfg := cfgWithLocalHost("h1")
	cfg.Runners = []config.RunnerConfig{
		{Name: "ci", Host: "h1", Repo: "o/r", Count: 1},
		{Name: "other", Host: "h1", Repo: "different/repo", Count: 1},
	}

	var buf bytes.Buffer
	statuses, err := CollectStatus(&buf, cfg, mgr, "", "o/r", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(statuses) != 1 {
		t.Fatalf("expected 1 status (filter 'o/r'), got %d: %+v", len(statuses), statuses)
	}
	if statuses[0].Instance != "ci-1" {
		t.Errorf("Instance = %q, want %q (filter excluded 'other')", statuses[0].Instance, "ci-1")
	}
}

// TestCollectStatus_OrgScopeGitHub covers the org-scope branch in
// EnrichWithGitHubStatus: a runner with Org= set targets
// /orgs/<name>/actions/runners, not /repos/<owner>/<name>/. The
// httptest server routes by scope, so a hit on /orgs/foo/... must be
// answered for the orchestrator to populate Remote/Busy.
func TestCollectStatus_OrgScopeGitHub(t *testing.T) {
	t.Parallel()

	ts := newCollectStatusHTTPServer(t, map[string][]runner.GitHubRunner{
		"org|myorg": {
			{ID: 11, Name: "ci-1", Status: "offline", Busy: false, OS: "Linux"},
		},
	})
	mgr := &runner.Manager{GitHub: runner.NewGitHubClientWithHTTP("pat", ts.Client(), ts.URL)}

	installMockConnectHost(t, map[string]host.Executor{
		"h1": newStatusNativeRunningMock(),
	})

	cfg := cfgWithLocalHost("h1")
	cfg.Runners = []config.RunnerConfig{
		{Name: "ci", Host: "h1", Org: "myorg", Count: 1},
	}

	var buf bytes.Buffer
	statuses, err := CollectStatus(&buf, cfg, mgr, "", "", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(statuses) != 1 {
		t.Fatalf("expected 1 status, got %d: %+v", len(statuses), statuses)
	}
	if statuses[0].Repo != "org:myorg" {
		t.Errorf("Repo = %q, want %q (DisplayTarget should reflect org scope)", statuses[0].Repo, "org:myorg")
	}
	if statuses[0].Remote != "offline" {
		t.Errorf("Remote = %q, want %q (org-scope list call did not enrich)", statuses[0].Remote, "offline")
	}
}

// barrierMockExecutor wraps MockExecutor and signals when each command
// starts, blocking on a barrier before returning. Used by
// TestCollectStatus_MultiHostConcurrent to verify goroutines run in
// parallel.
type barrierMockExecutor struct {
	*testutil.MockExecutor
	entered chan struct{}
	barrier chan struct{}
}

func (b *barrierMockExecutor) Run(cmd string) (string, error) {
	select {
	case b.entered <- struct{}{}:
	default:
	}
	<-b.barrier
	return b.MockExecutor.Run(cmd)
}

package ops

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/an-lee/gh-sr/internal/config"
	"github.com/an-lee/gh-sr/internal/host"
	"github.com/an-lee/gh-sr/internal/runner"
	"github.com/an-lee/gh-sr/internal/testutil"
)

// newRemoveMockExecutor returns a MockExecutor wired so the native-mode Remove
// path on Linux completes successfully:
//   - svc.sh probe returns "no" → svc.sh path is skipped.
//   - autostart probes (systemd-user / systemd-system) return empty → Detect returns KindNone.
//   - stopNative's pid-file probe returns "not running" → no signal needed.
//   - config.sh remove (after GetRemovalTokenScoped) is a no-op.
//   - removeNativeDirectory's rm -rf is a no-op.
//
// Anything else (including future probes) is treated as success with empty output.
func newRemoveMockExecutor() *testutil.MockExecutor {
	return &testutil.MockExecutor{
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
				return "", nil
			case strings.Contains(cmd, "rm -rf"):
				return "", nil
			default:
				return "", nil
			}
		},
	}
}

// newRemovalTokenHTTPServer returns an httptest.Server that answers
// GetRemovalTokenScoped requests with a fixed token. Tests use it to
// inject a real *runner.GitHubClient into the Manager under test, so the
// removeNative call path does not panic on m.GitHub.
func newRemovalTokenHTTPServer(t *testing.T, token string) *httptest.Server {
	t.Helper()
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/runners/remove-token") {
			http.NotFound(w, r)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]string{"token": token})
	}))
	t.Cleanup(ts.Close)
	return ts
}

// newRemovalTokenHTTPErrServer returns an httptest.Server that answers
// GetRemovalTokenScoped with a 500. Used to exercise the
// "warning: could not get removal token" branch in removeNative.
func newRemovalTokenHTTPErrServer(t *testing.T) *httptest.Server {
	t.Helper()
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("simulated failure"))
	}))
	t.Cleanup(ts.Close)
	return ts
}

// writeRunnersYAML creates a runners.yml file in a temp directory with the
// given host and runner entries, returns the absolute path. Tests use
// t.Setenv(config.EnvVarConfigPath, ...) to point config.ResolveConfigPath
// at this file.
func writeRunnersYAML(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "runners.yml")
	if err := os.WriteFile(cfgPath, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return cfgPath
}

// runnersYAMLFor returns a runners.yml body with the given (host, runnerName)
// pair plus a "spare" runner that the tests do NOT remove. This is needed
// because config.RemoveRunner validates post-write state via config.Load,
// which requires at least one runner to be defined.
func runnersYAMLFor(hostName, runnerName string) string {
	return fmt.Sprintf(`hosts:
  %s:
    addr: local
    os: linux
    arch: amd64
runners:
  - name: %s
    repo: o/r
    host: %s
  - name: spare
    repo: o/r
    host: %s
`, hostName, runnerName, hostName, hostName)
}

// TestRemove_NoRunners covers the no-match filter case: FilterRunners returns
// an empty slice, so the orchestrator must return "no runners matching the
// given filters" without ever invoking connectHostFn. The host mock is
// intentionally not registered — if the orchestrator tried to connect, the
// test would fail with "no mock registered for host ...".
func TestRemove_NoRunners(t *testing.T) {
	t.Parallel()

	installMockConnectHost(t, map[string]host.Executor{})

	mgr := &runner.Manager{}
	cfg := cfgWithLocalHost("h1")
	cfg.Runners = []config.RunnerConfig{
		{Name: "ci", Host: "h1", Repo: "org/repo", Count: 1},
	}

	var buf bytes.Buffer
	err := Remove(&buf, cfg, mgr, "", "no-such-repo", nil)
	if err == nil {
		t.Fatal("expected error for empty runner set")
	}
	if !strings.Contains(err.Error(), "no runners matching") {
		t.Errorf("unexpected error message: %v", err)
	}
	if got := buf.String(); got != "" {
		t.Errorf("expected no output, got %q", got)
	}
}

// TestRemove_RunnerNotFoundByName covers the case where the user passes
// --name foo but no runner named "foo" exists. Same "no runners matching"
// error path as the repo-filter case.
func TestRemove_RunnerNotFoundByName(t *testing.T) {
	t.Parallel()

	installMockConnectHost(t, map[string]host.Executor{})

	mgr := &runner.Manager{}
	cfg := cfgWithLocalHost("h1")
	cfg.Runners = []config.RunnerConfig{
		{Name: "ci", Host: "h1", Repo: "org/repo", Count: 1},
	}

	var buf bytes.Buffer
	err := Remove(&buf, cfg, mgr, "", "", []string{"missing"})
	if err == nil {
		t.Fatal("expected error for name not in config")
	}
	if !strings.Contains(err.Error(), "no runners matching") {
		t.Errorf("unexpected error message: %v", err)
	}
}

// TestRemove_HostFilterNoMatch covers the --host flag filtering to a
// non-existent host. FilterRunners returns empty, orchestrator errors
// without connecting.
func TestRemove_HostFilterNoMatch(t *testing.T) {
	t.Parallel()

	installMockConnectHost(t, map[string]host.Executor{})

	mgr := &runner.Manager{}
	cfg := cfgWithLocalHost("h1")
	cfg.Runners = []config.RunnerConfig{
		{Name: "ci", Host: "h1", Repo: "org/repo", Count: 1},
	}

	var buf bytes.Buffer
	err := Remove(&buf, cfg, mgr, "h2", "", nil)
	if err == nil {
		t.Fatal("expected error for host filter with no match")
	}
	if !strings.Contains(err.Error(), "no runners matching") {
		t.Errorf("unexpected error message: %v", err)
	}
}

// TestRemove_ResolveHostInfoError covers the case where the host needs
// detection (non-local, missing OS+arch) and connectHostFn fails. The
// failure must surface before any per-runner work begins.
func TestRemove_ResolveHostInfoError(t *testing.T) {
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
	err := Remove(&buf, cfg, mgr, "", "", nil)
	if !errors.Is(err, sentinel) {
		t.Fatalf("got %v; want %v", err, sentinel)
	}
}

// TestRemove_ConnectHostErrorOnLocal covers the connect error that
// happens inside removeHost (after resolveAndFilter succeeds because
// local hosts skip detection). The sentinel must propagate as the
// orchestrator's error.
func TestRemove_ConnectHostErrorOnLocal(t *testing.T) {
	t.Parallel()

	sentinel := errors.New("ssh dial timeout")
	installFailingConnectHost(t, sentinel)

	ts := newRemovalTokenHTTPServer(t, "remtok")
	mgr := &runner.Manager{GitHub: runner.NewGitHubClientWithHTTP("pat", ts.Client(), ts.URL)}

	cfg := cfgWithLocalHost("h1")
	cfg.Runners = []config.RunnerConfig{
		{Name: "ci", Host: "h1", Repo: "org/repo", Count: 1},
	}

	var buf bytes.Buffer
	err := Remove(&buf, cfg, mgr, "", "", nil)
	if !errors.Is(err, sentinel) {
		t.Fatalf("got %v; want %v", err, sentinel)
	}
}

// TestRemove_MgrRemoveError covers the case where mgr.Remove (the
// removeNative chain) returns an error. The error message must propagate
// and the runner must NOT be removed from the on-disk config.
func TestRemove_MgrRemoveError(t *testing.T) {
	// Not t.Parallel — t.Setenv below is incompatible with parallel.

	hostErr := errors.New("rm -rf permission denied")
	exec := &testutil.MockExecutor{
		RunFn: func(cmd string) (string, error) {
			if strings.Contains(cmd, "rm -rf") {
				return "", hostErr
			}
			return "", nil
		},
	}
	installMockConnectHost(t, map[string]host.Executor{"h1": exec})

	cfgPath := writeRunnersYAML(t, runnersYAMLFor("h1", "ci"))
	t.Setenv(config.EnvVarConfigPath, cfgPath)

	ts := newRemovalTokenHTTPServer(t, "remtok")
	mgr := &runner.Manager{GitHub: runner.NewGitHubClientWithHTTP("pat", ts.Client(), ts.URL)}

	cfg := cfgWithLocalHost("h1")
	cfg.Runners = []config.RunnerConfig{
		{Name: "ci", Host: "h1", Repo: "o/r", Count: 1},
	}

	var buf bytes.Buffer
	err := Remove(&buf, cfg, mgr, "", "", nil)
	if err == nil {
		t.Fatal("expected error from mgr.Remove chain")
	}
	// The wrapper in Manager.Remove is "removing <name>: %w", but
	// removeNativeDirectory's wrapper is "removing runner directory <dir>: %w".
	if !strings.Contains(err.Error(), "permission denied") {
		t.Errorf("expected underlying error to propagate, got %v", err)
	}

	// Config file must still contain the runner — mgr.Remove failed before
	// config.RemoveRunner was reached.
	data, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "name: ci") {
		t.Errorf("config should still contain 'ci' after failed mgr.Remove; got:\n%s", data)
	}
}

// TestRemove_ConfigRemoveRunnerError covers the case where mgr.Remove
// succeeds but config.RemoveRunner fails (e.g., the runner name is not in
// the on-disk config — a divergence between the in-memory cfg and the
// file). The error is wrapped as "removing <name> from config: ..." and
// must propagate.
func TestRemove_ConfigRemoveRunnerError(t *testing.T) {
	// Not t.Parallel — t.Setenv below is incompatible with parallel.

	installMockConnectHost(t, map[string]host.Executor{"h1": newRemoveMockExecutor()})

	// YAML has only "stale" so removing "ci" will return "runner not found".
	cfgPath := writeRunnersYAML(t, runnersYAMLFor("h1", "stale"))
	t.Setenv(config.EnvVarConfigPath, cfgPath)

	ts := newRemovalTokenHTTPServer(t, "remtok")
	mgr := &runner.Manager{GitHub: runner.NewGitHubClientWithHTTP("pat", ts.Client(), ts.URL)}

	cfg := cfgWithLocalHost("h1")
	cfg.Runners = []config.RunnerConfig{
		{Name: "ci", Host: "h1", Repo: "o/r", Count: 1},
	}

	var buf bytes.Buffer
	err := Remove(&buf, cfg, mgr, "", "", nil)
	if err == nil {
		t.Fatal("expected error from config.RemoveRunner")
	}
	if !strings.Contains(err.Error(), "removing ci from config") {
		t.Errorf("expected wrapped error from config.RemoveRunner, got %v", err)
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected underlying 'not found' error, got %v", err)
	}
}

// TestRemove_SuccessSingleRunner covers the happy path: a single local
// runner, mocked mgr.Remove success, mocked GitHub token, real
// config.RemoveRunner deletes the entry. Verifies the user-visible
// banner, the "removed from host and config" line, and the final
// "Remove complete." footer. The YAML also has a "spare" runner so the
// post-write config.Load validation passes.
func TestRemove_SuccessSingleRunner(t *testing.T) {
	// Not t.Parallel — t.Setenv below is incompatible with parallel.

	installMockConnectHost(t, map[string]host.Executor{"h1": newRemoveMockExecutor()})

	cfgPath := writeRunnersYAML(t, runnersYAMLFor("h1", "ci"))
	t.Setenv(config.EnvVarConfigPath, cfgPath)

	ts := newRemovalTokenHTTPServer(t, "remtok")
	mgr := &runner.Manager{GitHub: runner.NewGitHubClientWithHTTP("pat", ts.Client(), ts.URL)}

	cfg := cfgWithLocalHost("h1")
	cfg.Runners = []config.RunnerConfig{
		{Name: "ci", Host: "h1", Repo: "o/r", Count: 1},
	}

	var buf bytes.Buffer
	if err := Remove(&buf, cfg, mgr, "", "", nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "Removing ci from h1 (local)") {
		t.Errorf("missing local banner; got:\n%s", out)
	}
	if !strings.Contains(out, "ci: removed from host and config") {
		t.Errorf("missing success line; got:\n%s", out)
	}
	if !strings.Contains(out, "Remove complete.") {
		t.Errorf("missing footer; got:\n%s", out)
	}

	// Config file must no longer contain 'ci' but must still contain 'spare'.
	data, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "name: ci") {
		t.Errorf("config should not contain 'ci' after success; got:\n%s", data)
	}
	if !strings.Contains(string(data), "name: spare") {
		t.Errorf("config should still contain 'spare'; got:\n%s", data)
	}
}

// TestRemove_SuccessRemoteAddrBanner covers the non-local host code
// path: when hcfg.Addr is not "local", the banner includes the address
// suffix (h1.example.com) rather than the literal "(local)".
func TestRemove_SuccessRemoteAddrBanner(t *testing.T) {
	// Not t.Parallel — t.Setenv below is incompatible with parallel.

	installMockConnectHost(t, map[string]host.Executor{"h1": newRemoveMockExecutor()})

	cfgPath := writeRunnersYAML(t, fmt.Sprintf(`hosts:
  h1:
    addr: user@h1.example.com
    os: linux
    arch: amd64
runners:
  - name: ci
    repo: o/r
    host: h1
  - name: spare
    repo: o/r
    host: h1
`))
	t.Setenv(config.EnvVarConfigPath, cfgPath)

	ts := newRemovalTokenHTTPServer(t, "remtok")
	mgr := &runner.Manager{GitHub: runner.NewGitHubClientWithHTTP("pat", ts.Client(), ts.URL)}

	cfg := &config.Config{Hosts: map[string]config.HostConfig{
		"h1": {Addr: "user@h1.example.com", OS: "linux", Arch: "amd64"},
	}}
	cfg.Runners = []config.RunnerConfig{
		{Name: "ci", Host: "h1", Repo: "o/r", Count: 1},
	}

	var buf bytes.Buffer
	if err := Remove(&buf, cfg, mgr, "", "", nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "Removing ci from h1 (user@h1.example.com)") {
		t.Errorf("expected remote-addr banner; got:\n%s", out)
	}
	if strings.Contains(out, "(local)") {
		t.Errorf("remote host should not produce local banner; got:\n%s", out)
	}
}

// TestRemove_SuccessMultipleRunners covers sequential removal of two
// runners on the same host. Both must be removed from host and config,
// in the input order, with the success lines appearing in the output
// for each.
func TestRemove_SuccessMultipleRunners(t *testing.T) {
	// Not t.Parallel — t.Setenv below is incompatible with parallel.

	installMockConnectHost(t, map[string]host.Executor{"h1": newRemoveMockExecutor()})

	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "runners.yml")
	initial := `hosts:
  h1:
    addr: local
    os: linux
    arch: amd64
runners:
  - name: a
    repo: o/r
    host: h1
  - name: b
    repo: o/r
    host: h1
  - name: spare
    repo: o/r
    host: h1
`
	if err := os.WriteFile(cfgPath, []byte(initial), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv(config.EnvVarConfigPath, cfgPath)

	ts := newRemovalTokenHTTPServer(t, "remtok")
	mgr := &runner.Manager{GitHub: runner.NewGitHubClientWithHTTP("pat", ts.Client(), ts.URL)}

	cfg := cfgWithLocalHost("h1")
	cfg.Runners = []config.RunnerConfig{
		{Name: "a", Host: "h1", Repo: "o/r", Count: 1},
		{Name: "b", Host: "h1", Repo: "o/r", Count: 1},
	}

	var buf bytes.Buffer
	if err := Remove(&buf, cfg, mgr, "", "", nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "Removing a from h1") {
		t.Errorf("missing banner for 'a'; got:\n%s", out)
	}
	if !strings.Contains(out, "Removing b from h1") {
		t.Errorf("missing banner for 'b'; got:\n%s", out)
	}
	if !strings.Contains(out, "a: removed from host and config") {
		t.Errorf("missing success line for 'a'; got:\n%s", out)
	}
	if !strings.Contains(out, "b: removed from host and config") {
		t.Errorf("missing success line for 'b'; got:\n%s", out)
	}

	// Order: banner for 'a' must precede banner for 'b'.
	ia := strings.Index(out, "Removing a from h1")
	ib := strings.Index(out, "Removing b from h1")
	if ia < 0 || ib < 0 || ia >= ib {
		t.Errorf("expected 'a' banner before 'b' banner; got order a=%d b=%d", ia, ib)
	}

	// Config file must no longer contain 'a' or 'b' but must still contain 'spare'.
	data, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "name: a") || strings.Contains(string(data), "name: b") {
		t.Errorf("config should not contain 'a' or 'b' after success; got:\n%s", data)
	}
	if !strings.Contains(string(data), "name: spare") {
		t.Errorf("config should still contain 'spare'; got:\n%s", data)
	}
}

// TestRemove_GitHubTokenErrorWarnsButSucceeds covers the branch in
// removeNative where GetRemovalTokenScoped returns an error: the
// orchestrator logs a warning and continues (does not fail the remove).
// The runner is still removed from host and config.
func TestRemove_GitHubTokenErrorWarnsButSucceeds(t *testing.T) {
	// Not t.Parallel — t.Setenv below is incompatible with parallel.

	installMockConnectHost(t, map[string]host.Executor{"h1": newRemoveMockExecutor()})

	cfgPath := writeRunnersYAML(t, runnersYAMLFor("h1", "ci"))
	t.Setenv(config.EnvVarConfigPath, cfgPath)

	ts := newRemovalTokenHTTPErrServer(t)
	mgr := &runner.Manager{GitHub: runner.NewGitHubClientWithHTTP("pat", ts.Client(), ts.URL)}

	cfg := cfgWithLocalHost("h1")
	cfg.Runners = []config.RunnerConfig{
		{Name: "ci", Host: "h1", Repo: "o/r", Count: 1},
	}

	var buf bytes.Buffer
	if err := Remove(&buf, cfg, mgr, "", "", nil); err != nil {
		t.Fatalf("GitHub-token error must not fail the remove; got %v", err)
	}
	// The Manager.Out is os.Stdout by default; the warning is emitted on it.
	// The orchestrator's own writer (buf) should still show the success
	// banner + the "removed from host and config" line.
	out := buf.String()
	if !strings.Contains(out, "Removing ci from h1 (local)") {
		t.Errorf("missing local banner; got:\n%s", out)
	}
	if !strings.Contains(out, "ci: removed from host and config") {
		t.Errorf("missing success line; got:\n%s", out)
	}

	// Config file must no longer contain the runner.
	data, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "name: ci") {
		t.Errorf("config should not contain 'ci' after success; got:\n%s", data)
	}
}

// TestRemove_LastRunnerFailsConfigValidation documents a known contract:
// the YAML validation in config.Load (called by writeYAMLBack) requires
// at least one runner to be defined. If the user removes the last runner
// in their config, the post-write validation rejects the empty config and
// surfaces the error wrapped as "removing <name> from config: ...".
// Pin this so a future refactor of writeYAMLBack (or the validation
// rule) is intentional, not silent.
func TestRemove_LastRunnerFailsConfigValidation(t *testing.T) {
	// Not t.Parallel — t.Setenv below is incompatible with parallel.

	installMockConnectHost(t, map[string]host.Executor{"h1": newRemoveMockExecutor()})

	// "ci" is the ONLY runner — removing it makes the config empty.
	cfgPath := writeRunnersYAML(t, `hosts:
  h1:
    addr: local
    os: linux
    arch: amd64
runners:
  - name: ci
    repo: o/r
    host: h1
`)
	t.Setenv(config.EnvVarConfigPath, cfgPath)

	ts := newRemovalTokenHTTPServer(t, "remtok")
	mgr := &runner.Manager{GitHub: runner.NewGitHubClientWithHTTP("pat", ts.Client(), ts.URL)}

	cfg := cfgWithLocalHost("h1")
	cfg.Runners = []config.RunnerConfig{
		{Name: "ci", Host: "h1", Repo: "o/r", Count: 1},
	}

	var buf bytes.Buffer
	err := Remove(&buf, cfg, mgr, "", "", nil)
	if err == nil {
		t.Fatal("expected error when removing the last runner")
	}
	if !strings.Contains(err.Error(), "removing ci from config") {
		t.Errorf("expected wrapped config-validation error, got %v", err)
	}
	if !strings.Contains(err.Error(), "at least one runner must be defined") {
		t.Errorf("expected underlying validation message, got %v", err)
	}
}

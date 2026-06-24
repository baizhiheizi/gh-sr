package doctor

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/an-lee/gh-sr/internal/agentic"
	"github.com/an-lee/gh-sr/internal/config"
	"github.com/an-lee/gh-sr/internal/host"
	"github.com/an-lee/gh-sr/internal/runner"
	"github.com/an-lee/gh-sr/internal/testutil"
)

func TestExitCode(t *testing.T) {
	t.Parallel()
	if got := ExitCode(Result{Fail: 0, Warn: 0}, false); got != 0 {
		t.Fatalf("expected 0, got %d", got)
	}
	if got := ExitCode(Result{Fail: 1, Warn: 0}, false); got != 1 {
		t.Fatalf("expected 1, got %d", got)
	}
	if got := ExitCode(Result{Fail: 0, Warn: 1}, false); got != 0 {
		t.Fatalf("strict=false: expected 0, got %d", got)
	}
	if got := ExitCode(Result{Fail: 0, Warn: 1}, true); got != 1 {
		t.Fatalf("strict=true: expected 1, got %d", got)
	}
}

func TestUniqueStringsBy(t *testing.T) {
	t.Parallel()
	t.Run("ReposKeyed", func(t *testing.T) {
		t.Parallel()
		runners := []config.RunnerConfig{
			{Repo: "z/z", Host: "a"},
			{Repo: "a/b", Host: "a"},
			{Repo: "a/b", Host: "b"},
		}
		got := uniqueStringsBy(runners, func(rc config.RunnerConfig) string { return rc.Repo })
		want := []string{"a/b", "z/z"}
		if len(got) != len(want) {
			t.Fatalf("len %d, want %d", len(got), len(want))
		}
		for i := range want {
			if got[i] != want[i] {
				t.Fatalf("got %v, want %v", got, want)
			}
		}
	})
	t.Run("HostNamesKeyedSorted", func(t *testing.T) {
		t.Parallel()
		runners := []config.RunnerConfig{
			{Host: "beta", Repo: "o/r"},
			{Host: "alpha", Repo: "o/r"},
			{Host: "beta", Repo: "o/r2"},
		}
		got := uniqueStringsBy(runners, func(rc config.RunnerConfig) string { return rc.Host })
		want := []string{"alpha", "beta"}
		if len(got) != len(want) {
			t.Fatalf("got %v", got)
		}
		for i := range want {
			if got[i] != want[i] {
				t.Fatalf("got %v, want %v", got, want)
			}
		}
	})
	t.Run("SkipsEmptyKey", func(t *testing.T) {
		t.Parallel()
		// Verify the asymmetry fix: empty host names (and empty repos/orgs)
		// are now skipped uniformly across all three call sites.
		runners := []config.RunnerConfig{
			{Host: "alpha", Repo: "o/r"},
			{Host: "", Repo: "o/r"},
			{Host: "alpha", Repo: ""},
		}
		got := uniqueStringsBy(runners, func(rc config.RunnerConfig) string { return rc.Host })
		want := []string{"alpha"}
		if len(got) != len(want) || got[0] != want[0] {
			t.Fatalf("got %v, want %v", got, want)
		}
		gotRepos := uniqueStringsBy(runners, func(rc config.RunnerConfig) string { return rc.Repo })
		if len(gotRepos) != 1 || gotRepos[0] != "o/r" {
			t.Fatalf("repos: got %v, want [o/r]", gotRepos)
		}
	})
}

func TestEnsureDoctorHostOS_LocalFillsRuntimeGOOS(t *testing.T) {
	t.Parallel()
	h := host.NewHost("local", config.HostConfig{Addr: config.LocalAddr})
	if err := ensureDoctorHostOS(h, config.LocalAddr); err != nil {
		t.Fatal(err)
	}
	if h.OS != runtime.GOOS {
		t.Fatalf("got %q want %q", h.OS, runtime.GOOS)
	}
}

func TestInstallTargetsForHost(t *testing.T) {
	t.Parallel()
	native := func(rc *config.RunnerConfig) bool { return !rc.IsContainerMode() }
	container := func(rc *config.RunnerConfig) bool { return rc.IsContainerMode() }
	agentic := func(rc *config.RunnerConfig) bool { return rc.IsAgentic() }

	t.Run("NativeMode", func(t *testing.T) {
		t.Parallel()
		runners := []config.RunnerConfig{
			{Name: "a", Host: "h1", Repo: "o/r", Count: 2},
			{Name: "b", Host: "h2", Repo: "o/r", Count: 1},
		}
		got := installTargetsForHost(runners, "h1", native)
		want := [][2]string{{"a-1", "a"}, {"a-2", "a"}}
		if len(got) != len(want) {
			t.Fatalf("len %d, want %d: %#v", len(got), len(want), got)
		}
		for i := range want {
			if got[i][0] != want[i][0] || got[i][1] != want[i][1] {
				t.Fatalf("idx %d: got %#v want %#v", i, got[i], want[i])
			}
		}
		if len(installTargetsForHost(runners, "h2", native)) != 1 {
			t.Fatalf("h2 should have one target")
		}

		mixed := []config.RunnerConfig{
			{Name: "native", Host: "hx", Repo: "o/r", Count: 1},
			{Name: "din", Host: "hx", Repo: "o/r", Count: 1, RunnerMode: config.RunnerModeContainer},
		}
		gotN := installTargetsForHost(mixed, "hx", native)
		if len(gotN) != 1 || gotN[0][0] != "native-1" {
			t.Fatalf("native targets should exclude container mode: %#v", gotN)
		}
	})

	t.Run("ContainerMode", func(t *testing.T) {
		t.Parallel()
		runners := []config.RunnerConfig{
			{Name: "n", Host: "h1", Repo: "o/r", Count: 1},
			{Name: "c", Host: "h1", Repo: "o/r", Count: 2, RunnerMode: config.RunnerModeContainer},
		}
		got := installTargetsForHost(runners, "h1", container)
		want := [][2]string{{"c-1", "c"}, {"c-2", "c"}}
		if len(got) != len(want) {
			t.Fatalf("len %d want %d: %#v", len(got), len(want), got)
		}
		for i := range want {
			if got[i][0] != want[i][0] || got[i][1] != want[i][1] {
				t.Fatalf("idx %d: got %#v want %#v", i, got[i], want[i])
			}
		}
	})

	t.Run("Agentic", func(t *testing.T) {
		t.Parallel()
		runners := []config.RunnerConfig{
			{Name: "ci", Host: "h1", Repo: "o/r", Count: 1, RunnerMode: config.RunnerModeContainer},
			{Name: "ag", Host: "h1", Repo: "o/r", Count: 1, RunnerMode: config.RunnerModeContainer, Profile: "agentic"},
		}
		got := installTargetsForHost(runners, "h1", agentic)
		if len(got) != 1 || got[0][0] != "ag-1" || got[0][1] != "ag" {
			t.Fatalf("want single agentic container instance, got %#v", got)
		}

		// profile: agentic alone implies container mode (no explicit runner_mode).
		implicit := []config.RunnerConfig{
			{Name: "aw", Host: "h1", Repo: "o/r", Count: 1, Profile: "agentic"},
		}
		gotImplicit := installTargetsForHost(implicit, "h1", agentic)
		if len(gotImplicit) != 1 || gotImplicit[0][0] != "aw-1" {
			t.Fatalf("profile: agentic should resolve to container agentic target, got %#v", gotImplicit)
		}
	})

	t.Run("PredicateReceivesPointer", func(t *testing.T) {
		t.Parallel()
		// Pins the predicate signature: callers receive a pointer so they can
		// safely call pointer-receiver methods on RunnerConfig.
		var got *config.RunnerConfig
		runners := []config.RunnerConfig{
			{Name: "a", Host: "h1", Repo: "o/r", Count: 1},
		}
		installTargetsForHost(runners, "h1", func(rc *config.RunnerConfig) bool {
			got = rc
			return true
		})
		if got == nil || got.Name != "a" {
			t.Fatalf("predicate should receive a pointer to the runner, got %#v", got)
		}
	})

	t.Run("EmptyHostReturnsNil", func(t *testing.T) {
		t.Parallel()
		runners := []config.RunnerConfig{
			{Name: "a", Host: "h1", Repo: "o/r", Count: 1},
		}
		if got := installTargetsForHost(runners, "h2", native); len(got) != 0 {
			t.Fatalf("host mismatch should return empty slice, got %#v", got)
		}
	})
}

func TestRunnersForHost(t *testing.T) {
	t.Parallel()
	runners := []config.RunnerConfig{
		{Name: "a", Host: "h1", Repo: "o/one"},
		{Name: "b", Host: "h1", Repo: "o/two"},
		{Name: "c", Host: "h2", Repo: "o/one"},
	}
	got := runnersForHost(runners, "h1")
	if len(got) != 2 {
		t.Fatalf("len %d want 2: %#v", len(got), got)
	}
	if got[0].Name != "a" || got[1].Name != "b" {
		t.Fatalf("got %#v", got)
	}
	if len(runnersForHost(runners, "missing")) != 0 {
		t.Fatal("expected no runners for unknown host")
	}
}

func TestHasNativeModeRunners(t *testing.T) {
	t.Parallel()
	containerOnly := []config.RunnerConfig{
		{Name: "c", Host: "h1", Repo: "o/r", RunnerMode: config.RunnerModeContainer},
		{Name: "a", Host: "h1", Repo: "o/r", Profile: "agentic"},
	}
	if hasNativeModeRunners(containerOnly) {
		t.Fatal("container-only and profile: agentic should not require native checks")
	}
	mixed := []config.RunnerConfig{
		{Name: "n", Host: "h1", Repo: "o/r"},
		{Name: "c", Host: "h1", Repo: "o/r", RunnerMode: config.RunnerModeContainer},
	}
	if !hasNativeModeRunners(mixed) {
		t.Fatal("mixed host should require native checks")
	}
}

func TestHasContainerModeRunners(t *testing.T) {
	t.Parallel()
	nativeOnly := []config.RunnerConfig{
		{Name: "n", Host: "h1", Repo: "o/r"},
	}
	if hasContainerModeRunners(nativeOnly) {
		t.Fatal("native-only should not trigger container checks")
	}
	agenticOnly := []config.RunnerConfig{
		{Name: "a", Host: "h1", Repo: "o/r", Profile: "agentic"},
	}
	if !hasContainerModeRunners(agenticOnly) {
		t.Fatal("profile: agentic should trigger container checks")
	}
}

type failIfRunExec struct {
	t *testing.T
}

func (f failIfRunExec) Run(cmd string) (string, error) {
	// The dirSizesPOSIX script now uses `du --max-depth=1` (or `du -d 1`
	// on BSD) and a `while read` parser. Match the new shape; the old
	// `du -sk` form is no longer used.
	if strings.Contains(cmd, "du --max-depth=1") || strings.Contains(cmd, "du -d 1") ||
		strings.Contains(cmd, `dir="$HOME/.gh-sr/runners/`) {
		return "0 0 0 0\n", nil
	}
	if strings.Contains(cmd, `ls -1 "$HOME/.gh-sr/runners"`) {
		return "", nil
	}
	if strings.Contains(cmd, `ls -1 "$HOME/.gh-sr/runners"`) {
		return "", nil
	}
	if strings.Contains(cmd, "com.github.ghsr.runner.") {
		return "", nil
	}
	f.t.Fatalf("unexpected remote command: %s", cmd)
	return "", nil
}

func (f failIfRunExec) Upload(string, string) error { return nil }
func (f failIfRunExec) Close() error                { return nil }

func TestRunHostChecks_ContainerOnlySkipsNative(t *testing.T) {
	t.Parallel()
	h := host.NewHost("mac", config.HostConfig{OS: "darwin", Addr: "user@mac"})
	h.SetConn(failIfRunExec{t: t})
	runners := []config.RunnerConfig{
		{Name: "aw", Host: "mac", Repo: "o/r", Profile: "agentic"},
	}
	var buf strings.Builder
	var r Result
	runHostChecks(&buf, "mac", h, runners, &r)
	out := buf.String()
	if strings.Contains(out, "native:") {
		t.Fatalf("container-only host should skip native checks, got:\n%s", out)
	}
}

func TestRunHostChecks_NonLinuxContainerFails(t *testing.T) {
	t.Parallel()
	h := host.NewHost("mac", config.HostConfig{OS: "darwin", Addr: "user@mac"})
	var buf strings.Builder
	var r Result
	runners := []config.RunnerConfig{
		{Name: "aw", Host: "mac", Repo: "o/r", Profile: "agentic"},
	}
	runHostChecks(&buf, "mac", h, runners, &r)
	if r.Fail != 1 {
		t.Fatalf("expected one FAIL for container mode on darwin, got %+v", r)
	}
	if !strings.Contains(buf.String(), "only supported on Linux") {
		t.Fatalf("expected linux-only container message, got:\n%s", buf.String())
	}
}

func TestFilteredHostRunners_ExcludeContainerWhenRepoFiltered(t *testing.T) {
	t.Parallel()
	filtered := []config.RunnerConfig{
		{Name: "n", Host: "h1", Repo: "o/one"},
	}
	hostRunners := runnersForHost(filtered, "h1")
	if hasContainerModeRunners(hostRunners) {
		t.Fatal("repo-filtered native-only slice should not enable container checks")
	}
	if !hasNativeModeRunners(hostRunners) {
		t.Fatal("expected native checks for filtered native runner")
	}
}

func TestRun_ConfigErrorSkipsGitHub(t *testing.T) {
	t.Parallel()
	var buf strings.Builder
	cfgPath := t.TempDir() + "/missing.yml"
	envPath := t.TempDir() + "/env"
	res := Run(&buf, cfgPath, envPath, nil, assertError(t, "no config"), nil, "", "", false)
	if res.Fail < 1 {
		t.Fatalf("expected at least one FAIL, got %+v", res)
	}
	out := buf.String()
	if strings.Contains(out, "=== GitHub API ===") {
		t.Fatalf("should not reach GitHub section:\n%s", out)
	}
}

func assertError(t *testing.T, msg string) error {
	t.Helper()
	return &testErr{msg}
}

type testErr struct{ s string }

func (e *testErr) Error() string { return e.s }

func TestRun_GitHubListRunnersUsesAPI(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method", http.StatusMethodNotAllowed)
			return
		}
		if !strings.HasSuffix(r.URL.Path, "/repos/o/r/actions/runners") {
			http.NotFound(w, r)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"total_count": 0,
			"runners":     []any{},
		})
	}))
	t.Cleanup(srv.Close)

	cfg := &config.Config{
		GitHub: config.GitHubConfig{},
		Hosts: map[string]config.HostConfig{
			"localh": {Addr: config.LocalAddr, OS: "linux", Arch: "amd64"},
		},
		Runners: []config.RunnerConfig{
			{Name: "only-gh", Repo: "o/r", Host: "localh", Labels: []string{"x"}},
		},
	}
	gh := runner.NewGitHubClientWithHTTP("test-token", srv.Client(), srv.URL)

	tmp := t.TempDir()
	cfgPath := filepath.Join(tmp, "runners.yml")
	if err := os.WriteFile(cfgPath, []byte("#"), 0o600); err != nil {
		t.Fatal(err)
	}
	envPath := filepath.Join(tmp, "env")
	if err := os.WriteFile(envPath, []byte("#"), 0o600); err != nil {
		t.Fatal(err)
	}

	var buf strings.Builder
	res := Run(&buf, cfgPath, envPath, cfg, nil, gh, "", "", false)

	out := buf.String()
	if !strings.Contains(out, "list runners OK (0 registered)") {
		t.Fatalf("expected GitHub OK line, got:\n%s", out)
	}
	// Local host probes may FAIL in CI without curl/tar; only assert GitHub section ran.
	if !strings.Contains(out, "=== GitHub API ===") {
		t.Fatalf("missing GitHub section:\n%s", out)
	}
	_ = res
}

func TestFormatOrgAPIError_permissionHint(t *testing.T) {
	t.Parallel()
	err403 := fmt.Errorf("listing runners for my-org: HTTP 403: forbidden")
	got := formatOrgAPIError("my-org", err403)
	if !strings.Contains(got, "admin:org") {
		t.Fatalf("expected admin:org hint, got: %s", got)
	}
	if !strings.Contains(got, "org my-org:") {
		t.Fatalf("expected org prefix, got: %s", got)
	}

	plain := formatOrgAPIError("my-org", fmt.Errorf("connection reset"))
	if strings.Contains(plain, "admin:org") {
		t.Fatalf("non-permission error should not get hint: %s", plain)
	}
}

func TestRun_GitHubOrgListRunnersUsesAPI(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method", http.StatusMethodNotAllowed)
			return
		}
		if strings.HasSuffix(r.URL.Path, "/orgs/my-org/actions/runners") {
			_ = json.NewEncoder(w).Encode(map[string]any{
				"total_count": 2,
				"runners":     []any{},
			})
			return
		}
		http.NotFound(w, r)
	}))
	t.Cleanup(srv.Close)

	cfg := &config.Config{
		GitHub: config.GitHubConfig{},
		Hosts: map[string]config.HostConfig{
			"localh": {Addr: config.LocalAddr, OS: "linux", Arch: "amd64"},
		},
		Runners: []config.RunnerConfig{
			{Name: "org-ci", Org: "my-org", Host: "localh", Labels: []string{"x"}},
		},
	}
	gh := runner.NewGitHubClientWithHTTP("test-token", srv.Client(), srv.URL)

	tmp := t.TempDir()
	cfgPath := filepath.Join(tmp, "runners.yml")
	if err := os.WriteFile(cfgPath, []byte("#"), 0o600); err != nil {
		t.Fatal(err)
	}
	envPath := filepath.Join(tmp, "env")
	if err := os.WriteFile(envPath, []byte("#"), 0o600); err != nil {
		t.Fatal(err)
	}

	var buf strings.Builder
	res := Run(&buf, cfgPath, envPath, cfg, nil, gh, "", "", false)

	out := buf.String()
	if !strings.Contains(out, "org my-org: list runners OK (0 registered)") {
		t.Fatalf("expected org GitHub OK line, got:\n%s", out)
	}
	_ = res
}

func TestRun_GitHubOrgListRunners403ShowsHint(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/orgs/my-org/actions/runners") {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		http.NotFound(w, r)
	}))
	t.Cleanup(srv.Close)

	cfg := &config.Config{
		GitHub: config.GitHubConfig{},
		Hosts: map[string]config.HostConfig{
			"localh": {Addr: config.LocalAddr, OS: "linux", Arch: "amd64"},
		},
		Runners: []config.RunnerConfig{
			{Name: "org-ci", Org: "my-org", Host: "localh"},
		},
	}
	gh := runner.NewGitHubClientWithHTTP("test-token", srv.Client(), srv.URL)

	tmp := t.TempDir()
	cfgPath := filepath.Join(tmp, "runners.yml")
	if err := os.WriteFile(cfgPath, []byte("#"), 0o600); err != nil {
		t.Fatal(err)
	}
	envPath := filepath.Join(tmp, "env")
	if err := os.WriteFile(envPath, []byte("#"), 0o600); err != nil {
		t.Fatal(err)
	}

	var buf strings.Builder
	res := Run(&buf, cfgPath, envPath, cfg, nil, gh, "", "", false)

	out := buf.String()
	if !strings.Contains(out, "admin:org") {
		t.Fatalf("expected org permission hint, got:\n%s", out)
	}
	if res.Fail < 1 {
		t.Fatalf("expected FAIL for org 403, got %+v", res)
	}
}

func TestCheckRunnerDiskUsage_warnsOrphanOverThreshold(t *testing.T) {
	t.Parallel()
	h := host.NewHost("linux", config.HostConfig{OS: "linux", Addr: "local"})
	h.SetConn(&testutil.MockExecutor{
		RunFn: func(cmd string) (string, error) {
			if strings.Contains(cmd, `ls -1 "$HOME/.gh-sr/runners"`) {
				return "orphan-1\n", nil
			}
			if strings.Contains(cmd, "du --max-depth=1") || strings.Contains(cmd, "du -d 1") {
				return fmt.Sprintf("%d 0 0 0\n", runner.DiskWarnThresholdBytes()+1), nil
			}
			return "0 0 0 0\n", nil
		},
	})
	var buf strings.Builder
	var r Result
	checkRunnerDiskUsage(&buf, "linux", h, nil, &r)
	out := buf.String()
	if !strings.Contains(out, "orphan-1 (orphan)") {
		t.Fatalf("expected orphan warning, got:\n%s", out)
	}
	if r.Warn != 1 {
		t.Fatalf("expected 1 warn, got %+v", r)
	}
}

func TestPrintAgenticFailures_SeverityError(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	r := Result{}
	printAgenticFailures(&buf, "h1", &r, sevWarn, "container: ", []agentic.PrereqFailure{
		{Message: "docker down", Severity: agentic.SeverityError},
	})
	if !strings.Contains(buf.String(), "FAIL  [h1          ] container: docker down") {
		t.Fatalf("missing FAIL line, got:\n%s", buf.String())
	}
	if r.Fail != 1 || r.Warn != 0 {
		t.Fatalf("counters: got %+v, want Fail=1 Warn=0", r)
	}
}

func TestPrintAgenticFailures_SeverityWarning(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	r := Result{}
	printAgenticFailures(&buf, "h1", &r, sevFail, "container: ", []agentic.PrereqFailure{
		{Message: "slow", Severity: agentic.SeverityWarning},
	})
	if !strings.Contains(buf.String(), "WARN  [h1          ] container: slow") {
		t.Fatalf("missing WARN line, got:\n%s", buf.String())
	}
	if r.Fail != 0 || r.Warn != 1 {
		t.Fatalf("counters: got %+v, want Fail=0 Warn=1", r)
	}
}

func TestPrintAgenticFailures_DefaultSevFail(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	r := Result{}
	printAgenticFailures(&buf, "h1", &r, sevFail, "container: ", []agentic.PrereqFailure{
		{Message: "unknown", Severity: "info"},
	})
	if !strings.Contains(buf.String(), "FAIL  [h1          ] container: unknown") {
		t.Fatalf("expected FAIL line, got:\n%s", buf.String())
	}
	if r.Fail != 1 || r.Warn != 0 {
		t.Fatalf("counters: got %+v, want Fail=1 Warn=0", r)
	}
}

func TestPrintAgenticFailures_DefaultSevWarn(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	r := Result{}
	printAgenticFailures(&buf, "h1", &r, sevWarn, "container(agent): ", []agentic.PrereqFailure{
		{Message: "unknown", Severity: "info"},
	})
	if !strings.Contains(buf.String(), "WARN  [h1          ] container(agent): unknown") {
		t.Fatalf("expected WARN line, got:\n%s", buf.String())
	}
	if r.Fail != 0 || r.Warn != 1 {
		t.Fatalf("counters: got %+v, want Fail=0 Warn=1", r)
	}
}

func TestPrintAgenticFailures_RemediationAndDocRef(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	r := Result{}
	printAgenticFailures(&buf, "h1", &r, sevFail, "container: ", []agentic.PrereqFailure{
		{
			Message:     "out of disk",
			Severity:    agentic.SeverityError,
			Remediation: "rm -rf /tmp/junk\nsudo systemctl restart docker",
			DocRef:      "ops.md §7",
		},
	})
	out := buf.String()
	for _, want := range []string{
		"FAIL  [h1          ] container: out of disk",
		"       rm -rf /tmp/junk",
		"       sudo systemctl restart docker",
		"       See: ops.md §7",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in output:\n%s", want, out)
		}
	}
	if r.Fail != 1 || r.Warn != 0 {
		t.Fatalf("counters: got %+v, want Fail=1 Warn=0", r)
	}
}

func TestPrintAgenticFailures_EmptySlice(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	r := Result{}
	printAgenticFailures(&buf, "h1", &r, sevFail, "container: ", nil)
	if buf.Len() != 0 {
		t.Fatalf("expected no output for empty failures, got:\n%s", buf.String())
	}
	if r.Fail != 0 || r.Warn != 0 {
		t.Fatalf("counters: got %+v, want zero", r)
	}
}

func TestPrintAgenticFailures_MixedSeverities(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	r := Result{}
	printAgenticFailures(&buf, "h1", &r, sevFail, "container(agent): ", []agentic.PrereqFailure{
		{Message: "err", Severity: agentic.SeverityError},
		{Message: "warn", Severity: agentic.SeverityWarning},
		{Message: "info", Severity: "info"},
	})
	out := buf.String()
	for _, want := range []string{
		"FAIL  [h1          ] container(agent): err",
		"WARN  [h1          ] container(agent): warn",
		"FAIL  [h1          ] container(agent): info", // defaultSev=sevFail
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in output:\n%s", want, out)
		}
	}
	if r.Fail != 2 || r.Warn != 1 {
		t.Fatalf("counters: got %+v, want Fail=2 Warn=1", r)
	}
}

// TestCheckShellOK_OkBranch verifies checkShellOK returns true, prints the
// OK line, and leaves r.Fail untouched when trimmed stdout matches want.
func TestCheckShellOK_OkBranch(t *testing.T) {
	t.Parallel()
	h := host.NewHost("h1", config.HostConfig{OS: "linux", Addr: "local"})
	h.SetConn(&testutil.MockExecutor{
		RunFn: func(cmd string) (string, error) { return "ok\n", nil },
	})
	var buf bytes.Buffer
	r := Result{}
	if !checkShellOK(&buf, "h1", h, &r, "true", "ok", "ready", "not ready") {
		t.Fatal("checkShellOK should return true on matching stdout")
	}
	if r.Fail != 0 {
		t.Fatalf("r.Fail should stay 0 on OK, got %d", r.Fail)
	}
	if !strings.Contains(buf.String(), "ready") || strings.Contains(buf.String(), "not ready") {
		t.Fatalf("OK output should contain okMsg and not failMsg, got: %q", buf.String())
	}
}

// TestCheckShellOK_Mismatch verifies checkShellOK returns false, prints the
// FAIL line with err suffix, and increments r.Fail when trimmed stdout != want.
func TestCheckShellOK_Mismatch(t *testing.T) {
	t.Parallel()
	h := host.NewHost("h1", config.HostConfig{OS: "linux", Addr: "local"})
	h.SetConn(&testutil.MockExecutor{
		RunFn: func(cmd string) (string, error) { return "missing\n", nil },
	})
	var buf bytes.Buffer
	r := Result{}
	if checkShellOK(&buf, "h1", h, &r, "true", "ok", "ready", "not ready") {
		t.Fatal("checkShellOK should return false on non-matching stdout")
	}
	if r.Fail != 1 {
		t.Fatalf("r.Fail should increment to 1, got %d", r.Fail)
	}
	if !strings.Contains(buf.String(), "not ready") {
		t.Fatalf("FAIL output should contain failMsg, got: %q", buf.String())
	}
}

// TestCheckShellOK_RunError verifies checkShellOK treats a non-nil err from
// h.Run as a failure even when stdout happens to match want (defensive: err
// is a stronger signal than stdout in some failure modes, e.g. SSH closed).
func TestCheckShellOK_RunError(t *testing.T) {
	t.Parallel()
	h := host.NewHost("h1", config.HostConfig{OS: "linux", Addr: "local"})
	h.SetConn(&testutil.MockExecutor{
		RunFn:  func(cmd string) (string, error) { return "ok", errors.New("ssh: handshake timeout") },
		RunErr: nil,
	})
	var buf bytes.Buffer
	r := Result{}
	if checkShellOK(&buf, "h1", h, &r, "true", "ok", "ready", "not ready") {
		t.Fatal("checkShellOK should return false when h.Run returns an error")
	}
	if r.Fail != 1 {
		t.Fatalf("r.Fail should increment to 1, got %d", r.Fail)
	}
	if !strings.Contains(buf.String(), "ssh: handshake timeout") {
		t.Fatalf("FAIL output should include underlying err, got: %q", buf.String())
	}
}

// TestCheckShellOK_TrimsWhitespace verifies checkShellOK trims trailing
// newlines from h.Run output before comparing to want, matching the previous
// inline behavior at the linux/darwin call sites in checkNative.
func TestCheckShellOK_TrimsWhitespace(t *testing.T) {
	t.Parallel()
	h := host.NewHost("h1", config.HostConfig{OS: "linux", Addr: "local"})
	h.SetConn(&testutil.MockExecutor{
		RunFn: func(cmd string) (string, error) { return "  ok  \n", nil },
	})
	var buf bytes.Buffer
	r := Result{}
	if !checkShellOK(&buf, "h1", h, &r, "true", "ok", "ready", "not ready") {
		t.Fatal("checkShellOK should tolerate leading/trailing whitespace")
	}
	if r.Fail != 0 {
		t.Fatalf("r.Fail should stay 0 after trim, got %d", r.Fail)
	}
}

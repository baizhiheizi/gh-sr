package doctor

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/an-lee/ghr/internal/config"
	"github.com/an-lee/ghr/internal/runner"
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

func TestUniqueRepos(t *testing.T) {
	t.Parallel()
	runners := []config.RunnerConfig{
		{Repo: "z/z", Host: "a"},
		{Repo: "a/b", Host: "a"},
		{Repo: "a/b", Host: "b"},
	}
	got := uniqueRepos(runners)
	want := []string{"a/b", "z/z"}
	if len(got) != len(want) {
		t.Fatalf("len %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v, want %v", got, want)
		}
	}
}

func TestUniqueHostNames(t *testing.T) {
	t.Parallel()
	runners := []config.RunnerConfig{
		{Host: "beta", Repo: "o/r"},
		{Host: "alpha", Repo: "o/r"},
		{Host: "beta", Repo: "o/r2"},
	}
	got := uniqueHostNames(runners)
	want := []string{"alpha", "beta"}
	if len(got) != len(want) {
		t.Fatalf("got %v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v, want %v", got, want)
		}
	}
}

func TestModesForHost(t *testing.T) {
	t.Parallel()
	runners := []config.RunnerConfig{
		{Name: "r1", Host: "h1", Repo: "o/r", Mode: "docker"},
		{Name: "r2", Host: "h1", Repo: "o/r", Mode: "native"},
		{Name: "r3", Host: "h2", Repo: "o/r"},
	}
	m := modesForHost(runners, "h1", "linux")
	if !m["docker"] || !m["native"] {
		t.Fatalf("h1 linux: got %#v", m)
	}
	m2 := modesForHost(runners, "h2", "linux")
	if !m2["docker"] || len(m2) != 1 {
		t.Fatalf("h2 linux default docker: got %#v", m2)
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
		GitHub: config.GitHubConfig{PAT: "test-token"},
		Hosts: map[string]config.HostConfig{
			"localh": {Addr: config.LocalAddr, OS: "linux", Arch: "amd64"},
		},
		Runners: []config.RunnerConfig{
			{Name: "only-gh", Repo: "o/r", Host: "localh", Labels: []string{"x"}, Mode: "native"},
		},
	}
	gh := runner.NewGitHubClientWithHTTP(cfg.GitHub.PAT, srv.Client(), srv.URL)

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

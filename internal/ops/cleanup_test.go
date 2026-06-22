package ops

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/an-lee/gh-sr/internal/config"
	"github.com/an-lee/gh-sr/internal/runner"
)

// newCleanupHTTPServer returns an httptest.Server that simulates the GitHub
// "list runners" + "delete runner" endpoints. The delete callback is invoked
// once per successful DELETE so tests can assert which runner IDs were
// removed. The list callback decides what runners to return.
func newCleanupHTTPServer(t *testing.T, listFn func(scope, target string) []runner.GitHubRunner, onDelete func(scope, target string, id int64)) *httptest.Server {
	t.Helper()
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			scope, target, ok := parseActionsPath(r.URL.Path, "runners")
			if !ok {
				http.NotFound(w, r)
				return
			}
			runners := listFn(scope, target)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"total_count": len(runners),
				"runners":     runners,
			})
		case http.MethodDelete:
			scope, target, id, ok := parseDeletePath(r.URL.Path)
			if !ok {
				http.NotFound(w, r)
				return
			}
			if onDelete != nil {
				onDelete(scope, target, id)
			}
			w.WriteHeader(http.StatusNoContent)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(ts.Close)
	return ts
}

// parseActionsPath returns (scope, target, ok) for a path of the form
// "/repos/<target>/actions/<anything>" or "/orgs/<target>/actions/<anything>".
// The target can itself contain a slash (owner/repo), so we split on
// "/actions/" rather than on "/".
func parseActionsPath(path, _ string) (string, string, bool) {
	const sep = "/actions/"
	idx := strings.Index(path, sep)
	if idx < 0 {
		return "", "", false
	}
	prefix := strings.TrimPrefix(path[:idx], "/")
	parts := strings.SplitN(prefix, "/", 2)
	if len(parts) != 2 {
		return "", "", false
	}
	scope := "repo"
	if parts[0] == "orgs" {
		scope = "org"
	}
	return scope, parts[1], true
}

// parseDeletePath returns (scope, target, id, ok) for a path of the form
// "/repos/<target>/actions/runners/<id>" or "/orgs/<target>/actions/runners/<id>".
// The target can itself contain a slash (owner/repo), so we split on
// "/actions/runners/" rather than on "/".
func parseDeletePath(path string) (string, string, int64, bool) {
	const sep = "/actions/runners/"
	idx := strings.Index(path, sep)
	if idx < 0 {
		return "", "", 0, false
	}
	prefix := strings.TrimPrefix(path[:idx], "/")
	idPart := path[idx+len(sep):]
	var id int64
	if _, err := fmt.Sscanf(idPart, "%d", &id); err != nil {
		return "", "", 0, false
	}
	parts := strings.SplitN(prefix, "/", 2)
	if len(parts) != 2 {
		return "", "", 0, false
	}
	scope := "repo"
	if parts[0] == "orgs" {
		scope = "org"
	}
	return scope, parts[1], id, true
}

// TestCleanupOffline_NoOffline covers the contract that the orchestrator
// emits "No offline runners found." when the GitHub API reports zero
// offline runners, and returns (0, nil). No DELETE calls should be issued.
func TestCleanupOffline_NoOffline(t *testing.T) {
	t.Parallel()

	ts := newCleanupHTTPServer(t,
		func(scope, target string) []runner.GitHubRunner {
			return []runner.GitHubRunner{{ID: 1, Name: "r-online", Status: "online"}}
		},
		nil,
	)

	mgr := &runner.Manager{GitHub: runner.NewGitHubClientWithHTTP("pat", ts.Client(), ts.URL)}
	cfg := &config.Config{
		Runners: []config.RunnerConfig{{Name: "ci", Host: "h1", Repo: "o/r"}},
	}

	var buf bytes.Buffer
	removed, err := CleanupOffline(&buf, cfg, mgr)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if removed != 0 {
		t.Errorf("removed = %d, want 0", removed)
	}
	if !strings.Contains(buf.String(), "Cleaning up offline runners...") {
		t.Errorf("missing 'Cleaning up offline runners...' line; got:\n%s", buf.String())
	}
	if !strings.Contains(buf.String(), "No offline runners found.") {
		t.Errorf("missing 'No offline runners found.' line; got:\n%s", buf.String())
	}
}

// TestCleanupOffline_RemovesOffline pins the happy path: a mix of online +
// offline runners, the orchestrator returns the count of deletes and prints
// "Removed N offline runner(s)." with the correct number.
func TestCleanupOffline_RemovesOffline(t *testing.T) {
	t.Parallel()

	var mu sync.Mutex
	var deleted []int64
	ts := newCleanupHTTPServer(t,
		func(scope, target string) []runner.GitHubRunner {
			return []runner.GitHubRunner{
				{ID: 11, Name: "r-offline-1", Status: "offline"},
				{ID: 22, Name: "r-online", Status: "online"},
				{ID: 33, Name: "r-offline-2", Status: "offline"},
			}
		},
		func(scope, target string, id int64) {
			mu.Lock()
			defer mu.Unlock()
			deleted = append(deleted, id)
		},
	)

	mgr := &runner.Manager{GitHub: runner.NewGitHubClientWithHTTP("pat", ts.Client(), ts.URL)}
	cfg := &config.Config{
		Runners: []config.RunnerConfig{{Name: "ci", Host: "h1", Repo: "o/r"}},
	}

	var buf bytes.Buffer
	removed, err := CleanupOffline(&buf, cfg, mgr)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if removed != 2 {
		t.Errorf("removed = %d, want 2", removed)
	}
	if !strings.Contains(buf.String(), "Removed 2 offline runner(s).") {
		t.Errorf("missing 'Removed 2 offline runner(s).' line; got:\n%s", buf.String())
	}

	// Both offline IDs must have been deleted, in list order.
	mu.Lock()
	defer mu.Unlock()
	if len(deleted) != 2 || deleted[0] != 11 || deleted[1] != 33 {
		t.Errorf("deleted IDs = %v, want [11 33]", deleted)
	}
}

// TestCleanupOffline_OrgScope covers the org-scope branch of mgr.CleanupOffline:
// a runner with Scope=org targets /orgs/.../actions/runners, not /repos/.../.
// Verifies the right URL is built and one delete per offline runner is issued.
func TestCleanupOffline_OrgScope(t *testing.T) {
	t.Parallel()

	ts := newCleanupHTTPServer(t,
		func(scope, target string) []runner.GitHubRunner {
			return []runner.GitHubRunner{{ID: 7, Name: "org-offline", Status: "offline"}}
		},
		nil,
	)

	mgr := &runner.Manager{GitHub: runner.NewGitHubClientWithHTTP("pat", ts.Client(), ts.URL)}
	cfg := &config.Config{
		Runners: []config.RunnerConfig{{
			Name: "ci",
			Host: "h1",
			Org:  "myorg",
		}},
	}

	var buf bytes.Buffer
	removed, err := CleanupOffline(&buf, cfg, mgr)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if removed != 1 {
		t.Errorf("removed = %d, want 1", removed)
	}
	if !strings.Contains(buf.String(), "Removed 1 offline runner(s).") {
		t.Errorf("missing 'Removed 1 offline runner(s).' line; got:\n%s", buf.String())
	}
}

// TestCleanupOffline_ListError covers the case where the GitHub "list
// runners" call fails. The orchestrator must surface the wrapped error
// from mgr.CleanupOffline and emit the "Cleaning up offline runners..."
// banner (the orchestrator always prints the banner before the call, even
// if the call subsequently fails).
func TestCleanupOffline_ListError(t *testing.T) {
	t.Parallel()

	sentinel := errors.New("simulated list failure")
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(sentinel.Error()))
	}))
	t.Cleanup(ts.Close)

	mgr := &runner.Manager{GitHub: runner.NewGitHubClientWithHTTP("pat", ts.Client(), ts.URL)}
	cfg := &config.Config{
		Runners: []config.RunnerConfig{{Name: "ci", Host: "h1", Repo: "o/r"}},
	}

	var buf bytes.Buffer
	removed, err := CleanupOffline(&buf, cfg, mgr)
	if err == nil {
		t.Fatal("expected error from list call")
	}
	if !strings.Contains(err.Error(), "listing runners") {
		t.Errorf("expected wrapped 'listing runners' error, got %v", err)
	}
	if removed != 0 {
		t.Errorf("removed = %d, want 0 on error", removed)
	}
	if !strings.Contains(buf.String(), "Cleaning up offline runners...") {
		t.Errorf("missing 'Cleaning up offline runners...' line; got:\n%s", buf.String())
	}
}

// TestCleanupOffline_DuplicateScopesDeduped covers the dedup-on-scope branch
// in mgr.CleanupOffline: multiple runners pointing at the same repo should
// only trigger ONE list call. Two runners with the same (scope, target)
// result in a single API round-trip; only then are the offline entries
// deleted.
func TestCleanupOffline_DuplicateScopesDeduped(t *testing.T) {
	t.Parallel()

	var listCalls int
	var mu sync.Mutex
	var deleted []int64
	ts := newCleanupHTTPServer(t,
		func(scope, target string) []runner.GitHubRunner {
			mu.Lock()
			listCalls++
			mu.Unlock()
			return []runner.GitHubRunner{
				{ID: 100, Name: "shared-offline", Status: "offline"},
			}
		},
		func(scope, target string, id int64) {
			mu.Lock()
			defer mu.Unlock()
			deleted = append(deleted, id)
		},
	)

	mgr := &runner.Manager{GitHub: runner.NewGitHubClientWithHTTP("pat", ts.Client(), ts.URL)}
	cfg := &config.Config{
		Runners: []config.RunnerConfig{
			{Name: "a", Host: "h1", Repo: "o/r"},
			{Name: "b", Host: "h1", Repo: "o/r"},
			{Name: "c", Host: "h1", Repo: "o/r"},
		},
	}

	var buf bytes.Buffer
	removed, err := CleanupOffline(&buf, cfg, mgr)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if removed != 1 {
		t.Errorf("removed = %d, want 1", removed)
	}
	if got := listCalls; got != 1 {
		t.Errorf("list calls = %d, want 1 (dedup failed)", got)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(deleted) != 1 || deleted[0] != 100 {
		t.Errorf("deleted IDs = %v, want [100]", deleted)
	}
}

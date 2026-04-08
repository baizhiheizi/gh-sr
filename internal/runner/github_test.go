package runner

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestGitHubClient_GetRegistrationToken(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/repos/o/r/actions/runners/registration-token" {
			http.NotFound(w, r)
			return
		}
		_ = json.NewEncoder(w).Encode(tokenResponse{Token: "regtok"})
	}))
	defer ts.Close()

	g := NewGitHubClientWithHTTP("pat", ts.Client(), ts.URL)
	tok, err := g.GetRegistrationToken("o/r")
	if err != nil {
		t.Fatal(err)
	}
	if tok != "regtok" {
		t.Errorf("token %q", tok)
	}
}

func TestGitHubClient_GetRegistrationToken_errors(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("fail"))
	}))
	defer ts.Close()

	g := NewGitHubClientWithHTTP("pat", ts.Client(), ts.URL)
	_, err := g.GetRegistrationToken("o/r")
	if err == nil || !strings.Contains(err.Error(), "HTTP") {
		t.Fatalf("expected HTTP error: %v", err)
	}

	ts2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(tokenResponse{Token: ""})
	}))
	defer ts2.Close()

	g2 := NewGitHubClientWithHTTP("pat", ts2.Client(), ts2.URL)
	_, err = g2.GetRegistrationToken("o/r")
	if err == nil || !strings.Contains(err.Error(), "empty registration token") {
		t.Fatalf("expected empty token error: %v", err)
	}
}

func TestGitHubClient_GetRemovalToken(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/repos/o/r/actions/runners/remove-token" {
			http.NotFound(w, r)
			return
		}
		_ = json.NewEncoder(w).Encode(tokenResponse{Token: "rem"})
	}))
	defer ts.Close()

	g := NewGitHubClientWithHTTP("pat", ts.Client(), ts.URL)
	tok, err := g.GetRemovalToken("o/r")
	if err != nil || tok != "rem" {
		t.Fatalf("got %q %v", tok, err)
	}
}

func TestGitHubClient_ListRunners(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/repos/o/r/actions/runners" {
			http.NotFound(w, r)
			return
		}
		_ = json.NewEncoder(w).Encode(runnersResponse{
			Runners: []GitHubRunner{{ID: 1, Name: "r-1", Status: "online"}},
		})
	}))
	defer ts.Close()

	g := NewGitHubClientWithHTTP("pat", ts.Client(), ts.URL)
	runners, err := g.ListRunners("o/r")
	if err != nil {
		t.Fatal(err)
	}
	if len(runners) != 1 || runners[0].Name != "r-1" {
		t.Fatalf("got %+v", runners)
	}
}

func TestGitHubClient_ListRunnersScoped_pagination(t *testing.T) {
	t.Parallel()
	// Page 1 returns 100 runners; page 2 returns 1 runner — total 101.
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/repos/o/r/actions/runners" {
			http.NotFound(w, r)
			return
		}
		page := r.URL.Query().Get("page")
		if page == "1" {
			runners := make([]GitHubRunner, 100)
			for i := range runners {
				runners[i] = GitHubRunner{ID: int64(i + 1), Name: fmt.Sprintf("r-%d", i+1)}
			}
			_ = json.NewEncoder(w).Encode(runnersResponse{Runners: runners})
			return
		}
		_ = json.NewEncoder(w).Encode(runnersResponse{
			Runners: []GitHubRunner{{ID: 101, Name: "r-101"}},
		})
	}))
	defer ts.Close()

	g := NewGitHubClientWithHTTP("pat", ts.Client(), ts.URL)
	runners, err := g.ListRunners("o/r")
	if err != nil {
		t.Fatal(err)
	}
	if len(runners) != 101 {
		t.Errorf("expected 101 runners across 2 pages, got %d", len(runners))
	}
	if runners[100].Name != "r-101" {
		t.Errorf("last runner: %+v", runners[100])
	}
}

func TestGitHubClient_DeleteRunner(t *testing.T) {
	var method string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method = r.Method
		if r.URL.Path != "/repos/o/r/actions/runners/42" {
			http.NotFound(w, r)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer ts.Close()

	g := NewGitHubClientWithHTTP("pat", ts.Client(), ts.URL)
	if err := g.DeleteRunner("o/r", 42); err != nil {
		t.Fatal(err)
	}
	if method != http.MethodDelete {
		t.Errorf("method %s", method)
	}
}

func TestGitHubClient_DeleteRunner_errorStatus(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = io.WriteString(w, "nope")
	}))
	defer ts.Close()

	g := NewGitHubClientWithHTTP("pat", ts.Client(), ts.URL)
	err := g.DeleteRunner("o/r", 1)
	if err == nil || !strings.Contains(err.Error(), "HTTP 400") {
		t.Fatalf("expected 400: %v", err)
	}
}

func TestGitHubClient_GetLatestRunnerVersion(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/repos/actions/runner/releases/latest" {
			http.NotFound(w, r)
			return
		}
		_ = json.NewEncoder(w).Encode(releaseResponse{TagName: "v2.330.0"})
	}))
	defer ts.Close()

	g := NewGitHubClientWithHTTP("pat", ts.Client(), ts.URL)
	v, err := g.GetLatestRunnerVersion()
	if err != nil || v != "2.330.0" {
		t.Fatalf("got %q %v", v, err)
	}
}

func TestGitHubClient_GetRegistrationTokenScoped_org(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/orgs/my-org/actions/runners/registration-token" {
			http.NotFound(w, r)
			return
		}
		_ = json.NewEncoder(w).Encode(tokenResponse{Token: "org-regtok"})
	}))
	defer ts.Close()

	g := NewGitHubClientWithHTTP("pat", ts.Client(), ts.URL)
	tok, err := g.GetRegistrationTokenScoped("org", "my-org")
	if err != nil {
		t.Fatal(err)
	}
	if tok != "org-regtok" {
		t.Errorf("token %q", tok)
	}
}

func TestGitHubClient_GetRemovalTokenScoped_org(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/orgs/my-org/actions/runners/remove-token" {
			http.NotFound(w, r)
			return
		}
		_ = json.NewEncoder(w).Encode(tokenResponse{Token: "org-rem"})
	}))
	defer ts.Close()

	g := NewGitHubClientWithHTTP("pat", ts.Client(), ts.URL)
	tok, err := g.GetRemovalTokenScoped("org", "my-org")
	if err != nil || tok != "org-rem" {
		t.Fatalf("got %q %v", tok, err)
	}
}

func TestGitHubClient_ListRunnersScoped_org(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/orgs/my-org/actions/runners" {
			http.NotFound(w, r)
			return
		}
		_ = json.NewEncoder(w).Encode(runnersResponse{
			Runners: []GitHubRunner{{ID: 1, Name: "org-r-1", Status: "online"}},
		})
	}))
	defer ts.Close()

	g := NewGitHubClientWithHTTP("pat", ts.Client(), ts.URL)
	runners, err := g.ListRunnersScoped("org", "my-org")
	if err != nil {
		t.Fatal(err)
	}
	if len(runners) != 1 || runners[0].Name != "org-r-1" {
		t.Fatalf("got %+v", runners)
	}
}

func TestGitHubClient_DeleteRunnerScoped_org(t *testing.T) {
	var method, path string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method = r.Method
		path = r.URL.Path
		w.WriteHeader(http.StatusNoContent)
	}))
	defer ts.Close()

	g := NewGitHubClientWithHTTP("pat", ts.Client(), ts.URL)
	if err := g.DeleteRunnerScoped("org", "my-org", 42); err != nil {
		t.Fatal(err)
	}
	if method != http.MethodDelete {
		t.Errorf("method %s", method)
	}
	if path != "/orgs/my-org/actions/runners/42" {
		t.Errorf("path %s", path)
	}
}

func TestGitHubClient_actionsURL(t *testing.T) {
	t.Parallel()
	g := NewGitHubClientWithHTTP("pat", nil, "https://api.github.com")
	if u := g.actionsURL("repo", "o/r", "runners"); u != "https://api.github.com/repos/o/r/actions/runners" {
		t.Errorf("repo URL: %s", u)
	}
	if u := g.actionsURL("org", "my-org", "runners"); u != "https://api.github.com/orgs/my-org/actions/runners" {
		t.Errorf("org URL: %s", u)
	}
}

func TestGitHubClient_GetLatestRunnerVersion_emptyTag(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(releaseResponse{TagName: "v"})
	}))
	defer ts.Close()

	g := NewGitHubClientWithHTTP("pat", ts.Client(), ts.URL)
	_, err := g.GetLatestRunnerVersion()
	if err == nil || !strings.Contains(err.Error(), "empty version") {
		t.Fatalf("expected empty version err: %v", err)
	}
}

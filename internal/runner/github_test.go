package runner

import (
	"encoding/base64"
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
	tok, err := g.GetRegistrationTokenScoped("repo", "o/r")
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
	_, err := g.GetRegistrationTokenScoped("repo", "o/r")
	if err == nil || !strings.Contains(err.Error(), "HTTP") {
		t.Fatalf("expected HTTP error: %v", err)
	}

	ts2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(tokenResponse{Token: ""})
	}))
	defer ts2.Close()

	g2 := NewGitHubClientWithHTTP("pat", ts2.Client(), ts2.URL)
	_, err = g2.GetRegistrationTokenScoped("repo", "o/r")
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
	tok, err := g.GetRemovalTokenScoped("repo", "o/r")
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
	runners, err := g.ListRunnersScoped("repo", "o/r")
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
	runners, err := g.ListRunnersScoped("repo", "o/r")
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
	if err := g.DeleteRunnerScoped("repo", "o/r", 42); err != nil {
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
	err := g.DeleteRunnerScoped("repo", "o/r", 1)
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

// TestGitHubClient_get_etagCache verifies the If-None-Match / 304 Not Modified
// round trip: the first GET stores the ETag, the second GET sends
// If-None-Match, the server returns 304, and we serve the cached body without
// a second body transfer. Hit-count assertions confirm the server saw one
// unconditional and one conditional GET — proving the cache actually
// short-circuited the second request rather than silently re-fetching.
func TestGitHubClient_get_etagCache(t *testing.T) {
	t.Parallel()
	const etag = `"W/abc123"`
	body := []byte(`{"runners":[{"id":1,"name":"r-1","status":"online","busy":false,"os":"Linux","labels":[]}]}`)
	var hits int
	var sawIfNoneMatch bool
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		if r.Header.Get("If-None-Match") == etag {
			sawIfNoneMatch = true
			w.WriteHeader(http.StatusNotModified)
			return
		}
		w.Header().Set("ETag", etag)
		_, _ = w.Write(body)
	}))
	defer ts.Close()

	g := NewGitHubClientWithHTTP("pat", ts.Client(), ts.URL)
	first, err := g.ListRunnersScoped("repo", "o/r")
	if err != nil {
		t.Fatal(err)
	}
	if len(first) != 1 || first[0].Name != "r-1" {
		t.Fatalf("first call: got %+v", first)
	}

	second, err := g.ListRunnersScoped("repo", "o/r")
	if err != nil {
		t.Fatal(err)
	}
	if len(second) != 1 || second[0].Name != "r-1" {
		t.Fatalf("second call: got %+v", second)
	}

	if hits != 2 {
		t.Errorf("server hits: got %d, want 2", hits)
	}
	if !sawIfNoneMatch {
		t.Error("server never saw If-None-Match header on second call")
	}
}

// TestGitHubClient_get_etagRefresh verifies that when the server reports the
// resource has changed (200 with a new ETag), the client stores the fresh
// payload and the third call still uses conditional GET semantics against the
// updated ETag. This guards against an off-by-one where the cache only stores
// etags but never refreshes the body.
func TestGitHubClient_get_etagRefresh(t *testing.T) {
	t.Parallel()
	var hits int
	var lastETag string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		incoming := r.Header.Get("If-None-Match")
		switch {
		case hits == 1:
			w.Header().Set("ETag", `"v1"`)
			_, _ = w.Write([]byte(`{"runners":[{"id":1,"name":"r-1","status":"online","busy":false,"os":"Linux","labels":[]}]}`))
		case hits == 2 && incoming == `"v1"`:
			w.Header().Set("ETag", `"v2"`)
			_, _ = w.Write([]byte(`{"runners":[{"id":2,"name":"r-2","status":"offline","busy":false,"os":"Linux","labels":[]}]}`))
		case hits == 3 && incoming == `"v2"`:
			lastETag = incoming
			w.WriteHeader(http.StatusNotModified)
		default:
			t.Errorf("unexpected request: hit=%d incoming=%q", hits, incoming)
		}
	}))
	defer ts.Close()

	g := NewGitHubClientWithHTTP("pat", ts.Client(), ts.URL)
	if _, err := g.ListRunnersScoped("repo", "o/r"); err != nil {
		t.Fatal(err)
	}
	second, err := g.ListRunnersScoped("repo", "o/r")
	if err != nil {
		t.Fatal(err)
	}
	if len(second) != 1 || second[0].Name != "r-2" {
		t.Errorf("after refresh, expected r-2, got %+v", second)
	}
	if _, err := g.ListRunnersScoped("repo", "o/r"); err != nil {
		t.Fatal(err)
	}
	if lastETag != `"v2"` {
		t.Errorf("third call If-None-Match: got %q, want %q", lastETag, `"v2"`)
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

func TestGitHubClient_GetLatestRunnerVersion_cachesResult(t *testing.T) {
	var requestCount int
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		_ = json.NewEncoder(w).Encode(releaseResponse{TagName: "v2.331.0"})
	}))
	defer ts.Close()

	g := NewGitHubClientWithHTTP("pat", ts.Client(), ts.URL)
	for i := 0; i < 5; i++ {
		v, err := g.GetLatestRunnerVersion()
		if err != nil {
			t.Fatalf("call %d: got %v", i+1, err)
		}
		if v != "2.331.0" {
			t.Fatalf("call %d: got %q, want 2.331.0", i+1, v)
		}
	}
	if requestCount != 1 {
		t.Errorf("expected 1 request, got %d", requestCount)
	}
}

func TestGitHubClient_GetLatestRunnerVersion_errorNotRetried(t *testing.T) {
	var requestCount int
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()

	g := NewGitHubClientWithHTTP("pat", ts.Client(), ts.URL)
	for i := 0; i < 3; i++ {
		_, err := g.GetLatestRunnerVersion()
		if err == nil {
			t.Fatalf("call %d: expected error", i+1)
		}
	}
	if requestCount != 1 {
		t.Errorf("expected 1 request (error should not be retried), got %d", requestCount)
	}
}

func TestGitHubClient_fetchToken_emptyTokenWithHint(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(tokenResponse{Token: ""})
	}))
	defer ts.Close()

	g := NewGitHubClientWithHTTP("pat", ts.Client(), ts.URL)
	_, err := g.fetchToken("repo", "o/r", "runners/registration-token", "registration", "check GitHub token and repo admin access")
	if err == nil {
		t.Fatal("expected error")
	}
	msg := err.Error()
	want := "empty registration token for o/r (check GitHub token and repo admin access)"
	if msg != want {
		t.Errorf("error mismatch\n got: %q\nwant: %q", msg, want)
	}
}

func TestGitHubClient_fetchToken_emptyTokenNoHint(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(tokenResponse{Token: ""})
	}))
	defer ts.Close()

	g := NewGitHubClientWithHTTP("pat", ts.Client(), ts.URL)
	_, err := g.fetchToken("repo", "o/r", "runners/remove-token", "removal", "")
	if err == nil {
		t.Fatal("expected error")
	}
	msg := err.Error()
	want := "empty removal token for o/r"
	if msg != want {
		t.Errorf("error mismatch\n got: %q\nwant: %q", msg, want)
	}
}

func TestGitHubClient_fetchToken_parseError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("not-json"))
	}))
	defer ts.Close()

	g := NewGitHubClientWithHTTP("pat", ts.Client(), ts.URL)
	_, err := g.fetchToken("repo", "o/r", "runners/registration-token", "registration", "")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "parsing registration token") {
		t.Fatalf("expected parsing error, got: %v", err)
	}
}

func TestGitHubClient_fetchToken_postError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("boom"))
	}))
	defer ts.Close()

	g := NewGitHubClientWithHTTP("pat", ts.Client(), ts.URL)
	_, err := g.fetchToken("repo", "o/r", "runners/registration-token", "registration", "")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "getting registration token for o/r") {
		t.Fatalf("expected wrapped get-error, got: %v", err)
	}
}

func TestGitHubClient_fetchToken_orgScope(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/orgs/my-org/actions/runners/registration-token" {
			http.NotFound(w, r)
			return
		}
		_ = json.NewEncoder(w).Encode(tokenResponse{Token: "org-via-helper"})
	}))
	defer ts.Close()

	g := NewGitHubClientWithHTTP("pat", ts.Client(), ts.URL)
	tok, err := g.fetchToken("org", "my-org", "runners/registration-token", "registration", "")
	if err != nil {
		t.Fatal(err)
	}
	if tok != "org-via-helper" {
		t.Errorf("token %q", tok)
	}
}

func TestGitHubClient_ListLockWorkflows(t *testing.T) {
	listing := []contentsEntry{
		{Name: "build.yml", Type: "file"},
		{Name: "sub", Type: "dir"},
		{Name: "b.lock.yml", Type: "file"},
		{Name: "a.lock.yml", Type: "file"},
	}
	rawFiles := map[string]string{
		"a.lock.yml": "jobs: a",
		"b.lock.yml": "jobs: b\nwith: multiline",
	}
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/repos/o/r/contents/.github/workflows" {
			_ = json.NewEncoder(w).Encode(listing)
			return
		}
		name := strings.TrimPrefix(r.URL.Path, "/repos/o/r/contents/.github/workflows/")
		body, ok := rawFiles[name]
		if !ok {
			http.NotFound(w, r)
			return
		}
		// GitHub wraps base64 payloads at 60 chars; decoding must strip the newlines.
		enc := base64.StdEncoding.EncodeToString([]byte(body))
		var wrapped strings.Builder
		for i := 0; i < len(enc); i += 60 {
			end := min(i+60, len(enc))
			wrapped.WriteString(enc[i:end])
			wrapped.WriteString("\n")
		}
		_ = json.NewEncoder(w).Encode(contentsFile{Name: name, Content: wrapped.String(), Encoding: "base64"})
	}))
	defer ts.Close()

	g := NewGitHubClientWithHTTP("pat", ts.Client(), ts.URL)

	files, err := g.ListLockWorkflows("o/r", 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 2 || files[0].Name != "a.lock.yml" || files[1].Name != "b.lock.yml" {
		t.Fatalf("expected sorted .lock.yml files only, got %+v", files)
	}
	if files[0].Content != rawFiles["a.lock.yml"] || files[1].Content != rawFiles["b.lock.yml"] {
		t.Fatalf("expected decoded contents, got %+v", files)
	}

	trunc, err := g.ListLockWorkflows("o/r", 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(trunc) != 1 || trunc[0].Name != "a.lock.yml" {
		t.Fatalf("expected truncation to first sorted file, got %+v", trunc)
	}
}

func TestGitHubClient_ListLockWorkflows_edgeCases(t *testing.T) {
	t.Run("plain encoding passthrough", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if strings.HasSuffix(r.URL.Path, "/b.lock.yml") {
				_ = json.NewEncoder(w).Encode(contentsFile{Name: "b.lock.yml", Content: "raw: text"})
				return
			}
			_ = json.NewEncoder(w).Encode([]contentsEntry{{Name: "b.lock.yml", Type: "file"}})
		}))
		defer ts.Close()

		g := NewGitHubClientWithHTTP("pat", ts.Client(), ts.URL)
		files, err := g.ListLockWorkflows("o/r", 5)
		if err != nil {
			t.Fatal(err)
		}
		if len(files) != 1 || files[0].Content != "raw: text" {
			t.Fatalf("expected verbatim content, got %+v", files)
		}
	})

	t.Run("no lock files", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_ = json.NewEncoder(w).Encode([]contentsEntry{{Name: "ci.yml", Type: "file"}})
		}))
		defer ts.Close()

		g := NewGitHubClientWithHTTP("pat", ts.Client(), ts.URL)
		files, err := g.ListLockWorkflows("o/r", 5)
		if err != nil {
			t.Fatal(err)
		}
		if len(files) != 0 {
			t.Fatalf("expected no files, got %+v", files)
		}
	})

	t.Run("listing fetch error", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "boom", http.StatusInternalServerError)
		}))
		defer ts.Close()

		g := NewGitHubClientWithHTTP("pat", ts.Client(), ts.URL)
		_, err := g.ListLockWorkflows("o/r", 5)
		if err == nil || !strings.Contains(err.Error(), "listing .github/workflows for o/r") {
			t.Fatalf("expected listing error, got: %v", err)
		}
	})

	t.Run("listing parse error", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("{}"))
		}))
		defer ts.Close()

		g := NewGitHubClientWithHTTP("pat", ts.Client(), ts.URL)
		_, err := g.ListLockWorkflows("o/r", 5)
		if err == nil || !strings.Contains(err.Error(), "parsing workflows listing for o/r") {
			t.Fatalf("expected listing parse error, got: %v", err)
		}
	})

	t.Run("file fetch error", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if strings.HasSuffix(r.URL.Path, "/a.lock.yml") {
				http.Error(w, "boom", http.StatusInternalServerError)
				return
			}
			_ = json.NewEncoder(w).Encode([]contentsEntry{{Name: "a.lock.yml", Type: "file"}})
		}))
		defer ts.Close()

		g := NewGitHubClientWithHTTP("pat", ts.Client(), ts.URL)
		_, err := g.ListLockWorkflows("o/r", 5)
		if err == nil || !strings.Contains(err.Error(), "fetching a.lock.yml in o/r") {
			t.Fatalf("expected file fetch error, got: %v", err)
		}
	})

	t.Run("bad base64", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if strings.HasSuffix(r.URL.Path, "/a.lock.yml") {
				_ = json.NewEncoder(w).Encode(contentsFile{Name: "a.lock.yml", Content: "!!!not-base64!!!", Encoding: "base64"})
				return
			}
			_ = json.NewEncoder(w).Encode([]contentsEntry{{Name: "a.lock.yml", Type: "file"}})
		}))
		defer ts.Close()

		g := NewGitHubClientWithHTTP("pat", ts.Client(), ts.URL)
		_, err := g.ListLockWorkflows("o/r", 5)
		if err == nil || !strings.Contains(err.Error(), "decoding a.lock.yml in o/r") {
			t.Fatalf("expected decode error, got: %v", err)
		}
	})
}

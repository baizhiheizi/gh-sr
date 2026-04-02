package runner

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/an-lee/ghr/internal/config"
)

func TestManager_EnrichWithGitHubStatus(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/repos/o/r/actions/runners" {
			http.NotFound(w, r)
			return
		}
		_ = json.NewEncoder(w).Encode(runnersResponse{
			Runners: []GitHubRunner{
				{Name: "ci-1", Status: "online", Busy: false},
				{Name: "other", Status: "offline", Busy: false},
			},
		})
	}))
	defer ts.Close()

	cfg := &config.Config{
		GitHub: config.GitHubConfig{PAT: "p"},
		Hosts: map[string]config.HostConfig{
			"h": {Addr: "a@b", OS: "linux", Arch: "amd64"},
		},
		Runners: []config.RunnerConfig{
			{Name: "ci", Repo: "o/r", Host: "h", Count: 1},
		},
	}

	m := &Manager{GitHub: NewGitHubClientWithHTTP("p", ts.Client(), ts.URL)}
	statuses := []RunnerStatus{
		{Instance: "ci-1", Repo: "o/r"},
	}
	m.EnrichWithGitHubStatus(statuses, cfg)
	if statuses[0].Remote != "online" || statuses[0].Busy {
		t.Fatalf("got %+v", statuses[0])
	}
}

func TestManager_CleanupOffline(t *testing.T) {
	var deletePaths []string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/repos/o/r/actions/runners":
			_ = json.NewEncoder(w).Encode(runnersResponse{
				Runners: []GitHubRunner{
					{ID: 10, Name: "gone", Status: "offline"},
					{ID: 11, Name: "up", Status: "online"},
				},
			})
		case r.Method == http.MethodDelete && r.URL.Path == "/repos/o/r/actions/runners/10":
			w.WriteHeader(http.StatusNoContent)
		default:
			http.NotFound(w, r)
		}
		if r.Method == http.MethodDelete {
			deletePaths = append(deletePaths, r.URL.Path)
		}
	}))
	defer ts.Close()

	cfg := &config.Config{
		GitHub: config.GitHubConfig{PAT: "p"},
		Hosts: map[string]config.HostConfig{
			"h": {Addr: "a@b", OS: "linux", Arch: "amd64"},
		},
		Runners: []config.RunnerConfig{
			{Name: "ci", Repo: "o/r", Host: "h", Count: 1},
		},
	}

	m := &Manager{GitHub: NewGitHubClientWithHTTP("p", ts.Client(), ts.URL)}
	n, err := m.CleanupOffline(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("removed count: %d", n)
	}
	if len(deletePaths) != 1 || deletePaths[0] != "/repos/o/r/actions/runners/10" {
		t.Fatalf("delete paths: %v", deletePaths)
	}
}

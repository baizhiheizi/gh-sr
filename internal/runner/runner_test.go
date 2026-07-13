package runner

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/an-lee/gh-sr/internal/config"
	"github.com/an-lee/gh-sr/internal/host"
	"github.com/an-lee/gh-sr/internal/testutil"
)

func TestManager_EnrichWithGitHubStatus(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/repos/o/r/actions/runners" {
			http.NotFound(w, r)
			return
		}
		_ = json.NewEncoder(w).Encode(runnersResponse{
			Runners: []GitHubRunner{
				{Name: "ci-1", Status: "online", Busy: false, OS: "Linux"},
				{Name: "other", Status: "offline", Busy: false, OS: "Linux"},
			},
		})
	}))
	defer ts.Close()

	cfg := &config.Config{
		GitHub: config.GitHubConfig{},
		Hosts: map[string]config.HostConfig{
			"h": {Addr: "a@b", OS: "linux", Arch: "amd64"},
		},
		Runners: []config.RunnerConfig{
			{Name: "ci", Repo: "o/r", Host: "h", Count: 1},
		},
	}

	m := &Manager{GitHub: NewGitHubClientWithHTTP("p", ts.Client(), ts.URL)}
	statuses := []RunnerStatus{
		{Instance: "ci-1", Repo: "o/r", Host: "h", Mode: "docker"},
	}
	m.EnrichWithGitHubStatus(statuses, cfg)
	if statuses[0].Remote != "online" || statuses[0].Busy {
		t.Fatalf("got %+v", statuses[0])
	}
}

func TestManager_EnrichWithGitHubStatus_OS_mismatch_skips(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/repos/o/r/actions/runners" {
			http.NotFound(w, r)
			return
		}
		_ = json.NewEncoder(w).Encode(runnersResponse{
			Runners: []GitHubRunner{
				{Name: "ci-1", Status: "online", Busy: false, OS: "Linux"},
			},
		})
	}))
	defer ts.Close()

	cfg := &config.Config{
		GitHub: config.GitHubConfig{},
		Hosts: map[string]config.HostConfig{
			"win": {Addr: "a@b", OS: "windows", Arch: "amd64"},
		},
		Runners: []config.RunnerConfig{
			{Name: "ci", Repo: "o/r", Host: "win", Count: 1},
		},
	}

	m := &Manager{GitHub: NewGitHubClientWithHTTP("p", ts.Client(), ts.URL)}
	statuses := []RunnerStatus{
		{Instance: "ci-1", Repo: "o/r", Host: "win", Mode: "native"},
	}
	m.EnrichWithGitHubStatus(statuses, cfg)
	if statuses[0].Remote != "" {
		t.Fatalf("expected no GitHub match for OS mismatch, got Remote=%q", statuses[0].Remote)
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
		GitHub: config.GitHubConfig{},
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

// TestManager_Start_OneSshRoundTripPerInstance pins the perf shape of the
// native Start orchestrator: each instance must contribute exactly one SSH
// round-trip on the probe path (combined linuxSvcAndAutostartProbe), not
// two (the old svcShPresent + autostart.Detect pair). The test mocks three
// instances so a regression that loops back to the legacy two-probe pattern
// would surface as `probeCount > 3`.
func TestManager_Start_OneSshRoundTripPerInstance(t *testing.T) {
	t.Parallel()

	probeCount := 0
	startCount := 0
	mock := &testutil.MockExecutor{
		RunFn: func(cmd string) (string, error) {
			switch {
			// Combined probe from linuxSvcAndAutostartProbe — emit "S" so the
			// svc.sh branch is taken (no GitHub API calls), and so the probe
			// result is distinguishable from the start cmd below.
			case strings.Contains(cmd, "svc.sh") && strings.Contains(cmd, ".config/systemd/user/") && strings.Contains(cmd, "/etc/systemd/system/"):
				probeCount++
				return "S\n", nil
			case strings.Contains(cmd, "svc.sh"):
				startCount++
				return "", nil
			default:
				return "", nil
			}
		},
	}
	h := host.NewHost("linux", config.HostConfig{OS: "linux", Addr: "local"})
	h.SetConn(mock)

	m := NewManager("")
	var buf bytes.Buffer
	m.Out = &buf

	rc := config.RunnerConfig{Name: "ci", Host: "h", Repo: "o/r", Count: 3}
	if err := m.Start(h, rc); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	if probeCount != 3 {
		t.Errorf("expected 3 combined-probe round-trips (1 per instance), got %d", probeCount)
	}
	if startCount != 3 {
		t.Errorf("expected 3 start invocations, got %d", startCount)
	}
}

// TestManager_Stop_OneSshRoundTripPerInstance mirrors Start: each instance
// on the native Stop orchestrator must contribute exactly one SSH round-trip
// on the probe path (combined linuxSvcAndAutostartProbe).
func TestManager_Stop_OneSshRoundTripPerInstance(t *testing.T) {
	t.Parallel()

	probeCount := 0
	stopCount := 0
	mock := &testutil.MockExecutor{
		RunFn: func(cmd string) (string, error) {
			switch {
			case strings.Contains(cmd, "svc.sh") && strings.Contains(cmd, ".config/systemd/user/") && strings.Contains(cmd, "/etc/systemd/system/"):
				probeCount++
				return "", nil
			case strings.Contains(cmd, ".runner_pid") || strings.Contains(cmd, "systemctl"):
				stopCount++
				return "stopped\n", nil
			default:
				return "", nil
			}
		},
	}
	h := host.NewHost("linux", config.HostConfig{OS: "linux", Addr: "local"})
	h.SetConn(mock)

	m := NewManager("")
	var buf bytes.Buffer
	m.Out = &buf

	rc := config.RunnerConfig{Name: "ci", Host: "h", Repo: "o/r", Count: 3}
	if err := m.Stop(h, rc); err != nil {
		t.Fatalf("Stop failed: %v", err)
	}
	if probeCount != 3 {
		t.Errorf("expected 3 combined-probe round-trips (1 per instance), got %d", probeCount)
	}
	if stopCount != 3 {
		t.Errorf("expected 3 stop invocations, got %d", stopCount)
	}
}

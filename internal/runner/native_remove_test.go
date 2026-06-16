package runner

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
	"github.com/an-lee/gh-sr/internal/testutil"
)

func TestRemoveNativeCleansServicesAndDirectory(t *testing.T) {
	t.Parallel()

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/repos/o/r/actions/runners/remove-token" {
			http.NotFound(w, r)
			return
		}
		_ = json.NewEncoder(w).Encode(tokenResponse{Token: "rem"})
	}))
	defer ts.Close()

	var sawAutostartUninstall, sawRemoveDir bool
	mock := &testutil.MockExecutor{
		RunFn: func(cmd string) (string, error) {
			switch {
			case strings.Contains(cmd, "test -f") && strings.Contains(cmd, "svc.sh"):
				return "no\n", nil
			case strings.Contains(cmd, "&& echo user || true"):
				return "user\n", nil
			case strings.Contains(cmd, "systemctl --user disable"):
				sawAutostartUninstall = true
				return "", nil
			case strings.Contains(cmd, ".runner_pid"):
				return "not running\n", nil
			case strings.Contains(cmd, "rm -rf"):
				sawRemoveDir = true
				return "", nil
			case strings.Contains(cmd, "config.sh remove"):
				return "", nil
			default:
				return "", nil
			}
		},
	}

	h := host.NewHost("linux", config.HostConfig{OS: "linux", Addr: "local"})
	h.SetConn(mock)

	var buf bytes.Buffer
	m := NewManager("")
	m.Out = &buf
	m.GitHub = NewGitHubClientWithHTTP("pat", ts.Client(), ts.URL)

	rc := config.RunnerConfig{Name: "ci", Host: "linux", Repo: "o/r", Count: 1}
	if err := m.removeNative(h, rc, "ci-1"); err != nil {
		t.Fatal(err)
	}
	if !sawAutostartUninstall {
		t.Fatal("expected autostart uninstall")
	}
	if !sawRemoveDir {
		t.Fatal("expected runner directory removal")
	}
	out := buf.String()
	if !strings.Contains(out, "autostart removed") {
		t.Fatalf("expected autostart removed message, got:\n%s", out)
	}
	if !strings.Contains(out, "runner directory removed") {
		t.Fatalf("expected runner directory removed message, got:\n%s", out)
	}
}

func TestRemoveNativeDirectoryError(t *testing.T) {
	t.Parallel()

	mock := &testutil.MockExecutor{
		RunFn: func(cmd string) (string, error) {
			if strings.Contains(cmd, "rm -rf") {
				return "", errors.New("permission denied")
			}
			return "", nil
		},
	}
	h := host.NewHost("linux", config.HostConfig{OS: "linux", Addr: "local"})
	h.SetConn(mock)

	m := NewManager("")

	err := m.removeNativeDirectory(h, "ci-1")
	if err == nil || !strings.Contains(err.Error(), "permission denied") {
		t.Fatalf("expected directory removal error, got %v", err)
	}
}

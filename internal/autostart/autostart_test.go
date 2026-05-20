package autostart

import (
	"strings"
	"testing"

	"github.com/an-lee/gh-sr/internal/config"
	"github.com/an-lee/gh-sr/internal/host"
)

// mockExecutor implements the Executor interface for testing.
type mockExecutor struct {
	output string
	runErr error

	runFn func(cmd string) (string, error)
}

func (m *mockExecutor) Run(cmd string) (string, error) {
	if m.runFn != nil {
		return m.runFn(cmd)
	}
	return m.output, m.runErr
}

func (m *mockExecutor) Upload(localPath, remotePath string) error {
	return nil
}

func (m *mockExecutor) Close() error {
	return nil
}

func newMockHost(name string, cfg config.HostConfig, mock *mockExecutor) *host.Host {
	h := host.NewHost(name, cfg)
	h.SetConn(mock)
	return h
}

func TestServiceBasename(t *testing.T) {
	t.Parallel()
	if got := ServiceBasename("ci-1"); got != "ghsr-runner-ci-1" {
		t.Errorf("got %q", got)
	}
	if got := ServiceBasename("my-runner"); got != "ghsr-runner-my-runner" {
		t.Errorf("got %q", got)
	}
}

func TestAbsRunnerDir(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name  string
		os    string
		home  string
		inst  string
		want  string
	}{
		{"linux", "linux", "/home/u", "ci-1", "/home/u/.gh-sr/runners/ci-1"},
		{"linux trailing slash", "linux", "/home/u/", "ci-1", "/home/u/.gh-sr/runners/ci-1"},
		{"darwin", "darwin", "/Users/u", "ci-1", "/Users/u/.gh-sr/runners/ci-1"},
		{"windows", "windows", `C:\Users\u`, "ci-1", `C:\Users\u\.gh-sr\runners\ci-1`},
		{"windows trailing backslash", "windows", `C:\Users\u\`, "ci-1", `C:\Users\u\.gh-sr\runners\ci-1`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			h := host.NewHost("test", config.HostConfig{OS: tc.os})
			got := absRunnerDir(h, tc.home, tc.inst)
			if got != tc.want {
				t.Errorf("absRunnerDir(%q,%q,%q) = %q, want %q", tc.os, tc.home, tc.inst, got, tc.want)
			}
		})
	}
}

func TestIsServiceActive(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name   string
		os     string
		mock   *mockExecutor
		inst   string
		kind   Kind
		want   bool
		err    bool
	}{
		{
			name: "systemd-user active",
			os:   "linux",
			mock: &mockExecutor{output: "active"},
			inst: "ci-1",
			kind: KindSystemdUser,
			want: true,
		},
		{
			name: "systemd-user inactive",
			os:   "linux",
			mock: &mockExecutor{output: "inactive"},
			inst: "ci-1",
			kind: KindSystemdUser,
			want: false,
		},
		{
			name: "launchd running",
			os:   "darwin",
			mock: &mockExecutor{
				runFn: func(cmd string) (string, error) {
					if strings.Contains(cmd, "launchctl print") {
						return "state = running\n", nil
					}
					return "", nil
				},
			},
			inst: "ci-1",
			kind: KindLaunchd,
			want: true,
		},
		{
			name: "launchd not running",
			os:   "darwin",
			mock: &mockExecutor{
				runFn: func(cmd string) (string, error) {
					if strings.Contains(cmd, "launchctl print") {
						return "state = stopped\n", nil
					}
					return "", nil
				},
			},
			inst: "ci-1",
			kind: KindLaunchd,
			want: false,
		},
		{
			name: "windows task running",
			os:   "windows",
			mock: &mockExecutor{output: "Running"},
			inst: "ci-1",
			kind: KindWindowsTask,
			want: true,
		},
		{
			name: "windows task stopped",
			os:   "windows",
			mock: &mockExecutor{output: "Stopped"},
			inst: "ci-1",
			kind: KindWindowsTask,
			want: false,
		},
		{
			name: "unknown kind",
			os:   "linux",
			mock: &mockExecutor{},
			inst: "ci-1",
			kind: KindNone,
			want: false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			h := newMockHost("test", config.HostConfig{OS: tc.os}, tc.mock)
			got, err := IsServiceActive(h, tc.inst, tc.kind)
			if tc.err {
				if err == nil {
					t.Errorf("expected error, got active=%v", got)
				}
				return
			}
			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}
			if got != tc.want {
				t.Errorf("IsServiceActive = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestIsServiceActive_errors(t *testing.T) {
	t.Parallel()
	t.Run("systemd-user command fails", func(t *testing.T) {
		t.Parallel()
		h := newMockHost("test", config.HostConfig{OS: "linux"}, &mockExecutor{runErr: errCalled})
		_, err := IsServiceActive(h, "ci-1", KindSystemdUser)
		if err == nil {
			t.Error("expected error")
		}
	})
	t.Run("launchd command fails", func(t *testing.T) {
		t.Parallel()
		h := newMockHost("test", config.HostConfig{OS: "darwin"}, &mockExecutor{runErr: errCalled})
		_, err := IsServiceActive(h, "ci-1", KindLaunchd)
		if err == nil {
			t.Error("expected error")
		}
	})
	t.Run("windows task command fails", func(t *testing.T) {
		t.Parallel()
		h := newMockHost("test", config.HostConfig{OS: "windows"}, &mockExecutor{runErr: errCalled})
		_, err := IsServiceActive(h, "ci-1", KindWindowsTask)
		if err == nil {
			t.Error("expected error")
		}
	})
}

var errCalled = calledError{}

type calledError struct{}

func (calledError) Error() string { return "called" }
package autostart

import (
	"strings"
	"testing"

	"github.com/an-lee/gh-sr/internal/config"
	"github.com/an-lee/gh-sr/internal/host"
	"github.com/an-lee/gh-sr/internal/testutil"
)

func newMockHost(name string, cfg config.HostConfig, mock *testutil.MockExecutor) *host.Host {
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
		name string
		os   string
		home string
		inst string
		want string
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
		name string
		os   string
		mock *testutil.MockExecutor
		inst string
		kind Kind
		want bool
		err  bool
	}{
		{
			name: "systemd-user active",
			os:   "linux",
			mock: &testutil.MockExecutor{Output: "active"},
			inst: "ci-1",
			kind: KindSystemdUser,
			want: true,
		},
		{
			name: "systemd-user inactive",
			os:   "linux",
			mock: &testutil.MockExecutor{Output: "inactive"},
			inst: "ci-1",
			kind: KindSystemdUser,
			want: false,
		},
		{
			name: "launchd running",
			os:   "darwin",
			mock: &testutil.MockExecutor{
				RunFn: func(cmd string) (string, error) {
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
			mock: &testutil.MockExecutor{
				RunFn: func(cmd string) (string, error) {
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
			mock: &testutil.MockExecutor{Output: "Running"},
			inst: "ci-1",
			kind: KindWindowsTask,
			want: true,
		},
		{
			name: "windows task stopped",
			os:   "windows",
			mock: &testutil.MockExecutor{Output: "Stopped"},
			inst: "ci-1",
			kind: KindWindowsTask,
			want: false,
		},
		{
			name: "unknown kind",
			os:   "linux",
			mock: &testutil.MockExecutor{},
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

func TestDetect(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		os   string
		mock *testutil.MockExecutor
		want Kind
		err  bool
	}{
		{
			name: "linux user unit present",
			os:   "linux",
			mock: &testutil.MockExecutor{
				RunFn: func(cmd string) (string, error) {
					if strings.Contains(cmd, ".config/systemd/user/") {
						return "user\n", nil
					}
					return "", nil
				},
			},
			want: KindSystemdUser,
		},
		{
			name: "linux system unit present (user absent)",
			os:   "linux",
			mock: &testutil.MockExecutor{
				RunFn: func(cmd string) (string, error) {
					if strings.Contains(cmd, ".config/systemd/user/") {
						return "\n", nil
					}
					if strings.Contains(cmd, "/etc/systemd/system/") {
						return "system\n", nil
					}
					return "", nil
				},
			},
			want: KindSystemdSystem,
		},
		{
			name: "linux neither installed",
			os:   "linux",
			mock: &testutil.MockExecutor{Output: "\n"},
			want: KindNone,
		},
		{
			name: "darwin launchd plist present",
			os:   "darwin",
			mock: &testutil.MockExecutor{
				RunFn: func(cmd string) (string, error) {
					if strings.Contains(cmd, "LaunchAgents/") {
						return "yes\n", nil
					}
					return "", nil
				},
			},
			want: KindLaunchd,
		},
		{
			name: "darwin launchd plist absent",
			os:   "darwin",
			mock: &testutil.MockExecutor{Output: "\n"},
			want: KindNone,
		},
		{
			name: "windows scheduled task present",
			os:   "windows",
			mock: &testutil.MockExecutor{Output: "yes\n"},
			want: KindWindowsTask,
		},
		{
			name: "windows scheduled task absent",
			os:   "windows",
			mock: &testutil.MockExecutor{Output: "no\n"},
			want: KindNone,
		},
		{
			name: "unsupported OS returns error",
			os:   "freebsd",
			mock: &testutil.MockExecutor{},
			want: KindNone,
			err:  true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			h := newMockHost("test", config.HostConfig{OS: tc.os}, tc.mock)
			got, err := Detect(h, "ci-1")
			if tc.err {
				if err == nil {
					t.Errorf("expected error, got kind=%q", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Errorf("Detect = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestIsServiceActive_errors(t *testing.T) {
	t.Parallel()
	t.Run("systemd-user command fails", func(t *testing.T) {
		t.Parallel()
		h := newMockHost("test", config.HostConfig{OS: "linux"}, &testutil.MockExecutor{RunErr: errCalled})
		_, err := IsServiceActive(h, "ci-1", KindSystemdUser)
		if err == nil {
			t.Error("expected error")
		}
	})
	t.Run("launchd command fails", func(t *testing.T) {
		t.Parallel()
		h := newMockHost("test", config.HostConfig{OS: "darwin"}, &testutil.MockExecutor{RunErr: errCalled})
		_, err := IsServiceActive(h, "ci-1", KindLaunchd)
		if err == nil {
			t.Error("expected error")
		}
	})
	t.Run("windows task command fails", func(t *testing.T) {
		t.Parallel()
		h := newMockHost("test", config.HostConfig{OS: "windows"}, &testutil.MockExecutor{RunErr: errCalled})
		_, err := IsServiceActive(h, "ci-1", KindWindowsTask)
		if err == nil {
			t.Error("expected error")
		}
	})
}

var errCalled = calledError{}

type calledError struct{}

func (calledError) Error() string { return "called" }

func TestResolveAutostartTarget(t *testing.T) {
	t.Parallel()
	t.Run("detect error propagates", func(t *testing.T) {
		t.Parallel()
		h := newMockHost("test", config.HostConfig{OS: "linux"}, &testutil.MockExecutor{RunErr: errCalled})
		_, _, _, err := resolveAutostartTarget(h, "ci-1")
		if err == nil {
			t.Fatal("expected error from Detect, got nil")
		}
	})
	t.Run("invalid instance name error propagates", func(t *testing.T) {
		t.Parallel()
		h := newMockHost("test", config.HostConfig{OS: "linux"}, &testutil.MockExecutor{Output: "user"})
		// SanitizeInstance rejects names that collapse to "" after sanitization
		// (e.g. "@@@"). Detect is mocked to succeed but the instance name
		// triggers SanitizeInstance failure.
		_, _, _, err := resolveAutostartTarget(h, "@@@")
		if err == nil {
			t.Fatal("expected error from SanitizeInstance, got nil")
		}
	})
	t.Run("detected kind + sanitized name + base name returned", func(t *testing.T) {
		t.Parallel()
		h := newMockHost("test", config.HostConfig{OS: "linux"}, &testutil.MockExecutor{Output: "user"})
		kind, san, base, err := resolveAutostartTarget(h, "ci-1")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if kind != KindSystemdUser {
			t.Errorf("kind = %q, want %q", kind, KindSystemdUser)
		}
		if san != "ci-1" {
			t.Errorf("san = %q, want %q", san, "ci-1")
		}
		if base != "ghsr-runner-ci-1" {
			t.Errorf("base = %q, want %q", base, "ghsr-runner-ci-1")
		}
	})
}

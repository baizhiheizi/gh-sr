package autostart

import (
	"strings"
	"testing"

	"github.com/an-lee/gh-sr/internal/config"
	"github.com/an-lee/gh-sr/internal/testutil"
)

func TestInstanceFromServiceBasename(t *testing.T) {
	t.Parallel()
	inst, ok := instanceFromServiceBasename("ghsr-runner-rune-x1-2")
	if !ok || inst != "rune-x1-2" {
		t.Fatalf("got %q ok=%v", inst, ok)
	}
	if _, ok := instanceFromServiceBasename("other.service"); ok {
		t.Fatal("expected false for unrelated basename")
	}
}

func TestDedupeInstances(t *testing.T) {
	t.Parallel()
	got := dedupeInstances([]string{"b", "a", "b"})
	if len(got) != 2 || got[0] != "a" || got[1] != "b" {
		t.Fatalf("got %v", got)
	}
}

func TestListInstalledLinux(t *testing.T) {
	t.Parallel()
	mock := &testutil.MockExecutor{Output: "ghsr-runner-ci-1\nghsr-runner-ci-2\n"}
	h := newMockHost("linux", config.HostConfig{OS: "linux"}, mock)
	got, err := ListInstalled(h)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0] != "ci-1" || got[1] != "ci-2" {
		t.Fatalf("got %v", got)
	}
	if len(mock.Calls) == 0 || !strings.Contains(mock.Calls[0], "ghsr-runner-") {
		t.Fatalf("unexpected cmd: %v", mock.Calls)
	}
}

func TestIsStaleLinux(t *testing.T) {
	t.Parallel()
	// "no" = the runner dir + run.sh are present (the canonical yes/no probe
	// prints "yes" when present), so IsStale inverts to true.
	mock := &testutil.MockExecutor{Output: "no\n"}
	h := newMockHost("linux", config.HostConfig{OS: "linux"}, mock)
	stale, err := IsStale(h, "missing-1")
	if err != nil {
		t.Fatal(err)
	}
	if !stale {
		t.Fatal("expected stale")
	}
}

func TestIsStale(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		os   string
		mock *testutil.MockExecutor
		want bool
		err  bool
	}{
		{
			name: "linux dir missing",
			os:   "linux",
			// "no" = dir/run.sh absent (probe prints "yes" only when present).
			mock: &testutil.MockExecutor{Output: "no\n"},
			want: true,
		},
		{
			name: "linux dir present with run.sh",
			os:   "linux",
			// "yes" = dir + run.sh present → not stale.
			mock: &testutil.MockExecutor{Output: "yes\n"},
			want: false,
		},
		{
			name: "windows task dir and run.cmd present",
			os:   "windows",
			mock: &testutil.MockExecutor{Output: "no\n"},
			want: false,
		},
		{
			name: "windows task dir missing",
			os:   "windows",
			mock: &testutil.MockExecutor{Output: "yes\n"},
			want: true,
		},
		{
			name: "linux command error propagates",
			os:   "linux",
			mock: &testutil.MockExecutor{RunErr: errCalled},
			want: false,
			err:  true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			h := newMockHost("test", config.HostConfig{OS: tc.os}, tc.mock)
			got, err := IsStale(h, "ci-1")
			if tc.err {
				if err == nil {
					t.Errorf("expected error, got stale=%v", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Errorf("IsStale = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestCleanupStaleDryRun(t *testing.T) {
	t.Parallel()
	mock := &testutil.MockExecutor{
		RunFn: func(cmd string) (string, error) {
			switch {
			case strings.Contains(cmd, "for f in \"$HOME/.config/systemd/user/ghsr-runner-\""):
				return "ghsr-runner-old-1\n", nil
			case strings.Contains(cmd, "test -d"):
				// "no" = dir/run.sh absent → stale. IsStale uses the canonical
				// yes/no probe (present → "yes") and inverts, so a stale
				// instance prints "no".
				return "no\n", nil
			default:
				return "", nil
			}
		},
	}
	h := newMockHost("linux", config.HostConfig{OS: "linux"}, mock)
	removed, found, err := CleanupStale(h, true)
	if err != nil {
		t.Fatal(err)
	}
	if found != 1 || len(removed) != 1 || removed[0] != "old-1" {
		t.Fatalf("found=%d removed=%v", found, removed)
	}
}

func TestCleanupStaleRemoves(t *testing.T) {
	t.Parallel()
	var uninstallCalled bool
	mock := &testutil.MockExecutor{
		RunFn: func(cmd string) (string, error) {
			switch {
			case strings.Contains(cmd, "systemctl --user disable"):
				uninstallCalled = true
				return "", nil
			case strings.Contains(cmd, "&& echo user || true"):
				return "user\n", nil
			case strings.Contains(cmd, "for f in \"$HOME/.config/systemd/user/ghsr-runner-\""):
				return "ghsr-runner-old-1\n", nil
			case strings.Contains(cmd, "test -d"):
				// "no" = dir/run.sh absent → stale (see TestCleanupStaleDryRun).
				return "no\n", nil
			default:
				return "", nil
			}
		},
	}
	h := newMockHost("linux", config.HostConfig{OS: "linux"}, mock)
	removed, found, err := CleanupStale(h, false)
	if err != nil {
		t.Fatal(err)
	}
	if found != 1 || len(removed) != 1 {
		t.Fatalf("found=%d removed=%v", found, removed)
	}
	if !uninstallCalled {
		t.Fatal("expected uninstall to run")
	}
}

func TestServiceActiveStateFailed(t *testing.T) {
	t.Parallel()
	mock := &testutil.MockExecutor{Output: "failed\n"}
	h := newMockHost("linux", config.HostConfig{OS: "linux"}, mock)
	state, err := ServiceActiveState(h, "ci-1", KindSystemdUser)
	if err != nil {
		t.Fatal(err)
	}
	if state != "failed" {
		t.Fatalf("got %q", state)
	}
}

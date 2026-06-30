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
			case strings.Contains(cmd, "for f in \"$HOME/.config/systemd/user/ghsr-runner-\""):
				// listInstalledLinux iterates both user+system dirs in one script.
				return "ghsr-runner-old-1\n", nil
			case strings.Contains(cmd, ".config/systemd/user/") && strings.Contains(cmd, "/etc/systemd/system/"):
				// Detect: combined if/elif probe (no `for f in`).
				return "user\n", nil
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

// TestListInstalledDarwin exercises the launchd plist path of ListInstalled,
// which had no direct coverage before. Each subtest pins a distinct contract
// of listInstalledDarwin: prefix stripping, empty output, filter of unrelated
// basenames, deduplication, and command-error propagation.
func TestListInstalledDarwin(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name   string
		output string
		want   []string
	}{
		{
			name:   "happy path strips prefix and dedupes",
			output: "com.github.ghsr.runner.ci-1\ncom.github.ghsr.runner.ci-2\ncom.github.ghsr.runner.ci-1\n",
			want:   []string{"ci-1", "ci-2"},
		},
		{
			name:   "skips blank lines and unrelated basenames",
			output: "\nother.runner.ci-3\ncom.github.ghsr.runner.rune-x1-2\n",
			want:   []string{"rune-x1-2"},
		},
		{
			name:   "empty output returns empty slice",
			output: "",
			want:   nil,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			mock := &testutil.MockExecutor{Output: tc.output}
			h := newMockHost("darwin", config.HostConfig{OS: "darwin"}, mock)
			got, err := ListInstalled(h)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !equalInstances(got, tc.want) {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
			if len(mock.Calls) != 1 {
				t.Fatalf("expected 1 command, got %d: %v", len(mock.Calls), mock.Calls)
			}
			cmd := mock.Calls[0]
			if !strings.Contains(cmd, "Library/LaunchAgents") || !strings.Contains(cmd, ".plist") {
				t.Errorf("expected darwin plist glob in cmd, got: %s", cmd)
			}
		})
	}
}

// TestListInstalledWindows exercises the Get-ScheduledTask path. RunShell
// wraps the script via h.wrapCommand; for a local host the script is passed
// through unchanged and the MockExecutor sees the raw PowerShell pipeline.
func TestListInstalledWindows(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name   string
		output string
		want   []string
	}{
		{
			name:   "happy path returns ghsr-runner task names",
			output: "ghsr-runner-ci-1\nghsr-runner-rune-x1-2\n",
			want:   []string{"ci-1", "rune-x1-2"},
		},
		{
			name:   "skips blank lines and unrelated task names",
			output: "\nOtherTask\nghsr-runner-ci-1\n",
			want:   []string{"ci-1"},
		},
		{
			name:   "empty output returns empty slice",
			output: "",
			want:   nil,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			mock := &testutil.MockExecutor{Output: tc.output}
			// Addr="local" disables the powershell.exe -EncodedCommand wrap so
			// the MockExecutor sees the raw script and we can assert on it.
			h := newMockHost("windows", config.HostConfig{OS: "windows", Addr: "local"}, mock)
			got, err := ListInstalled(h)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !equalInstances(got, tc.want) {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
			if len(mock.Calls) != 1 {
				t.Fatalf("expected 1 command, got %d: %v", len(mock.Calls), mock.Calls)
			}
			if !strings.Contains(mock.Calls[0], "Get-ScheduledTask") {
				t.Errorf("expected Get-ScheduledTask in cmd, got: %s", mock.Calls[0])
			}
		})
	}
}

// TestListInstalledDispatch pins the OS→impl dispatch in ListInstalled itself.
// Darwin and Windows paths are covered by their own subtests above; this one
// focuses on the dispatch table (unsupported OS + Run-error propagation per OS)
// so a future refactor that accidentally drops one of the branches (e.g.
// collapsing the switch into a single map lookup) is caught.
func TestListInstalledDispatch(t *testing.T) {
	t.Parallel()
	t.Run("unsupported OS returns error", func(t *testing.T) {
		t.Parallel()
		mock := &testutil.MockExecutor{Output: "should not be called"}
		h := newMockHost("plan9", config.HostConfig{OS: "plan9"}, mock)
		got, err := ListInstalled(h)
		if err == nil {
			t.Fatalf("expected error for unsupported OS, got %v", got)
		}
		if !strings.Contains(err.Error(), "plan9") {
			t.Errorf("error should mention OS %q, got: %v", "plan9", err)
		}
		if len(mock.Calls) != 0 {
			t.Errorf("expected 0 calls for unsupported OS, got: %v", mock.Calls)
		}
	})
	t.Run("run error propagates per OS", func(t *testing.T) {
		t.Parallel()
		for _, os := range []string{"linux", "darwin", "windows"} {
			os := os
			t.Run(os, func(t *testing.T) {
				t.Parallel()
				mock := &testutil.MockExecutor{RunErr: errCalled}
				// Windows path goes through RunShell→wrapCommand; local Addr
				// disables the powershell.exe -EncodedCommand wrap so the mock
				// sees the raw script and the dispatch-test stays generic.
				h := newMockHost("h-"+os, config.HostConfig{OS: os, Addr: "local"}, mock)
				got, err := ListInstalled(h)
				if err == nil {
					t.Fatalf("expected error for %s, got %v", os, got)
				}
				if len(mock.Calls) != 1 {
					t.Errorf("expected 1 command before error, got %d: %v", len(mock.Calls), mock.Calls)
				}
			})
		}
	})
}

// TestParseInstanceLines pins the helper used by listInstalledLinux so the
// SplitSeq refactor (PR #298) cannot silently change behavior (e.g. trimming
// the trailing empty line that SplitSeq yields for a "\n"-terminated input).
// Note: parseInstanceLines preserves input order and does NOT dedupe — dedupe
// is applied by the caller via dedupeInstances. The contract here is purely
// about line-splitting + prefix-stripping + blank/unrelated-line skipping.
func TestParseInstanceLines(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		in   string
		want []string
	}{
		{"empty", "", nil},
		{"single line no newline", "ghsr-runner-ci-1", []string{"ci-1"}},
		{"multi line preserves order and duplicates", "ghsr-runner-b\nghsr-runner-a\nghsr-runner-b\n", []string{"b", "a", "b"}},
		{"skips unrelated and blank", "\nfoo\n\nghsr-runner-ci-1\n\n", []string{"ci-1"}},
		{"trim whitespace per line", "  ghsr-runner-ci-1  \n", []string{"ci-1"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := parseInstanceLines(tc.in)
			if !equalInstances(got, tc.want) {
				t.Fatalf("parseInstanceLines(%q) = %v, want %v", tc.in, got, tc.want)
			}
		})
	}
}

// equalInstances compares two sorted-string slices element-wise so the
// existing dedupeInstances contract (sort + dedupe) is what we test against,
// not whatever incidental ordering the implementation happens to produce.
func equalInstances(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

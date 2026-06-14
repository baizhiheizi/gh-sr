package autostart

import (
	"strings"
	"testing"

	"github.com/an-lee/gh-sr/internal/config"
	"github.com/an-lee/gh-sr/internal/testutil"
)

func TestRunActiveCheck_DispatchesPerKind(t *testing.T) {
	t.Parallel()
	// Pin runActiveCheck's per-kind execution so a future drift between
	// IsServiceActive and Status (e.g. dropping the `|| echo inactive`
	// fallback on one site) surfaces immediately. Each case asserts the
	// helper routes through the right executor and returns the raw output.
	cases := []struct {
		name       string
		os         string
		kind       Kind
		mockOut    string
		wantOut    string
		wantCmdHas []string // substrings required in the executed command
	}{
		{
			name:       "systemd-user uses h.Run",
			os:         "linux",
			kind:       KindSystemdUser,
			mockOut:    "active",
			wantOut:    "active",
			wantCmdHas: []string{"systemctl --user is-active", "ghsr-runner-ci-1", "|| echo inactive"},
		},
		{
			name:       "systemd-system wraps with sudo prelude",
			os:         "linux",
			kind:       KindSystemdSystem,
			mockOut:    "active",
			wantOut:    "active",
			wantCmdHas: []string{"$SUDO systemctl is-active", "ghsr-runner-ci-1", "|| echo inactive"},
		},
		{
			name:       "launchd runs launchdPrintScript",
			os:         "darwin",
			kind:       KindLaunchd,
			mockOut:    "state = running\n",
			wantOut:    "state = running\n",
			wantCmdHas: []string{"launchctl print", "com.github.ghsr.runner.ci-1"},
		},
		{
			name:    "windows task uses h.RunShell with Get-ScheduledTask",
			os:      "windows",
			kind:    KindWindowsTask,
			mockOut: "Running",
			wantOut: "Running",
			// h.RunShell base64-encodes the script, so we cannot substring-match
			// the literal PowerShell text. Verify the wrapper fired instead.
			wantCmdHas: []string{"powershell.exe", "-EncodedCommand"},
		},
		{
			name:    "unknown kind returns empty",
			os:      "linux",
			kind:    KindNone,
			wantOut: "",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			mock := &testutil.MockExecutor{Output: tc.mockOut}
			h := newMockHost("test", config.HostConfig{OS: tc.os}, mock)
			out, err := runActiveCheck(h, tc.kind, "ci-1", "ghsr-runner-ci-1")
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if out != tc.wantOut {
				t.Errorf("out = %q, want %q", out, tc.wantOut)
			}
			if len(tc.wantCmdHas) == 0 {
				// KindNone — no command should be executed.
				if len(mock.Calls) != 0 {
					t.Errorf("KindNone should not execute, but got calls: %v", mock.Calls)
				}
				return
			}
			if len(mock.Calls) != 1 {
				t.Fatalf("expected 1 command, got %d: %v", len(mock.Calls), mock.Calls)
			}
			cmd := mock.Calls[0]
			for _, frag := range tc.wantCmdHas {
				if !strings.Contains(cmd, frag) {
					t.Errorf("command missing %q\ncmd: %s", frag, cmd)
				}
			}
		})
	}
}

func TestRunActiveCheck_NoCrossKindFragment(t *testing.T) {
	t.Parallel()
	// Catches accidental copy-paste between arms (e.g. a refactor that
	// inlines the launchd arm into KindSystemdUser and forgets to swap the
	// shell command). Each kind's command must not contain fragments from
	// the other arms. The mock is created inside the loop so each subtest
	// sees only its own call.
	cases := []struct {
		kind           Kind
		mustNotContain []string
	}{
		{
			kind:           KindSystemdUser,
			mustNotContain: []string{"$SUDO", "Get-ScheduledTask", "launchctl print"},
		},
		{
			kind:           KindSystemdSystem,
			mustNotContain: []string{"systemctl --user", "Get-ScheduledTask", "launchctl print"},
		},
		{
			kind:           KindLaunchd,
			mustNotContain: []string{"systemctl is-active", "Get-ScheduledTask"},
		},
		{
			kind:           KindWindowsTask,
			mustNotContain: []string{"systemctl is-active", "launchctl print"},
		},
	}
	for _, tc := range cases {
		t.Run(string(tc.kind), func(t *testing.T) {
			t.Parallel()
			mock := &testutil.MockExecutor{Output: "ok"}
			h := newMockHost("test", config.HostConfig{OS: "linux"}, mock)
			_, _ = runActiveCheck(h, tc.kind, "ci-1", "ghsr-runner-ci-1")
			if len(mock.Calls) != 1 {
				t.Fatalf("expected 1 command, got %d: %v", len(mock.Calls), mock.Calls)
			}
			cmd := mock.Calls[0]
			for _, frag := range tc.mustNotContain {
				if strings.Contains(cmd, frag) {
					t.Errorf("command contains forbidden %q (cross-kind leakage)\ncmd: %s", frag, cmd)
				}
			}
		})
	}
}

func TestRunActiveCheck_PropagatesError(t *testing.T) {
	// Note: this test is intentionally NOT t.Parallel() at the top level,
	// and subtests also run serially. Sharing a single MockExecutor across
	// concurrent subtests triggers a data race on m.Calls (mock.go appends
	// without a mutex). The table-driven form below pins the same contract
	// while keeping the mock state isolated per case.
	for _, k := range []Kind{KindSystemdUser, KindSystemdSystem, KindLaunchd} {
		t.Run(string(k), func(t *testing.T) {
			mock := &testutil.MockExecutor{RunErr: errCalled}
			h := newMockHost("test", config.HostConfig{OS: "linux"}, mock)
			_, err := runActiveCheck(h, k, "ci-1", "ghsr-runner-ci-1")
			if err == nil {
				t.Error("expected error to propagate")
			}
		})
	}
}

func TestKindLabel(t *testing.T) {
	t.Parallel()
	cases := map[Kind]string{
		KindSystemdUser:   "user",
		KindSystemdSystem: "system",
		KindLaunchd:       "launchd",
		KindWindowsTask:   "task",
	}
	for k, want := range cases {
		if got := kindLabel(k); got != want {
			t.Errorf("kindLabel(%q) = %q, want %q", k, got, want)
		}
	}
}

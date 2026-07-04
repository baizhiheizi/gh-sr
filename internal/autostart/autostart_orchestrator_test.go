package autostart

import (
	"strings"
	"testing"

	"github.com/an-lee/gh-sr/internal/config"
	"github.com/an-lee/gh-sr/internal/testutil"
)

// startKinds enumerates the (os, kind) combos Start has to dispatch through.
// One table per orchestrator keeps each test focused on what the function is
// responsible for: turning a detected Kind into the right SSH command.
var startKinds = []struct {
	os        string
	kind      Kind
	wantSub   string // substring expected in the SSH command
	wantShell bool   // true if expected to go through h.RunShell (PowerShell)
}{
	{"linux", KindSystemdUser, "systemctl --user start ghsr-runner-ci-1.service", false},
	{"linux", KindSystemdSystem, "systemctl start ghsr-runner-ci-1.service", false},
	{"darwin", KindLaunchd, "launchctl bootstrap", false},
	{"windows", KindWindowsTask, "Start-ScheduledTask -TaskName 'ghsr-ci-1'", true},
}

func TestStart(t *testing.T) {
	t.Parallel()

	t.Run("detect error propagates", func(t *testing.T) {
		t.Parallel()
		h := newMockHost("h1", config.HostConfig{OS: "linux"}, &testutil.MockExecutor{RunErr: errCalled})
		if err := Start(h, "ci-1"); err == nil {
			t.Error("expected error from Detect, got nil")
		}
	})

	t.Run("not installed returns descriptive error", func(t *testing.T) {
		t.Parallel()
		// Mock returns "" (Detect empty stdout) → KindNone → Start must refuse.
		h := newMockHost("h1", config.HostConfig{OS: "linux"}, &testutil.MockExecutor{Output: "\n"})
		err := Start(h, "ci-1")
		if err == nil {
			t.Fatal("expected error when autostart is not installed, got nil")
		}
		if !strings.Contains(err.Error(), "ci-1") {
			t.Errorf("error should mention the instance name, got %q", err.Error())
		}
	})

	t.Run("invalid instance name error propagates", func(t *testing.T) {
		t.Parallel()
		// SanitizeInstance rejects "@@@" (collapses to "" after sanitization).
		// Detect is mocked to succeed.
		h := newMockHost("h1", config.HostConfig{OS: "linux"}, &testutil.MockExecutor{Output: "user"})
		if err := Start(h, "@@@"); err == nil {
			t.Error("expected error from SanitizeInstance, got nil")
		}
	})

	for _, tc := range startKinds {
		name := string(tc.os) + " " + string(tc.kind)
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			mock := &testutil.MockExecutor{
				RunFn: func(cmd string) (string, error) {
					// Windows RunShell base64-encodes the script — the mock
					// cannot substring-match the literal PowerShell. Return
					// "yes" for every call so Detect resolves to
					// KindWindowsTask; the action command also returns "yes"
					// (we only care that the SSH call was made).
					if tc.os == "windows" {
						return "yes", nil
					}
					if strings.Contains(cmd, "systemctl") || strings.Contains(cmd, "LaunchAgents/") || strings.Contains(cmd, ".config/systemd/user/") {
						switch tc.kind {
						case KindSystemdUser:
							return "user", nil
						case KindSystemdSystem:
							return "system", nil
						case KindLaunchd:
							return "yes", nil
						}
					}
					if strings.Contains(cmd, "printf %s \"$HOME\"") {
						return "/Users/u\n", nil
					}
					return "", nil
				},
			}
			h := newMockHost("h1", config.HostConfig{OS: tc.os}, mock)

			if err := Start(h, "ci-1"); err != nil {
				t.Fatalf("Start: %v", err)
			}

			// Linux/darwin: substring-match the last SSH call (the action).
			// Windows: substring-match the PowerShell wrapper (the script
			// body is base64-encoded and unobservable from the mock).
			last := mock.Calls[len(mock.Calls)-1]
			if tc.wantShell {
				if !strings.Contains(last, "powershell.exe") || !strings.Contains(last, "-EncodedCommand") {
					t.Errorf("windows Start should go through RunShell wrapper, got %q", last)
				}
			} else {
				if !strings.Contains(last, tc.wantSub) {
					t.Errorf("last SSH call = %q, want substring %q", last, tc.wantSub)
				}
			}
		})
	}

	t.Run("kind-system ssh command contains sudo prelude", func(t *testing.T) {
		t.Parallel()
		mock := &testutil.MockExecutor{
			RunFn: func(cmd string) (string, error) {
				if strings.Contains(cmd, ".config/systemd/user/") && strings.Contains(cmd, "/etc/systemd/system/") {
					return "system", nil
				}
				return "", nil
			},
		}
		h := newMockHost("h1", config.HostConfig{OS: "linux"}, mock)
		if err := Start(h, "ci-1"); err != nil {
			t.Fatalf("Start: %v", err)
		}
		// Last call must contain both the elevation prelude and `systemctl start`.
		last := mock.Calls[len(mock.Calls)-1]
		if !strings.Contains(last, "systemctl start ghsr-runner-ci-1.service") {
			t.Errorf("system start missing systemctl: %q", last)
		}
		// The system install/start path runs under $SUDO. Confirm the prelude
		// snippet is present (the prelude is the hostshell.LinuxElevatePrelude
		// fragment — we don't pin its exact wording, only the $SUDO guard).
		if !strings.Contains(last, "$SUDO") {
			t.Errorf("system start missing $SUDO guard: %q", last)
		}
	})
}

func TestStop(t *testing.T) {
	t.Parallel()

	t.Run("detect error propagates", func(t *testing.T) {
		t.Parallel()
		h := newMockHost("h1", config.HostConfig{OS: "linux"}, &testutil.MockExecutor{RunErr: errCalled})
		if err := Stop(h, "ci-1"); err == nil {
			t.Error("expected error from Detect, got nil")
		}
	})

	t.Run("not installed returns descriptive error", func(t *testing.T) {
		t.Parallel()
		h := newMockHost("h1", config.HostConfig{OS: "linux"}, &testutil.MockExecutor{Output: "\n"})
		err := Stop(h, "ci-1")
		if err == nil {
			t.Fatal("expected error when autostart is not installed, got nil")
		}
		if !strings.Contains(err.Error(), "ci-1") {
			t.Errorf("error should mention the instance name, got %q", err.Error())
		}
	})

	for _, tc := range startKinds {
		name := string(tc.os) + " " + string(tc.kind)
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			mock := &testutil.MockExecutor{
				RunFn: func(cmd string) (string, error) {
					if tc.os == "windows" {
						return "yes", nil
					}
					if strings.Contains(cmd, "systemctl") || strings.Contains(cmd, "LaunchAgents/") || strings.Contains(cmd, ".config/systemd/user/") {
						switch tc.kind {
						case KindSystemdUser:
							return "user", nil
						case KindSystemdSystem:
							return "system", nil
						case KindLaunchd:
							return "yes", nil
						}
					}
					return "", nil
				},
			}
			h := newMockHost("h1", config.HostConfig{OS: tc.os}, mock)

			if err := Stop(h, "ci-1"); err != nil {
				t.Fatalf("Stop: %v", err)
			}

			last := mock.Calls[len(mock.Calls)-1]
			if tc.wantShell {
				if !strings.Contains(last, "powershell.exe") || !strings.Contains(last, "-EncodedCommand") {
					t.Errorf("windows Stop should go through RunShell wrapper, got %q", last)
				}
			} else {
				var wantSub string
				switch tc.kind {
				case KindSystemdUser:
					wantSub = "systemctl --user stop ghsr-runner-ci-1.service"
				case KindSystemdSystem:
					wantSub = "systemctl stop ghsr-runner-ci-1.service"
				case KindLaunchd:
					wantSub = "launchctl bootout"
				}
				if !strings.Contains(last, wantSub) {
					t.Errorf("last SSH call = %q, want substring %q", last, wantSub)
				}
			}
		})
	}
}

func TestStatus(t *testing.T) {
	t.Parallel()

	t.Run("docker mode returns docker detail without Detect", func(t *testing.T) {
		t.Parallel()
		mock := &testutil.MockExecutor{}
		h := newMockHost("h1", config.HostConfig{OS: "linux"}, mock)
		row, err := Status(h, "h1", "ci-1", "docker")
		if err != nil {
			t.Fatalf("Status: %v", err)
		}
		if row.Kind != KindNone {
			t.Errorf("Kind = %q, want %q (docker mode)", row.Kind, KindNone)
		}
		if !strings.Contains(row.Detail, "docker") {
			t.Errorf("Detail should mention docker, got %q", row.Detail)
		}
		// docker mode must NOT issue Detect probes (the Docker runner manages
		// its own lifecycle via --restart, not via the autostart unit).
		if len(mock.Calls) != 0 {
			t.Errorf("docker mode issued %d SSH call(s), want 0: %q", len(mock.Calls), mock.Calls)
		}
	})

	t.Run("detect error is reported in row detail", func(t *testing.T) {
		t.Parallel()
		// Detect errored → Status returns row + error (the caller shows the
		// error in the table); Kind is unset, Detail is empty.
		h := newMockHost("h1", config.HostConfig{OS: "linux"}, &testutil.MockExecutor{RunErr: errCalled})
		row, err := Status(h, "h1", "ci-1", "native")
		if err == nil {
			t.Fatal("expected error from Detect, got nil")
		}
		if row.Kind != KindNone {
			t.Errorf("Kind = %q, want %q (Detect failed)", row.Kind, KindNone)
		}
	})

	t.Run("not installed reports not-installed detail", func(t *testing.T) {
		t.Parallel()
		h := newMockHost("h1", config.HostConfig{OS: "linux"}, &testutil.MockExecutor{Output: "\n"})
		row, err := Status(h, "h1", "ci-1", "native")
		if err != nil {
			t.Fatalf("Status: %v", err)
		}
		if row.Kind != KindNone {
			t.Errorf("Kind = %q, want %q", row.Kind, KindNone)
		}
		if !strings.Contains(row.Detail, "not installed") {
			t.Errorf("Detail should say 'not installed', got %q", row.Detail)
		}
	})

	t.Run("active check error is captured in detail", func(t *testing.T) {
		t.Parallel()
		// Detect succeeds → kind=SystemdUser; the subsequent
		// `systemctl --user is-active` probe fails; Status should report
		// "installed (user): check failed: ..." without bubbling the error.
		mock := &testutil.MockExecutor{
			RunFn: func(cmd string) (string, error) {
				if strings.Contains(cmd, ".config/systemd/user/") && strings.Contains(cmd, "/etc/systemd/system/") {
					return "user", nil
				}
				return "", errCalled
			},
		}
		h := newMockHost("h1", config.HostConfig{OS: "linux"}, mock)
		row, err := Status(h, "h1", "ci-1", "native")
		if err != nil {
			t.Fatalf("Status: %v (should swallow active-check errors into Detail)", err)
		}
		if row.Kind != KindSystemdUser {
			t.Errorf("Kind = %q, want %q", row.Kind, KindSystemdUser)
		}
		if !strings.Contains(row.Detail, "check failed") {
			t.Errorf("Detail should report check failed, got %q", row.Detail)
		}
	})

	for _, tc := range []struct {
		name     string
		os       string
		kind     Kind
		probeOut string
		wantSub  string
	}{
		{"linux user active", "linux", KindSystemdUser, "active", "installed (user): active"},
		{"linux system active", "linux", KindSystemdSystem, "active", "installed (system): active"},
		{"darwin launchd active", "darwin", KindLaunchd, "state = running\n", "installed (launchd): state = running"},
		{"windows task active", "windows", KindWindowsTask, "Running", "installed (task): Running"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			// Declare mock first so the RunFn closure can reference mock.Calls
			// (forward references inside a struct literal field initializer
			// are not allowed in Go).
			mock := &testutil.MockExecutor{}
			mock.RunFn = func(cmd string) (string, error) {
				// Windows RunShell base64-encodes — return probeOut for
				// every call. Detect's `Get-ScheduledTask` probe AND
				// runActiveCheck's state probe both go through this path,
				// so both see probeOut; Detect interprets the first call
				// as "yes" (truthy string), the state probe as the value.
				if tc.os == "windows" {
					// First call (Detect) needs "yes" specifically so the
					// Windows branch in Detect matches "yes" → KindWindowsTask.
					// Subsequent calls return the probeOut. Detect calls
					// RunShell once → first response wins for both.
					if len(mock.Calls) == 1 {
						return "yes", nil
					}
					return tc.probeOut, nil
				}
				if tc.kind == KindLaunchd {
					if strings.Contains(cmd, "LaunchAgents/") {
						return "yes", nil
					}
					if strings.Contains(cmd, "launchctl print") {
						return tc.probeOut, nil
					}
					return "", nil
				}
				if strings.Contains(cmd, ".config/systemd/user/") && strings.Contains(cmd, "/etc/systemd/system/") {
					if tc.kind == KindSystemdSystem {
						return "system", nil
					}
					return "user", nil
				}
				if strings.Contains(cmd, "is-active") {
					return tc.probeOut, nil
				}
				return "", nil
			}
			h := newMockHost("h1", config.HostConfig{OS: tc.os}, mock)
			row, err := Status(h, "h1", "ci-1", "native")
			if err != nil {
				t.Fatalf("Status: %v", err)
			}
			if row.Kind != tc.kind {
				t.Errorf("Kind = %q, want %q", row.Kind, tc.kind)
			}
			if !strings.Contains(row.Detail, tc.wantSub) {
				t.Errorf("Detail = %q, want substring %q", row.Detail, tc.wantSub)
			}
			// row fields populated from args
			if row.Instance != "ci-1" {
				t.Errorf("Instance = %q, want %q", row.Instance, "ci-1")
			}
			if row.Host != "h1" {
				t.Errorf("Host = %q, want %q", row.Host, "h1")
			}
			if row.Mode != "native" {
				t.Errorf("Mode = %q, want %q", row.Mode, "native")
			}
		})
	}

	t.Run("launchd output is capped at 5 lines", func(t *testing.T) {
		t.Parallel()
		sixLines := "state = running\nfoo\nbar\nbaz\nqux\nquux\n"
		mock := &testutil.MockExecutor{
			RunFn: func(cmd string) (string, error) {
				if strings.Contains(cmd, "LaunchAgents/") {
					return "yes", nil
				}
				if strings.Contains(cmd, "launchctl print") {
					return sixLines, nil
				}
				return "", nil
			},
		}
		h := newMockHost("h1", config.HostConfig{OS: "darwin"}, mock)
		row, err := Status(h, "h1", "ci-1", "native")
		if err != nil {
			t.Fatalf("Status: %v", err)
		}
		// 6 input lines → head -n 5 → 5 lines → flattened to spaces.
		// The 6th line ("quux") must NOT appear in Detail.
		if strings.Contains(row.Detail, "quux") {
			t.Errorf("Detail should not contain 6th line 'quux', got %q", row.Detail)
		}
		if !strings.Contains(row.Detail, "state = running") {
			t.Errorf("Detail should contain first line 'state = running', got %q", row.Detail)
		}
	})
}

// TestUninstall_kinds exercises the per-Kind dispatch on Uninstall, which is
// covered for KindSystemdUser (tested by other suites) but left at 31.6%
// overall. Each case pins the SSH call shape so a refactor that drops a kind
// or rewrites a script fails the assertion.
func TestUninstall_kinds(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		os      string
		mock    *testutil.MockExecutor
		wantSub string
	}{
		{
			name: "systemd-system disables system unit",
			os:   "linux",
			mock: &testutil.MockExecutor{
				RunFn: func(cmd string) (string, error) {
					if strings.Contains(cmd, ".config/systemd/user/") && strings.Contains(cmd, "/etc/systemd/system/") {
						return "system", nil
					}
					return "", nil
				},
			},
			wantSub: "systemctl disable --now ghsr-runner-ci-1.service",
		},
		{
			name: "launchd bootouts label",
			os:   "darwin",
			mock: &testutil.MockExecutor{
				RunFn: func(cmd string) (string, error) {
					if strings.Contains(cmd, "LaunchAgents/") {
						return "yes", nil
					}
					return "", nil
				},
			},
			wantSub: "launchctl bootout",
		},
		{
			name: "windows task unregisters",
			os:   "windows",
			mock: &testutil.MockExecutor{
				RunFn: func(cmd string) (string, error) {
					// Windows RunShell base64-encodes the script — return
					// "yes" for the Detect probe (RunShell-wrapped) so it
					// resolves to KindWindowsTask. The Uninstall action then
					// also returns "yes".
					return "yes", nil
				},
			},
			// Windows wraps in powershell.exe — verify the wrapper fired.
			wantSub: "powershell.exe",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			h := newMockHost("h1", config.HostConfig{OS: tc.os}, tc.mock)
			if err := Uninstall(h, "ci-1"); err != nil {
				t.Fatalf("Uninstall: %v", err)
			}
			found := false
			for _, c := range tc.mock.Calls {
				if strings.Contains(c, tc.wantSub) {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("expected call containing %q, got calls: %q", tc.wantSub, tc.mock.Calls)
			}
		})
	}
}

// installKinds enumerates the (os, kind) combos Install has to dispatch through.
// Mirrors startKinds but the (system) opt toggle splits Linux into two arms.
var installKinds = []struct {
	name       string
	os         string
	system     bool
	wantShell  bool   // true if expected to go through h.RunShell (PowerShell wrapper)
	wantAction string // substring expected in the action SSH call
	wantWrite  string // substring expected in the unit-file-write SSH call (base64)
}{
	{"linux user", "linux", false, false, "systemctl --user enable", ".config/systemd/user/ghsr-runner-ci-1.service"},
	{"linux system", "linux", true, false, "systemctl enable", "/etc/systemd/system/ghsr-runner-ci-1.service"},
	// launchd labels look like "com.github.ghsr.runner.<instance>" (see LaunchdLabel).
	{"darwin launchd", "darwin", false, false, "launchctl bootstrap", "Library/LaunchAgents/com.github.ghsr.runner.ci-1.plist"},
	// Windows uses PowerShell for both the unit write and the registration;
	// the script body is base64-encoded (only the wrapper substring survives).
	{"windows task", "windows", false, true, "Register-ScheduledTask", "powershell.exe"},
}

func TestInstall(t *testing.T) {
	t.Parallel()

	t.Run("invalid instance name error propagates", func(t *testing.T) {
		t.Parallel()
		mock := &testutil.MockExecutor{Output: "user\n"}
		h := newMockHost("h1", config.HostConfig{OS: "linux"}, mock)
		if err := Install(h, "@@@", InstallOpts{}); err == nil {
			t.Error("expected error from SanitizeInstance, got nil")
		}
	})

	t.Run("unsupported OS returns error", func(t *testing.T) {
		t.Parallel()
		h := newMockHost("h1", config.HostConfig{OS: "freebsd"}, &testutil.MockExecutor{})
		err := Install(h, "ci-1", InstallOpts{})
		if err == nil {
			t.Fatal("expected error for unsupported OS, got nil")
		}
		if !strings.Contains(err.Error(), "freebsd") {
			t.Errorf("error should mention OS, got %q", err.Error())
		}
	})

	t.Run("remote home error propagates", func(t *testing.T) {
		t.Parallel()
		h := newMockHost("h1", config.HostConfig{OS: "linux"}, &testutil.MockExecutor{RunErr: errCalled})
		if err := Install(h, "ci-1", InstallOpts{}); err == nil {
			t.Error("expected error from remoteHome, got nil")
		}
	})

	for _, tc := range installKinds {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			mock := &testutil.MockExecutor{
				RunFn: func(cmd string) (string, error) {
					if tc.os == "windows" {
						// Windows RunShell wraps the script in powershell.exe
						// -EncodedCommand (host.NewHost defaults to non-local
						// Addr, so wrapCommand fires). We only need to drive
						// remoteHome (USERPROFILE) and the Register call.
						if strings.Contains(cmd, "powershell.exe") {
							return `C:\Users\test`, nil
						}
						return "", nil
					}
					if strings.Contains(cmd, `printf %s "$HOME"`) {
						return "/home/test\n", nil
					}
					if strings.Contains(cmd, "id -un") && strings.Contains(cmd, "id -gn") {
						return "test\ntest\n", nil
					}
					if strings.Contains(cmd, "systemctl") || strings.Contains(cmd, "launchctl") {
						return "", nil
					}
					return "", nil
				},
			}
			h := newMockHost("h1", config.HostConfig{OS: tc.os}, mock)

			if err := Install(h, "ci-1", InstallOpts{System: tc.system}); err != nil {
				t.Fatalf("Install: %v", err)
			}

			// Linux/darwin: substring-match the action SSH call.
			// Windows: the RunShell wrapper fired (the action script body
			// is base64-encoded and unobservable from the mock).
			if tc.wantShell {
				last := mock.Calls[len(mock.Calls)-1]
				if !strings.Contains(last, "powershell.exe") || !strings.Contains(last, "-EncodedCommand") {
					t.Errorf("windows Install should go through RunShell wrapper, got %q", last)
				}
			} else {
				last := mock.Calls[len(mock.Calls)-1]
				if !strings.Contains(last, tc.wantAction) {
					t.Errorf("last SSH call = %q, want substring %q", last, tc.wantAction)
				}
			}
			// All four arms write the unit file via hostshell.WriteRemoteBytes,
			// which streams the base64 payload over h.Run (POSIX) or
			// h.RunShell (Windows). Confirm the write path was exercised.
			if tc.wantWrite != "" {
				found := false
				for _, c := range mock.Calls {
					if strings.Contains(c, tc.wantWrite) {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("expected unit-file write call containing %q for %s, got calls: %q", tc.wantWrite, tc.name, mock.Calls)
				}
			}
		})
	}

	t.Run("linux user upload failure propagates", func(t *testing.T) {
		t.Parallel()
		// WriteRemoteBytes fails by returning an error from the
		// base64-decode SSH call.
		mock := &testutil.MockExecutor{
			RunFn: func(cmd string) (string, error) {
				if strings.Contains(cmd, `printf %s "$HOME"`) {
					return "/home/test\n", nil
				}
				if strings.Contains(cmd, "base64 -d") {
					return "", errCalled
				}
				return "", nil
			},
		}
		h := newMockHost("h1", config.HostConfig{OS: "linux"}, mock)
		err := Install(h, "ci-1", InstallOpts{})
		if err == nil {
			t.Fatal("expected error from unit write, got nil")
		}
		if !strings.Contains(err.Error(), "writing systemd user unit") {
			t.Errorf("error should mention 'writing systemd user unit', got %q", err.Error())
		}
	})

	t.Run("linux user systemctl enable failure propagates", func(t *testing.T) {
		t.Parallel()
		mock := &testutil.MockExecutor{
			RunFn: func(cmd string) (string, error) {
				if strings.Contains(cmd, `printf %s "$HOME"`) {
					return "/home/test\n", nil
				}
				if strings.Contains(cmd, "systemctl") {
					return "", errCalled
				}
				return "", nil
			},
		}
		h := newMockHost("h1", config.HostConfig{OS: "linux"}, mock)
		err := Install(h, "ci-1", InstallOpts{})
		if err == nil {
			t.Fatal("expected error from systemctl, got nil")
		}
		if !strings.Contains(err.Error(), "enabling systemd user unit") {
			t.Errorf("error should mention 'enabling systemd user unit', got %q", err.Error())
		}
	})

	t.Run("linux system id probe failure propagates", func(t *testing.T) {
		t.Parallel()
		mock := &testutil.MockExecutor{
			RunFn: func(cmd string) (string, error) {
				if strings.Contains(cmd, `printf %s "$HOME"`) {
					return "/home/test\n", nil
				}
				if strings.Contains(cmd, "id -un") {
					return "", errCalled
				}
				return "", nil
			},
		}
		h := newMockHost("h1", config.HostConfig{OS: "linux"}, mock)
		err := Install(h, "ci-1", InstallOpts{System: true})
		if err == nil {
			t.Fatal("expected error from id probe, got nil")
		}
		if !strings.Contains(err.Error(), "id -un") {
			t.Errorf("error should mention 'id -un', got %q", err.Error())
		}
	})

	t.Run("linux system id probe malformed output returns error", func(t *testing.T) {
		t.Parallel()
		mock := &testutil.MockExecutor{
			RunFn: func(cmd string) (string, error) {
				if strings.Contains(cmd, `printf %s "$HOME"`) {
					return "/home/test\n", nil
				}
				if strings.Contains(cmd, "id -un") && strings.Contains(cmd, "id -gn") {
					// Only one line — installSystemdSystem expects 2.
					return "test\n", nil
				}
				return "", nil
			},
		}
		h := newMockHost("h1", config.HostConfig{OS: "linux"}, mock)
		err := Install(h, "ci-1", InstallOpts{System: true})
		if err == nil {
			t.Fatal("expected error from malformed id output, got nil")
		}
		if !strings.Contains(err.Error(), "expected 2 lines") {
			t.Errorf("error should mention 'expected 2 lines', got %q", err.Error())
		}
	})

	t.Run("linux system upload failure propagates", func(t *testing.T) {
		t.Parallel()
		mock := &testutil.MockExecutor{
			RunFn: func(cmd string) (string, error) {
				if strings.Contains(cmd, `printf %s "$HOME"`) {
					return "/home/test\n", nil
				}
				if strings.Contains(cmd, "id -un") && strings.Contains(cmd, "id -gn") {
					return "test\ntest\n", nil
				}
				if strings.Contains(cmd, "base64 -d") {
					return "", errCalled
				}
				return "", nil
			},
		}
		h := newMockHost("h1", config.HostConfig{OS: "linux"}, mock)
		err := Install(h, "ci-1", InstallOpts{System: true})
		if err == nil {
			t.Fatal("expected error from unit write, got nil")
		}
		if !strings.Contains(err.Error(), "staging systemd system unit") {
			t.Errorf("error should mention 'staging systemd system unit', got %q", err.Error())
		}
	})

	t.Run("linux system systemctl enable failure propagates", func(t *testing.T) {
		t.Parallel()
		mock := &testutil.MockExecutor{
			RunFn: func(cmd string) (string, error) {
				if strings.Contains(cmd, `printf %s "$HOME"`) {
					return "/home/test\n", nil
				}
				if strings.Contains(cmd, "id -un") && strings.Contains(cmd, "id -gn") {
					return "test\ntest\n", nil
				}
				if strings.Contains(cmd, "systemctl") {
					return "", errCalled
				}
				return "", nil
			},
		}
		h := newMockHost("h1", config.HostConfig{OS: "linux"}, mock)
		err := Install(h, "ci-1", InstallOpts{System: true})
		if err == nil {
			t.Fatal("expected error from systemctl, got nil")
		}
		if !strings.Contains(err.Error(), "installing system systemd unit") {
			t.Errorf("error should mention 'installing system systemd unit', got %q", err.Error())
		}
	})

	t.Run("darwin upload failure propagates", func(t *testing.T) {
		t.Parallel()
		mock := &testutil.MockExecutor{
			RunFn: func(cmd string) (string, error) {
				if strings.Contains(cmd, `printf %s "$HOME"`) {
					return "/Users/test\n", nil
				}
				if strings.Contains(cmd, "base64 -d") {
					return "", errCalled
				}
				return "", nil
			},
		}
		h := newMockHost("h1", config.HostConfig{OS: "darwin"}, mock)
		err := Install(h, "ci-1", InstallOpts{})
		if err == nil {
			t.Fatal("expected error from unit write, got nil")
		}
		if !strings.Contains(err.Error(), "writing LaunchAgent plist") {
			t.Errorf("error should mention 'writing LaunchAgent plist', got %q", err.Error())
		}
	})

	t.Run("darwin launchctl failure propagates", func(t *testing.T) {
		t.Parallel()
		mock := &testutil.MockExecutor{
			RunFn: func(cmd string) (string, error) {
				if strings.Contains(cmd, `printf %s "$HOME"`) {
					return "/Users/test\n", nil
				}
				if strings.Contains(cmd, "launchctl") {
					return "", errCalled
				}
				return "", nil
			},
		}
		h := newMockHost("h1", config.HostConfig{OS: "darwin"}, mock)
		err := Install(h, "ci-1", InstallOpts{})
		if err == nil {
			t.Fatal("expected error from launchctl, got nil")
		}
		if !strings.Contains(err.Error(), "loading LaunchAgent") {
			t.Errorf("error should mention 'loading LaunchAgent', got %q", err.Error())
		}
	})

	t.Run("windows task registration failure propagates", func(t *testing.T) {
		t.Parallel()
		// remoteHome on Windows uses RunShell (wrapped in powershell.exe
		// -EncodedCommand). The Register-ScheduledTask call also goes
		// through RunShell. Fail on the second powershell call to
		// exercise the "registering scheduled task" error branch.
		mock := &testutil.MockExecutor{}
		mock.RunFn = func(cmd string) (string, error) {
			if !strings.Contains(cmd, "powershell.exe") {
				return "", nil
			}
			// len(mock.Calls) is the index *after* append, so this fires
			// on the second wrapped call (Register-ScheduledTask).
			if len(mock.Calls) >= 2 {
				return "", errCalled
			}
			return `C:\Users\test`, nil
		}
		h := newMockHost("h1", config.HostConfig{OS: "windows"}, mock)
		err := Install(h, "ci-1", InstallOpts{})
		if err == nil {
			t.Fatal("expected error from Register-ScheduledTask, got nil")
		}
		if !strings.Contains(err.Error(), "registering scheduled task") {
			t.Errorf("error should mention 'registering scheduled task', got %q", err.Error())
		}
	})
}

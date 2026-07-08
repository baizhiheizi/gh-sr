package diskschedule

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/an-lee/gh-sr/internal/hostshell"
)

func resetDiskScheduleSeams(t *testing.T) {
	t.Helper()
	oldGOOS := runtimeGOOS
	oldLookPath := execLookPath
	oldCombinedOutput := execCombinedOutput
	oldRun := execRun
	oldRunInDir := execRunInDir
	oldPSExec := powerShellExec
	oldPSCombinedOutput := powerShellCombinedOutput
	t.Cleanup(func() {
		runtimeGOOS = oldGOOS
		execLookPath = oldLookPath
		execCombinedOutput = oldCombinedOutput
		execRun = oldRun
		execRunInDir = oldRunInDir
		powerShellExec = oldPSExec
		powerShellCombinedOutput = oldPSCombinedOutput
	})
}

type commandCall struct {
	Dir  string
	Name string
	Args []string
}

func TestDetect(t *testing.T) {
	cases := []struct {
		name    string
		goos    string
		setup   func(t *testing.T, home string)
		psOut   []byte
		psErr   error
		want    ScheduleKind
		wantErr bool
	}{
		{
			name: "linux timer exists",
			goos: "linux",
			setup: func(t *testing.T, home string) {
				t.Helper()
				timerPath := filepath.Join(home, ".config", "systemd", "user", serviceBase+".timer")
				if err := os.MkdirAll(filepath.Dir(timerPath), 0o700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(timerPath, []byte("timer"), 0o600); err != nil {
					t.Fatal(err)
				}
			},
			want: KindSystemdUser,
		},
		{
			name: "linux missing timer",
			goos: "linux",
			want: KindNone,
		},
		{
			name: "darwin plist exists",
			goos: "darwin",
			setup: func(t *testing.T, home string) {
				t.Helper()
				plistPath := filepath.Join(home, "Library", "LaunchAgents", labelBase+".plist")
				if err := os.MkdirAll(filepath.Dir(plistPath), 0o700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(plistPath, []byte("plist"), 0o600); err != nil {
					t.Fatal(err)
				}
			},
			want: KindLaunchd,
		},
		{
			name: "darwin missing plist",
			goos: "darwin",
			want: KindNone,
		},
		{
			name:  "windows task exists",
			goos:  "windows",
			psOut: []byte("yes\r\n"),
			want:  KindWindowsTask,
		},
		{
			name:  "windows task missing",
			goos:  "windows",
			psOut: []byte("no\r\n"),
			want:  KindNone,
		},
		{
			name:    "windows powershell error",
			goos:    "windows",
			psErr:   errors.New("powershell unavailable"),
			want:    KindNone,
			wantErr: true,
		},
		{
			name:    "unsupported goos",
			goos:    "plan9",
			want:    KindNone,
			wantErr: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resetDiskScheduleSeams(t)
			runtimeGOOS = tc.goos
			home := t.TempDir()
			t.Setenv("HOME", home)
			if tc.setup != nil {
				tc.setup(t, home)
			}
			powerShellExec = func(script string) ([]byte, error) {
				if !strings.Contains(script, "Get-ScheduledTask") || !strings.Contains(script, serviceBase) {
					t.Fatalf("PowerShell script %q does not inspect %s", script, serviceBase)
				}
				return tc.psOut, tc.psErr
			}

			got, err := Detect()
			if tc.wantErr {
				if err == nil {
					t.Fatalf("Detect() = (%q, nil); want error", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("Detect(): unexpected error %v", err)
			}
			if got != tc.want {
				t.Fatalf("Detect() = %q; want %q", got, tc.want)
			}
		})
	}
}

func TestInstall(t *testing.T) {
	t.Run("validates required config before path lookup", func(t *testing.T) {
		resetDiskScheduleSeams(t)
		execLookPath = func(string) (string, error) {
			t.Fatal("execLookPath called before ConfigPath validation")
			return "", nil
		}
		if err := Install(InstallOpts{}); err == nil || !strings.Contains(err.Error(), "config path is required") {
			t.Fatalf("Install() error = %v; want config path error", err)
		}
	})

	t.Run("looks up gh path when omitted", func(t *testing.T) {
		resetDiskScheduleSeams(t)
		runtimeGOOS = "linux"
		t.Setenv("HOME", t.TempDir())
		execLookPath = func(name string) (string, error) {
			if name != "gh" {
				t.Fatalf("execLookPath(%q); want gh", name)
			}
			return "/usr/local/bin/gh", nil
		}
		var calls []commandCall
		execCombinedOutput = func(name string, args ...string) ([]byte, error) {
			calls = append(calls, commandCall{Name: name, Args: append([]string(nil), args...)})
			return nil, nil
		}

		if err := Install(InstallOpts{ConfigPath: "/tmp/config.yml", AtTime: "04:05"}); err != nil {
			t.Fatalf("Install(): unexpected error %v", err)
		}
		servicePath := filepath.Join(os.Getenv("HOME"), ".config", "systemd", "user", serviceBase+".service")
		service, err := os.ReadFile(servicePath)
		if err != nil {
			t.Fatalf("read service: %v", err)
		}
		if !strings.Contains(string(service), "ExecStart=/usr/local/bin/gh sr disk prune --yes -c /tmp/config.yml") {
			t.Fatalf("service ExecStart missing looked-up gh path:\n%s", service)
		}
		want := []commandCall{
			{Name: "systemctl", Args: []string{"--user", "daemon-reload"}},
			{Name: "systemctl", Args: []string{"--user", "enable", serviceBase + ".timer"}},
			{Name: "systemctl", Args: []string{"--user", "start", serviceBase + ".timer"}},
		}
		if !reflect.DeepEqual(calls, want) {
			t.Fatalf("systemctl calls = %#v; want %#v", calls, want)
		}
	})

	t.Run("reports gh lookup failure", func(t *testing.T) {
		resetDiskScheduleSeams(t)
		execLookPath = func(string) (string, error) {
			return "", errors.New("not found")
		}
		if err := Install(InstallOpts{ConfigPath: "/tmp/config.yml"}); err == nil || !strings.Contains(err.Error(), "gh not found on PATH") {
			t.Fatalf("Install() error = %v; want gh lookup error", err)
		}
	})

	t.Run("rejects invalid time before platform work", func(t *testing.T) {
		resetDiskScheduleSeams(t)
		execCombinedOutput = func(string, ...string) ([]byte, error) {
			t.Fatal("execCombinedOutput called after invalid AtTime")
			return nil, nil
		}
		if err := Install(InstallOpts{ConfigPath: "/tmp/config.yml", GhPath: "/bin/gh", AtTime: "25:00"}); err == nil || !strings.Contains(err.Error(), "invalid hour") {
			t.Fatalf("Install() error = %v; want invalid hour", err)
		}
	})

	t.Run("linux writes timer and propagates command output", func(t *testing.T) {
		resetDiskScheduleSeams(t)
		runtimeGOOS = "linux"
		home := t.TempDir()
		t.Setenv("HOME", home)
		var calls []commandCall
		execCombinedOutput = func(name string, args ...string) ([]byte, error) {
			calls = append(calls, commandCall{Name: name, Args: append([]string(nil), args...)})
			if len(calls) == 2 {
				return []byte("timer already exists"), errors.New("enable failed")
			}
			return nil, nil
		}

		err := Install(InstallOpts{ConfigPath: "/path/with space/config.yml", GhPath: "/opt/gh cli/gh", AtTime: "06:07"})
		if err == nil || !strings.Contains(err.Error(), "timer already exists") {
			t.Fatalf("Install() error = %v; want systemctl output", err)
		}
		if len(calls) != 2 {
			t.Fatalf("systemctl calls = %d; want 2 before failure", len(calls))
		}
		timerPath := filepath.Join(home, ".config", "systemd", "user", serviceBase+".timer")
		timer, readErr := os.ReadFile(timerPath)
		if readErr != nil {
			t.Fatalf("read timer: %v", readErr)
		}
		if !strings.Contains(string(timer), "OnCalendar=*-*-* 06:07:00") {
			t.Fatalf("timer missing schedule:\n%s", timer)
		}
		servicePath := filepath.Join(home, ".config", "systemd", "user", serviceBase+".service")
		service, readErr := os.ReadFile(servicePath)
		if readErr != nil {
			t.Fatalf("read service: %v", readErr)
		}
		if !strings.Contains(string(service), `ExecStart="/opt/gh cli/gh" sr disk prune --yes -c "/path/with space/config.yml"`) {
			t.Fatalf("service missing quoted ExecStart:\n%s", service)
		}
	})

	t.Run("darwin writes plist and bootstraps launchd", func(t *testing.T) {
		resetDiskScheduleSeams(t)
		runtimeGOOS = "darwin"
		home := t.TempDir()
		t.Setenv("HOME", home)
		var calls []commandCall
		execCombinedOutput = func(name string, args ...string) ([]byte, error) {
			calls = append(calls, commandCall{Name: name, Args: append([]string(nil), args...)})
			return nil, nil
		}

		if err := Install(InstallOpts{ConfigPath: "/Users/me/a&b.yml", GhPath: "/opt/gh", AtTime: "08:09"}); err != nil {
			t.Fatalf("Install(): unexpected error %v", err)
		}
		plistPath := filepath.Join(home, "Library", "LaunchAgents", labelBase+".plist")
		plist, err := os.ReadFile(plistPath)
		if err != nil {
			t.Fatalf("read plist: %v", err)
		}
		for _, want := range []string{"<key>Hour</key><integer>8</integer>", "<key>Minute</key><integer>9</integer>", "/Users/me/a&amp;b.yml"} {
			if !strings.Contains(string(plist), want) {
				t.Fatalf("plist missing %q:\n%s", want, plist)
			}
		}
		if len(calls) != 1 || calls[0].Name != "launchctl" || !reflect.DeepEqual(calls[0].Args[:2], []string{"bootstrap", fmt.Sprintf("gui/%d", os.Getuid())}) || calls[0].Args[2] != plistPath {
			t.Fatalf("launchctl calls = %#v", calls)
		}
	})

	t.Run("darwin reports launchctl output", func(t *testing.T) {
		resetDiskScheduleSeams(t)
		runtimeGOOS = "darwin"
		t.Setenv("HOME", t.TempDir())
		execCombinedOutput = func(string, ...string) ([]byte, error) {
			return []byte("bootstrap denied"), errors.New("failed")
		}

		err := Install(InstallOpts{ConfigPath: "/tmp/config.yml", GhPath: "/bin/gh", AtTime: "08:09"})
		if err == nil || !strings.Contains(err.Error(), "bootstrap denied") {
			t.Fatalf("Install() error = %v; want launchctl output", err)
		}
	})

	t.Run("windows registers scheduled task", func(t *testing.T) {
		resetDiskScheduleSeams(t)
		runtimeGOOS = "windows"
		var script string
		powerShellCombinedOutput = func(ps string) ([]byte, error) {
			script = ps
			return nil, nil
		}

		if err := Install(InstallOpts{ConfigPath: `C:\Users\me\cfg.yml`, GhPath: `C:\Program Files\GitHub CLI\gh.exe`, AtTime: "10:11"}); err != nil {
			t.Fatalf("Install(): unexpected error %v", err)
		}
		for _, want := range []string{"Register-ScheduledTask", "New-ScheduledTaskTrigger -Daily -At (Get-Date '10:11').TimeOfDay", `'C:\Program Files\GitHub CLI\gh.exe'`, `'C:\Users\me\cfg.yml'`} {
			if !strings.Contains(script, want) {
				t.Fatalf("PowerShell script missing %q:\n%s", want, script)
			}
		}
	})

	t.Run("windows reports register output", func(t *testing.T) {
		resetDiskScheduleSeams(t)
		runtimeGOOS = "windows"
		powerShellCombinedOutput = func(string) ([]byte, error) {
			return []byte("access denied"), errors.New("failed")
		}

		err := Install(InstallOpts{ConfigPath: `C:\cfg.yml`, GhPath: `C:\gh.exe`})
		if err == nil || !strings.Contains(err.Error(), "access denied") {
			t.Fatalf("Install() error = %v; want register output", err)
		}
	})

	t.Run("unsupported goos", func(t *testing.T) {
		resetDiskScheduleSeams(t)
		runtimeGOOS = "plan9"
		if err := Install(InstallOpts{ConfigPath: "/tmp/config.yml", GhPath: "/bin/gh"}); err == nil || !strings.Contains(err.Error(), "not supported") {
			t.Fatalf("Install() error = %v; want unsupported goos", err)
		}
	})
}

func TestUninstall(t *testing.T) {
	t.Run("linux removes files and ignores command failures", func(t *testing.T) {
		resetDiskScheduleSeams(t)
		runtimeGOOS = "linux"
		home := t.TempDir()
		t.Setenv("HOME", home)
		dir := filepath.Join(home, ".config", "systemd", "user")
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatal(err)
		}
		for _, name := range []string{serviceBase + ".timer", serviceBase + ".service"} {
			if err := os.WriteFile(filepath.Join(dir, name), []byte(name), 0o600); err != nil {
				t.Fatal(err)
			}
		}
		var calls []commandCall
		execRun = func(name string, args ...string) error {
			calls = append(calls, commandCall{Name: name, Args: append([]string(nil), args...)})
			return errors.New("ignored")
		}

		if err := Uninstall(); err != nil {
			t.Fatalf("Uninstall(): unexpected error %v", err)
		}
		for _, name := range []string{serviceBase + ".timer", serviceBase + ".service"} {
			if _, err := os.Stat(filepath.Join(dir, name)); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("%s still exists or stat failed with %v", name, err)
			}
		}
		want := []commandCall{
			{Name: "systemctl", Args: []string{"--user", "disable", "--now", serviceBase + ".timer"}},
			{Name: "systemctl", Args: []string{"--user", "daemon-reload"}},
		}
		if !reflect.DeepEqual(calls, want) {
			t.Fatalf("execRun calls = %#v; want %#v", calls, want)
		}
	})

	t.Run("darwin runs launchd bootout from home", func(t *testing.T) {
		resetDiskScheduleSeams(t)
		runtimeGOOS = "darwin"
		home := t.TempDir()
		t.Setenv("HOME", home)
		var call commandCall
		execRunInDir = func(dir, name string, args ...string) error {
			call = commandCall{Dir: dir, Name: name, Args: append([]string(nil), args...)}
			return errors.New("ignored")
		}

		if err := Uninstall(); err != nil {
			t.Fatalf("Uninstall(): unexpected error %v", err)
		}
		if call.Dir != home || call.Name != "sh" || len(call.Args) != 2 || call.Args[0] != "-c" || !strings.Contains(call.Args[1], labelBase) || !strings.Contains(call.Args[1], labelBase+".plist") {
			t.Fatalf("execRunInDir call = %#v", call)
		}
	})

	t.Run("windows unregisters task", func(t *testing.T) {
		resetDiskScheduleSeams(t)
		runtimeGOOS = "windows"
		var script string
		powerShellCombinedOutput = func(ps string) ([]byte, error) {
			script = ps
			return nil, errors.New("propagated")
		}

		err := Uninstall()
		if err == nil || !strings.Contains(err.Error(), "propagated") {
			t.Fatalf("Uninstall() error = %v; want propagated PowerShell error", err)
		}
		if !strings.Contains(script, "Unregister-ScheduledTask") || !strings.Contains(script, serviceBase) {
			t.Fatalf("PowerShell script = %q", script)
		}
	})

	t.Run("unsupported goos", func(t *testing.T) {
		resetDiskScheduleSeams(t)
		runtimeGOOS = "plan9"
		if err := Uninstall(); err == nil || !strings.Contains(err.Error(), "not supported") {
			t.Fatalf("Uninstall() error = %v; want unsupported goos", err)
		}
	})
}

func TestStatus(t *testing.T) {
	t.Run("detect error propagates", func(t *testing.T) {
		resetDiskScheduleSeams(t)
		runtimeGOOS = "windows"
		powerShellExec = func(string) ([]byte, error) {
			return nil, errors.New("detect failed")
		}

		kind, detail, err := Status()
		if err == nil || kind != KindNone || detail != "" {
			t.Fatalf("Status() = (%q, %q, %v); want detect error", kind, detail, err)
		}
	})

	t.Run("not installed", func(t *testing.T) {
		resetDiskScheduleSeams(t)
		runtimeGOOS = "linux"
		t.Setenv("HOME", t.TempDir())

		kind, detail, err := Status()
		if err != nil || kind != KindNone || detail != "not installed" {
			t.Fatalf("Status() = (%q, %q, %v); want not installed", kind, detail, err)
		}
	})

	t.Run("systemd enabled detail", func(t *testing.T) {
		resetDiskScheduleSeams(t)
		runtimeGOOS = "linux"
		home := t.TempDir()
		t.Setenv("HOME", home)
		timerPath := filepath.Join(home, ".config", "systemd", "user", serviceBase+".timer")
		if err := os.MkdirAll(filepath.Dir(timerPath), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(timerPath, []byte("timer"), 0o600); err != nil {
			t.Fatal(err)
		}
		execCombinedOutput = func(name string, args ...string) ([]byte, error) {
			if name != "systemctl" || !reflect.DeepEqual(args, []string{"--user", "is-enabled", serviceBase + ".timer"}) {
				t.Fatalf("execCombinedOutput(%q, %#v)", name, args)
			}
			return []byte("enabled\n"), nil
		}

		kind, detail, err := Status()
		if err != nil || kind != KindSystemdUser || detail != "installed (systemd user timer): enabled" {
			t.Fatalf("Status() = (%q, %q, %v)", kind, detail, err)
		}
	})

	t.Run("systemd check failure remains nonfatal", func(t *testing.T) {
		resetDiskScheduleSeams(t)
		runtimeGOOS = "linux"
		home := t.TempDir()
		t.Setenv("HOME", home)
		timerPath := filepath.Join(home, ".config", "systemd", "user", serviceBase+".timer")
		if err := os.MkdirAll(filepath.Dir(timerPath), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(timerPath, []byte("timer"), 0o600); err != nil {
			t.Fatal(err)
		}
		execCombinedOutput = func(string, ...string) ([]byte, error) {
			return []byte("disabled\n"), errors.New("exit status 1")
		}

		kind, detail, err := Status()
		if err != nil || kind != KindSystemdUser || !strings.Contains(detail, "disabled (check failed: exit status 1)") {
			t.Fatalf("Status() = (%q, %q, %v)", kind, detail, err)
		}
	})

	t.Run("launchd detail", func(t *testing.T) {
		resetDiskScheduleSeams(t)
		runtimeGOOS = "darwin"
		home := t.TempDir()
		t.Setenv("HOME", home)
		plistPath := filepath.Join(home, "Library", "LaunchAgents", labelBase+".plist")
		if err := os.MkdirAll(filepath.Dir(plistPath), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(plistPath, []byte("plist"), 0o600); err != nil {
			t.Fatal(err)
		}

		kind, detail, err := Status()
		if err != nil || kind != KindLaunchd || detail != "installed (launchd): "+labelBase+".plist" {
			t.Fatalf("Status() = (%q, %q, %v)", kind, detail, err)
		}
	})

	t.Run("windows task state", func(t *testing.T) {
		resetDiskScheduleSeams(t)
		runtimeGOOS = "windows"
		calls := 0
		powerShellExec = func(script string) ([]byte, error) {
			calls++
			if calls == 1 {
				return []byte("yes\n"), nil
			}
			if !strings.Contains(script, "Select-Object -ExpandProperty State") {
				t.Fatalf("state script = %q", script)
			}
			return []byte("Ready\r\n"), nil
		}

		kind, detail, err := Status()
		if err != nil || kind != KindWindowsTask || detail != "installed (task): Ready" {
			t.Fatalf("Status() = (%q, %q, %v)", kind, detail, err)
		}
	})

	t.Run("windows state error remains nonfatal", func(t *testing.T) {
		resetDiskScheduleSeams(t)
		runtimeGOOS = "windows"
		calls := 0
		powerShellExec = func(string) ([]byte, error) {
			calls++
			if calls == 1 {
				return []byte("yes\n"), nil
			}
			return nil, errors.New("state failed")
		}

		kind, detail, err := Status()
		if err != nil || kind != KindWindowsTask || detail != "installed (task): error state failed" {
			t.Fatalf("Status() = (%q, %q, %v)", kind, detail, err)
		}
	})
}

// TestParseAtTime pins the HH:MM parser used by Install to validate AtTime
// before any platform-specific work runs. The contract is:
//   - trim leading/trailing whitespace
//   - require exactly one colon
//   - hour ∈ [0,23], minute ∈ [0,59]
//   - both parts must be base-10 integers
func TestParseAtTime(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		in      string
		wantH   int
		wantM   int
		wantErr bool
	}{
		{"typical morning", "03:00", 3, 0, false},
		{"midnight", "00:00", 0, 0, false},
		{"late evening", "23:59", 23, 59, false},
		{"single-digit hour", "9:30", 9, 30, false},
		{"single-digit minute", "12:5", 12, 5, false},
		{"no leading zero on hour", "9:00", 9, 0, false},
		{"surrounding whitespace", "  03:00  ", 3, 0, false},
		{"tab whitespace", "\t12:34\t", 12, 34, false},
		{"no colon", "bad", 0, 0, true},
		{"trailing colon only", "12:", 0, 0, true},
		{"leading colon only", ":34", 0, 0, true},
		{"three parts", "12:34:56", 0, 0, true},
		{"non-numeric hour", "ab:00", 0, 0, true},
		{"non-numeric minute", "12:xy", 0, 0, true},
		{"negative hour", "-1:00", 0, 0, true},
		{"negative minute", "12:-1", 0, 0, true},
		{"hour too large", "24:00", 0, 0, true},
		{"hour far too large", "99:00", 0, 0, true},
		{"minute too large", "12:60", 0, 0, true},
		{"minute far too large", "12:99", 0, 0, true},
		{"empty string", "", 0, 0, true},
		{"whitespace only", "   ", 0, 0, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			h, m, err := parseAtTime(tc.in)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("parseAtTime(%q) = (%d, %d, nil); want error", tc.in, h, m)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseAtTime(%q): unexpected error %v", tc.in, err)
			}
			if h != tc.wantH || m != tc.wantM {
				t.Fatalf("parseAtTime(%q) = (%d, %d); want (%d, %d)", tc.in, h, m, tc.wantH, tc.wantM)
			}
		})
	}
}

// TestSystemdQuoteArg pins the systemd ExecStart argument-quoting contract
// used by installSystemdUser when embedding GhPath / ConfigPath into the
// generated .service unit. The contract is:
//   - safe chars [A-Za-z0-9_/.-] pass through unquoted
//   - anything containing space, tab, double-quote, single-quote, or
//     backslash is wrapped in double quotes
//   - inside the wrapper, backslash and double-quote are themselves
//     backslash-escaped; other chars (including single-quote) pass through
//     verbatim
func TestSystemdQuoteArg(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"plain path passes through", "/usr/bin/gh", "/usr/bin/gh"},
		{"path with safe punctuation", "/usr/local/bin/gh-1.2.3", "/usr/local/bin/gh-1.2.3"},
		{"single space wrapped", "/home/me/my config.yml", "\"/home/me/my config.yml\""},
		{"tab wrapped", "/path/with\ttab", "\"/path/with\ttab\""},
		{"double quote escaped", `/path/with"quote`, `"/path/with\"quote"`},
		{"backslash escaped", `/path/with\backslash`, `"/path/with\\backslash"`},
		{"single quote is not special", `/path/with'squote`, `"/path/with'squote"`},
		{"multiple special chars", `a b"c\d`, `"a b\"c\\d"`},
		{"empty string passes through", "", ""},
		{"only space wrapped", " ", "\" \""},
		{"only double quote wrapped and escaped", `"`, `"\""`},
		{"only backslash wrapped and escaped", `\`, `"\\"`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := systemdQuoteArg(tc.in); got != tc.want {
				t.Errorf("systemdQuoteArg(%q) = %q; want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestPlistEscape(t *testing.T) {
	t.Parallel()
	got := hostshell.PlistEscape(`a&b"c<d>e`)
	want := `a&amp;b&quot;c&lt;d&gt;e`
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestDefaultAtTime(t *testing.T) {
	t.Parallel()
	if DefaultAtTime != "03:00" {
		t.Fatalf("got %q", DefaultAtTime)
	}
}

// TestEscapePS pins the PowerShell single-quote escape contract used by
// installWindowsTask (diskschedule.go:314) to embed GhPath / ConfigPath into
// the `powershell -Command` string. The escape rule is `'` → `”` — doubling
// the apostrophe — which is how PowerShell escapes a single quote inside an
// already-single-quoted literal. A future change to use backslash-escape
// (or to skip escaping) would break the Windows task install path.
func TestPowerShellSingleQuote_forDiskSchedule(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"empty string", "", "''"},
		{"no apostrophes", `C:\Users\me\bin\gh.exe`, `'C:\Users\me\bin\gh.exe'`},
		{"single apostrophe doubles", `O'Brien`, `'O''Brien'`},
		{"consecutive apostrophes all double", `it's a 'test'`, `'it''s a ''test'''`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := hostshell.PowerShellSingleQuote(tc.in); got != tc.want {
				t.Errorf("PowerShellSingleQuote(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

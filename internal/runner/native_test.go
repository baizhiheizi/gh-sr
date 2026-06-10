package runner

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/an-lee/gh-sr/internal/config"
	"github.com/an-lee/gh-sr/internal/host"
)

func Test_archForGitHub(t *testing.T) {
	t.Parallel()
	if archForGitHub("amd64") != "x64" {
		t.Errorf("amd64 -> x64")
	}
	if archForGitHub("arm64") != "arm64" {
		t.Errorf("arm64")
	}
	if archForGitHub("riscv") != "riscv" {
		t.Errorf("passthrough: got %q", archForGitHub("riscv"))
	}
}

func Test_runnerTarballURL(t *testing.T) {
	t.Parallel()
	cases := []struct {
		ver, os, arch, wantSub string
	}{
		{"2.320.0", "linux", "x64", "actions-runner-linux-x64-2.320.0.tar.gz"},
		{"2.320.0", "darwin", "arm64", "actions-runner-osx-arm64-2.320.0.tar.gz"},
		{"2.320.0", "windows", "x64", "actions-runner-win-x64-2.320.0.zip"},
		{"2.320.0", "windows", "arm64", "actions-runner-win-arm64-2.320.0.zip"},
	}
	for _, tc := range cases {
		u := runnerTarballURL(tc.ver, tc.os, tc.arch)
		if !strings.Contains(u, tc.wantSub) {
			t.Errorf("%s/%s/%s: got %q want substring %q", tc.ver, tc.os, tc.arch, u, tc.wantSub)
		}
		if !strings.HasPrefix(u, "https://github.com/actions/runner/releases/download/v") {
			t.Errorf("prefix: %q", u)
		}
	}
	if runnerTarballURL("1", "freebsd", "amd64") != "" {
		t.Errorf("unsupported OS should return empty")
	}
}

func Test_windowsNativeInstallScript_usesPowerShellExpressions(t *testing.T) {
	t.Parallel()
	h := host.NewHost("win", config.HostConfig{Addr: "u@h", OS: "windows", Arch: "amd64"})

	script := windowsNativeInstallScript(
		h,
		"unwx-1",
		"2.333.1",
		"https://github.com/actions/runner/releases/download/v2.333.1/actions-runner-win-x64-2.333.1.zip",
	)

	if !strings.Contains(script, "$runnerDir = Join-Path (Join-Path $env:USERPROFILE '.gh-sr\\runners') 'unwx-1'") {
		t.Fatalf("runner dir should be built from PowerShell expressions: %q", script)
	}
	if !strings.Contains(script, "$zip = Join-Path $env:TEMP 'actions-runner-2.333.1.zip'") {
		t.Fatalf("zip path should use Join-Path with $env:TEMP: %q", script)
	}
	if strings.Contains(script, "'$env:USERPROFILE") || strings.Contains(script, "'$env:TEMP") {
		t.Fatalf("script should not quote env expressions literally: %q", script)
	}
}

func Test_windowsNativeConfigScript_usesRunnerDirVariable(t *testing.T) {
	t.Parallel()
	h := host.NewHost("win", config.HostConfig{Addr: "u@h", OS: "windows", Arch: "amd64"})
	rc := config.RunnerConfig{
		Repo:   "an-lee/gh-sr",
		Labels: []string{"windows", "native"},
	}

	script := windowsNativeConfigScript(h, rc, "unwx-1", "token-value", 0)

	if !strings.Contains(script, "$runnerDir = Join-Path (Join-Path $env:USERPROFILE '.gh-sr\\runners') 'unwx-1'") {
		t.Fatalf("runner dir should be assigned from a PowerShell expression: %q", script)
	}
	if !strings.Contains(script, "Set-Location -Path $runnerDir; & .\\config.cmd --unattended") {
		t.Fatalf("script should configure from the resolved runner dir: %q", script)
	}
	if strings.Contains(script, "cd '$env:USERPROFILE") {
		t.Fatalf("script should not cd into a literalized env path: %q", script)
	}
}

func Test_windowsStartNative_usesCimProcessCreateForMergedLogs(t *testing.T) {
	t.Parallel()
	for _, addr := range []string{config.LocalAddr, "u@h"} {
		h := host.NewHost("win", config.HostConfig{Addr: addr, OS: "windows", Arch: "amd64"})
		script := windowsNativeStartScript(h, "x-1")
		if !strings.Contains(script, "Invoke-CimMethod") || !strings.Contains(script, "Win32_Process") {
			t.Fatalf("addr=%s: expected Invoke-CimMethod Win32_Process.Create: %q", addr, script)
		}
		if !strings.Contains(script, "cmd.exe") || !strings.Contains(script, "run.cmd") || !strings.Contains(script, "2>&1") {
			t.Fatalf("addr=%s: expected cmd.exe run.cmd shell redirection to runner.log: %q", addr, script)
		}
		if strings.Contains(script, "Start-Process") {
			t.Fatalf("addr=%s: Start-Process is tied to the SSH job on Win32-OpenSSH; use CIM instead: %q", addr, script)
		}
		if !strings.Contains(script, "ReturnValue") {
			t.Fatalf("addr=%s: should check Win32_Process.Create ReturnValue: %q", addr, script)
		}
		if !strings.Contains(script, "CreateFlags=0x08000000") {
			t.Fatalf("addr=%s: should use CREATE_NO_WINDOW (0x08000000) to suppress console: %q", addr, script)
		}
		if !strings.Contains(script, "Win32_ProcessStartup") {
			t.Fatalf("addr=%s: should use Win32_ProcessStartup: %q", addr, script)
		}
		if !strings.Contains(script, "ProcessStartupInformation") {
			t.Fatalf("addr=%s: should pass ProcessStartupInformation to Win32_Process.Create: %q", addr, script)
		}
	}
}

func Test_staleRegistrationMsg(t *testing.T) {
	t.Parallel()
	logLine := `Failed to create a session. The runner registration has been deleted from the server, please re-configure.`
	if !strings.Contains(logLine, staleRegistrationMsg) {
		t.Fatalf("staleRegistrationMsg %q not found in typical log line", staleRegistrationMsg)
	}
}

func Test_windowsRunnerDirAssignment(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name         string
		instanceName string
		wantSuffix   string // expected trailing expression from h.RunnerDirPS(...)
	}{
		{
			name:         "simple instance name",
			instanceName: "r1",
			wantSuffix:   "Join-Path (Join-Path $env:USERPROFILE '.gh-sr\\runners') 'r1'",
		},
		{
			name:         "hyphenated instance name",
			instanceName: "runner-1",
			wantSuffix:   "Join-Path (Join-Path $env:USERPROFILE '.gh-sr\\runners') 'runner-1'",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			h := &host.Host{HostConfig: config.HostConfig{OS: "windows"}}
			got := windowsRunnerDirAssignment(h, tc.instanceName)
			want := "$runnerDir = " + tc.wantSuffix
			if got != want {
				t.Errorf("windowsRunnerDirAssignment(windows, %q): got %q, want %q",
					tc.instanceName, got, want)
			}
			if !strings.HasPrefix(got, "$runnerDir = ") {
				t.Errorf("result must start with %q, got %q", "$runnerDir = ", got)
			}
		})
	}
}

func Test_windowsDeleteRunnerConfig_removesCredentialFiles(t *testing.T) {
	t.Parallel()
	h := host.NewHost("win", config.HostConfig{Addr: "u@h", OS: "windows", Arch: "amd64"})
	script := windowsDeleteRunnerConfig(h, "runner-x")

	for _, file := range []string{".runner", ".credentials", ".credentials_rsaparams"} {
		if !strings.Contains(script, "Remove-Item") || !strings.Contains(script, file) {
			t.Errorf("script should Remove-Item %s: %q", file, script)
		}
	}
	if !strings.Contains(script, "-Force") || !strings.Contains(script, "-EA SilentlyContinue") {
		t.Errorf("script should use -Force and -EA SilentlyContinue: %q", script)
	}
}

func Test_windowsCheckStaleRegistration_containsPatternAndSleep(t *testing.T) {
	t.Parallel()
	h := host.NewHost("win", config.HostConfig{Addr: "u@h", OS: "windows", Arch: "amd64"})
	script := windowsCheckStaleRegistration(h, "x-1")
	if !strings.Contains(script, "Start-Sleep") {
		t.Fatalf("should wait before checking: %q", script)
	}
	if !strings.Contains(script, staleRegistrationMsg) {
		t.Fatalf("should search for stale registration message: %q", script)
	}
	if !strings.Contains(script, "Select-String") {
		t.Fatalf("should use Select-String to search runner.log: %q", script)
	}
}

func TestNativeRunnerConfigPresent_local(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("unix-style paths and sh -c probe")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}
	inst := "ghsr-native-probe-" + filepath.Base(t.TempDir())

	base := filepath.Join(home, ".gh-sr", "runners", inst)
	if err := os.MkdirAll(base, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(base) })

	if err := os.WriteFile(filepath.Join(base, ".runner"), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	// run.sh is required by the NeedsSetup check
	if err := os.WriteFile(filepath.Join(base, "run.sh"), []byte("#!/bin/sh\necho hi"), 0o755); err != nil {
		t.Fatal(err)
	}

	h := host.NewHost("local-test", config.HostConfig{Addr: config.LocalAddr, OS: "linux", Arch: "amd64"})
	if err := h.Connect(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = h.Close() })

	ok, err := NativeRunnerConfigPresent(h, inst)
	if err != nil || !ok {
		t.Fatalf("expected installed: ok=%v err=%v", ok, err)
	}

	okMissing, errMissing := NativeRunnerConfigPresent(h, inst+"-not-there")
	if errMissing != nil || okMissing {
		t.Fatalf("expected not installed: ok=%v err=%v", okMissing, errMissing)
	}
}

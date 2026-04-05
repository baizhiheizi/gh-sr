package runner

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/an-lee/ghr/internal/config"
	"github.com/an-lee/ghr/internal/host"
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

	if !strings.Contains(script, "$runnerDir = Join-Path (Join-Path $env:USERPROFILE '.ghr\\runners') 'unwx-1'") {
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
		Repo:   "an-lee/gh-runners",
		Labels: []string{"windows", "native"},
	}

	script := windowsNativeConfigScript(h, rc, "unwx-1", "token-value")

	if !strings.Contains(script, "$runnerDir = Join-Path (Join-Path $env:USERPROFILE '.ghr\\runners') 'unwx-1'") {
		t.Fatalf("runner dir should be assigned from a PowerShell expression: %q", script)
	}
	if !strings.Contains(script, "Set-Location -Path $runnerDir; & .\\config.cmd --unattended") {
		t.Fatalf("script should configure from the resolved runner dir: %q", script)
	}
	if strings.Contains(script, "cd '$env:USERPROFILE") {
		t.Fatalf("script should not cd into a literalized env path: %q", script)
	}
}

func Test_windowsStartNative_usesCmdForMergedLogs(t *testing.T) {
	t.Parallel()
	for _, addr := range []string{config.LocalAddr, "u@h"} {
		h := host.NewHost("win", config.HostConfig{Addr: addr, OS: "windows", Arch: "amd64"})
		script := windowsNativeStartScript(h, "x-1")
		if !strings.Contains(script, "Start-Process") || !strings.Contains(script, "cmd.exe") || !strings.Contains(script, "2>&1") {
			t.Fatalf("addr=%s: expected Start-Process cmd.exe merged redirection: %q", addr, script)
		}
		if strings.Contains(script, "RedirectStandardOutput") {
			t.Fatalf("addr=%s: should not use Start-Process stream redirects to a single log file: %q", addr, script)
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
	inst := "ghr-native-probe-" + filepath.Base(t.TempDir())

	base := filepath.Join(home, ".ghr", "runners", inst)
	if err := os.MkdirAll(base, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(base) })

	if err := os.WriteFile(filepath.Join(base, ".runner"), []byte("{}"), 0o644); err != nil {
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

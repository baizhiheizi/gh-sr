package host

import (
	"strings"
	"testing"

	"github.com/an-lee/ghr/internal/config"
)

func Test_parseAddr(t *testing.T) {
	t.Parallel()
	u, a := parseAddr("user@host.example:2222")
	if u != "user" || a != "host.example:2222" {
		t.Fatalf("with user: got %q %q", u, a)
	}
	u, a = parseAddr("192.168.1.1")
	if u != "" || a != "192.168.1.1" {
		t.Fatalf("no user: got %q %q", u, a)
	}
}

func TestHost_wrapCommand(t *testing.T) {
	t.Parallel()
	h := NewHost("w", config.HostConfig{Addr: "u@h", OS: "windows", Arch: "amd64"})
	got := h.wrapCommand(`Write-Host "ok"`)
	if !strings.Contains(got, "powershell") {
		t.Fatalf("windows should wrap powershell: %q", got)
	}
	ln := NewHost("l", config.HostConfig{Addr: "u@h", OS: "linux", Arch: "amd64"})
	if ln.wrapCommand("echo hi") != "echo hi" {
		t.Fatalf("linux should pass through")
	}
}

func TestHost_paths(t *testing.T) {
	t.Parallel()
	win := NewHost("w", config.HostConfig{Addr: "u@h", OS: "windows", Arch: "amd64"})
	if win.RunnerBaseDir() != `C:\actions-runner` {
		t.Errorf("windows base: %q", win.RunnerBaseDir())
	}
	if win.RunnerDir("r1") != `C:\actions-runner\r1` {
		t.Errorf("windows runner dir: %q", win.RunnerDir("r1"))
	}
	if win.TempDir() != "$env:TEMP" {
		t.Errorf("windows temp: %q", win.TempDir())
	}
	if win.PathSep() != `\` {
		t.Errorf("windows sep: %q", win.PathSep())
	}

	ln := NewHost("l", config.HostConfig{Addr: "u@h", OS: "linux", Arch: "arm64"})
	if ln.RunnerBaseDir() != "$HOME/actions-runner" {
		t.Errorf("linux base: %q", ln.RunnerBaseDir())
	}
	if ln.RunnerDir("r1") != "$HOME/actions-runner/r1" {
		t.Errorf("linux runner dir: %q", ln.RunnerDir("r1"))
	}
	if ln.TempDir() != "/tmp" {
		t.Errorf("linux temp: %q", ln.TempDir())
	}
	if ln.PathSep() != "/" {
		t.Errorf("linux sep: %q", ln.PathSep())
	}
}

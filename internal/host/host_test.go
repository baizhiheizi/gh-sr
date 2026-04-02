package host

import (
	"encoding/base64"
	"strings"
	"testing"
	"unicode/utf16"

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

func TestEncodePowerShellScript_roundTrip(t *testing.T) {
	t.Parallel()
	script := "Write-Host \"a\"'\nline2"
	enc := encodePowerShellScript(script)
	raw, err := base64.StdEncoding.DecodeString(enc)
	if err != nil {
		t.Fatal(err)
	}
	if len(raw)%2 != 0 {
		t.Fatalf("UTF-16LE byte length should be even, got %d", len(raw))
	}
	u16 := make([]uint16, len(raw)/2)
	for i := range u16 {
		u16[i] = uint16(raw[i*2]) | uint16(raw[i*2+1])<<8
	}
	got := string(utf16.Decode(u16))
	if got != script {
		t.Errorf("round-trip: got %q want %q", got, script)
	}
}

func TestHost_wrapCommand(t *testing.T) {
	t.Parallel()
	h := NewHost("w", config.HostConfig{Addr: "u@h", OS: "windows", Arch: "amd64"})
	got := h.wrapCommand(`Write-Host "ok"`)
	if !strings.Contains(got, "powershell.exe") {
		t.Fatalf("windows default exe: %q", got)
	}
	if !strings.Contains(got, "-EncodedCommand") {
		t.Fatalf("should use EncodedCommand: %q", got)
	}
	if strings.Contains(got, "-Command") {
		t.Fatalf("should not use -Command (quoting): %q", got)
	}

	hPW := NewHost("w", config.HostConfig{Addr: "u@h", OS: "windows", Arch: "amd64", WindowsPS: "pwsh"})
	gotPW := hPW.wrapCommand(`1`)
	if !strings.Contains(gotPW, "pwsh.exe") {
		t.Fatalf("windows_ps pwsh: %q", gotPW)
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

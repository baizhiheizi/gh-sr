package host

import (
	"encoding/base64"
	"strings"
	"testing"
	"unicode/utf16"

	"github.com/an-lee/gh-wm/internal/config"
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

func TestIsLocal(t *testing.T) {
	t.Parallel()
	cases := []struct {
		addr string
		want bool
	}{
		{"local", true},
		{"Local", true},
		{"LOCAL", true},
		{" local ", true},
		{"user@host", false},
		{"", false},
		{"localhost", false},
	}
	for _, tc := range cases {
		if got := IsLocal(tc.addr); got != tc.want {
			t.Errorf("IsLocal(%q) = %v, want %v", tc.addr, got, tc.want)
		}
	}
}

func TestHost_ConnectLocal(t *testing.T) {
	t.Parallel()
	h := NewHost("local-box", config.HostConfig{Addr: "local", OS: "linux", Arch: "amd64"})
	if err := h.Connect(); err != nil {
		t.Fatalf("Connect local: %v", err)
	}
	defer h.Close()

	if _, ok := h.conn.(*LocalConnection); !ok {
		t.Fatalf("expected *LocalConnection, got %T", h.conn)
	}

	out, err := h.Run("echo works")
	if err != nil {
		t.Fatalf("Run on local: %v", err)
	}
	if out != "works" {
		t.Errorf("got %q, want %q", out, "works")
	}
}

func TestHost_wrapCommand_localWindows(t *testing.T) {
	t.Parallel()
	h := NewHost("local-win", config.HostConfig{Addr: "local", OS: "windows", Arch: "amd64"})
	got := h.wrapCommand("Get-Date")
	if got != "Get-Date" {
		t.Errorf("local windows should pass through, got %q", got)
	}
}

func TestHost_paths(t *testing.T) {
	t.Parallel()
	win := NewHost("w", config.HostConfig{Addr: "u@h", OS: "windows", Arch: "amd64"})
	if win.RunnerBaseDir() != `$env:USERPROFILE\.gh-wm\runners` {
		t.Errorf("windows base: %q", win.RunnerBaseDir())
	}
	if win.RunnerBaseDirPS() != `Join-Path $env:USERPROFILE '.gh-wm\runners'` {
		t.Errorf("windows base ps: %q", win.RunnerBaseDirPS())
	}
	if win.RunnerDir("r1") != `$env:USERPROFILE\.gh-wm\runners\r1` {
		t.Errorf("windows runner dir: %q", win.RunnerDir("r1"))
	}
	if win.RunnerDirPS("r1") != `Join-Path (Join-Path $env:USERPROFILE '.gh-wm\runners') 'r1'` {
		t.Errorf("windows runner dir ps: %q", win.RunnerDirPS("r1"))
	}
	if win.TempDir() != "$env:TEMP" {
		t.Errorf("windows temp: %q", win.TempDir())
	}
	if win.TempDirPS() != "$env:TEMP" {
		t.Errorf("windows temp ps: %q", win.TempDirPS())
	}
	if win.PathSep() != `\` {
		t.Errorf("windows sep: %q", win.PathSep())
	}

	ln := NewHost("l", config.HostConfig{Addr: "u@h", OS: "linux", Arch: "arm64"})
	if ln.RunnerBaseDir() != "$HOME/.gh-wm/runners" {
		t.Errorf("linux base: %q", ln.RunnerBaseDir())
	}
	if ln.RunnerBaseDirPS() != "$HOME/.gh-wm/runners" {
		t.Errorf("linux base ps: %q", ln.RunnerBaseDirPS())
	}
	if ln.RunnerDir("r1") != "$HOME/.gh-wm/runners/r1" {
		t.Errorf("linux runner dir: %q", ln.RunnerDir("r1"))
	}
	if ln.RunnerDirPS("r1") != "$HOME/.gh-wm/runners/r1" {
		t.Errorf("linux runner dir ps: %q", ln.RunnerDirPS("r1"))
	}
	if ln.TempDir() != "/tmp" {
		t.Errorf("linux temp: %q", ln.TempDir())
	}
	if ln.TempDirPS() != "/tmp" {
		t.Errorf("linux temp ps: %q", ln.TempDirPS())
	}
	if ln.PathSep() != "/" {
		t.Errorf("linux sep: %q", ln.PathSep())
	}
}

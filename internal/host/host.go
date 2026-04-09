package host

import (
	"encoding/base64"
	"fmt"
	"strings"
	"unicode/utf16"

	"github.com/an-lee/gh-sr/internal/config"
)

type Host struct {
	Name string
	config.HostConfig
	conn Executor
}

func IsLocal(addr string) bool {
	return config.IsLocalAddr(addr)
}

func NewHost(name string, cfg config.HostConfig) *Host {
	return &Host{
		Name:       name,
		HostConfig: cfg,
	}
}

func (h *Host) Connect() error {
	if IsLocal(h.Addr) {
		h.conn = NewLocalConnection()
		return nil
	}
	user, addr := parseAddr(h.Addr)
	conn, err := NewConnection(user, addr)
	if err != nil {
		return fmt.Errorf("connecting to %s (%s): %w", h.Name, h.Addr, err)
	}
	h.conn = conn
	return nil
}

func (h *Host) Close() error {
	if h.conn != nil {
		return h.conn.Close()
	}
	return nil
}

func (h *Host) Run(cmd string) (string, error) {
	if h.conn == nil {
		if err := h.Connect(); err != nil {
			return "", err
		}
	}
	return h.conn.Run(cmd)
}

func (h *Host) RunShell(cmd string) (string, error) {
	wrapped := h.wrapCommand(cmd)
	return h.Run(wrapped)
}

func (h *Host) Upload(localPath, remotePath string) error {
	if h.conn == nil {
		if err := h.Connect(); err != nil {
			return err
		}
	}
	return h.conn.Upload(localPath, remotePath)
}

// encodePowerShellScript returns the base64 payload required by powershell.exe / pwsh -EncodedCommand (UTF-16LE).
func encodePowerShellScript(script string) string {
	u16 := utf16.Encode([]rune(script))
	b := make([]byte, len(u16)*2)
	for i, v := range u16 {
		b[i*2] = byte(v)
		b[i*2+1] = byte(v >> 8)
	}
	return base64.StdEncoding.EncodeToString(b)
}

func (h *Host) windowsPowerShellExe() string {
	switch strings.ToLower(strings.TrimSpace(h.WindowsPS)) {
	case "pwsh":
		return "pwsh.exe"
	default:
		return "powershell.exe"
	}
}

func (h *Host) wrapCommand(cmd string) string {
	if h.OS == "windows" && !IsLocal(h.Addr) {
		enc := encodePowerShellScript(cmd)
		exe := h.windowsPowerShellExe()
		return fmt.Sprintf("%s -NoProfile -NonInteractive -EncodedCommand %s", exe, enc)
	}
	return cmd
}

func (h *Host) RunnerBaseDir() string {
	if h.OS == "windows" {
		return `$env:USERPROFILE\.gh-sr\runners`
	}
	return "$HOME/.gh-sr/runners"
}

func (h *Host) RunnerBaseDirPS() string {
	if h.OS == "windows" {
		return `Join-Path $env:USERPROFILE '.gh-sr\runners'`
	}
	return h.RunnerBaseDir()
}

func (h *Host) RunnerDir(instanceName string) string {
	base := h.RunnerBaseDir()
	if h.OS == "windows" {
		return base + `\` + instanceName
	}
	return base + "/" + instanceName
}

func (h *Host) RunnerDirPS(instanceName string) string {
	base := h.RunnerBaseDirPS()
	if h.OS == "windows" {
		return fmt.Sprintf("Join-Path (%s) '%s'", base, strings.ReplaceAll(instanceName, "'", "''"))
	}
	return h.RunnerDir(instanceName)
}

func (h *Host) TempDir() string {
	if h.OS == "windows" {
		return "$env:TEMP"
	}
	return "/tmp"
}

func (h *Host) TempDirPS() string {
	if h.OS == "windows" {
		return "$env:TEMP"
	}
	return h.TempDir()
}

func (h *Host) PathSep() string {
	if h.OS == "windows" {
		return `\`
	}
	return "/"
}

// SSHUser returns the SSH login from Addr when it uses user@host form; otherwise "" (e.g. local, bare hostname).
func (h *Host) SSHUser() string {
	if IsLocal(h.Addr) {
		return ""
	}
	u, _ := parseAddr(h.Addr)
	return u
}

func parseAddr(addr string) (user, host string) {
	if i := strings.Index(addr, "@"); i >= 0 {
		return addr[:i], addr[i+1:]
	}
	return "", addr
}

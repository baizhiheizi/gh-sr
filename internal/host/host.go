package host

import (
	"fmt"
	"strings"

	"github.com/an-lee/ghr/internal/config"
)

type Host struct {
	Name string
	config.HostConfig
	conn *Connection
}

func NewHost(name string, cfg config.HostConfig) *Host {
	return &Host{
		Name:       name,
		HostConfig: cfg,
	}
}

func (h *Host) Connect() error {
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

func (h *Host) wrapCommand(cmd string) string {
	if h.OS == "windows" {
		return fmt.Sprintf("powershell -NoProfile -NonInteractive -Command %q", cmd)
	}
	return cmd
}

func (h *Host) RunnerBaseDir() string {
	if h.OS == "windows" {
		return `C:\actions-runner`
	}
	return "$HOME/actions-runner"
}

func (h *Host) RunnerDir(instanceName string) string {
	base := h.RunnerBaseDir()
	if h.OS == "windows" {
		return base + `\` + instanceName
	}
	return base + "/" + instanceName
}

func (h *Host) TempDir() string {
	if h.OS == "windows" {
		return "$env:TEMP"
	}
	return "/tmp"
}

func (h *Host) PathSep() string {
	if h.OS == "windows" {
		return `\`
	}
	return "/"
}

func parseAddr(addr string) (user, host string) {
	if i := strings.Index(addr, "@"); i >= 0 {
		return addr[:i], addr[i+1:]
	}
	return "", addr
}

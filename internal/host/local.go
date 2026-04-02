package host

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// LocalConnection executes commands on the local machine via os/exec.
type LocalConnection struct{}

func NewLocalConnection() *LocalConnection {
	return &LocalConnection{}
}

func (c *LocalConnection) Run(cmd string) (string, error) {
	var proc *exec.Cmd
	if runtime.GOOS == "windows" {
		proc = exec.Command("powershell.exe", "-NoProfile", "-NonInteractive", "-Command", cmd)
	} else {
		proc = exec.Command("sh", "-c", cmd)
	}

	var stdout, stderr bytes.Buffer
	proc.Stdout = &stdout
	proc.Stderr = &stderr

	if err := proc.Run(); err != nil {
		return stdout.String(), fmt.Errorf("running %q: %w\nstderr: %s", cmd, err, stderr.String())
	}

	return strings.TrimSpace(stdout.String()), nil
}

func (c *LocalConnection) Upload(localPath, remotePath string) error {
	if err := os.MkdirAll(filepath.Dir(remotePath), 0o755); err != nil {
		return fmt.Errorf("creating directory for %s: %w", remotePath, err)
	}

	src, err := os.Open(localPath)
	if err != nil {
		return fmt.Errorf("opening source file: %w", err)
	}
	defer src.Close()

	dst, err := os.Create(remotePath)
	if err != nil {
		return fmt.Errorf("creating destination file: %w", err)
	}
	defer dst.Close()

	if _, err := io.Copy(dst, src); err != nil {
		return fmt.Errorf("copying file: %w", err)
	}

	return nil
}

func (c *LocalConnection) Close() error {
	return nil
}

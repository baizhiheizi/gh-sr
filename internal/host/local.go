package host

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"github.com/an-lee/gh-sr/internal/hostshell/ps"
)

// LocalConnection executes commands on the local machine via os/exec.
type LocalConnection struct{}

func NewLocalConnection() *LocalConnection {
	return &LocalConnection{}
}

func (c *LocalConnection) Run(cmd string) (string, error) {
	var proc *exec.Cmd
	if runtime.GOOS == "windows" {
		args := ps.CommandArgs(cmd)
		proc = exec.Command(args[0], args[1:]...)
	} else {
		proc = exec.Command("sh", "-c", cmd)
	}

	return runWithCapture(cmd, func(stdout, stderr io.Writer) error {
		proc.Stdout = stdout
		proc.Stderr = stderr
		return proc.Run()
	})
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

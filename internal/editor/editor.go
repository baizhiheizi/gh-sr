package editor

import (
	"os"
	"os/exec"
	"runtime"
)

// Preferred returns VISUAL, then EDITOR, then a platform default.
func Preferred() string {
	if v := os.Getenv("VISUAL"); v != "" {
		return v
	}
	if v := os.Getenv("EDITOR"); v != "" {
		return v
	}
	if runtime.GOOS == "windows" {
		return "notepad"
	}
	return "vim"
}

// Command returns an exec.Cmd to open path in the preferred editor (stdin/stdout/stderr unset).
func Command(path string) *exec.Cmd {
	return exec.Command(Preferred(), path)
}

// Open launches the preferred editor with path, attached to the terminal.
func Open(path string) error {
	cmd := Command(path)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

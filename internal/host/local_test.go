package host

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestLocalConnection_Run(t *testing.T) {
	t.Parallel()
	c := NewLocalConnection()

	if runtime.GOOS == "windows" {
		out, err := c.Run("Write-Output 'hello'")
		if err != nil {
			t.Fatalf("Run failed: %v", err)
		}
		if strings.TrimSpace(out) != "hello" {
			t.Errorf("got %q, want %q", out, "hello")
		}
	} else {
		out, err := c.Run("echo hello")
		if err != nil {
			t.Fatalf("Run failed: %v", err)
		}
		if out != "hello" {
			t.Errorf("got %q, want %q", out, "hello")
		}
	}
}

func TestLocalConnection_RunError(t *testing.T) {
	t.Parallel()
	c := NewLocalConnection()

	_, err := c.Run("exit 1")
	if err == nil {
		t.Fatal("expected error from failing command")
	}
}

func TestLocalConnection_Upload(t *testing.T) {
	t.Parallel()
	c := NewLocalConnection()

	dir := t.TempDir()
	src := filepath.Join(dir, "src.txt")
	dst := filepath.Join(dir, "sub", "dst.txt")

	if err := os.WriteFile(src, []byte("payload"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := c.Upload(src, dst); err != nil {
		t.Fatalf("Upload failed: %v", err)
	}

	data, err := os.ReadFile(dst)
	if err != nil {
		t.Fatalf("reading uploaded file: %v", err)
	}
	if string(data) != "payload" {
		t.Errorf("got %q, want %q", string(data), "payload")
	}
}

func TestLocalConnection_Close(t *testing.T) {
	t.Parallel()
	c := NewLocalConnection()
	if err := c.Close(); err != nil {
		t.Fatalf("Close should be a no-op: %v", err)
	}
}

// TestLocalConnection_Upload_errorPaths covers the three error-wrapping
// branches in LocalConnection.Upload. These are the only branches in
// the function that don't appear in the happy-path test above, and a
// regression that drops any one of them would otherwise compile and
// pass CI on Linux.
func TestLocalConnection_Upload_sourceMissing(t *testing.T) {
	t.Parallel()
	c := NewLocalConnection()
	dir := t.TempDir()
	missing := filepath.Join(dir, "does-not-exist.txt")
	dst := filepath.Join(dir, "dst.txt")

	err := c.Upload(missing, dst)
	if err == nil {
		t.Fatal("expected error for missing source, got nil")
	}
	if !strings.Contains(err.Error(), "opening source file") {
		t.Errorf("error should wrap 'opening source file', got %v", err)
	}
}

// TestLocalConnection_Upload_mkdirFailure covers the
// "creating directory for %s" branch by pointing dst's parent at a
// path under an existing regular file. MkdirAll on that path fails
// with ENOTDIR, which mirrors the real-world "destination's parent is
// gone" failure mode without requiring filesystem permissions tricks.
func TestLocalConnection_Upload_mkdirFailure(t *testing.T) {
	t.Parallel()
	c := NewLocalConnection()
	dir := t.TempDir()
	src := filepath.Join(dir, "src.txt")
	if err := os.WriteFile(src, []byte("payload"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Create a regular file that will stand in for a path component,
	// so MkdirAll(parent) cannot succeed.
	blocking := filepath.Join(dir, "blocker")
	if err := os.WriteFile(blocking, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	dst := filepath.Join(blocking, "sub", "dst.txt")

	err := c.Upload(src, dst)
	if err == nil {
		t.Fatal("expected error when parent path blocks MkdirAll, got nil")
	}
	if !strings.Contains(err.Error(), "creating directory for") {
		t.Errorf("error should wrap 'creating directory for', got %v", err)
	}
}

// TestLocalConnection_Run_stderrInError covers the stderr-in-error
// branch in runWithCapture. The happy-path Run test asserts the stdout
// trim; this one pins that stderr written by the failed process ends
// up in the returned error's "stderr: …" tail so operators can debug
// without re-running the command.
func TestLocalConnection_Run_stderrInError(t *testing.T) {
	t.Parallel()
	c := NewLocalConnection()

	_, err := c.Run("echo to-stdout; echo to-stderr 1>&2; exit 1")
	if err == nil {
		t.Fatal("expected error from failing command, got nil")
	}
	if !strings.Contains(err.Error(), "to-stderr") {
		t.Errorf("error should include stderr output, got %v", err)
	}
	if !strings.Contains(err.Error(), "stderr:") {
		t.Errorf("error should wrap with 'stderr:' label, got %v", err)
	}
}

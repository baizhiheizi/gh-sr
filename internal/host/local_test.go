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

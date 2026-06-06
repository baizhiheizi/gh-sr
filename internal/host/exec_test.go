package host

import (
	"errors"
	"io"
	"strings"
	"testing"
)

func TestRunWithCapture_Success(t *testing.T) {
	t.Parallel()

	out, err := runWithCapture("echo hi", func(stdout, stderr io.Writer) error {
		_, _ = stdout.Write([]byte("  hi\n"))
		_, _ = stderr.Write([]byte(""))
		return nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out != "hi" {
		t.Errorf("got %q, want %q", out, "hi")
	}
}

func TestRunWithCapture_EmptyStdout(t *testing.T) {
	t.Parallel()

	out, err := runWithCapture("silent", func(stdout, stderr io.Writer) error {
		_, _ = stdout.Write([]byte(""))
		_, _ = stderr.Write([]byte(""))
		return nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out != "" {
		t.Errorf("got %q, want empty string", out)
	}
}

func TestRunWithCapture_StderrOnFailure(t *testing.T) {
	t.Parallel()

	baseErr := errors.New("exit status 1")
	out, err := runWithCapture("false", func(stdout, stderr io.Writer) error {
		_, _ = stdout.Write([]byte("partial stdout\n"))
		_, _ = stderr.Write([]byte("boom\n"))
		return baseErr
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if out != "partial stdout\n" {
		t.Errorf("stdout on failure: got %q, want %q", out, "partial stdout\n")
	}
	msg := err.Error()
	if !strings.Contains(msg, `"false"`) {
		t.Errorf("error %q should include the failed command", msg)
	}
	if !strings.Contains(msg, "boom") {
		t.Errorf("error %q should include stderr", msg)
	}
	if !errors.Is(err, baseErr) {
		t.Errorf("expected wrapped error to satisfy errors.Is, got %v", err)
	}
}

func TestRunWithCapture_WritersAreDistinct(t *testing.T) {
	t.Parallel()

	// If the helper accidentally passed the same writer twice, the captured
	// stdout would contain "stderr payload" too. Confirms the two writers
	// route to separate buffers.
	out, err := runWithCapture("route", func(stdout, stderr io.Writer) error {
		_, _ = stdout.Write([]byte("to stdout"))
		_, _ = stderr.Write([]byte("to stderr"))
		return nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out != "to stdout" {
		t.Errorf("stdout contaminated by stderr: %q", out)
	}
}

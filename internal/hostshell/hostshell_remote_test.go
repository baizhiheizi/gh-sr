package hostshell

import (
	"encoding/base64"
	"errors"
	"strings"
	"testing"

	"github.com/an-lee/gh-sr/internal/config"
	"github.com/an-lee/gh-sr/internal/host"
)

// recordingExecutor captures every Run/Upload/Close call so tests can
// inspect the exact shell command that hostshell.WriteRemoteBytes emits.
// Mirrors the mockExecutor pattern in internal/host/mock_test.go but
// lives in this package (hostshell is an import of host, not the other
// way around, so we cannot reuse host's unexported mock).
type recordingExecutor struct {
	runs    []string
	runErr  error
	uploadC int
	closeC  int
}

func (r *recordingExecutor) Run(cmd string) (string, error) {
	r.runs = append(r.runs, cmd)
	return "", r.runErr
}

func (r *recordingExecutor) Upload(_, _ string) error {
	r.uploadC++
	return nil
}

func (r *recordingExecutor) Close() error {
	r.closeC++
	return nil
}

// newLocalMockHost builds a *host.Host whose Run/RunShell is satisfied by
// the supplied recordingExecutor. Addr is set to "local" so
// config.IsLocalAddr reports true and Host.wrapCommand is a no-op — the
// recordingExecutor sees the literal script that WriteRemoteBytes emitted.
func newLocalMockHost(t *testing.T, osKind string, rec *recordingExecutor) *host.Host {
	t.Helper()
	h := host.NewHost("test", config.HostConfig{OS: osKind, Addr: "local"})
	h.SetConn(rec)
	t.Cleanup(func() { _ = h.Close() })
	return h
}

// extractPosixPayload returns the base64 payload that WriteRemoteBytes
// embedded inside the single-quoted second argument of `printf '%s' ...`.
// The script shape is:
//
//	set -e; d=$(dirname '<path>'); mkdir -p "$d"; printf '%s' '<b64>' | base64 -d > '<path>'
//
// so the payload is the second single-quoted run after the `printf '%s' `.
// Like extractPosixPath, this walks the concatenation of single-quoted runs
// joined by the POSIX `'\”` escape so an embedded apostrophe round-trips.
func extractPosixPayload(t *testing.T, script string) []byte {
	t.Helper()
	const marker = "printf '%s' "
	i := strings.Index(script, marker)
	if i < 0 {
		t.Fatalf("posix script missing %q marker:\n%s", marker, script)
	}
	rest := script[i+len(marker):]
	if !strings.HasPrefix(rest, "'") {
		t.Fatalf("posix script payload not single-quoted: %q", rest)
	}
	var b strings.Builder
	for j := 1; j < len(rest); j++ {
		if rest[j] != '\'' {
			b.WriteByte(rest[j])
			continue
		}
		if j+3 < len(rest) && rest[j+1] == '\\' && rest[j+2] == '\'' && rest[j+3] == '\'' {
			b.WriteByte('\'')
			j += 3
			continue
		}
		break
	}
	out, err := base64.StdEncoding.DecodeString(b.String())
	if err != nil {
		t.Fatalf("posix payload base64 decode failed: %v\ndecoded=%q", err, b.String())
	}
	return out
}

// extractPosixPath returns the dirname path from a WriteRemoteBytes POSIX
// script. It pulls the first single-quoted argument after `dirname ` and
// reverses PosixSingleQuote escaping. PosixSingleQuote encodes an embedded
// apostrophe as `'\”` (close, escaped quote, reopen), so the parser walks
// the concatenation of single-quoted runs joined by that escape.
func extractPosixPath(t *testing.T, script string) string {
	t.Helper()
	const marker = "d=$(dirname "
	i := strings.Index(script, marker)
	if i < 0 {
		t.Fatalf("posix script missing %q marker:\n%s", marker, script)
	}
	rest := script[i+len(marker):]
	if !strings.HasPrefix(rest, "'") {
		t.Fatalf("posix dirname arg not single-quoted: %q", rest)
	}
	var b strings.Builder
	for j := 1; j < len(rest); j++ {
		// Inside a single-quoted run: every byte is literal until the next "'".
		if rest[j] != '\'' {
			b.WriteByte(rest[j])
			continue
		}
		// Look ahead for the POSIX escape `'\''` (close, escape, reopen):
		// the apostrophe we just saw ends the current quote, then `\`
		// escapes the next `'`, then `'` reopens a new run.
		if j+3 < len(rest) && rest[j+1] == '\\' && rest[j+2] == '\'' && rest[j+3] == '\'' {
			b.WriteByte('\'')
			j += 3
			continue
		}
		// Plain closing quote — done.
		break
	}
	return b.String()
}

// extractPowerShellPath returns the first single-quoted run after `$p = `
// in the PowerShell script. It reverses PowerShellSingleQuote escaping so
// callers can assert on the round-tripped original path. The PowerShell
// escape rule is `'` → `”` (double the apostrophe inside a single-quoted
// run), so the parser treats any `”` as a single literal apostrophe.
func extractPowerShellPath(t *testing.T, script string) string {
	t.Helper()
	const marker = "$p = "
	i := strings.Index(script, marker)
	if i < 0 {
		t.Fatalf("powershell script missing %q marker:\n%s", marker, script)
	}
	rest := script[i+len(marker):]
	if !strings.HasPrefix(rest, "'") {
		t.Fatalf("powershell path not single-quoted: %q", rest)
	}
	raw := rest[1:]
	var b strings.Builder
	for j := 0; j < len(raw); j++ {
		if raw[j] != '\'' {
			b.WriteByte(raw[j])
			continue
		}
		// `''` is a PowerShell literal-apostrophe escape inside the run.
		if j+1 < len(raw) && raw[j+1] == '\'' {
			b.WriteByte('\'')
			j++
			continue
		}
		// Lone `'` closes the single-quoted run.
		return b.String()
	}
	t.Fatalf("powershell path has no closing quote: %q", raw)
	return ""
}

// extractPowerShellPayload returns the base64 payload that WriteRemoteBytes
// embedded inside the `$d = [Convert]::FromBase64String(<b64>)` argument of
// the PowerShell script. It walks the single-quoted run, treating `”` as a
// literal apostrophe escape, then base64-decodes the result.
func extractPowerShellPayload(t *testing.T, script string) []byte {
	t.Helper()
	const marker = "[Convert]::FromBase64String("
	i := strings.Index(script, marker)
	if i < 0 {
		t.Fatalf("powershell script missing %q marker:\n%s", marker, script)
	}
	rest := script[i+len(marker):]
	if !strings.HasPrefix(rest, "'") {
		t.Fatalf("powershell payload not single-quoted: %q", rest)
	}
	raw := rest[1:]
	var b strings.Builder
	for j := 0; j < len(raw); j++ {
		if raw[j] != '\'' {
			b.WriteByte(raw[j])
			continue
		}
		if j+1 < len(raw) && raw[j+1] == '\'' {
			b.WriteByte('\'')
			j++
			continue
		}
		out, err := base64.StdEncoding.DecodeString(b.String())
		if err != nil {
			t.Fatalf("powershell payload base64 decode failed: %v\ndecoded=%q", err, b.String())
		}
		return out
	}
	t.Fatalf("powershell payload has no closing quote: %q", raw)
	return nil
}

// TestWriteRemoteBytes_Posix covers the Linux/Darwin branch: WriteRemoteBytes
// must base64-encode the input, build a `set -e; mkdir -p; printf | base64 -d`
// script, and decode back to the exact input. Path strings with apostrophes
// exercise PosixSingleQuote in the embedded arg.
func TestWriteRemoteBytes_Posix(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		path string
		data []byte
	}{
		{"simple path and text", "/tmp/hello.txt", []byte("hello world\n")},
		{"path with spaces", "/tmp/has space/file.txt", []byte("x = 1\n")},
		{"path with apostrophe", "/tmp/it's/file.txt", []byte("payload")},
		{"empty payload", "/tmp/empty.txt", []byte{}},
		{"binary payload with nulls", "/tmp/bin.dat", []byte{0x00, 0xff, 0x7f, 0x00, 0x01, 0xfe}},
		{"path under nested dir", "/var/lib/gh-sr/runners/r1/config.json", []byte(`{"k":"v"}`)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			rec := &recordingExecutor{}
			h := newLocalMockHost(t, "linux", rec)
			if err := WriteRemoteBytes(h, tc.path, tc.data); err != nil {
				t.Fatalf("WriteRemoteBytes: %v", err)
			}
			if got := len(rec.runs); got != 1 {
				t.Fatalf("expected 1 Run call, got %d", got)
			}
			script := rec.runs[0]
			// Script shape contract: set -e; dirname with single-quoted path;
			// mkdir -p; printf | base64 -d > <path>.
			for _, sub := range []string{
				"set -e",
				"mkdir -p",
				"base64 -d",
			} {
				if !strings.Contains(script, sub) {
					t.Errorf("posix script missing substring %q in:\n%s", sub, script)
				}
			}
			if got := extractPosixPath(t, script); got != tc.path {
				t.Errorf("posix path round-trip mismatch: got %q want %q", got, tc.path)
			}
			if got := extractPosixPayload(t, script); !bytesEqual(got, tc.data) {
				t.Errorf("posix payload round-trip mismatch: got %x want %x", got, tc.data)
			}
		})
	}
}

// TestWriteRemoteBytes_Windows covers the Windows branch: WriteRemoteBytes
// must emit a PowerShell script that creates the parent directory and
// base64-decodes the payload. The script uses PowerShellSingleQuote (').
func TestWriteRemoteBytes_Windows(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		path string
		data []byte
	}{
		{"simple path and text", `C:\temp\hello.txt`, []byte("hello world\n")},
		{"path with apostrophe", `C:\Users\O'Brien\file.txt`, []byte("payload")},
		{"empty payload", `C:\empty.txt`, []byte{}},
		{"binary payload with nulls", `C:\bin.dat`, []byte{0x00, 0xff, 0x7f, 0x00, 0x01, 0xfe}},
		{"path with spaces and parens", `C:\Program Files\gh-sr\config.yml`, []byte("k=v\n")},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			rec := &recordingExecutor{}
			h := newLocalMockHost(t, "windows", rec)
			if err := WriteRemoteBytes(h, tc.path, tc.data); err != nil {
				t.Fatalf("WriteRemoteBytes: %v", err)
			}
			if got := len(rec.runs); got != 1 {
				t.Fatalf("expected 1 Run call, got %d", got)
			}
			script := rec.runs[0]
			// PowerShell script contract: New-Item -ItemType Directory for the
			// parent, [Convert]::FromBase64String, [IO.File]::WriteAllBytes.
			for _, sub := range []string{
				"Split-Path -Parent",
				"New-Item -ItemType Directory -Force -Path",
				"[Convert]::FromBase64String(",
				"[IO.File]::WriteAllBytes(",
			} {
				if !strings.Contains(script, sub) {
					t.Errorf("powershell script missing substring %q in:\n%s", sub, script)
				}
			}
			if got := extractPowerShellPath(t, script); got != tc.path {
				t.Errorf("powershell path round-trip mismatch: got %q want %q", got, tc.path)
			}
			if got := extractPowerShellPayload(t, script); !bytesEqual(got, tc.data) {
				t.Errorf("powershell payload round-trip mismatch: got %x want %x", got, tc.data)
			}
		})
	}
}

// TestWriteRemoteBytes_ErrorPropagates covers error propagation from the
// underlying executor: a non-nil error from Run must surface unchanged.
func TestWriteRemoteBytes_ErrorPropagates(t *testing.T) {
	t.Parallel()
	sentinel := errors.New("executor boom")
	cases := []struct {
		name string
		os   string
	}{
		{"posix executor error", "linux"},
		{"windows executor error", "windows"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			rec := &recordingExecutor{runErr: sentinel}
			h := newLocalMockHost(t, tc.os, rec)
			err := WriteRemoteBytes(h, "/tmp/x", []byte("y"))
			if err == nil {
				t.Fatal("expected error from executor, got nil")
			}
			if !strings.Contains(err.Error(), sentinel.Error()) {
				t.Errorf("expected error to wrap sentinel %q, got %q", sentinel.Error(), err.Error())
			}
		})
	}
}

// TestWriteRemoteBytes_DoesNotUpload is a regression guard: WriteRemoteBytes
// streams bytes through a shell pipeline, so it must never call Upload.
// (Upload is for files that already exist locally; piping bytes through
// stdin avoids creating a local tempfile just to transfer it.)
func TestWriteRemoteBytes_DoesNotUpload(t *testing.T) {
	t.Parallel()
	for _, osKind := range []string{"linux", "windows"} {
		t.Run(osKind, func(t *testing.T) {
			t.Parallel()
			rec := &recordingExecutor{}
			h := newLocalMockHost(t, osKind, rec)
			if err := WriteRemoteBytes(h, "/tmp/x", []byte("y")); err != nil {
				t.Fatalf("WriteRemoteBytes: %v", err)
			}
			if rec.uploadC != 0 {
				t.Errorf("expected 0 Upload calls, got %d", rec.uploadC)
			}
		})
	}
}

func bytesEqual(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

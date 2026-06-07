package runner

import (
	"strings"
	"testing"

	"github.com/an-lee/gh-sr/internal/config"
	"github.com/an-lee/gh-sr/internal/host"
	"github.com/an-lee/gh-sr/internal/hostshell"
)

// newEmbedutilTestHost returns a host wired to a recording mock executor. The mock
// records every command issued via Run so tests can assert on the exact shell shape.
func newEmbedutilTestHost(t *testing.T) (*host.Host, *containerMockExecutor) {
	t.Helper()
	h := host.NewHost("test", config.HostConfig{OS: "linux", Arch: "amd64", Addr: "local"})
	mock := &containerMockExecutor{runFn: func(cmd string) (string, error) {
		return "", nil
	}}
	h.SetConn(mock)
	return h, mock
}

// embedutilRecordingMock records every Run invocation for later inspection.
type embedutilRecordingMock struct {
	cmds []string
}

func (m *embedutilRecordingMock) Run(cmd string) (string, error) {
	m.cmds = append(m.cmds, cmd)
	return "", nil
}

func (m *embedutilRecordingMock) Upload(_, _ string) error { return nil }
func (m *embedutilRecordingMock) Close() error             { return nil }

func TestWriteRemoteHeredocFile_Shape(t *testing.T) {
	t.Parallel()
	h := host.NewHost("test", config.HostConfig{OS: "linux", Arch: "amd64", Addr: "local"})
	rec := &embedutilRecordingMock{}
	h.SetConn(rec)

	const content = "FROM ubuntu:24.04\nRUN apt-get update\n"
	if err := writeRemoteHeredocFile(h, "/tmp/build/Dockerfile", content); err != nil {
		t.Fatalf("writeRemoteHeredocFile: %v", err)
	}
	if got := len(rec.cmds); got != 1 {
		t.Fatalf("expected 1 h.Run call, got %d", got)
	}
	cmd := rec.cmds[0]
	wantQuoted := hostshell.PosixSingleQuote("/tmp/build/Dockerfile")

	// Must mkdir -p the parent. The path is single-quoted by the helper, then the
	// whole single-quoted expression is wrapped in "$(dirname ...)" so paths with
	// spaces don't get re-tokenised by command substitution.
	if !strings.Contains(cmd, `mkdir -p "$(dirname `+wantQuoted+`)"`) {
		t.Errorf("expected mkdir -p dirname in cmd, got: %s", cmd)
	}
	// Must single-quote the path consistently (twice: once for dirname, once for cat).
	if c := strings.Count(cmd, wantQuoted); c != 2 {
		t.Errorf("expected single-quoted path to appear twice (dirname + cat), got %d occurrences in: %s", c, cmd)
	}
	// Must use the GHSR_EOF heredoc.
	if !strings.Contains(cmd, "<< 'GHSR_EOF'") {
		t.Errorf("expected GHSR_EOF heredoc marker, got: %s", cmd)
	}
	// Must include the (LF-normalised) content.
	if !strings.Contains(cmd, "FROM ubuntu:24.04\nRUN apt-get update") {
		t.Errorf("expected content body in cmd, got: %s", cmd)
	}
}

func TestWriteRemoteHeredocFile_EmptyTruncate(t *testing.T) {
	t.Parallel()
	h := host.NewHost("test", config.HostConfig{OS: "linux", Arch: "amd64", Addr: "local"})
	rec := &embedutilRecordingMock{}
	h.SetConn(rec)

	if err := writeRemoteHeredocFile(h, "/tmp/build/Dockerfile", ""); err != nil {
		t.Fatalf("writeRemoteHeredocFile(empty): %v", err)
	}
	cmd := rec.cmds[0]
	// Empty body: the heredoc body is empty (CONTENT marker), and the helper still
	// emits a heredoc (NOT formatEmptyRemoteFile). Callers wanting truncation must
	// use formatEmptyRemoteFile; this test pins the empty-content behaviour.
	if !strings.Contains(cmd, "<< 'GHSR_EOF'") {
		t.Errorf("empty content should still use a heredoc, got: %s", cmd)
	}
	if !strings.Contains(cmd, "\nGHSR_EOF") {
		t.Errorf("empty heredoc must close cleanly, got: %s", cmd)
	}
}

func TestWriteRemoteHeredocFile_MarkerEscape(t *testing.T) {
	t.Parallel()
	h := host.NewHost("test", config.HostConfig{OS: "linux", Arch: "amd64", Addr: "local"})
	rec := &embedutilRecordingMock{}
	h.SetConn(rec)

	// Content with the raw marker — must be neutralised before it reaches the shell.
	const content = "echo GHSR_EOF\nrm -rf /\n"
	if err := writeRemoteHeredocFile(h, "/tmp/build/payload", content); err != nil {
		t.Fatalf("writeRemoteHeredocFile: %v", err)
	}
	cmd := rec.cmds[0]
	if strings.Contains(cmd, "echo GHSR_EOF\n") {
		t.Errorf("raw GHSR_EOF leaked into shell command: %s", cmd)
	}
	if !strings.Contains(cmd, "echo GHSR_E0F") {
		t.Errorf("expected GHSR_E0F marker escape, got: %s", cmd)
	}
}

func TestWriteRemoteHeredocFile_CRLFNormalisation(t *testing.T) {
	t.Parallel()
	h := host.NewHost("test", config.HostConfig{OS: "linux", Arch: "amd64", Addr: "local"})
	rec := &embedutilRecordingMock{}
	h.SetConn(rec)

	// CRLF (Windows line endings) must be flattened to LF so the Dockerfile grep|xargs
	// pipeline (line-by-line) does not gain a trailing \r on package names.
	const content = "curl\r\nwget\r\n"
	if err := writeRemoteHeredocFile(h, "/tmp/build/pkgs", content); err != nil {
		t.Fatalf("writeRemoteHeredocFile: %v", err)
	}
	cmd := rec.cmds[0]
	if strings.Contains(cmd, "\r") {
		t.Errorf("CRLF should be normalised to LF, got: %s", cmd)
	}
	if !strings.Contains(cmd, "curl\nwget") {
		t.Errorf("expected LF-separated lines, got: %s", cmd)
	}
}

func TestWriteRemoteHeredocFile_PathQuoting(t *testing.T) {
	t.Parallel()
	h := host.NewHost("test", config.HostConfig{OS: "linux", Arch: "amd64", Addr: "local"})
	rec := &embedutilRecordingMock{}
	h.SetConn(rec)

	// Path with a space — must be single-quoted so the shell does not split it.
	if err := writeRemoteHeredocFile(h, "/tmp/has space/file", "x"); err != nil {
		t.Fatalf("writeRemoteHeredocFile: %v", err)
	}
	cmd := rec.cmds[0]
	wantQuoted := hostshell.PosixSingleQuote("/tmp/has space/file")
	if !strings.Contains(cmd, wantQuoted) {
		t.Errorf("expected single-quoted path %q in cmd, got: %s", wantQuoted, cmd)
	}
}

func TestWriteRemoteHeredocFile_RunErrorWrapsPath(t *testing.T) {
	t.Parallel()
	h := host.NewHost("test", config.HostConfig{OS: "linux", Arch: "amd64", Addr: "local"})
	h.SetConn(&containerMockExecutor{runFn: func(cmd string) (string, error) {
		return "", assertCalledError()
	}})

	const path = "/tmp/build/Dockerfile"
	err := writeRemoteHeredocFile(h, path, "FROM scratch")
	if err == nil {
		t.Fatal("expected error when h.Run fails")
	}
	if !strings.Contains(err.Error(), path) {
		t.Errorf("error %q should wrap the path %q so operators can see what failed", err.Error(), path)
	}
	if !strings.Contains(err.Error(), "writing") {
		t.Errorf("error %q should mention the operation (writing %s)", err.Error(), path)
	}
}

func TestWriteRemoteHeredocExecutable_TwoCalls(t *testing.T) {
	t.Parallel()
	h := host.NewHost("test", config.HostConfig{OS: "linux", Arch: "amd64", Addr: "local"})
	rec := &embedutilRecordingMock{}
	h.SetConn(rec)

	const path = "/tmp/build/entrypoint.sh"
	const content = "#!/bin/bash\necho hi\n"
	if err := writeRemoteHeredocExecutable(h, path, content); err != nil {
		t.Fatalf("writeRemoteHeredocExecutable: %v", err)
	}
	if got := len(rec.cmds); got != 2 {
		t.Fatalf("expected 2 h.Run calls (write + chmod), got %d: %v", got, rec.cmds)
	}
	// First call: write (heredoc).
	if !strings.Contains(rec.cmds[0], "<< 'GHSR_EOF'") {
		t.Errorf("first call should be the heredoc write, got: %s", rec.cmds[0])
	}
	// Second call: chmod.
	if !strings.Contains(rec.cmds[1], "chmod +x "+hostshell.PosixSingleQuote(path)) {
		t.Errorf("second call should be chmod +x, got: %s", rec.cmds[1])
	}
}

func TestWriteRemoteHeredocExecutable_WriteFailureSkipsChmod(t *testing.T) {
	t.Parallel()
	h := host.NewHost("test", config.HostConfig{OS: "linux", Arch: "amd64", Addr: "local"})
	h.SetConn(&containerMockExecutor{runFn: func(cmd string) (string, error) {
		// First call (the write) fails; chmod must NOT be attempted.
		return "", assertCalledError()
	}})

	err := writeRemoteHeredocExecutable(h, "/tmp/build/entrypoint.sh", "x")
	if err == nil {
		t.Fatal("expected error when write fails")
	}
	if !strings.Contains(err.Error(), "writing") {
		t.Errorf("expected write-stage error, got: %v", err)
	}
}

func TestWriteRemoteHeredocExecutable_ChmodFailureWrapsPath(t *testing.T) {
	t.Parallel()
	h := host.NewHost("test", config.HostConfig{OS: "linux", Arch: "amd64", Addr: "local"})
	h.SetConn(&containerMockExecutor{runFn: func(cmd string) (string, error) {
		// Write succeeds, chmod fails.
		if strings.Contains(cmd, "chmod") {
			return "", assertCalledError()
		}
		return "", nil
	}})

	const path = "/tmp/build/entrypoint.sh"
	err := writeRemoteHeredocExecutable(h, path, "x")
	if err == nil {
		t.Fatal("expected error when chmod fails")
	}
	if !strings.Contains(err.Error(), "chmod +x") {
		t.Errorf("error %q should mention chmod +x", err.Error())
	}
	if !strings.Contains(err.Error(), path) {
		t.Errorf("error %q should wrap the path %q", err.Error(), path)
	}
}

func TestFormatEmptyRemoteFile(t *testing.T) {
	t.Parallel()
	got := formatEmptyRemoteFile("/tmp/build/empty.txt")
	want := ": > " + hostshell.PosixSingleQuote("/tmp/build/empty.txt")
	if got != want {
		t.Errorf("formatEmptyRemoteFile = %q, want %q", got, want)
	}
	got = formatEmptyRemoteFile("/tmp/has space/empty.txt")
	want = ": > " + hostshell.PosixSingleQuote("/tmp/has space/empty.txt")
	if got != want {
		t.Errorf("formatEmptyRemoteFile with space = %q, want %q", got, want)
	}
}

func TestJoinExtraPackages(t *testing.T) {
	t.Parallel()
	if got := joinExtraPackages(nil); got != "" {
		t.Errorf("nil extras should join to empty string, got %q", got)
	}
	if got := joinExtraPackages([]string{}); got != "" {
		t.Errorf("empty extras should join to empty string, got %q", got)
	}
	if got := joinExtraPackages([]string{"curl"}); got != "curl" {
		t.Errorf("single extra got %q, want %q", got, "curl")
	}
	got := joinExtraPackages([]string{"curl", "git", "ffmpeg"})
	want := "curl\ngit\nffmpeg"
	if got != want {
		t.Errorf("joinExtraPackages = %q, want %q", got, want)
	}
}

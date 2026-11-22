package host

import (
	"crypto/ed25519"
	"crypto/rand"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"

	"github.com/an-lee/gh-sr/internal/config"
	"github.com/an-lee/gh-sr/internal/testutil"
)

// dialTestServer calls NewConnection against the in-process server and fails
// the test if the dial does not succeed.
func dialTestServer(t *testing.T, s *testutil.SSHServer) *Connection {
	t.Helper()
	conn, err := NewConnection("root", s.Addr())
	if err != nil {
		t.Fatalf("NewConnection(%s): %v", s.Addr(), err)
	}
	t.Cleanup(func() { conn.Close() })
	return conn
}

func TestNewConnectionDial(t *testing.T) {
	t.Run("succeeds with a key and no known_hosts", func(t *testing.T) {
		testutil.InstallClientKey(t)
		srv := testutil.StartSSHServer(t, func(cmd string, ch ssh.Channel) testutil.ExecResult {
			return testutil.ExecResult{Status: 0}
		})
		conn := dialTestServer(t, srv)
		if got := conn.Close(); got != nil {
			t.Fatalf("Close() = %v, want nil", got)
		}
	})

	t.Run("succeeds when known_hosts pins the presented host key", func(t *testing.T) {
		testutil.InstallClientKey(t)
		srv := testutil.StartSSHServer(t, func(cmd string, ch ssh.Channel) testutil.ExecResult {
			return testutil.ExecResult{Status: 0}
		})
		if err := os.MkdirAll(filepath.Join(os.Getenv("HOME"), ".ssh"), 0o700); err != nil {
			t.Fatal(err)
		}
		line := knownhosts.Line([]string{testutil.BracketedHostPattern(srv.Addr())}, srv.HostPublicKey())
		if err := os.WriteFile(filepath.Join(os.Getenv("HOME"), ".ssh", "known_hosts"), []byte(line+"\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		conn, err := NewConnection("root", srv.Addr())
		if err != nil {
			t.Fatalf("NewConnection with pinned matching host key: %v", err)
		}
		if err := conn.Close(); err != nil {
			t.Fatalf("Close() = %v, want nil", err)
		}
	})

	t.Run("host key mismatch appends the ssh-keygen -R hint", func(t *testing.T) {
		testutil.InstallClientKey(t)
		srv := testutil.StartSSHServer(t, func(cmd string, ch ssh.Channel) testutil.ExecResult {
			return testutil.ExecResult{Status: 0}
		})
		// Pin a different ed25519 key for the server's [host]:port pattern.
		otherPub, _, err := ed25519.GenerateKey(rand.Reader)
		if err != nil {
			t.Fatal(err)
		}
		sshPub, err := ssh.NewPublicKey(otherPub)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(filepath.Join(os.Getenv("HOME"), ".ssh"), 0o700); err != nil {
			t.Fatal(err)
		}
		line := knownhosts.Line([]string{testutil.BracketedHostPattern(srv.Addr())}, sshPub)
		if err := os.WriteFile(filepath.Join(os.Getenv("HOME"), ".ssh", "known_hosts"), []byte(line+"\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		_, err = NewConnection("root", srv.Addr())
		if err == nil {
			t.Fatal("NewConnection with mismatched pin = nil error, want dial failure")
		}
		msg := err.Error()
		host, _, _ := net.SplitHostPort(srv.Addr())
		if !strings.Contains(msg, "SSH dial root@"+srv.Addr()) {
			t.Fatalf("error %q missing SSH dial prefix", msg)
		}
		if !strings.Contains(msg, "ssh-keygen -R "+host) {
			t.Fatalf("error %q missing ssh-keygen -R hint", msg)
		}
	})

	t.Run("unknown host key appends the plain-ssh pin hint", func(t *testing.T) {
		testutil.InstallClientKey(t)
		srv := testutil.StartSSHServer(t, func(cmd string, ch ssh.Channel) testutil.ExecResult {
			return testutil.ExecResult{Status: 0}
		})
		// known_hosts exists but pins only an unrelated host.
		otherPub, _, err := ed25519.GenerateKey(rand.Reader)
		if err != nil {
			t.Fatal(err)
		}
		sshPub, err := ssh.NewPublicKey(otherPub)
		if err != nil {
			t.Fatal(err)
		}
		khDir := filepath.Join(os.Getenv("HOME"), ".ssh")
		if err := os.MkdirAll(khDir, 0o700); err != nil {
			t.Fatal(err)
		}
		line := knownhosts.Line([]string{"[10.9.8.7]:22"}, sshPub)
		if err := os.WriteFile(filepath.Join(khDir, "known_hosts"), []byte(line+"\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		_, err = NewConnection("root", srv.Addr())
		if err == nil {
			t.Fatal("NewConnection with unpinned host = nil error, want dial failure")
		}
		msg := err.Error()
		if !strings.Contains(msg, "SSH dial root@"+srv.Addr()) {
			t.Fatalf("error %q missing SSH dial prefix", msg)
		}
		if !strings.Contains(msg, "is not in ~/.ssh/known_hosts") {
			t.Fatalf("error %q missing unknown-key hint", msg)
		}
	})

	t.Run("refused dial reports the SSH dial error without a hint", func(t *testing.T) {
		testutil.InstallClientKey(t)
		// Reserve a port, close the listener, then dial the now-dead port.
		ln, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		addr := ln.Addr().String()
		ln.Close()
		_, err = NewConnection("root", addr)
		if err == nil {
			t.Fatal("NewConnection to closed port = nil error, want dial failure")
		}
		msg := err.Error()
		if !strings.Contains(msg, "SSH dial root@"+addr) {
			t.Fatalf("error %q missing SSH dial prefix", msg)
		}
		if strings.Contains(msg, "known_hosts") {
			t.Fatalf("error %q unexpectedly carries a known_hosts hint", msg)
		}
	})
}

func TestConnectionRun(t *testing.T) {
	t.Run("captures stdout and trims it", func(t *testing.T) {
		testutil.InstallClientKey(t)
		srv := testutil.StartSSHServer(t, func(cmd string, ch ssh.Channel) testutil.ExecResult {
			return testutil.ExecResult{Stdout: "  hello gh-sr  \n", Status: 0}
		})
		conn := dialTestServer(t, srv)
		got, err := conn.Run("uname -a")
		if err != nil {
			t.Fatalf("Run(uname -a): %v", err)
		}
		if got != "hello gh-sr" {
			t.Fatalf("Run() = %q, want %q", got, "hello gh-sr")
		}
		if served := srv.ServedCommands(); len(served) != 1 || served[0] != "uname -a" {
			t.Fatalf("server received commands %v, want [uname -a]", served)
		}
	})

	t.Run("non-zero exit wraps the command and captured stderr", func(t *testing.T) {
		testutil.InstallClientKey(t)
		srv := testutil.StartSSHServer(t, func(cmd string, ch ssh.Channel) testutil.ExecResult {
			return testutil.ExecResult{Stdout: "partial output", Stderr: "boom: disk full", Status: 1}
		})
		conn := dialTestServer(t, srv)
		got, err := conn.Run("df -h")
		if err == nil {
			t.Fatalf("Run(df -h) = %q, want an error on exit status 1", got)
		}
		msg := err.Error()
		if !strings.Contains(msg, `running "df -h"`) {
			t.Fatalf("error %q missing wrapped command", msg)
		}
		if !strings.Contains(msg, "boom: disk full") {
			t.Fatalf("error %q missing captured stderr", msg)
		}
	})

	t.Run("session open failure surfaces as creating-SSH-session", func(t *testing.T) {
		testutil.InstallClientKey(t)
		srv := testutil.StartSSHServer(t, func(cmd string, ch ssh.Channel) testutil.ExecResult {
			return testutil.ExecResult{Status: 0}
		})
		srv.SetRejectSessions(true)
		conn := dialTestServer(t, srv)
		if _, err := conn.Run("true"); err == nil || !strings.Contains(err.Error(), "creating SSH session") {
			t.Fatalf("Run with rejected sessions = %v, want creating-SSH-session error", err)
		}
	})
}

func TestConnectionUpload(t *testing.T) {
	writeSource := func(t *testing.T, content string) string {
		t.Helper()
		path := filepath.Join(t.TempDir(), "payload.bin")
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
		return path
	}

	t.Run("speaks scp framing and lands the content", func(t *testing.T) {
		testutil.InstallClientKey(t)
		srv := testutil.StartSSHServer(t, func(cmd string, ch ssh.Channel) testutil.ExecResult {
			return testutil.ExecResult{Status: 0}
		})
		conn := dialTestServer(t, srv)
		src := writeSource(t, "binary payload \x00 bytes\n")
		if err := conn.Upload(src, "/opt/bin/gh-sr-runner"); err != nil {
			t.Fatalf("Upload: %v", err)
		}
		uploads := srv.ServedUploads()
		if len(uploads) != 1 {
			t.Fatalf("server received %d uploads, want 1", len(uploads))
		}
		up := uploads[0]
		if up.Perms != "C0644" {
			t.Fatalf("scp perms = %q, want C0644", up.Perms)
		}
		if up.Name != "gh-sr-runner" {
			t.Fatalf("scp name = %q, want gh-sr-runner", up.Name)
		}
		want, _ := os.ReadFile(src)
		if string(up.Content) != string(want) {
			t.Fatalf("scp content = %q, want %q", up.Content, want)
		}
		if !up.TrailingNUL {
			t.Fatal("scp transfer missing the trailing NUL sentinel")
		}
		if served := srv.ServedCommands(); len(served) != 1 || served[0] != "scp -t '/opt/bin'" {
			t.Fatalf("server received commands %v, want [scp -t '/opt/bin']", served)
		}
	})

	t.Run("escapes apostrophes in the remote directory", func(t *testing.T) {
		testutil.InstallClientKey(t)
		srv := testutil.StartSSHServer(t, func(cmd string, ch ssh.Channel) testutil.ExecResult {
			return testutil.ExecResult{Status: 0}
		})
		conn := dialTestServer(t, srv)
		src := writeSource(t, "x")
		if err := conn.Upload(src, "/tmp/runner's bin/gh-sr"); err != nil {
			t.Fatalf("Upload: %v", err)
		}
		served := srv.ServedCommands()
		if len(served) != 1 || served[0] != `scp -t '/tmp/runner'\''s bin'` {
			t.Fatalf("server received commands %v, want [scp -t '/tmp/runner'\\'''s bin']", served)
		}
	})

	t.Run("missing local file fails before running any remote command", func(t *testing.T) {
		testutil.InstallClientKey(t)
		srv := testutil.StartSSHServer(t, func(cmd string, ch ssh.Channel) testutil.ExecResult {
			return testutil.ExecResult{Status: 0}
		})
		conn := dialTestServer(t, srv)
		err := conn.Upload(filepath.Join(t.TempDir(), "absent"), "/remote/x")
		if err == nil || !strings.Contains(err.Error(), "opening local file") {
			t.Fatalf("Upload(absent) = %v, want open-local-file error", err)
		}
		// Upload opens the SSH session before it opens the local file, so the
		// server is contacted even though no transfer ever starts.
		srv.WaitForSessionOpen(t)
		if opened := srv.SessionOpenCount(); opened != 1 {
			t.Fatalf("server opened %d sessions, want 1", opened)
		}
		if served := srv.ServedCommands(); len(served) != 0 {
			t.Fatalf("server received commands %v, want none", served)
		}
	})

	t.Run("session open failure surfaces as creating-SSH-session-for-upload", func(t *testing.T) {
		testutil.InstallClientKey(t)
		srv := testutil.StartSSHServer(t, func(cmd string, ch ssh.Channel) testutil.ExecResult {
			return testutil.ExecResult{Status: 0}
		})
		srv.SetRejectSessions(true)
		conn := dialTestServer(t, srv)
		src := writeSource(t, "x")
		err := conn.Upload(src, "/remote/x")
		if err == nil || !strings.Contains(err.Error(), "creating SSH session for upload") {
			t.Fatalf("Upload with rejected sessions = %v, want creating-SSH-session-for-upload error", err)
		}
	})
}

func TestConnectionCloseWithoutClient(t *testing.T) {
	if err := (&Connection{}).Close(); err != nil {
		t.Fatalf("Close() on zero-value Connection = %v, want nil", err)
	}
}

// deadAddr reserves a port, closes the listener, and returns the address so a
// dial to it is deterministically refused.
func deadAddr(t *testing.T) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().String()
	ln.Close()
	return addr
}

// TestHostConnect exercises Host.Connect's SSH branch — the path every
// orchestrator takes through ops.ConnectHost on a non-local host.
func TestHostConnect(t *testing.T) {
	t.Run("dials the SSH server and Run executes through it", func(t *testing.T) {
		testutil.InstallClientKey(t)
		srv := testutil.StartSSHServer(t, func(cmd string, ch ssh.Channel) testutil.ExecResult {
			return testutil.ExecResult{Stdout: "Linux srv\n", Status: 0}
		})
		h := NewHost("srv", config.HostConfig{Addr: "root@" + srv.Addr()})
		if err := h.Connect(); err != nil {
			t.Fatalf("Connect(): %v", err)
		}
		got, err := h.Run("uname")
		if err != nil {
			t.Fatalf("Run(uname): %v", err)
		}
		if got != "Linux srv" {
			t.Fatalf("Run(uname) = %q, want %q", got, "Linux srv")
		}
		if served := srv.ServedCommands(); len(served) != 1 || served[0] != "uname" {
			t.Fatalf("server received commands %v, want [uname]", served)
		}
		if err := h.Close(); err != nil {
			t.Fatalf("Close(): %v", err)
		}
	})

	t.Run("Run wraps the connect failure with the host name and addr", func(t *testing.T) {
		testutil.InstallClientKey(t)
		addr := deadAddr(t)
		h := NewHost("deadhost", config.HostConfig{Addr: "root@" + addr})
		_, err := h.Run("true")
		if err == nil {
			t.Fatal("Run on unreachable host = nil error, want connect failure")
		}
		// "connecting to <host>" wraps every NewConnection failure, so only
		// the SSH-dial marker proves the failure happened at dial time and
		// not earlier (e.g. no auth methods available) — same strength as
		// the refused-dial subtests above.
		msg := err.Error()
		if !strings.Contains(msg, "SSH dial root@"+addr) {
			t.Fatalf("error %q missing SSH dial prefix", msg)
		}
		if !strings.Contains(msg, "connecting to deadhost") {
			t.Fatalf("error %q missing host name", msg)
		}
		if !strings.Contains(msg, addr) {
			t.Fatalf("error %q missing addr %s", msg, addr)
		}
	})
}

func TestReadScpSink(t *testing.T) {
	t.Run("parses a well-formed frame", func(t *testing.T) {
		ch := testutil.NewFakeSSHChannel("C0644 5 hello\nabcde\x00")
		upload, err := testutil.ReadScpSink(ch)
		if err != nil {
			t.Fatalf("ReadScpSink: %v", err)
		}
		if upload.Perms != "C0644" || upload.Name != "hello" || upload.Size != 5 {
			t.Fatalf("upload = %+v, want C0644/hello/5", upload)
		}
		if string(upload.Content) != "abcde" {
			t.Fatalf("upload content = %q, want %q", upload.Content, "abcde")
		}
		if !upload.TrailingNUL {
			t.Fatal("upload missing the trailing NUL sentinel")
		}
	})

	// A malformed frame must surface as an error, not an index-out-of-range
	// panic that would take the whole test binary down with it.
	for _, tc := range []struct {
		name    string
		frame   string
		wantErr string
	}{
		{name: "header with no size or name", frame: "C0644\n", wantErr: "malformed header"},
		{name: "header with no name", frame: "C0644 5\n", wantErr: "malformed header"},
		{name: "size is not a number", frame: "C0644 abc hello\n", wantErr: "malformed size"},
		{name: "size is negative", frame: "C0644 -1 hello\n", wantErr: "negative size"},
		{name: "size exceeds the frame limit", frame: "C0644 134217729 hello\n", wantErr: "exceeds the"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ch := testutil.NewFakeSSHChannel(tc.frame)
			upload, err := testutil.ReadScpSink(ch)
			if err == nil {
				t.Fatalf("ReadScpSink(%q) = %+v, want an error", tc.frame, upload)
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("ReadScpSink(%q) error = %v, want it to mention %q", tc.frame, err, tc.wantErr)
			}
		})
	}
}

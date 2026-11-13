package host

import (
	"bufio"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/pem"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

// execOutcome is what the fake remote command produces for one exec request.
type execOutcome struct {
	stdout string
	stderr string
	status uint32
}

// scpUpload records one completed scp sink transfer.
type scpUpload struct {
	perms       string
	name        string
	size        int64
	content     []byte
	trailingNUL bool
}

// testSSHServer is a minimal in-process SSH server: publickey auth is always
// accepted, every exec command is recorded, and each session is delegated to
// the caller-supplied handler. It exercises the real client path in
// connection.go (dial, handshake, exec framing, scp sink) without an sshd.
type testSSHServer struct {
	t              *testing.T
	ln             net.Listener
	cfg            *ssh.ServerConfig
	handle         func(cmd string, ch ssh.Channel) execOutcome
	rejectSessions bool
	hostPub        ssh.PublicKey

	mu       sync.Mutex
	commands []string
	uploads  []scpUpload
}

func startTestSSHServer(t *testing.T, handle func(cmd string, ch ssh.Channel) execOutcome) *testSSHServer {
	t.Helper()
	_, hostKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	signer, err := ssh.NewSignerFromKey(hostKey)
	if err != nil {
		t.Fatal(err)
	}
	cfg := &ssh.ServerConfig{
		PublicKeyCallback: func(ssh.ConnMetadata, ssh.PublicKey) (*ssh.Permissions, error) { return nil, nil },
	}
	cfg.AddHostKey(signer)
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	s := &testSSHServer{
		t:       t,
		ln:      ln,
		cfg:     cfg,
		handle:  handle,
		hostPub: signer.PublicKey(),
	}
	go s.serve()
	t.Cleanup(func() { ln.Close() })
	return s
}

func (s *testSSHServer) addr() string { return s.ln.Addr().String() }

func (s *testSSHServer) servedCommands() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string(nil), s.commands...)
}

func (s *testSSHServer) servedUploads() []scpUpload {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]scpUpload(nil), s.uploads...)
}

func (s *testSSHServer) serve() {
	for {
		c, err := s.ln.Accept()
		if err != nil {
			return
		}
		go s.handleConn(c)
	}
}

func (s *testSSHServer) handleConn(c net.Conn) {
	srvConn, chans, reqs, err := ssh.NewServerConn(c, s.cfg)
	if err != nil {
		return
	}
	defer srvConn.Close()
	go ssh.DiscardRequests(reqs)
	for newCh := range chans {
		if newCh.ChannelType() != "session" {
			newCh.Reject(ssh.UnknownChannelType, "only session channels are supported")
			continue
		}
		if s.rejectSessions {
			newCh.Reject(ssh.Prohibited, "sessions disabled")
			continue
		}
		ch, chReqs, err := newCh.Accept()
		if err != nil {
			continue
		}
		go s.handleSession(ch, chReqs)
	}
}

func (s *testSSHServer) handleSession(ch ssh.Channel, reqs <-chan *ssh.Request) {
	defer ch.Close()
	for req := range reqs {
		if req.Type != "exec" {
			if req.WantReply {
				req.Reply(false, nil)
			}
			continue
		}
		var payload struct{ Command string }
		if err := ssh.Unmarshal(req.Payload, &payload); err != nil {
			if req.WantReply {
				req.Reply(false, nil)
			}
			return
		}
		s.mu.Lock()
		s.commands = append(s.commands, payload.Command)
		s.mu.Unlock()
		if req.WantReply {
			req.Reply(true, nil)
		}
		if strings.HasPrefix(payload.Command, "scp -t ") {
			s.handleScpSink(payload.Command, ch)
			return
		}
		out := s.handle(payload.Command, ch)
		if out.stdout != "" {
			fmt.Fprint(ch, out.stdout)
		}
		if out.stderr != "" {
			fmt.Fprint(ch.Stderr(), out.stderr)
		}
		ch.SendRequest("exit-status", false, ssh.Marshal(struct{ Status uint32 }{out.status}))
		return
	}
}

// handleScpSink consumes the scp sink protocol Upload speaks: a
// "C0644 <size> <name>\n" header, exactly <size> content bytes, and a
// trailing NUL.
func (s *testSSHServer) handleScpSink(cmd string, ch ssh.Channel) {
	r := bufio.NewReader(ch)
	header, err := r.ReadString('\n')
	if err != nil {
		fmt.Fprint(ch.Stderr(), "scp: reading header\n")
		ch.SendRequest("exit-status", false, ssh.Marshal(struct{ Status uint32 }{1}))
		return
	}
	fields := strings.Fields(header)
	size, _ := strconv.ParseInt(fields[1], 10, 64)
	content := make([]byte, size)
	if _, err := io.ReadFull(r, content); err != nil {
		fmt.Fprint(ch.Stderr(), "scp: short read\n")
		ch.SendRequest("exit-status", false, ssh.Marshal(struct{ Status uint32 }{1}))
		return
	}
	nul := make([]byte, 1)
	_, readErr := io.ReadFull(r, nul)
	s.mu.Lock()
	s.uploads = append(s.uploads, scpUpload{
		perms:       fields[0],
		name:        fields[2],
		size:        size,
		content:     content,
		trailingNUL: readErr == nil && nul[0] == 0,
	})
	s.mu.Unlock()
	ch.SendRequest("exit-status", false, ssh.Marshal(struct{ Status uint32 }{0}))
}

// installClientKey points HOME at a temp dir holding one usable ed25519 key
// under .ssh/id_ed25519 and clears SSH_AUTH_SOCK, so collectAuthMethods
// yields exactly that key.
func installClientKey(t *testing.T) {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("SSH_AUTH_SOCK", "")
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	block, err := ssh.MarshalPrivateKey(priv, "test")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(home, ".ssh"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, ".ssh", "id_ed25519"), pem.EncodeToMemory(block), 0o600); err != nil {
		t.Fatal(err)
	}
}

// bracketedHostPattern renders addr as the known_hosts pattern for a
// non-standard port: "[host]:port".
func bracketedHostPattern(addr string) string {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return addr
	}
	return "[" + host + "]:" + port
}

// dialTestServer calls NewConnection against the in-process server and fails
// the test if the dial does not succeed.
func dialTestServer(t *testing.T, s *testSSHServer) *Connection {
	t.Helper()
	conn, err := NewConnection("root", s.addr())
	if err != nil {
		t.Fatalf("NewConnection(%s): %v", s.addr(), err)
	}
	t.Cleanup(func() { conn.Close() })
	return conn
}

func TestNewConnectionDial(t *testing.T) {
	t.Run("succeeds with a key and no known_hosts", func(t *testing.T) {
		installClientKey(t)
		srv := startTestSSHServer(t, func(cmd string, ch ssh.Channel) execOutcome {
			return execOutcome{status: 0}
		})
		conn := dialTestServer(t, srv)
		if got := conn.Close(); got != nil {
			t.Fatalf("Close() = %v, want nil", got)
		}
	})

	t.Run("succeeds when known_hosts pins the presented host key", func(t *testing.T) {
		installClientKey(t)
		srv := startTestSSHServer(t, func(cmd string, ch ssh.Channel) execOutcome {
			return execOutcome{status: 0}
		})
		if err := os.MkdirAll(filepath.Join(os.Getenv("HOME"), ".ssh"), 0o700); err != nil {
			t.Fatal(err)
		}
		line := knownhosts.Line([]string{bracketedHostPattern(srv.addr())}, srv.hostPub)
		if err := os.WriteFile(filepath.Join(os.Getenv("HOME"), ".ssh", "known_hosts"), []byte(line+"\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		conn, err := NewConnection("root", srv.addr())
		if err != nil {
			t.Fatalf("NewConnection with pinned matching host key: %v", err)
		}
		if err := conn.Close(); err != nil {
			t.Fatalf("Close() = %v, want nil", err)
		}
	})

	t.Run("host key mismatch appends the ssh-keygen -R hint", func(t *testing.T) {
		installClientKey(t)
		srv := startTestSSHServer(t, func(cmd string, ch ssh.Channel) execOutcome {
			return execOutcome{status: 0}
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
		line := knownhosts.Line([]string{bracketedHostPattern(srv.addr())}, sshPub)
		if err := os.WriteFile(filepath.Join(os.Getenv("HOME"), ".ssh", "known_hosts"), []byte(line+"\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		_, err = NewConnection("root", srv.addr())
		if err == nil {
			t.Fatal("NewConnection with mismatched pin = nil error, want dial failure")
		}
		msg := err.Error()
		host, _, _ := net.SplitHostPort(srv.addr())
		if !strings.Contains(msg, "SSH dial root@"+srv.addr()) {
			t.Fatalf("error %q missing SSH dial prefix", msg)
		}
		if !strings.Contains(msg, "ssh-keygen -R "+host) {
			t.Fatalf("error %q missing ssh-keygen -R hint", msg)
		}
	})

	t.Run("unknown host key appends the plain-ssh pin hint", func(t *testing.T) {
		installClientKey(t)
		srv := startTestSSHServer(t, func(cmd string, ch ssh.Channel) execOutcome {
			return execOutcome{status: 0}
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
		_, err = NewConnection("root", srv.addr())
		if err == nil {
			t.Fatal("NewConnection with unpinned host = nil error, want dial failure")
		}
		msg := err.Error()
		if !strings.Contains(msg, "SSH dial root@"+srv.addr()) {
			t.Fatalf("error %q missing SSH dial prefix", msg)
		}
		if !strings.Contains(msg, "is not in ~/.ssh/known_hosts") {
			t.Fatalf("error %q missing unknown-key hint", msg)
		}
	})

	t.Run("refused dial reports the SSH dial error without a hint", func(t *testing.T) {
		installClientKey(t)
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
		installClientKey(t)
		srv := startTestSSHServer(t, func(cmd string, ch ssh.Channel) execOutcome {
			return execOutcome{stdout: "  hello gh-sr  \n", status: 0}
		})
		conn := dialTestServer(t, srv)
		got, err := conn.Run("uname -a")
		if err != nil {
			t.Fatalf("Run(uname -a): %v", err)
		}
		if got != "hello gh-sr" {
			t.Fatalf("Run() = %q, want %q", got, "hello gh-sr")
		}
		if served := srv.servedCommands(); len(served) != 1 || served[0] != "uname -a" {
			t.Fatalf("server received commands %v, want [uname -a]", served)
		}
	})

	t.Run("non-zero exit wraps the command and captured stderr", func(t *testing.T) {
		installClientKey(t)
		srv := startTestSSHServer(t, func(cmd string, ch ssh.Channel) execOutcome {
			return execOutcome{stdout: "partial output", stderr: "boom: disk full", status: 1}
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
		installClientKey(t)
		srv := startTestSSHServer(t, func(cmd string, ch ssh.Channel) execOutcome {
			return execOutcome{status: 0}
		})
		srv.rejectSessions = true
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
		installClientKey(t)
		srv := startTestSSHServer(t, func(cmd string, ch ssh.Channel) execOutcome {
			return execOutcome{status: 0}
		})
		conn := dialTestServer(t, srv)
		src := writeSource(t, "binary payload \x00 bytes\n")
		if err := conn.Upload(src, "/opt/bin/gh-sr-runner"); err != nil {
			t.Fatalf("Upload: %v", err)
		}
		uploads := srv.servedUploads()
		if len(uploads) != 1 {
			t.Fatalf("server received %d uploads, want 1", len(uploads))
		}
		up := uploads[0]
		if up.perms != "C0644" {
			t.Fatalf("scp perms = %q, want C0644", up.perms)
		}
		if up.name != "gh-sr-runner" {
			t.Fatalf("scp name = %q, want gh-sr-runner", up.name)
		}
		want, _ := os.ReadFile(src)
		if string(up.content) != string(want) {
			t.Fatalf("scp content = %q, want %q", up.content, want)
		}
		if !up.trailingNUL {
			t.Fatal("scp transfer missing the trailing NUL sentinel")
		}
		if served := srv.servedCommands(); len(served) != 1 || served[0] != "scp -t '/opt/bin'" {
			t.Fatalf("server received commands %v, want [scp -t '/opt/bin']", served)
		}
	})

	t.Run("escapes apostrophes in the remote directory", func(t *testing.T) {
		installClientKey(t)
		srv := startTestSSHServer(t, func(cmd string, ch ssh.Channel) execOutcome {
			return execOutcome{status: 0}
		})
		conn := dialTestServer(t, srv)
		src := writeSource(t, "x")
		if err := conn.Upload(src, "/tmp/runner's bin/gh-sr"); err != nil {
			t.Fatalf("Upload: %v", err)
		}
		served := srv.servedCommands()
		if len(served) != 1 || served[0] != `scp -t '/tmp/runner'\''s bin'` {
			t.Fatalf("server received commands %v, want [scp -t '/tmp/runner'\\'''s bin']", served)
		}
	})

	t.Run("missing local file fails without contacting the server", func(t *testing.T) {
		installClientKey(t)
		srv := startTestSSHServer(t, func(cmd string, ch ssh.Channel) execOutcome {
			return execOutcome{status: 0}
		})
		conn := dialTestServer(t, srv)
		err := conn.Upload(filepath.Join(t.TempDir(), "absent"), "/remote/x")
		if err == nil || !strings.Contains(err.Error(), "opening local file") {
			t.Fatalf("Upload(absent) = %v, want open-local-file error", err)
		}
		if served := srv.servedCommands(); len(served) != 0 {
			t.Fatalf("server received commands %v, want none", served)
		}
	})
}

func TestConnectionCloseWithoutClient(t *testing.T) {
	if err := (&Connection{}).Close(); err != nil {
		t.Fatalf("Close() on zero-value Connection = %v, want nil", err)
	}
}

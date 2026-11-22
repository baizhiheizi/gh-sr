// Package testutil provides shared helpers for tests across gh-sr packages.
package testutil

import (
	"bufio"
	"bytes"
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
	"time"

	"golang.org/x/crypto/ssh"
)

// ExecResult is what the fake remote command produces for one exec request.
type ExecResult struct {
	Stdout string
	Stderr string
	Status uint32
}

// SCPUpload records one completed scp sink transfer.
type SCPUpload struct {
	Perms       string
	Name        string
	Size        int64
	Content     []byte
	TrailingNUL bool
}

// SSHServer is a minimal in-process SSH server: publickey auth is always
// accepted, every exec command is recorded, and each session is delegated to
// the caller-supplied handler. It exercises the real client dial/handshake/
// exec/scp-sink path without an sshd.
type SSHServer struct {
	t              *testing.T
	ln             net.Listener
	cfg            *ssh.ServerConfig
	handle         func(cmd string, ch ssh.Channel) ExecResult
	rejectSessions bool
	hostPub        ssh.PublicKey

	mu           sync.Mutex
	commands     []string
	uploads      []SCPUpload
	sessionOpens int
	// sessionOpened is signalled every time a session channel is accepted,
	// so a test can wait for the server to reach that point instead of
	// racing on the sessionOpens counter.
	sessionOpened chan struct{}
}

// StartSSHServer starts an in-process SSH server on 127.0.0.1 and registers a
// cleanup that stops it. The handler produces the ExecResult for each exec
// command; scp sink commands ("scp -t ...") are consumed by the harness.
func StartSSHServer(t *testing.T, handle func(cmd string, ch ssh.Channel) ExecResult) *SSHServer {
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
	s := &SSHServer{
		t:             t,
		ln:            ln,
		cfg:           cfg,
		handle:        handle,
		hostPub:       signer.PublicKey(),
		sessionOpened: make(chan struct{}, 4),
	}
	go s.serve()
	t.Cleanup(func() { ln.Close() })
	return s
}

// Addr returns the 127.0.0.1:port address the server listens on.
func (s *SSHServer) Addr() string { return s.ln.Addr().String() }

// HostPublicKey returns the server's host key, for pinning it in known_hosts.
func (s *SSHServer) HostPublicKey() ssh.PublicKey { return s.hostPub }

// ServedCommands returns the exec commands the server has received.
func (s *SSHServer) ServedCommands() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string(nil), s.commands...)
}

// ServedUploads returns the completed scp transfers the server has received.
func (s *SSHServer) ServedUploads() []SCPUpload {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]SCPUpload(nil), s.uploads...)
}

// SessionOpenCount reports how many session channels the server accepted.
func (s *SSHServer) SessionOpenCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.sessionOpens
}

// SetRejectSessions makes the server reject session channels, so a client
// NewSession call fails. It is safe to call before the client dials.
func (s *SSHServer) SetRejectSessions(reject bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.rejectSessions = reject
}

func (s *SSHServer) rejectSessionsEnabled() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.rejectSessions
}

// WaitForSessionOpen blocks until the server has accepted a session channel,
// so a test can assert on the counter without racing the server goroutine.
func (s *SSHServer) WaitForSessionOpen(t *testing.T) {
	t.Helper()
	select {
	case <-s.sessionOpened:
	case <-time.After(5 * time.Second):
		t.Fatal("server never accepted a session channel")
	}
}

func (s *SSHServer) serve() {
	for {
		c, err := s.ln.Accept()
		if err != nil {
			return
		}
		go s.handleConn(c)
	}
}

func (s *SSHServer) handleConn(c net.Conn) {
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
		if s.rejectSessionsEnabled() {
			newCh.Reject(ssh.Prohibited, "sessions disabled")
			continue
		}
		ch, chReqs, err := newCh.Accept()
		if err != nil {
			continue
		}
		s.mu.Lock()
		s.sessionOpens++
		s.mu.Unlock()
		select {
		case s.sessionOpened <- struct{}{}:
		default:
		}
		go s.handleSession(ch, chReqs)
	}
}

func (s *SSHServer) handleSession(ch ssh.Channel, reqs <-chan *ssh.Request) {
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
			s.handleScpSink(ch)
			return
		}
		out := s.handle(payload.Command, ch)
		if out.Stdout != "" {
			fmt.Fprint(ch, out.Stdout)
		}
		if out.Stderr != "" {
			fmt.Fprint(ch.Stderr(), out.Stderr)
		}
		ch.SendRequest("exit-status", false, ssh.Marshal(struct{ Status uint32 }{out.Status}))
		return
	}
}

// scpSinkTimeout bounds how long the server waits for an scp frame before
// giving up. Without it, a client regression that never sends a header (or
// omits the trailing NUL) would block until the package test timeout instead
// of failing the subtest.
const scpSinkTimeout = 10 * time.Second

// maxScpFrameSize bounds how many bytes the harness allocates for one
// transfer. A corrupt header claiming an enormous size would otherwise
// exhaust memory and kill the test binary. Real transfers are orders of
// magnitude smaller than this.
const maxScpFrameSize = 64 << 20 // 64 MiB

// scpSinkResult carries the outcome of one scp sink read.
type scpSinkResult struct {
	upload SCPUpload
	err    error
}

// handleScpSink consumes the scp sink protocol: a "C0644 <size> <name>\n"
// header, exactly <size> content bytes, and a trailing NUL.
//
// A malformed frame is reported as a non-zero exit status rather than
// panicking: the frame is produced by the code under test, so a crash here
// would take the whole test binary down instead of failing one subtest.
func (s *SSHServer) handleScpSink(ch ssh.Channel) {
	done := make(chan scpSinkResult, 1)
	go func() {
		upload, err := ReadScpSink(ch)
		done <- scpSinkResult{upload: upload, err: err}
	}()

	timer := time.NewTimer(scpSinkTimeout)
	defer timer.Stop()

	select {
	case res := <-done:
		if res.err != nil {
			fmt.Fprintf(ch.Stderr(), "scp: %v\n", res.err)
			ch.SendRequest("exit-status", false, ssh.Marshal(struct{ Status uint32 }{1}))
			return
		}
		s.mu.Lock()
		s.uploads = append(s.uploads, res.upload)
		s.mu.Unlock()
		ch.SendRequest("exit-status", false, ssh.Marshal(struct{ Status uint32 }{0}))
	case <-timer.C:
		fmt.Fprint(ch.Stderr(), "scp: timed out waiting for the transfer frame\n")
		ch.SendRequest("exit-status", false, ssh.Marshal(struct{ Status uint32 }{1}))
	}
}

// ReadScpSink reads one scp sink frame from ch and returns the parsed upload.
// It records nothing: the caller decides what to do with the outcome.
func ReadScpSink(ch ssh.Channel) (SCPUpload, error) {
	r := bufio.NewReader(ch)
	header, err := r.ReadString('\n')
	if err != nil {
		return SCPUpload{}, fmt.Errorf("reading header: %w", err)
	}
	fields := strings.Fields(header)
	if len(fields) < 3 {
		return SCPUpload{}, fmt.Errorf("malformed header %q: want %q", strings.TrimSpace(header), "C0644 <size> <name>")
	}
	size, err := strconv.ParseInt(fields[1], 10, 64)
	if err != nil {
		return SCPUpload{}, fmt.Errorf("malformed size in header %q: %w", strings.TrimSpace(header), err)
	}
	if size < 0 {
		return SCPUpload{}, fmt.Errorf("negative size in header %q", strings.TrimSpace(header))
	}
	if size > maxScpFrameSize {
		return SCPUpload{}, fmt.Errorf("size %d in header %q exceeds the %d byte limit", size, strings.TrimSpace(header), maxScpFrameSize)
	}
	content := make([]byte, size)
	if _, err := io.ReadFull(r, content); err != nil {
		return SCPUpload{}, fmt.Errorf("short read: %w", err)
	}
	nul := make([]byte, 1)
	_, readErr := io.ReadFull(r, nul)
	return SCPUpload{
		Perms:       fields[0],
		Name:        fields[2],
		Size:        size,
		Content:     content,
		TrailingNUL: readErr == nil && nul[0] == 0,
	}, nil
}

// InstallClientKey points HOME at a temp dir holding one usable ed25519 key
// under .ssh/id_ed25519 and clears SSH_AUTH_SOCK, so the SSH client's auth
// collection yields exactly that key. Uses t.Setenv, so tests calling it must
// not be parallel.
func InstallClientKey(t *testing.T) {
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

// BracketedHostPattern renders addr as the known_hosts pattern for a
// non-standard port: "[host]:port".
func BracketedHostPattern(addr string) string {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return addr
	}
	return "[" + host + "]:" + port
}

// FakeSSHChannel is an in-memory ssh.Channel for driving ReadScpSink (and
// similar readers) directly, without a live server.
type FakeSSHChannel struct {
	in     *bytes.Buffer
	out    bytes.Buffer
	stderr bytes.Buffer
}

// NewFakeSSHChannel builds a FakeSSHChannel whose readable side yields frame.
func NewFakeSSHChannel(frame string) *FakeSSHChannel {
	return &FakeSSHChannel{in: bytes.NewBufferString(frame)}
}

func (c *FakeSSHChannel) Read(p []byte) (int, error)  { return c.in.Read(p) }
func (c *FakeSSHChannel) Write(p []byte) (int, error) { return c.out.Write(p) }
func (c *FakeSSHChannel) Close() error                { return nil }
func (c *FakeSSHChannel) CloseWrite() error           { return nil }
func (c *FakeSSHChannel) SendRequest(string, bool, []byte) (bool, error) {
	return true, nil
}
func (c *FakeSSHChannel) Stderr() io.ReadWriter { return &c.stderr }

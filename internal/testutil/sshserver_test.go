package testutil

import (
	"crypto/ed25519"
	"crypto/rand"
	"strings"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"
)

// dialSSHServer connects a raw ssh client to the in-process server, so the
// harness itself stays verified even when only other packages' tests consume
// it (a x/crypto API drift then fails here first, at the source).
func dialSSHServer(t *testing.T, srv *SSHServer) *ssh.Client {
	t.Helper()
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	signer, err := ssh.NewSignerFromKey(priv)
	if err != nil {
		t.Fatal(err)
	}
	cfg := &ssh.ClientConfig{
		User:            "root",
		Auth:            []ssh.AuthMethod{ssh.PublicKeys(signer)},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         5 * time.Second,
	}
	client, err := ssh.Dial("tcp", srv.Addr(), cfg)
	if err != nil {
		t.Fatalf("ssh.Dial(%s): %v", srv.Addr(), err)
	}
	t.Cleanup(func() { client.Close() })
	return client
}

func TestSSHServerExecRoundTrip(t *testing.T) {
	srv := StartSSHServer(t, func(cmd string, ch ssh.Channel) ExecResult {
		return ExecResult{Stdout: "pong " + cmd, Status: 0}
	})
	if srv.HostPublicKey() == nil {
		t.Fatal("HostPublicKey = nil, want the server host key")
	}
	client := dialSSHServer(t, srv)
	session, err := client.NewSession()
	if err != nil {
		t.Fatalf("NewSession: %v", err)
	}
	defer session.Close()
	out, err := session.Output("ping")
	if err != nil {
		t.Fatalf("Output(ping): %v", err)
	}
	if string(out) != "pong ping" {
		t.Fatalf("Output(ping) = %q, want %q", out, "pong ping")
	}
	srv.WaitForSessionOpen(t)
	if served := srv.ServedCommands(); len(served) != 1 || served[0] != "ping" {
		t.Fatalf("ServedCommands() = %v, want [ping]", served)
	}
	if n := srv.SessionOpenCount(); n != 1 {
		t.Fatalf("SessionOpenCount() = %d, want 1", n)
	}
}

func TestSSHServerScpRoundTrip(t *testing.T) {
	srv := StartSSHServer(t, func(cmd string, ch ssh.Channel) ExecResult { return ExecResult{Status: 0} })
	client := dialSSHServer(t, srv)
	session, err := client.NewSession()
	if err != nil {
		t.Fatalf("NewSession: %v", err)
	}
	defer session.Close()
	session.Stdin = strings.NewReader("C0644 5 hello\nabcde\x00")
	if err := session.Run("scp -t /tmp"); err != nil {
		t.Fatalf("Run(scp -t /tmp): %v", err)
	}
	uploads := srv.ServedUploads()
	if len(uploads) != 1 {
		t.Fatalf("ServedUploads() = %d uploads, want 1", len(uploads))
	}
	up := uploads[0]
	if up.Perms != "C0644" || up.Name != "hello" || up.Size != 5 || string(up.Content) != "abcde" || !up.TrailingNUL {
		t.Fatalf("upload = %+v, want C0644/hello/5/abcde with trailing NUL", up)
	}
}

func TestSSHServerRejectSessions(t *testing.T) {
	srv := StartSSHServer(t, func(cmd string, ch ssh.Channel) ExecResult { return ExecResult{Status: 0} })
	srv.SetRejectSessions(true)
	client := dialSSHServer(t, srv)
	session, err := client.NewSession()
	if err == nil {
		session.Close()
		t.Fatal("NewSession with rejectSessions = nil error, want rejection")
	}
}

func TestBracketedHostPattern(t *testing.T) {
	for _, tc := range []struct {
		addr string
		want string
	}{
		{addr: "10.0.0.1:2222", want: "[10.0.0.1]:2222"},
		{addr: "no-port-here", want: "no-port-here"},
	} {
		if got := BracketedHostPattern(tc.addr); got != tc.want {
			t.Errorf("BracketedHostPattern(%q) = %q, want %q", tc.addr, got, tc.want)
		}
	}
}

func TestReadScpSinkMalformedFrame(t *testing.T) {
	ch := NewFakeSSHChannel("C0644 3 x\nab") // short read: fewer content bytes than the header claims
	if _, err := ReadScpSink(ch); err == nil {
		t.Fatal("ReadScpSink with short content = nil error, want short-read error")
	}
}

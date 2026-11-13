package host

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/pem"
	"errors"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
	"golang.org/x/crypto/ssh/knownhosts"
)

// writeTestKey generates a fresh ed25519 private key, writes it in
// openssh-PEM form under t.TempDir()/name, and returns the path. garbage
// writes non-key bytes instead (keyFileAuth must reject it).
func writeTestKey(t *testing.T, name string, garbage bool) string {
	t.Helper()
	var pemBytes []byte
	if garbage {
		pemBytes = []byte("not a key at all\n")
	} else {
		_, priv, err := ed25519.GenerateKey(rand.Reader)
		if err != nil {
			t.Fatal(err)
		}
		block, err := ssh.MarshalPrivateKey(priv, "test")
		if err != nil {
			t.Fatal(err)
		}
		pemBytes = pem.EncodeToMemory(block)
	}
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, pemBytes, 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

// startTestAgent serves an in-process ssh-agent keyring holding one fresh
// ed25519 key on a unix socket and returns the socket path.
func startTestAgent(t *testing.T) string {
	t.Helper()
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	ring := agent.NewKeyring()
	if err := ring.Add(agent.AddedKey{PrivateKey: priv}); err != nil {
		t.Fatal(err)
	}
	// AF_UNIX paths cap at 108 bytes. t.TempDir() nests the full test name
	// under TMPDIR, which overflows the limit on long temp roots and makes
	// net.Listen fail with "bind: invalid argument", so bind from a short
	// directory at the temp root instead.
	dir, err := os.MkdirTemp("", "gh-sr-agent-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	sock := filepath.Join(dir, "agent.sock")
	l, err := net.Listen("unix", sock)
	if err != nil {
		t.Fatal(err)
	}
	go func() {
		for {
			c, err := l.Accept()
			if err != nil {
				return
			}
			go agent.ServeAgent(ring, c)
		}
	}()
	t.Cleanup(func() { l.Close() })
	return sock
}

func TestKeyFileAuth(t *testing.T) {
	t.Run("valid private key yields an auth method", func(t *testing.T) {
		path := writeTestKey(t, "id_ed25519", false)
		if m := keyFileAuth(path); m == nil {
			t.Fatal("keyFileAuth(valid key) = nil, want an auth method")
		}
	})

	t.Run("garbage file yields nil", func(t *testing.T) {
		path := writeTestKey(t, "id_ed25519", true)
		if m := keyFileAuth(path); m != nil {
			t.Fatal("keyFileAuth(garbage) != nil, want nil")
		}
	})

	t.Run("missing file yields nil", func(t *testing.T) {
		if m := keyFileAuth(filepath.Join(t.TempDir(), "absent")); m != nil {
			t.Fatal("keyFileAuth(missing) != nil, want nil")
		}
	})
}

func TestDefaultKeyPaths(t *testing.T) {
	t.Run("lists the three default keys under $HOME/.ssh", func(t *testing.T) {
		home := t.TempDir()
		t.Setenv("HOME", home)
		got := defaultKeyPaths()
		want := []string{
			filepath.Join(home, ".ssh", "id_ed25519"),
			filepath.Join(home, ".ssh", "id_rsa"),
			filepath.Join(home, ".ssh", "id_ecdsa"),
		}
		if strings.Join(got, "\n") != strings.Join(want, "\n") {
			t.Fatalf("defaultKeyPaths() = %v, want %v", got, want)
		}
	})

	t.Run("empty HOME yields no paths", func(t *testing.T) {
		t.Setenv("HOME", "")
		if got := defaultKeyPaths(); got != nil {
			t.Fatalf("defaultKeyPaths() with empty HOME = %v, want nil", got)
		}
	})
}

func TestSSHAgentAuth(t *testing.T) {
	t.Run("unset SSH_AUTH_SOCK yields nil", func(t *testing.T) {
		t.Setenv("SSH_AUTH_SOCK", "")
		if m := sshAgentAuth(); m != nil {
			t.Fatal("sshAgentAuth() without SSH_AUTH_SOCK != nil, want nil")
		}
	})

	t.Run("undialable socket path yields nil", func(t *testing.T) {
		t.Setenv("SSH_AUTH_SOCK", filepath.Join(t.TempDir(), "no-socket"))
		if m := sshAgentAuth(); m != nil {
			t.Fatal("sshAgentAuth() on absent socket != nil, want nil")
		}
	})

	t.Run("live agent socket yields an auth method", func(t *testing.T) {
		t.Setenv("SSH_AUTH_SOCK", startTestAgent(t))
		if m := sshAgentAuth(); m == nil {
			t.Fatal("sshAgentAuth() on live agent = nil, want an auth method")
		}
	})
}

func TestCollectAuthMethods(t *testing.T) {
	t.Run("one usable key, no agent yields exactly one method", func(t *testing.T) {
		home := t.TempDir()
		t.Setenv("HOME", home)
		t.Setenv("SSH_AUTH_SOCK", "")
		src := writeTestKey(t, "id_ed25519", false)
		keyBytes, err := os.ReadFile(src)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(filepath.Join(home, ".ssh"), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(home, ".ssh", "id_ed25519"), keyBytes, 0o600); err != nil {
			t.Fatal(err)
		}
		if methods := collectAuthMethods(); len(methods) != 1 {
			t.Fatalf("collectAuthMethods() = %d methods, want 1 (key only)", len(methods))
		}
	})

	t.Run("live agent plus usable key yields two methods", func(t *testing.T) {
		home := t.TempDir()
		t.Setenv("HOME", home)
		t.Setenv("SSH_AUTH_SOCK", startTestAgent(t))
		src := writeTestKey(t, "id_ed25519", false)
		keyBytes, err := os.ReadFile(src)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(filepath.Join(home, ".ssh"), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(home, ".ssh", "id_ed25519"), keyBytes, 0o600); err != nil {
			t.Fatal(err)
		}
		if methods := collectAuthMethods(); len(methods) != 2 {
			t.Fatalf("collectAuthMethods() = %d methods, want 2 (agent + key)", len(methods))
		}
	})

	t.Run("no agent and only unusable keys yields none", func(t *testing.T) {
		home := t.TempDir()
		t.Setenv("HOME", home)
		t.Setenv("SSH_AUTH_SOCK", "")
		if err := os.MkdirAll(filepath.Join(home, ".ssh"), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(home, ".ssh", "id_ed25519"), []byte("garbage"), 0o600); err != nil {
			t.Fatal(err)
		}
		if methods := collectAuthMethods(); len(methods) != 0 {
			t.Fatalf("collectAuthMethods() = %d methods, want 0", len(methods))
		}
	})
}

func TestHasSSHAuth(t *testing.T) {
	t.Run("true when a default key parses", func(t *testing.T) {
		home := t.TempDir()
		t.Setenv("HOME", home)
		t.Setenv("SSH_AUTH_SOCK", "")
		src := writeTestKey(t, "id_ed25519", false)
		keyBytes, err := os.ReadFile(src)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(filepath.Join(home, ".ssh"), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(home, ".ssh", "id_ed25519"), keyBytes, 0o600); err != nil {
			t.Fatal(err)
		}
		if !HasSSHAuth() {
			t.Fatal("HasSSHAuth() = false with a usable default key, want true")
		}
	})

	t.Run("false with no agent and no usable key", func(t *testing.T) {
		t.Setenv("HOME", t.TempDir())
		t.Setenv("SSH_AUTH_SOCK", "")
		if HasSSHAuth() {
			t.Fatal("HasSSHAuth() = true with nothing available, want false")
		}
	})
}

func TestNewConnectionNoAuthMethods(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("SSH_AUTH_SOCK", "")
	_, err := NewConnection("root", "192.0.2.10")
	if err == nil || !strings.Contains(err.Error(), "no SSH authentication methods available") {
		t.Fatalf("NewConnection without auth = %v, want no-SSH-authentication error", err)
	}
}

func TestKnownHostsCallback(t *testing.T) {
	t.Run("valid known_hosts returns callback and path", func(t *testing.T) {
		home := t.TempDir()
		t.Setenv("HOME", home)
		pub, _, err := ed25519.GenerateKey(rand.Reader)
		if err != nil {
			t.Fatal(err)
		}
		sshPub, err := ssh.NewPublicKey(pub)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(filepath.Join(home, ".ssh"), 0o700); err != nil {
			t.Fatal(err)
		}
		line := knownhosts.Line([]string{"[192.0.2.10]:22"}, sshPub)
		if err := os.WriteFile(filepath.Join(home, ".ssh", "known_hosts"), []byte(line+"\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		cb, path, err := knownHostsCallback()
		if err != nil {
			t.Fatalf("knownHostsCallback() error: %v", err)
		}
		if cb == nil {
			t.Fatal("knownHostsCallback() = nil callback, want a verifier")
		}
		if want := filepath.Join(home, ".ssh", "known_hosts"); path != want {
			t.Fatalf("knownHostsCallback() path = %q, want %q", path, want)
		}
	})

	t.Run("missing known_hosts errors so dial falls back to insecure accept", func(t *testing.T) {
		t.Setenv("HOME", t.TempDir())
		cb, path, err := knownHostsCallback()
		if err == nil {
			t.Fatal("knownHostsCallback() with no known_hosts file = nil error, want error")
		}
		if cb != nil || path != "" {
			t.Fatalf("knownHostsCallback() = (cb=%v, path=%q) alongside error, want zero values", cb, path)
		}
		if !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("error = %v, want an os.ErrNotExist wrap", err)
		}
	})
}

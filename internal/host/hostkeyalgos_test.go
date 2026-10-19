package host

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/crypto/ssh"
)

func writeKnownHosts(t *testing.T, lines ...string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "known_hosts")
	if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

// hashHostForTest produces an OpenSSH "|1|salt|hash" pattern for hostname,
// re-deriving the HMAC independently of hashedHostMatches.
func hashHostForTest(t *testing.T, hostname, saltB64 string) string {
	t.Helper()
	salt, err := base64.StdEncoding.DecodeString(saltB64)
	if err != nil {
		t.Fatal(err)
	}
	mac := hmac.New(sha256.New, salt)
	mac.Write([]byte(hostname))
	return "|1|" + saltB64 + "|" + base64.StdEncoding.EncodeToString(mac.Sum(nil))
}

func TestHostKeyAlgosForDial(t *testing.T) {
	t.Run("ed25519 pin prefers ed25519 over Go's RSA/ECDSA-first default", func(t *testing.T) {
		path := writeKnownHosts(t,
			"192.0.2.10 ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAI DummyKeyMaterial",
		)
		algos := hostKeyAlgosForDial(path, "192.0.2.10:22")
		if len(algos) == 0 || algos[0] != ssh.KeyAlgoED25519 {
			t.Fatalf("ed25519 pin should negotiate ed25519 first, got %v", algos)
		}
		// Everything from the default list must still be offered after the pin.
		for _, want := range fallbackHostKeyAlgos {
			found := false
			for _, a := range algos {
				if a == want {
					found = true
					break
				}
			}
			if !found {
				t.Fatalf("fallback algo %q missing from %v", want, algos)
			}
		}
		if hasDupes(algos) {
			t.Fatalf("duplicates in %v", algos)
		}
	})

	t.Run("no pin returns nil so the library default stays", func(t *testing.T) {
		path := writeKnownHosts(t,
			"192.0.2.10 ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAI DummyKeyMaterial",
		)
		if algos := hostKeyAlgosForDial(path, "198.51.100.99:22"); algos != nil {
			t.Fatalf("expected nil for unpinned host, got %v", algos)
		}
	})

	t.Run("missing file returns nil", func(t *testing.T) {
		if algos := hostKeyAlgosForDial(filepath.Join(t.TempDir(), "absent"), "192.0.2.10:22"); algos != nil {
			t.Fatalf("expected nil for missing file, got %v", algos)
		}
	})

	t.Run("rsa pin expands to sha2 variants first", func(t *testing.T) {
		path := writeKnownHosts(t,
			"192.0.2.11 ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABgQ DummyKeyMaterial",
		)
		algos := hostKeyAlgosForDial(path, "192.0.2.11")
		want := []string{ssh.KeyAlgoRSASHA512, ssh.KeyAlgoRSASHA256, ssh.KeyAlgoRSA}
		if len(algos) < len(want) {
			t.Fatalf("expected at least %v, got %v", want, algos)
		}
		for i, w := range want {
			if algos[i] != w {
				t.Fatalf("rsa pin order = %v, want %v first", algos, want)
			}
		}
	})

	t.Run("multiple pins keep file order before fallback", func(t *testing.T) {
		path := writeKnownHosts(t,
			"example.com ecdsa-sha2-nistp256 AAAAE2VjZHNhDummyKey Material",
			"example.com ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAI DummyKeyMaterial",
		)
		algos := hostKeyAlgosForDial(path, "example.com:22")
		if len(algos) < 2 || algos[0] != ssh.KeyAlgoECDSA256 || algos[1] != ssh.KeyAlgoED25519 {
			t.Fatalf("expected [ecdsa256 ed25519 ...] in file order, got %v", algos)
		}
	})

	t.Run("comma-separated host list matches", func(t *testing.T) {
		path := writeKnownHosts(t,
			"other.example,192.0.2.12,also.example ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAI DummyKeyMaterial",
		)
		if algos := hostKeyAlgosForDial(path, "192.0.2.12:22"); algos == nil || algos[0] != ssh.KeyAlgoED25519 {
			t.Fatalf("comma list should match member host, got %v", algos)
		}
	})

	t.Run("hashed entry matches bare host on port 22", func(t *testing.T) {
		pattern := hashHostForTest(t, "192.0.2.13", "c2FsdHNhbHRzYWx0c2FsdA==")
		path := writeKnownHosts(t, pattern+" ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAI DummyKeyMaterial")
		if algos := hostKeyAlgosForDial(path, "192.0.2.13:22"); algos == nil || algos[0] != ssh.KeyAlgoED25519 {
			t.Fatalf("hashed pin should match, got %v", algos)
		}
		// A different host must not match the hash.
		if algos := hostKeyAlgosForDial(path, "192.0.2.14:22"); algos != nil {
			t.Fatalf("hash must not match other host, got %v", algos)
		}
	})

	t.Run("hashed entry matches bracketed non-22 port", func(t *testing.T) {
		pattern := hashHostForTest(t, "[192.0.2.15]:2222", "c2FsdHNhbHRzYWx0c2FsdA==")
		path := writeKnownHosts(t, pattern+" ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAI DummyKeyMaterial")
		if algos := hostKeyAlgosForDial(path, "192.0.2.15:2222"); algos == nil || algos[0] != ssh.KeyAlgoED25519 {
			t.Fatalf("hashed pin should match bracketed port, got %v", algos)
		}
		// Port 22 dials hash the bare host, so this entry must not apply.
		if algos := hostKeyAlgosForDial(path, "192.0.2.15:22"); algos != nil {
			t.Fatalf("port-22 dial must not match port-2222 hash, got %v", algos)
		}
	})

	t.Run("marker and unknown-type lines are ignored", func(t *testing.T) {
		path := writeKnownHosts(t,
			"# comment line",
			"@revoked 192.0.2.16 ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAI DummyKeyMaterial",
			"@cert-authority 192.0.2.16 ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABgQ DummyKeyMaterial",
			"192.0.2.16 not-a-real-key-type AAAAwhatever",
		)
		if algos := hostKeyAlgosForDial(path, "192.0.2.16:22"); algos != nil {
			t.Fatalf("marker/unknown lines must not pin algorithms, got %v", algos)
		}
	})

	t.Run("negated pattern does not match", func(t *testing.T) {
		path := writeKnownHosts(t,
			"!192.0.2.17,10.0.0.0/8 ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAI DummyKeyMaterial",
		)
		if algos := hostKeyAlgosForDial(path, "192.0.2.17:22"); algos != nil {
			t.Fatalf("negated pattern must not count as a pin, got %v", algos)
		}
	})

	t.Run("cert pin maps to cert algorithm", func(t *testing.T) {
		path := writeKnownHosts(t,
			"192.0.2.18 ssh-ed25519-cert-v01@openssh.com AAAAC3NzaC1lZDI1NTE5AAAAI DummyKeyMaterial",
		)
		algos := hostKeyAlgosForDial(path, "192.0.2.18:22")
		if algos == nil || algos[0] != ssh.CertAlgoED25519v01 {
			t.Fatalf("cert pin should prefer the cert algorithm, got %v", algos)
		}
	})
}

func TestSplitDialTarget(t *testing.T) {
	for _, tc := range []struct {
		addr, host, port string
	}{
		{"192.0.2.1:22", "192.0.2.1", "22"},
		{"192.0.2.1:2222", "192.0.2.1", "2222"},
		{"example.com:22", "example.com", "22"},
		{"[2001:db8::1]:2222", "2001:db8::1", "2222"},
		{"bare-host", "bare-host", "22"},
	} {
		host, port := splitDialTarget(tc.addr)
		if host != tc.host || port != tc.port {
			t.Errorf("splitDialTarget(%q) = (%q, %q), want (%q, %q)", tc.addr, host, port, tc.host, tc.port)
		}
	}
}

func TestHostKeyHint(t *testing.T) {
	mismatch := &fakeErr{"ssh: handshake failed: knownhosts: key mismatch"}
	unknown := &fakeErr{"ssh: handshake failed: knownhosts: key is unknown"}
	other := &fakeErr{"ssh: handshake failed: something else"}

	if h := hostKeyHint(mismatch, "an-lee", "192.0.2.1"); !strings.Contains(h, "ssh-keygen -R 192.0.2.1") {
		t.Errorf("mismatch hint should mention ssh-keygen -R, got %q", h)
	}
	if h := hostKeyHint(unknown, "an-lee", "192.0.2.1"); !strings.Contains(h, "ssh an-lee@192.0.2.1") {
		t.Errorf("unknown hint should mention connecting with ssh, got %q", h)
	}
	if h := hostKeyHint(other, "an-lee", "192.0.2.1"); h != "" {
		t.Errorf("non-host-key error should have no hint, got %q", h)
	}
}

type fakeErr struct{ msg string }

func (e *fakeErr) Error() string { return e.msg }

func hasDupes(algos []string) bool {
	seen := map[string]bool{}
	for _, a := range algos {
		if seen[a] {
			return true
		}
		seen[a] = true
	}
	return false
}

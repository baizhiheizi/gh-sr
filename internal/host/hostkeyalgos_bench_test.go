package host

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// benchKnownHosts builds a realistic known_hosts: comments, @marker lines,
// hashed entries, and plain pins for hosts that are not the dial target —
// hostKeyAlgosForDial walks every line on each dial.
func benchKnownHosts(b *testing.B, pinnedHost string) string {
	b.Helper()
	var lines []string
	lines = append(lines,
		"# gh-sr managed pins below",
		"@revoked 10.9.9.9 ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABgQDummy",
	)
	for i := 0; i < 14; i++ {
		lines = append(lines, fmt.Sprintf(
			"10.0.%d.%d,alt-%d.example ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIDummyKeyMaterial%d",
			i, i, i, i))
	}
	if pinnedHost != "" {
		lines = append(lines, pinnedHost+" ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIPinned")
	}
	path := filepath.Join(b.TempDir(), "known_hosts")
	if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0o600); err != nil {
		b.Fatal(err)
	}
	return path
}

func BenchmarkHostKeyAlgosForDial(b *testing.B) {
	b.Run("unpinned_scans_all_lines", func(b *testing.B) {
		path := benchKnownHosts(b, "")
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			if algos := hostKeyAlgosForDial(path, "203.0.113.1:22"); algos != nil {
				b.Fatalf("unpinned host must return nil, got %v", algos)
			}
		}
	})
	b.Run("pinned", func(b *testing.B) {
		path := benchKnownHosts(b, "203.0.113.2")
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			if algos := hostKeyAlgosForDial(path, "203.0.113.2:22"); len(algos) == 0 {
				b.Fatal("pinned host must resolve algorithms")
			}
		}
	})
}

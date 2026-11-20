package registry

import (
	"fmt"
	"strings"
	"testing"

	"github.com/an-lee/gh-sr/internal/config"
	"github.com/an-lee/gh-sr/internal/host"
	"github.com/an-lee/gh-sr/internal/testutil"
)

// BenchmarkMirrorStorages covers the du -sb output parse with an 8-mirror
// fixture (absolute StoragePath, so resolution needs no remote round trip).
func BenchmarkMirrorStorages(b *testing.B) {
	const n = 8
	names := make([]string, n)
	mirrors := make([]MirrorSettings, n)
	var du strings.Builder
	for i := 0; i < n; i++ {
		names[i] = fmt.Sprintf("mirror%d", i)
		mirrors[i] = MirrorSettings{Name: names[i], Port: 3001 + i}
		path := "/srv/registry-mirror"
		if i > 0 {
			path += "-" + names[i]
		}
		fmt.Fprintf(&du, "%d\t%s\n", 500_000_000+i, path)
	}
	s := Settings{Enabled: true, StoragePath: "/srv/registry-mirror", Mirrors: mirrors}
	h := host.NewHost("h", config.HostConfig{Addr: "runner@vps", OS: "linux", Arch: "amd64"})
	h.SetConn(&testutil.MockExecutor{Output: du.String()})
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		out, err := MirrorStorages(h, s)
		if err != nil {
			b.Fatal(err)
		}
		if len(out) != n {
			b.Fatalf("got %d rows, want %d", len(out), n)
		}
	}
}

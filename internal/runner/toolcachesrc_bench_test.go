package runner

import (
	"fmt"
	"strings"
	"testing"

	"github.com/an-lee/gh-sr/internal/config"
	"github.com/an-lee/gh-sr/internal/host"
	"github.com/an-lee/gh-sr/internal/testutil"
)

// BenchmarkSyncToolcacheSources covers the CACHED/FETCHED output parse with a
// 10-entry fixture (mock round trip, so the bench isolates script build +
// parse).
func BenchmarkSyncToolcacheSources(b *testing.B) {
	const n = 10
	entries := make([]config.ContainerToolcacheEntry, n)
	var out strings.Builder
	for i := 0; i < n; i++ {
		entries[i] = config.ContainerToolcacheEntry{
			URL: fmt.Sprintf("https://example.com/mirror/toolcache/node-v%d.tar.gz", i),
		}
		fmt.Fprintf(&out, "FETCHED %s\n", toolcacheSourceKey(entries[i].URL))
	}
	h := host.NewHost("h", config.HostConfig{Addr: "runner@vps", OS: "linux", Arch: "amd64"})
	h.SetConn(&testutil.MockExecutor{Output: out.String()})
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		blobs := syncToolcacheSources(h, entries)
		if len(blobs) != n {
			b.Fatalf("got %d blobs, want %d", len(blobs), n)
		}
	}
}

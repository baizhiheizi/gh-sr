package doctor

import (
	"fmt"
	"strings"
	"testing"

	"github.com/an-lee/gh-sr/internal/config"
	"github.com/an-lee/gh-sr/internal/host"
	"github.com/an-lee/gh-sr/internal/registry"
	"github.com/an-lee/gh-sr/internal/testutil"
)

// BenchmarkCheckOrphanMirrors covers the docker ps -a output parse with an
// 11-container fixture (2 configured mirrors, 8 orphans, 1 unrelated).
func BenchmarkCheckOrphanMirrors(b *testing.B) {
	configured := &registry.Settings{
		Enabled: true,
		Mirrors: []registry.MirrorSettings{
			{Name: "dockerhub", Port: 5577},
			{Name: "ghcr", Port: 5578},
		},
	}
	var list strings.Builder
	list.WriteString("gh-sr-registry\ngh-sr-registry-ghcr\n")
	for i := 0; i < 8; i++ {
		fmt.Fprintf(&list, "gh-sr-registry-orphan-%d\n", i)
	}
	list.WriteString("unrelated-container\n")
	h := host.NewHost("linux", config.HostConfig{OS: "linux", Addr: "local"})
	h.SetConn(&testutil.MockExecutor{Output: list.String()})
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var buf strings.Builder
		var r Result
		checkOrphanMirrors(&buf, "h1", h, configured, &r)
		if r.Warn != 8 {
			b.Fatalf("got %d warns, want 8", r.Warn)
		}
	}
}

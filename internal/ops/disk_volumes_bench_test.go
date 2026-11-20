package ops

import (
	"fmt"
	"strings"
	"testing"

	"github.com/an-lee/gh-sr/internal/config"
	"github.com/an-lee/gh-sr/internal/host"
	"github.com/an-lee/gh-sr/internal/testutil"
)

// BenchmarkMeasureSharedVolumes covers both line parses in the volume
// measurement (docker volume ls -q output and busybox du -sb output) with a
// 10-volume fixture.
func BenchmarkMeasureSharedVolumes(b *testing.B) {
	const n = 10
	var list, du strings.Builder
	for i := 0; i < n; i++ {
		fmt.Fprintf(&list, "gh-sr-cache-%d\n", i)
		fmt.Fprintf(&du, "%d\t%d\n", 1_000_000+i, i)
	}
	mock := &testutil.MockExecutor{RunFn: func(cmd string) (string, error) {
		if strings.Contains(cmd, "docker volume ls") {
			return list.String(), nil
		}
		return du.String(), nil
	}}
	h := host.NewHost("h", config.HostConfig{Addr: "runner@vps", OS: "linux", Arch: "amd64"})
	h.SetConn(mock)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		entries := measureSharedVolumes(h, "h")
		if len(entries) != n {
			b.Fatalf("got %d entries, want %d", len(entries), n)
		}
	}
}

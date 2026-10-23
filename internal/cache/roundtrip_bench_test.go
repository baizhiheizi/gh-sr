package cache

import (
	"strings"
	"testing"

	"github.com/an-lee/gh-sr/internal/config"
	"github.com/an-lee/gh-sr/internal/host"
	"github.com/an-lee/gh-sr/internal/testutil"
)

// BenchmarkInspectRoundTrips* measure the SSH round trips one cache Inspect
// costs.
//
// Inspect is on the `gh sr doctor` cache-reachability path, which runs once
// per host per doctor invocation. Each of its probes is a separate remote
// invocation — container state, storage path, gateway address, health, and
// disk usage — so the round-trip count is the network/energy cost of the
// check, and the metric below is what a batching change would move.
//
// The auto-bind case is the default configuration (no explicit bind address
// and no URL override): it resolves the docker0 gateway so the runner can
// reach the cache server. The URL-override case skips that resolve.
//
// Each case is a separate top-level benchmark so it gets its own row in the
// benchstat report.
func BenchmarkInspectRoundTripsAutoBind(b *testing.B) {
	benchmarkInspectRoundTrips(b, Settings{Enabled: true})
}

func BenchmarkInspectRoundTripsURLOverride(b *testing.B) {
	benchmarkInspectRoundTrips(b, Settings{Enabled: true, URLOverride: "http://override:9999/"})
}

func benchmarkInspectRoundTrips(b *testing.B, settings Settings) {
	b.Helper()
	mock := &testutil.RoundTripCounter{RunFn: func(cmd string) (string, error) {
		switch {
		case strings.Contains(cmd, "docker inspect --format="):
			return "running|example/image:1\n", nil
		case strings.Contains(cmd, "ip -4 -o addr show docker0"):
			return gwOutput, nil
		case strings.Contains(cmd, "curl -fsS"):
			return "healthy", nil
		case strings.Contains(cmd, "echo $HOME"):
			return "/root\n", nil
		case strings.Contains(cmd, "du -sb"):
			return "1024\n", nil
		default:
			return "", nil
		}
	}}
	h := host.NewHost("h", config.HostConfig{Addr: "runner@vps", OS: "linux", Arch: "amd64"})
	h.SetConn(mock)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		mock.Reset()
		info, err := Inspect(h, settings)
		if err != nil {
			b.Fatal(err)
		}
		if info.State != "running" {
			b.Fatalf("got state %q, want running", info.State)
		}
	}
	b.StopTimer()
	b.ReportMetric(float64(mock.RoundTrips), "round-trips/op")
}

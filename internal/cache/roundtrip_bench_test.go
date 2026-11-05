package cache

import (
	"testing"

	"github.com/an-lee/gh-sr/internal/config"
	"github.com/an-lee/gh-sr/internal/host"
	"github.com/an-lee/gh-sr/internal/testutil"
)

// BenchmarkInspectRoundTrips* measure the SSH round trips one cache Inspect
// costs.
//
// Inspect is on the `gh sr doctor` cache-reachability path, which runs once
// per host per doctor invocation (plus `gh sr cache status`). Its probes used
// to be five separate remote invocations — container state, storage path,
// gateway address, health, and disk usage — so the round-trip count was the
// network/energy cost of the check. They are folded into one script now; the
// benchmark pins that count at 1 so the probes cannot silently regress to
// per-probe calls (each one costs a network round trip, a remote shell start,
// and remote CPU).
//
// The auto-bind case is the default configuration (no explicit bind address
// and no URL override): the script also resolves the docker0 gateway so the
// runner can reach the cache server. The URL-override case skips that probe.
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
	gw := ""
	if settings.autoBind() {
		gw = gwOutput
	}
	mock := &testutil.RoundTripCounter{RunFn: func(cmd string) (string, error) {
		return oneShotOutput("running|example/image:1", "/root", gw, "healthy", "1024"), nil
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

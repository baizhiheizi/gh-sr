package agentic

import (
	"testing"

	"github.com/an-lee/gh-sr/internal/config"
	"github.com/an-lee/gh-sr/internal/host"
	"github.com/an-lee/gh-sr/internal/testutil"
)

// BenchmarkValidateContainerAgenticFanoutRoundTrips* measure the SSH round
// trips the container-agentic fanout probe costs.
//
// The fanout folds every inner-container prereq probe into a single
// `docker exec`, so its round-trip count is 1 regardless of how many
// individual checks it runs. This benchmark pins that invariant: a change
// that splits the fanout back into per-check probes would double the
// remote I/O of `gh sr doctor` on every container-mode host, and the
// round-trip metric is what would show it.
//
// The count is the energy proxy — each round trip costs a network hop, a
// remote shell start, and the remote CPU to run the probe.
//
// The names are flat (no "/" sub-benchmark parts): scripts/benchstat's
// parser only recognises flat names, so a sub-benchmark row would never
// reach the CI comparison report.
func BenchmarkValidateContainerAgenticFanoutRoundTrips(b *testing.B) {
	benchmarkValidateContainerAgenticFanoutRoundTrips(b, false)
}

func BenchmarkValidateContainerAgenticFanoutRoundTripsCacheEnabled(b *testing.B) {
	benchmarkValidateContainerAgenticFanoutRoundTrips(b, true)
}

func benchmarkValidateContainerAgenticFanoutRoundTrips(b *testing.B, cacheEnabled bool) {
	b.Helper()
	mock := &testutil.RoundTripCounter{RunFn: func(cmd string) (string, error) {
		return "#container-inner-host-docker-internal:0\n" +
			"#container-node-npm:0\n" +
			"#container-docker-socket-user:0\n" +
			"#container-zstd:0\n" +
			"#container-cache-env:0\n" +
			"#container-mtu:0\n", nil
	}}
	h := host.NewHost("h", config.HostConfig{OS: "linux", Arch: "amd64", Addr: "local"})
	h.SetConn(mock)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		mock.Reset()
		failures := ValidateContainerAgenticFanout(h, "gh-sr-ci-1", "ci-1", 1500, cacheEnabled)
		if len(failures) != 0 {
			b.Fatalf("got %d failures, want 0", len(failures))
		}
	}
	b.StopTimer()
	b.ReportMetric(float64(mock.RoundTrips), "round-trips/op")
}

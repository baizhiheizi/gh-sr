package agentic

import (
	"strings"
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
// Each case is a separate top-level benchmark so it gets its own row in the
// benchstat report.
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

// BenchmarkAgenticInnerBatchRoundTrips pins the batched agentic inner-hygiene
// probe's whole-host round-trip cost at 1 regardless of instance count. A
// change that reintroduces per-instance SSH round-trips here multiplies
// `gh sr doctor`'s remote I/O by the agentic container count, and this metric
// is what would show it.
func BenchmarkAgenticInnerBatchRoundTrips(b *testing.B) {
	batchOut := strings.Join([]string{
		"@@begin:1", "@@state:1|running",
		"@@hyg:1|0", "@@hygend:1|0", "@@hyg:1|1", "@@hygend:1|1",
		"#container-inner-host-docker-internal:0",
		"#container-node-npm:0",
		"#container-docker-socket-user:0",
		"#container-zstd:0",
		"@@end:1",
		"@@begin:2", "@@state:2|running",
		"@@hyg:2|0", "@@hygend:2|0", "@@hyg:2|1", "@@hygend:2|1",
		"#container-inner-host-docker-internal:0",
		"#container-node-npm:0",
		"#container-docker-socket-user:0",
		"#container-zstd:0",
		"@@end:2",
	}, "\n")
	mock := &testutil.RoundTripCounter{RunFn: func(cmd string) (string, error) {
		return batchOut, nil
	}}
	h := host.NewHost("h", config.HostConfig{OS: "linux", Addr: "local"})
	h.SetConn(mock)
	targets := []AgenticInnerTarget{
		{Instance: "aw-1", RunnerName: "aw"},
		{Instance: "aw-2", RunnerName: "aw"},
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		mock.Reset()
		probes, err := ValidateAgenticInnerBatch(h, targets, 1500, false)
		if err != nil {
			b.Fatalf("batch failed: %v", err)
		}
		for _, p := range probes {
			if !p.Reported || len(p.Failures) != 0 {
				b.Fatalf("unexpected probe result: %+v", p)
			}
		}
	}
	b.StopTimer()
	b.ReportMetric(float64(mock.RoundTrips), "round-trips/op")
}

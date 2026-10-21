package runner

import (
	"strings"
	"testing"

	"github.com/an-lee/gh-sr/internal/config"
	"github.com/an-lee/gh-sr/internal/host"
	"github.com/an-lee/gh-sr/internal/testutil"
)

// The benchmarks in this file measure the SSH round trips a gh-sr operation
// costs — the network/energy proxy that the CPU-side microbenchmarks
// elsewhere in this package cannot see. Every remote invocation costs a
// network round trip, a remote shell start, and the remote CPU to run the
// probe; cutting one out of an operation removes all three.
//
// Each benchmark reports the count as the custom metric "round-trips/op",
// so `make bench` prints it next to ns/op and the bench-compare workflow
// (scripts/benchstat) can flag a change in remote I/O the same way it flags
// a CPU regression.
//
// The benchmark names are deliberately flat (no "/" sub-benchmark parts):
// scripts/benchstat's parser only recognises flat names, so a sub-benchmark
// row would be dropped from the CI comparison entirely and the metric would
// never reach the report.

// nativeStatusMock returns the canned output for a healthy native Linux
// runner: svc.sh present, a user-level systemd unit installed, and a
// .runner-version file. The status path answers it with two probes per
// instance — the combined marker probe and the systemctl state probe — which
// is the shape of a real svc.sh-managed runner.
func nativeStatusMock(cmd string) (string, error) {
	if strings.Contains(cmd, "systemctl is-active") {
		return "active\n", nil
	}
	return "S\nU\nV2.320.0\n", nil
}

// containerStatusMock returns the canned output of the folded container
// status probe: "status|configImage|digest|imageRev".
func containerStatusMock(cmd string) (string, error) {
	return "running|gh-sr/agentic-runner:v1|sha256:0123456789abcdef|cafebabe\n", nil
}

// BenchmarkManagerStatusRoundTrips* measure how many SSH round trips one
// Manager.Status call costs, per instance count and runner mode.
//
// Status is the per-tick hot path: the TUI dashboard refreshes it on a
// timer, so its round-trip count is multiplied by the refresh rate and by
// the number of runners on the host. The counts are the baseline a batching
// change (N instances in one round trip instead of N) has to beat.
//
// A native instance costs two round trips (the combined marker probe and
// the systemctl state probe); a container instance costs one (the folded
// docker inspect). The native path therefore scales at 2N round trips for
// N instances on a host.
func BenchmarkManagerStatusRoundTripsNative1(b *testing.B) {
	benchmarkManagerStatusRoundTrips(b, config.RunnerModeNative, 1)
}

func BenchmarkManagerStatusRoundTripsNative4(b *testing.B) {
	benchmarkManagerStatusRoundTrips(b, config.RunnerModeNative, 4)
}

func BenchmarkManagerStatusRoundTripsNative16(b *testing.B) {
	benchmarkManagerStatusRoundTrips(b, config.RunnerModeNative, 16)
}

func BenchmarkManagerStatusRoundTripsContainer4(b *testing.B) {
	benchmarkManagerStatusRoundTrips(b, config.RunnerModeContainer, 4)
}

func benchmarkManagerStatusRoundTrips(b *testing.B, mode string, count int) {
	b.Helper()
	mock := &testutil.RoundTripCounter{RunFn: nativeStatusMock}
	if mode == config.RunnerModeContainer {
		mock.RunFn = containerStatusMock
	}
	h := host.NewHost("h", config.HostConfig{OS: "linux", Arch: "amd64", Addr: "local"})
	h.SetConn(mock)

	rc := config.RunnerConfig{
		Name:       "ci",
		Repo:       "owner/repo",
		Host:       "h",
		Count:      count,
		RunnerMode: mode,
	}
	mgr := NewManager("")

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		mock.Reset()
		statuses, err := mgr.Status(h, rc)
		if err != nil {
			b.Fatal(err)
		}
		if len(statuses) != count {
			b.Fatalf("got %d statuses, want %d", len(statuses), count)
		}
	}
	b.StopTimer()
	b.ReportMetric(float64(mock.RoundTrips), "round-trips/op")
}

// BenchmarkMeasureDiskUsageBatchRoundTrips* measure the round trips of the
// batched disk-usage probe. The batch API is the reference shape for this
// codebase: one SSH round trip for every instance on the host, regardless of
// how many instances there are. The per-instance API (MeasureDiskUsage)
// costs one round trip each.
func BenchmarkMeasureDiskUsageBatchRoundTrips1(b *testing.B) {
	benchmarkMeasureDiskUsageBatchRoundTrips(b, 1)
}

func BenchmarkMeasureDiskUsageBatchRoundTrips8(b *testing.B) {
	benchmarkMeasureDiskUsageBatchRoundTrips(b, 8)
}

func benchmarkMeasureDiskUsageBatchRoundTrips(b *testing.B, count int) {
	b.Helper()
	instances := (&config.RunnerConfig{Name: "ci", Count: count}).InstanceNames()
	mock := &testutil.RoundTripCounter{RunFn: func(cmd string) (string, error) {
		var sb strings.Builder
		for _, inst := range instances {
			sb.WriteString(inst)
			sb.WriteString("\t1048576\t524288\t262144\t2097152\n")
		}
		return sb.String(), nil
	}}
	h := host.NewHost("h", config.HostConfig{OS: "linux", Arch: "amd64", Addr: "local"})
	h.SetConn(mock)

	rcByInstance := make(map[string]*config.RunnerConfig, len(instances))
	rc := &config.RunnerConfig{Name: "ci", Repo: "owner/repo", Host: "h", Count: count}
	for _, inst := range instances {
		rcByInstance[inst] = rc
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		mock.Reset()
		entries := MeasureDiskUsageBatch(h, "h", instances, rcByInstance)
		if len(entries) != count {
			b.Fatalf("got %d entries, want %d", len(entries), count)
		}
	}
	b.StopTimer()
	b.ReportMetric(float64(mock.RoundTrips), "round-trips/op")
}

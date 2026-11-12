package runner

import (
	"fmt"
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
// Each mode is a separate top-level benchmark so it gets its own row in the
// benchstat report.

// statusScenarioVersion is the canned .runner-version of the healthy native
// runner both probe shapes answer.
const statusScenarioVersion = "2.320.0"

// nativeStatusMock answers the per-instance probe shape for a healthy native
// Linux runner: svc.sh present, a user-level systemd unit installed, and a
// .runner-version file. The per-instance path costs two probes per instance —
// the combined marker probe and the systemctl state probe — which is the
// shape of a real svc.sh-managed runner.
func nativeStatusMock(cmd string) (string, error) {
	if strings.Contains(cmd, "systemctl is-active") {
		return "active\n", nil
	}
	return "S\nU\nV" + statusScenarioVersion + "\n", nil
}

// containerStatusMock returns the canned output of the folded container
// status probe: "status|configImage|digest|imageRev".
func containerStatusMock(cmd string) (string, error) {
	return "running|gh-sr/agentic-runner:v1|sha256:0123456789abcdef|cafebabe\n", nil
}

// composeNativeStatusBatchOutput renders the batch script's answer for the
// same healthy-runner scenario nativeStatusMock covers, one marker line per
// instance: svc.sh-managed + systemctl active → running, version read.
func composeNativeStatusBatchOutput(count int) string {
	var sb strings.Builder
	for i := 1; i <= count; i++ {
		fmt.Fprintf(&sb, "%s%d|running|%s\n", nativeStatusBatchTag, i, statusScenarioVersion)
	}
	return sb.String()
}

// composeContainerStatusBatchOutput renders the batch script's answer for the
// same running-container scenario containerStatusMock covers.
func composeContainerStatusBatchOutput(count int) string {
	var sb strings.Builder
	for i := 1; i <= count; i++ {
		fmt.Fprintf(&sb, "%s%d|running|gh-sr/agentic-runner:v1|sha256:0123456789abcdef|cafebabe\n", containerStatusBatchTag, i)
	}
	return sb.String()
}

// batchScriptRowCount counts the heredoc rows one batch script carries. A
// chunk only emits markers for its own rows (1..count), so mocks must answer
// that many lines; the round-trip benchmarks stay correct even when a large
// instance-name list is split across several bounded execs.
func batchScriptRowCount(cmd string) int {
	open := "<<'" + statusBatchRowsDelim + "'"
	start := strings.Index(cmd, open)
	if start < 0 {
		return 0
	}
	rest := cmd[start+len(open):]
	end := strings.Index(rest, "\n"+statusBatchRowsDelim+"\n")
	if end < 0 {
		return 0
	}
	block := strings.TrimPrefix(rest[:end], "\n")
	if block == "" {
		return 0
	}
	return strings.Count(block, "\n") + 1
}

// statusBatchRunFn routes a mock executor by probe shape: the batch script
// gets its composed per-instance answer, everything else falls to fallbackFn
// (the per-instance probes). batchErr != nil simulates a failing batch
// transport so the benchmark can measure the fallback path.
func statusBatchRunFn(batchErr error, batchOut func(cmd string) (string, error), fallbackFn func(cmd string) (string, error)) func(string) (string, error) {
	return func(cmd string) (string, error) {
		if strings.Contains(cmd, statusBatchScriptHeader) {
			if batchErr != nil {
				return "", batchErr
			}
			return batchOut(cmd)
		}
		return fallbackFn(cmd)
	}
}

// BenchmarkManagerStatusRoundTrips* measure how many SSH round trips one
// Manager.Status call costs, per instance count and runner mode.
//
// Status is the per-tick hot path: the TUI dashboard refreshes it on a
// timer, so its round-trip count is multiplied by the refresh rate and by
// the number of runners on the host.
//
// The batch scripts (internal/runner/status_batch.go) answer every instance
// of the host in ONE round trip — 1.000 regardless of instance count. The
// Fallback benchmark forces the per-instance path (batch transport error) to
// document the degraded shape it must stay at least as good as: two probes
// per native instance (marker probe + systemctl state probe), one per
// container instance.
func BenchmarkManagerStatusRoundTripsNative1(b *testing.B) {
	benchmarkManagerStatusRoundTrips(b, config.RunnerModeNative, 1, false)
}

func BenchmarkManagerStatusRoundTripsNative4(b *testing.B) {
	benchmarkManagerStatusRoundTrips(b, config.RunnerModeNative, 4, false)
}

func BenchmarkManagerStatusRoundTripsNative16(b *testing.B) {
	benchmarkManagerStatusRoundTrips(b, config.RunnerModeNative, 16, false)
}

func BenchmarkManagerStatusRoundTripsContainer4(b *testing.B) {
	benchmarkManagerStatusRoundTrips(b, config.RunnerModeContainer, 4, false)
}

func BenchmarkManagerStatusRoundTripsNative4Fallback(b *testing.B) {
	benchmarkManagerStatusRoundTrips(b, config.RunnerModeNative, 4, true)
}

func benchmarkManagerStatusRoundTrips(b *testing.B, mode string, count int, forceFallback bool) {
	b.Helper()
	runFn := statusBatchRunFn(
		fallbackErr(forceFallback),
		func(cmd string) (string, error) {
			n := batchScriptRowCount(cmd)
			if mode == config.RunnerModeContainer {
				return composeContainerStatusBatchOutput(n), nil
			}
			return composeNativeStatusBatchOutput(n), nil
		},
		nativeStatusMock,
	)
	if mode == config.RunnerModeContainer {
		runFn = statusBatchRunFn(
			fallbackErr(forceFallback),
			func(cmd string) (string, error) {
				return composeContainerStatusBatchOutput(batchScriptRowCount(cmd)), nil
			},
			containerStatusMock,
		)
	}
	mock := &testutil.RoundTripCounter{RunFn: runFn}
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
		if statuses[0].Local != "running" {
			b.Fatalf("statuses[0].Local = %q, want running (mock drift)", statuses[0].Local)
		}
	}
	b.StopTimer()
	b.ReportMetric(float64(mock.RoundTrips), "round-trips/op")
}

func fallbackErr(force bool) error {
	if force {
		return errStatusBatchFallback
	}
	return nil
}

// errStatusBatchFallback simulates a batch transport failure for the fallback
// benchmark. Declared once so the error identity is comparable in tests.
var errStatusBatchFallback = fmt.Errorf("simulated batch transport failure")

// BenchmarkMeasureDiskUsageBatchRoundTrips* measure the round trips of the
// batched disk-usage probe. The batch API is the reference shape for this
// codebase: one SSH round trip for every instance on the host, regardless of
// how many instances there are.
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

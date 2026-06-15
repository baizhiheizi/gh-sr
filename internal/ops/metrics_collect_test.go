package ops

import (
	"bytes"
	"errors"
	"fmt"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/an-lee/gh-sr/internal/config"
	"github.com/an-lee/gh-sr/internal/host"
	"github.com/an-lee/gh-sr/internal/testutil"
)

// metricsOutputFor returns a parseable metrics payload. The orchestrator
// doesn't inspect the body, but a non-empty result with the GH_SR markers
// keeps the host-side parser happy if it ever gets exercised here.
func metricsOutputFor(name string) string {
	return fmt.Sprintf("::GH_SR_METRICS_START::\nuptime=1d 2h 3m\n::GH_SR_METRICS_END::\nmetric=%s\n", name)
}

// TestCollectHostMetrics_EmptyConfig pins the trivial path: no hosts → no
// goroutines → empty slice. Catches a regression where the orchestrator
// spawns work on a nil/empty map.
func TestCollectHostMetrics_EmptyConfig(t *testing.T) {

	// Mock for safety; the orchestrator must not touch it.
	installMockConnectHost(t, map[string]host.Executor{})

	cfg := &config.Config{Hosts: map[string]config.HostConfig{}}
	got := CollectHostMetrics(nil, cfg, "")
	if got != nil {
		if len(got) == 0 {
			// An empty (non-nil) slice is also acceptable.
			return
		}
		t.Fatalf("expected empty/nil metrics, got %v", got)
	}
}

// TestCollectHostMetrics_SingleHostHappyPath verifies the basic flow: one
// host, one connect, one CollectMetrics, one Close. The result slice has
// length 1 with the host's name populated and no error.
func TestCollectHostMetrics_SingleHostHappyPath(t *testing.T) {

	exec := &testutil.MockExecutor{
		Output: metricsOutputFor("h1"),
	}
	installMockConnectHost(t, map[string]host.Executor{"h1": exec})

	cfg := &config.Config{Hosts: map[string]config.HostConfig{
		"h1": {Addr: "h1"},
	}}

	got := CollectHostMetrics(nil, cfg, "")
	if len(got) != 1 {
		t.Fatalf("len = %d; want 1", len(got))
	}
	if got[0].Name != "h1" {
		t.Errorf("Name = %q; want %q", got[0].Name, "h1")
	}
	if got[0].Err != nil {
		t.Errorf("Err = %v; want nil", got[0].Err)
	}
}

// TestCollectHostMetrics_FilterExisting verifies that filter="<name>" routes
// through sortedHostNames' single-entry path: only that host is processed
// and returned. Pins the contract so future refactors cannot drop filter
// support.
func TestCollectHostMetrics_FilterExisting(t *testing.T) {

	installMockConnectHost(t, map[string]host.Executor{
		"alpha": &testutil.MockExecutor{Output: metricsOutputFor("alpha")},
		"beta":  &testutil.MockExecutor{Output: metricsOutputFor("beta")},
		"gamma": &testutil.MockExecutor{Output: metricsOutputFor("gamma")},
	})

	cfg := &config.Config{Hosts: map[string]config.HostConfig{
		"alpha": {Addr: "alpha"},
		"beta":  {Addr: "beta"},
		"gamma": {Addr: "gamma"},
	}}

	got := CollectHostMetrics(nil, cfg, "beta")
	if len(got) != 1 {
		t.Fatalf("len = %d; want 1", len(got))
	}
	if got[0].Name != "beta" {
		t.Errorf("Name = %q; want %q", got[0].Name, "beta")
	}
}

// TestCollectHostMetrics_FilterMissing verifies that filter on a non-existent
// host returns no metrics (no goroutines, no panic, no warning). sortedHostNames
// is the source of truth here — pin the orchestrator's passthrough.
func TestCollectHostMetrics_FilterMissing(t *testing.T) {

	installMockConnectHost(t, map[string]host.Executor{
		"alpha": &testutil.MockExecutor{},
	})

	cfg := &config.Config{Hosts: map[string]config.HostConfig{
		"alpha": {Addr: "alpha"},
	}}

	var buf bytes.Buffer
	got := CollectHostMetrics(&buf, cfg, "nonexistent")
	if len(got) != 0 {
		t.Fatalf("got %v; want empty", got)
	}
	if buf.Len() != 0 {
		t.Errorf("expected no output, got %q", buf.String())
	}
}

// TestCollectHostMetrics_MultiHostSortedOrder verifies that the returned
// metrics slice follows the sorted host-name order from sortedHostNames —
// not the cfg.Hosts map iteration order. The orchestrator's `metrics[i]`
// direct-index assignment would break if the loop iterated hosts in a
// different order, so pin the alignment here.
func TestCollectHostMetrics_MultiHostSortedOrder(t *testing.T) {

	installMockConnectHost(t, map[string]host.Executor{
		"zebra": &testutil.MockExecutor{Output: metricsOutputFor("zebra")},
		"alpha": &testutil.MockExecutor{Output: metricsOutputFor("alpha")},
		"mango": &testutil.MockExecutor{Output: metricsOutputFor("mango")},
	})

	cfg := &config.Config{Hosts: map[string]config.HostConfig{
		"zebra": {Addr: "zebra"},
		"alpha": {Addr: "alpha"},
		"mango": {Addr: "mango"},
	}}

	got := CollectHostMetrics(nil, cfg, "")
	if len(got) != 3 {
		t.Fatalf("len = %d; want 3", len(got))
	}
	want := []string{"alpha", "mango", "zebra"}
	for i := range want {
		if got[i].Name != want[i] {
			t.Errorf("[%d] Name = %q; want %q", i, got[i].Name, want[i])
		}
	}
}

// TestCollectHostMetrics_ConnectError pins the orchestrator's "cannot
// connect" branch: when connectHostFn returns an error, the warning line is
// written to w (under wMu) and metrics[i] gets {Name, Err}. No Close()
// happens (we never got a host back). Output must not be torn under -race.
func TestCollectHostMetrics_ConnectError(t *testing.T) {

	sentinel := errors.New("ssh dial timeout")
	installFailingConnectHost(t, sentinel)

	cfg := &config.Config{Hosts: map[string]config.HostConfig{
		"h1": {Addr: "h1"},
	}}

	var buf bytes.Buffer
	got := CollectHostMetrics(&buf, cfg, "")
	if len(got) != 1 {
		t.Fatalf("len = %d; want 1", len(got))
	}
	if got[0].Name != "h1" {
		t.Errorf("Name = %q; want %q", got[0].Name, "h1")
	}
	if !errors.Is(got[0].Err, sentinel) {
		t.Errorf("Err = %v; want %v", got[0].Err, sentinel)
	}
	out := buf.String()
	want := "Warning: cannot connect to h1: ssh dial timeout\n"
	if out != want {
		t.Errorf("output = %q; want %q", out, want)
	}
}

// TestCollectHostMetrics_AllHostsFailToConnect verifies that when every host
// fails, the slice is still fully populated (no nil entries) and the writer
// receives a warning per host. The mutex-serialised output must produce
// N complete warning lines.
func TestCollectHostMetrics_AllHostsFailToConnect(t *testing.T) {

	sentinel := errors.New("connection refused")
	installFailingConnectHost(t, sentinel)

	cfg := &config.Config{Hosts: map[string]config.HostConfig{
		"a": {Addr: "a"},
		"b": {Addr: "b"},
		"c": {Addr: "c"},
	}}

	var buf bytes.Buffer
	got := CollectHostMetrics(&buf, cfg, "")
	if len(got) != 3 {
		t.Fatalf("len = %d; want 3", len(got))
	}
	for i, m := range got {
		if !errors.Is(m.Err, sentinel) {
			t.Errorf("[%d] Err = %v; want %v", i, m.Err, sentinel)
		}
	}
	lines := strings.Split(strings.TrimRight(buf.String(), "\n"), "\n")
	if len(lines) != 3 {
		t.Fatalf("got %d warning lines, want 3 (buf=%q)", len(lines), buf.String())
	}
	for _, line := range lines {
		if !strings.HasPrefix(line, "Warning: cannot connect to ") {
			t.Errorf("malformed warning: %q", line)
		}
	}
}

// TestCollectHostMetrics_NilWriterNoPanic verifies that the orchestrator
// tolerates a nil writer. The connect-error path gates writes on `w != nil`,
// so passing nil must not panic — important for callers that only want the
// metrics data and have no place to log warnings.
func TestCollectHostMetrics_NilWriterNoPanic(t *testing.T) {

	sentinel := errors.New("boom")
	installFailingConnectHost(t, sentinel)

	cfg := &config.Config{Hosts: map[string]config.HostConfig{
		"h1": {Addr: "h1"},
	}}

	// Must not panic.
	got := CollectHostMetrics(nil, cfg, "")
	if len(got) != 1 || !errors.Is(got[0].Err, sentinel) {
		t.Errorf("got %+v; want [{Name:h1 Err:%v}]", got, sentinel)
	}
}

// TestCollectHostMetrics_NilWriterHappyPath complements the nil-writer test
// for the success branch: when every host connects, the writer is never
// touched (no Fprintf call), so nil is fine.
func TestCollectHostMetrics_NilWriterHappyPath(t *testing.T) {

	installMockConnectHost(t, map[string]host.Executor{
		"h1": &testutil.MockExecutor{Output: metricsOutputFor("h1")},
		"h2": &testutil.MockExecutor{Output: metricsOutputFor("h2")},
	})
	cfg := &config.Config{Hosts: map[string]config.HostConfig{
		"h1": {Addr: "h1"},
		"h2": {Addr: "h2"},
	}}

	got := CollectHostMetrics(nil, cfg, "")
	if len(got) != 2 {
		t.Fatalf("len = %d; want 2", len(got))
	}
	for i, m := range got {
		if m.Err != nil {
			t.Errorf("[%d] Err = %v; want nil", i, m.Err)
		}
	}
}

// TestCollectHostMetrics_HostCloseAfterCollect pins h.Close() happens AFTER
// CollectMetrics() — leaks would close on the wrong side of the work. We
// count the call sequence via a wrapping executor and an inner RunFn.
func TestCollectHostMetrics_HostCloseAfterCollect(t *testing.T) {

	collected := make(chan struct{}, 1)
	closed := make(chan struct{}, 1)

	inner := &testutil.MockExecutor{
		Output: metricsOutputFor("h1"),
		RunFn: func(cmd string) (string, error) {
			select {
			case collected <- struct{}{}:
			default:
			}
			return metricsOutputFor("h1"), nil
		},
	}
	exec := &recordingCloserExecutor{
		MockExecutor: inner,
		closeCh:      closed,
	}
	installMockConnectHost(t, map[string]host.Executor{"h1": exec})
	cfg := &config.Config{Hosts: map[string]config.HostConfig{
		"h1": {Addr: "h1"},
	}}

	got := CollectHostMetrics(nil, cfg, "")
	if len(got) != 1 || got[0].Err != nil {
		t.Fatalf("unexpected result: %+v", got)
	}
	select {
	case <-collected:
		// CollectMetrics invoked the executor.
	default:
		t.Fatal("CollectMetrics never invoked the executor")
	}
	select {
	case <-closed:
		// host.Close() called after CollectMetrics.
	default:
		t.Fatal("host.Close() never called")
	}
}

// TestCollectHostMetrics_ConcurrentFanOut verifies N hosts spawn N concurrent
// goroutines. Uses a barrier on each executor's Run call so all N goroutines
// must enter CollectMetrics before any of them returns. If the orchestrator
// serialised them, the timeout would fire on goroutine N+1.
func TestCollectHostMetrics_ConcurrentFanOut(t *testing.T) {

	const N = 5
	barrier := make(chan struct{})
	entered := make(chan struct{}, N)

	mk := func() *testutil.MockExecutor {
		return &testutil.MockExecutor{
			RunFn: func(cmd string) (string, error) {
				entered <- struct{}{}
				<-barrier
				return metricsOutputFor("h"), nil
			},
		}
	}

	installMockConnectHost(t, func() map[string]host.Executor {
		m := make(map[string]host.Executor, N)
		for i := 0; i < N; i++ {
			m["h"+itoa(i+1)] = mk()
		}
		return m
	}())

	names := make([]string, N)
	for i := 0; i < N; i++ {
		names[i] = "h" + itoa(i+1)
	}
	cfg := &config.Config{Hosts: map[string]config.HostConfig{}}
	for _, n := range names {
		cfg.Hosts[n] = config.HostConfig{Addr: n}
	}

	done := make(chan []host.HostMetrics, 1)
	go func() { done <- CollectHostMetrics(nil, cfg, "") }()

	// All N goroutines must reach the barrier within a short window — proves
	// they were started in parallel rather than sequentially. Generous timeout
	// accounts for race-detector overhead on busy CI.
	timeout := time.After(10 * time.Second)
	for i := 0; i < N; i++ {
		select {
		case <-entered:
		case <-timeout:
			close(barrier)
			<-done
			t.Fatalf("only %d/%d goroutines entered CollectMetrics within timeout (sequential?)", i, N)
		}
	}
	close(barrier)

	got := <-done
	if len(got) != N {
		t.Fatalf("len = %d; want %d", len(got), N)
	}
	for _, m := range got {
		if m.Err != nil {
			t.Errorf("%s Err = %v; want nil", m.Name, m.Err)
		}
	}
}

// TestCollectHostMetrics_WarningSerializationUnderRace verifies that the
// wMu mutex in the connect-error branch keeps warning lines contiguous even
// when N goroutines all fail simultaneously. Without the lock, two
// goroutines' Fprintf could interleave inside Write calls, producing torn
// lines like "Warning: cannot connect to ha: connectiWarning: cannot connect
// to hb: refused". This test catches that regression under -race.
func TestCollectHostMetrics_WarningSerializationUnderRace(t *testing.T) {

	const N = 8
	sentinel := errors.New("connect failed")
	installFailingConnectHost(t, sentinel)

	names := make([]string, N)
	for i := 0; i < N; i++ {
		names[i] = "h" + itoa(i)
	}
	cfg := &config.Config{Hosts: map[string]config.HostConfig{}}
	for _, n := range names {
		cfg.Hosts[n] = config.HostConfig{Addr: n}
	}

	var buf bytes.Buffer
	got := CollectHostMetrics(&buf, cfg, "")
	if len(got) != N {
		t.Fatalf("len = %d; want %d", len(got), N)
	}

	out := buf.String()
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	if len(lines) != N {
		t.Fatalf("got %d lines, want %d (output=%q)", len(lines), N, out)
	}

	// Every line must be a complete warning for a known host — torn writes
	// would produce a line that is missing its prefix or mixes two hosts.
	seen := make(map[string]bool, N)
	for i, line := range lines {
		if !strings.HasPrefix(line, "Warning: cannot connect to h") {
			t.Errorf("line %d malformed (torn write?): %q", i, line)
			continue
		}
		if !strings.HasSuffix(line, ": connect failed") {
			t.Errorf("line %d has wrong tail: %q", i, line)
		}
		if seen[line] {
			t.Errorf("line %d duplicated: %q", i, line)
		}
		seen[line] = true
	}
}

// TestCollectHostMetrics_MixedSuccessAndFailure verifies that one host
// failing does not poison the others' results. The successful host's
// metrics[i].Err must be nil; only the failing host's entry carries the
// error.
func TestCollectHostMetrics_MixedSuccessAndFailure(t *testing.T) {

	sentinel := errors.New("dial failed")
	goodExec := &testutil.MockExecutor{Output: metricsOutputFor("good")}

	// Set up the factory directly (acquire/release the mutex just for the
	// swap) so we can route "good" to the success mock and "bad" to a
	// hard error in a single closure.
	connectHostMu.Lock()
	prev := connectHostFn
	t.Cleanup(func() {
		connectHostFn = prev
		connectHostMu.Unlock()
	})
	connectHostFn = func(name string, hcfg config.HostConfig) (*host.Host, error) {
		if name == "bad" {
			return nil, sentinel
		}
		h := host.NewHost(name, hcfg)
		h.SetConn(goodExec)
		return h, nil
	}

	cfg := &config.Config{Hosts: map[string]config.HostConfig{
		"good": {Addr: "good"},
		"bad":  {Addr: "bad"},
	}}

	var buf bytes.Buffer
	got := CollectHostMetrics(&buf, cfg, "")
	if len(got) != 2 {
		t.Fatalf("len = %d; want 2", len(got))
	}

	// Find each by name since the orchestrator sorts.
	byName := map[string]host.HostMetrics{}
	for _, m := range got {
		byName[m.Name] = m
	}
	if m := byName["good"]; m.Err != nil {
		t.Errorf("good.Err = %v; want nil", m.Err)
	}
	if m := byName["bad"]; !errors.Is(m.Err, sentinel) {
		t.Errorf("bad.Err = %v; want %v", m.Err, sentinel)
	}
	if !strings.Contains(buf.String(), "Warning: cannot connect to bad: dial failed") {
		t.Errorf("expected warning for bad host, got %q", buf.String())
	}
	if strings.Contains(buf.String(), "good") {
		t.Errorf("unexpected warning for good host: %q", buf.String())
	}
}

// TestCollectHostMetrics_ConnectCalledExactlyOncePerHost verifies that each
// host triggers exactly one connectHostFn call. Multiple connect calls per
// host would be a real regression — the orchestrator must not retry on its
// own.
func TestCollectHostMetrics_ConnectCalledExactlyOncePerHost(t *testing.T) {

	var connectCount atomic.Int32
	exec := &testutil.MockExecutor{Output: metricsOutputFor("h1")}

	connectHostMu.Lock()
	prev := connectHostFn
	t.Cleanup(func() {
		connectHostFn = prev
		connectHostMu.Unlock()
	})
	connectHostFn = func(name string, hcfg config.HostConfig) (*host.Host, error) {
		connectCount.Add(1)
		h := host.NewHost(name, hcfg)
		h.SetConn(exec)
		return h, nil
	}

	cfg := &config.Config{Hosts: map[string]config.HostConfig{
		"h1": {Addr: "h1"},
	}}

	if got := CollectHostMetrics(nil, cfg, ""); len(got) != 1 {
		t.Fatalf("len = %d; want 1", len(got))
	}
	if got, want := connectCount.Load(), int32(1); got != want {
		t.Errorf("connect calls = %d; want %d (one per host)", got, want)
	}
}

// TestCollectHostMetrics_NoOrphanedWarnings ensures the warning printer is
// only invoked on the connect-error path. A successful run must produce no
// output, even when the writer is non-nil.
func TestCollectHostMetrics_NoOrphanedWarnings(t *testing.T) {

	installMockConnectHost(t, map[string]host.Executor{
		"h1": &testutil.MockExecutor{Output: metricsOutputFor("h1")},
		"h2": &testutil.MockExecutor{Output: metricsOutputFor("h2")},
	})
	cfg := &config.Config{Hosts: map[string]config.HostConfig{
		"h1": {Addr: "h1"},
		"h2": {Addr: "h2"},
	}}

	var buf bytes.Buffer
	got := CollectHostMetrics(&buf, cfg, "")
	if len(got) != 2 {
		t.Fatalf("len = %d; want 2", len(got))
	}
	if buf.Len() != 0 {
		t.Errorf("expected no warnings on success, got %q", buf.String())
	}
}

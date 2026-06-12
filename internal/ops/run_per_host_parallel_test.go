package ops

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/an-lee/gh-sr/internal/config"
	"github.com/an-lee/gh-sr/internal/host"
	"github.com/an-lee/gh-sr/internal/testutil"
)

// connectHostMu serialises swaps of the package-level connectHostFn across
// parallel tests so the race detector does not flag them. Tests that swap the
// factory must hold this mutex for the duration of their swap+orchestrator-call.
var connectHostMu sync.Mutex

// installMockConnectHost swaps connectHostFn for the duration of the test so
// the orchestrator under test receives a *host.Host with a testutil.MockExecutor
// already wired in (bypassing SSH). Holds connectHostMu for the duration of
// the test and restores the previous factory + releases the mutex in t.Cleanup.
func installMockConnectHost(t *testing.T, factories map[string]host.Executor) {
	t.Helper()
	connectHostMu.Lock()
	prev := connectHostFn
	t.Cleanup(func() {
		connectHostFn = prev
		connectHostMu.Unlock()
	})
	connectHostFn = func(name string, hcfg config.HostConfig) (*host.Host, error) {
		mock, ok := factories[name]
		if !ok {
			return nil, errors.New("no mock registered for host " + name)
		}
		h := host.NewHost(name, hcfg)
		h.SetConn(mock)
		return h, nil
	}
}

// installFailingConnectHost makes connectHostFn return a hard error for every
// host. Used to exercise the orchestrator's "cannot connect" branch.
func installFailingConnectHost(t *testing.T, sentinel error) {
	t.Helper()
	connectHostMu.Lock()
	prev := connectHostFn
	t.Cleanup(func() {
		connectHostFn = prev
		connectHostMu.Unlock()
	})
	connectHostFn = func(string, config.HostConfig) (*host.Host, error) {
		return nil, sentinel
	}
}

// cfgWithHosts builds a config with the given host entries pre-populated so
// runPerHostParallel's `cfg.Hosts[g.name]` lookup succeeds.
func cfgWithHosts(names ...string) *config.Config {
	c := &config.Config{Hosts: make(map[string]config.HostConfig)}
	for _, n := range names {
		c.Hosts[n] = config.HostConfig{Addr: n}
	}
	return c
}

// TestRunPerHostParallel_EmptyRunners verifies that an empty/nil runner slice
// takes the fast path: no ConnectHost call, fn never invoked, returns nil.
// Catches a regression where the orchestrator panics or makes a phantom
// connection on empty input.
func TestRunPerHostParallel_EmptyRunners(t *testing.T) {
	t.Parallel()

	var fnCalls atomic.Int32
	installMockConnectHost(t, map[string]host.Executor{})

	cfg := cfgWithHosts()
	err := runPerHostParallel(nil, cfg, nil, func(_ io.Writer, _ *host.Host, _ config.RunnerConfig) error {
		fnCalls.Add(1)
		return nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := fnCalls.Load(); got != 0 {
		t.Errorf("fn should not be called for empty runners, got %d calls", got)
	}
}

// TestRunPerHostParallel_SingleHostSequentialCalls pins the SSH-amortisation
// guarantee: when N runners share a host, fn is called N times on the same
// *host.Host (one SSH round trip), in input order. Pins both ConnectHost
// being called exactly once and the sequential ordering.
func TestRunPerHostParallel_SingleHostSequentialCalls(t *testing.T) {
	t.Parallel()

	var connectCount atomic.Int32
	exec := &testutil.MockExecutor{}
	installMockConnectHost(t, map[string]host.Executor{"h1": exec})

	prev := connectHostFn
	connectHostFn = func(name string, hcfg config.HostConfig) (*host.Host, error) {
		connectCount.Add(1)
		return prev(name, hcfg)
	}

	runners := []config.RunnerConfig{
		{Name: "first", Host: "h1"},
		{Name: "second", Host: "h1"},
		{Name: "third", Host: "h1"},
	}

	var order []string
	var orderMu sync.Mutex
	err := runPerHostParallel(nil, cfgWithHosts("h1"), runners, func(_ io.Writer, _ *host.Host, rc config.RunnerConfig) error {
		orderMu.Lock()
		order = append(order, rc.Name)
		orderMu.Unlock()
		return nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got, want := connectCount.Load(), int32(1); got != want {
		t.Errorf("connect count = %d; want %d (one SSH connection per host group)", got, want)
	}
	want := []string{"first", "second", "third"}
	if len(order) != len(want) {
		t.Fatalf("order len = %d; want %d", len(order), len(want))
	}
	for i := range want {
		if order[i] != want[i] {
			t.Errorf("order[%d] = %q; want %q", i, order[i], want[i])
		}
	}
}

// TestRunPerHostParallel_MultiHostConcurrent verifies that host groups run in
// parallel: N hosts ⇒ N goroutines reach fn concurrently, not sequentially.
// Uses a barrier on the mock executor so all N goroutines must enter fn before
// any of them returns. If the orchestrator spawned them sequentially, only
// one would reach fn before the timeout.
func TestRunPerHostParallel_MultiHostConcurrent(t *testing.T) {
	t.Parallel()

	const N = 5
	barrier := make(chan struct{})
	entered := make(chan struct{}, N)

	installMockConnectHost(t, map[string]host.Executor{
		"h1": &testutil.MockExecutor{},
		"h2": &testutil.MockExecutor{},
		"h3": &testutil.MockExecutor{},
		"h4": &testutil.MockExecutor{},
		"h5": &testutil.MockExecutor{},
	})

	runners := make([]config.RunnerConfig, N)
	hostNames := make([]string, N)
	for i := 0; i < N; i++ {
		hostNames[i] = "h" + itoa(i+1)
		runners[i] = config.RunnerConfig{Name: "r", Host: hostNames[i]}
	}

	cfg := cfgWithHosts(hostNames...)

	done := make(chan error, 1)
	go func() {
		done <- runPerHostParallel(nil, cfg, runners, func(_ io.Writer, _ *host.Host, _ config.RunnerConfig) error {
			entered <- struct{}{}
			<-barrier
			return nil
		})
	}()

	// All N goroutines must reach the barrier within a short window — proves
	// they were started in parallel rather than sequentially. Generous timeout
	// accounts for race-detector overhead on busy CI.
	timeout := time.After(10 * time.Second)
	for i := 0; i < N; i++ {
		select {
		case <-entered:
		case <-timeout:
			close(barrier)
			t.Fatalf("only %d/%d goroutines entered fn within timeout (sequential?)", i, N)
		}
	}
	close(barrier)
	if err := <-done; err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// TestRunPerHostParallel_ConnectError verifies that a connect failure on any
// host propagates as the orchestrator's error and fn is never invoked. The
// first failing host in the results slice wins (no aggregation — single
// error return).
func TestRunPerHostParallel_ConnectError(t *testing.T) {
	t.Parallel()

	sentinel := errors.New("ssh dial timeout")
	installFailingConnectHost(t, sentinel)

	var fnCalls atomic.Int32
	runners := []config.RunnerConfig{
		{Name: "a", Host: "h1"},
		{Name: "b", Host: "h2"},
	}

	err := runPerHostParallel(nil, cfgWithHosts("h1", "h2"), runners, func(_ io.Writer, _ *host.Host, _ config.RunnerConfig) error {
		fnCalls.Add(1)
		return nil
	})
	if !errors.Is(err, sentinel) {
		t.Fatalf("got %v; want %v", err, sentinel)
	}
	if got := fnCalls.Load(); got != 0 {
		t.Errorf("fn called %d times; want 0 (connect failure)", got)
	}
}

// TestRunPerHostParallel_FnErrorEarlyExit verifies that when fn returns an
// error on a runner, the orchestrator does NOT call fn for the remaining
// runners on the same host group. This is the "single SSH connection, stop
// on first failure" contract — important because subsequent runners on the
// same host would otherwise be configured-but-not-stopped.
func TestRunPerHostParallel_FnErrorEarlyExit(t *testing.T) {
	t.Parallel()

	installMockConnectHost(t, map[string]host.Executor{
		"h1": &testutil.MockExecutor{},
	})

	fnErr := errors.New("boom")
	runners := []config.RunnerConfig{
		{Name: "first", Host: "h1"},
		{Name: "second", Host: "h1"}, // must NOT be reached
		{Name: "third", Host: "h1"},  // must NOT be reached
	}

	var called []string
	var callMu sync.Mutex
	err := runPerHostParallel(nil, cfgWithHosts("h1"), runners, func(_ io.Writer, _ *host.Host, rc config.RunnerConfig) error {
		callMu.Lock()
		called = append(called, rc.Name)
		callMu.Unlock()
		if rc.Name == "first" {
			return fnErr
		}
		return nil
	})
	if !errors.Is(err, fnErr) {
		t.Fatalf("got %v; want %v", err, fnErr)
	}
	if len(called) != 1 || called[0] != "first" {
		t.Fatalf("called = %v; want [first]", called)
	}
}

// TestRunPerHostParallel_HostCloseAfterFn verifies the orchestrator's
// h.Close() happens exactly once per host group, after fn finishes (so
// long-running fns keep the host connection alive). We hook Close via a
// wrapping executor.
func TestRunPerHostParallel_HostCloseAfterFn(t *testing.T) {
	t.Parallel()

	closeCount := make(chan struct{}, 1)
	exec := &recordingCloserExecutor{
		MockExecutor: &testutil.MockExecutor{},
		closeCh:      closeCount,
	}
	installMockConnectHost(t, map[string]host.Executor{"h1": exec})

	runFnReturned := make(chan struct{})
	runners := []config.RunnerConfig{{Name: "r", Host: "h1"}}
	err := runPerHostParallel(nil, cfgWithHosts("h1"), runners, func(_ io.Writer, _ *host.Host, _ config.RunnerConfig) error {
		// fn is running; host must still be open.
		select {
		case <-closeCount:
			t.Errorf("host.Close() called BEFORE fn returned")
		default:
		}
		close(runFnReturned)
		return nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	select {
	case <-closeCount:
	case <-time.After(time.Second):
		t.Fatal("host.Close() never called")
	}
}

// TestRunPerHostParallel_NilWriterPassesDiscard verifies the orchestrator
// hands fn an io.Discard sink when called with w=nil, instead of panicking
// or forwarding nil. Catches a regression where the lockedWriter wiring
// dereferences nil.
func TestRunPerHostParallel_NilWriterPassesDiscard(t *testing.T) {
	t.Parallel()

	installMockConnectHost(t, map[string]host.Executor{"h1": &testutil.MockExecutor{}})
	runners := []config.RunnerConfig{{Name: "r", Host: "h1"}}

	// Should not panic on nil w.
	err := runPerHostParallel(nil, cfgWithHosts("h1"), runners, func(w io.Writer, _ *host.Host, _ config.RunnerConfig) error {
		if w == nil {
			t.Errorf("fn received nil Writer")
		}
		return nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// TestRunPerHostParallel_LockedWriterSerializesOutput verifies that the
// lockedWriter passed to fn prevents torn writes when N host goroutines all
// write a complete message via a single Write call. Without the lock, two
// goroutines calling Write simultaneously could produce a single buffer that
// mixes the bytes (e.g. "host=h0 ihost=h1nstance=r0\n") — breaking log
// parsing in downstream tooling. The lockedWriter serialises each Write call,
// so per-message bytes always arrive contiguously.
func TestRunPerHostParallel_LockedWriterSerializesOutput(t *testing.T) {
	t.Parallel()

	const N = 8

	installMockConnectHost(t, func() map[string]host.Executor {
		m := make(map[string]host.Executor, N)
		for i := 0; i < N; i++ {
			m["h"+itoa(i)] = &testutil.MockExecutor{}
		}
		return m
	}())

	runners := make([]config.RunnerConfig, N)
	hostNames := make([]string, N)
	for i := 0; i < N; i++ {
		hostNames[i] = "h" + itoa(i)
		runners[i] = config.RunnerConfig{Name: "r" + itoa(i), Host: hostNames[i]}
	}

	var buf bytes.Buffer
	cfg := &config.Config{Hosts: make(map[string]config.HostConfig)}
	for _, n := range hostNames {
		cfg.Hosts[n] = config.HostConfig{Addr: n}
	}

	err := runPerHostParallel(&buf, cfg, runners, func(w io.Writer, _ *host.Host, rc config.RunnerConfig) error {
		// Single Write call per message so the lockedWriter can guarantee
		// atomic delivery. Pre-formatted with sprintf to ensure no fmt
		// internals split the call.
		msg := fmt.Sprintf("host=%s instance=%s\n", rc.Host, rc.Name)
		if _, err := w.Write([]byte(msg)); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Each line must be exactly one complete "host=hN instance=rN\n". If
	// writes interleaved within a single Write call (the only thing the lock
	// protects), we'd see split tokens.
	lines := strings.Split(strings.TrimRight(buf.String(), "\n"), "\n")
	if len(lines) != N {
		t.Fatalf("got %d lines, want %d (buf=%q)", len(lines), N, buf.String())
	}
	seen := make(map[string]bool, N)
	for i, line := range lines {
		if !strings.HasPrefix(line, "host=h") || !strings.Contains(line, " instance=r") {
			t.Errorf("line %d malformed: %q", i, line)
			continue
		}
		if seen[line] {
			t.Errorf("line %d duplicated: %q (torn write would cause this)", i, line)
		}
		seen[line] = true
	}
}

// recordingCloserExecutor wraps a testutil.MockExecutor and signals on Close
// so TestRunPerHostParallel_HostCloseAfterFn can assert ordering.
type recordingCloserExecutor struct {
	*testutil.MockExecutor
	closeCh chan struct{}
	once    sync.Once
}

func (r *recordingCloserExecutor) Close() error {
	r.once.Do(func() { close(r.closeCh) })
	return r.MockExecutor.Close()
}

// itoa is a tiny strconv-free integer formatter used to keep the multi-host
// test readable. Negative numbers panic (caller's contract).
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var digits []byte
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	return string(digits)
}

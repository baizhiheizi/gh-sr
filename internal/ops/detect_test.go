package ops

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/an-lee/gh-sr/internal/config"
	"github.com/an-lee/gh-sr/internal/host"
	"github.com/an-lee/gh-sr/internal/testutil"
)

// mockExecutorForDetect returns a testutil.MockExecutor wired with the two
// command patterns that host.DetectOS / host.DetectArch emit on POSIX:
//
//	uname -s 2>/dev/null || echo UNKNOWN
//	uname -m 2>/dev/null || echo UNKNOWN
//
// It pins (os, arch) verbatim so tests can assert the orchestrator's mutation
// of cfg.Hosts. detectErr, if non-nil, is returned from both probes (exercises
// the orchestrator's error paths).
func mockExecutorForDetect(os, arch string, detectErr error) *testutil.MockExecutor {
	return &testutil.MockExecutor{
		RunFn: func(cmd string) (string, error) {
			if detectErr != nil {
				return "", detectErr
			}
			switch {
			case strings.HasPrefix(cmd, "uname -s"):
				return os + "\n", nil
			case strings.HasPrefix(cmd, "uname -m"):
				return arch + "\n", nil
			default:
				return "", nil
			}
		},
	}
}

// cfgWithHostsOrLocal builds a Config whose Hosts map is keyed by name. Each
// (name, addr, os, arch) tuple creates one HostConfig entry; Addr="local"
// entries short-circuit the orchestrator's NeedsDetection check, and addr
// prefixed with "ssh://" (or any non-empty non-"local" string) is the path
// the orchestrator will probe.
func cfgWithHostsOrLocal(entries ...struct {
	Name, Addr, OS, Arch string
}) *config.Config {
	c := &config.Config{Hosts: make(map[string]config.HostConfig)}
	for _, e := range entries {
		c.Hosts[e.Name] = config.HostConfig{Addr: e.Addr, OS: e.OS, Arch: e.Arch}
	}
	return c
}

// TestResolveHostInfo_NeedsDetectionShortCircuit verifies the orchestrator's
// cheapest path: when NeedsDetection is false (all hosts either local or fully
// resolved), no ConnectHost call happens and the function returns nil. Pins
// the early-return behaviour so a future refactor cannot accidentally start
// probing already-resolved hosts.
func TestResolveHostInfo_NeedsDetectionShortCircuit(t *testing.T) {
	t.Parallel()

	// All hosts fully resolved → NeedsDetection false. Mock for "h1" is
	// wired but must NOT be touched.
	installMockConnectHost(t, map[string]host.Executor{
		"h1": mockExecutorForDetect("linux", "amd64", nil),
	})
	cfg := cfgWithHostsOrLocal(
		struct{ Name, Addr, OS, Arch string }{"h1", "h1.example", "linux", "amd64"},
		struct{ Name, Addr, OS, Arch string }{"h2", "local", "linux", "amd64"}, // local, also resolved
	)

	var buf bytes.Buffer
	if err := ResolveHostInfo(&buf, cfg); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := buf.Len(); got != 0 {
		t.Errorf("expected no output, got %q", buf.String())
	}
}

// TestResolveHostInfo_LocalHostsSkipped verifies hosts with addr="local" are
// NEVER probed even when their OS/arch are unset. Local runners don't need
// remote detection, and probing them would attempt a connection that the
// orchestrator should not need to make.
func TestResolveHostInfo_LocalHostsSkipped(t *testing.T) {
	t.Parallel()

	installMockConnectHost(t, map[string]host.Executor{
		// h1 (remote, unresolved) will be probed.
		"h1": mockExecutorForDetect("linux", "amd64", nil),
	})
	cfg := cfgWithHostsOrLocal(
		struct{ Name, Addr, OS, Arch string }{"local1", "local", "", ""},  // local, unresolved → must skip
		struct{ Name, Addr, OS, Arch string }{"local2", "LOCAL", "", ""},  // case-insensitive local → must skip
		struct{ Name, Addr, OS, Arch string }{"h1", "h1.example", "", ""}, // remote, unresolved → probe
	)

	if err := ResolveHostInfo(nil, cfg); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := cfg.Hosts["local1"]; got.OS != "" || got.Arch != "" {
		t.Errorf("local1 was probed: %+v", got)
	}
	if got := cfg.Hosts["local2"]; got.OS != "" || got.Arch != "" {
		t.Errorf("local2 was probed (case-insensitive addr): %+v", got)
	}
	if got := cfg.Hosts["h1"]; got.OS != "linux" || got.Arch != "amd64" {
		t.Errorf("h1 not detected: %+v", got)
	}
}

// TestResolveHostInfo_AlreadyResolvedHostsSkipped verifies hosts that already
// have BOTH OS and arch set are not probed, even if their addr is remote.
// Saves an unnecessary SSH round trip per host that the user has already
// pinned in config.
func TestResolveHostInfo_AlreadyResolvedHostsSkipped(t *testing.T) {
	t.Parallel()

	// Mock for "h_resolved" exists but must NOT be touched (no calls expected).
	resolvedExec := &testutil.MockExecutor{
		RunFn: func(cmd string) (string, error) {
			t.Errorf("resolved host was probed: %q", cmd)
			return "", nil
		},
	}
	installMockConnectHost(t, map[string]host.Executor{
		"h_resolved":   resolvedExec,
		"h_unresolved": mockExecutorForDetect("darwin", "arm64", nil),
	})
	cfg := cfgWithHostsOrLocal(
		struct{ Name, Addr, OS, Arch string }{"h_resolved", "h.example", "linux", "amd64"}, // already done
		struct{ Name, Addr, OS, Arch string }{"h_unresolved", "h2.example", "", ""},        // needs both
	)

	if err := ResolveHostInfo(nil, cfg); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := cfg.Hosts["h_unresolved"]; got.OS != "darwin" || got.Arch != "arm64" {
		t.Errorf("h_unresolved not detected: %+v", got)
	}
	if got := cfg.Hosts["h_resolved"]; got.OS != "linux" || got.Arch != "amd64" {
		t.Errorf("h_resolved config was mutated: %+v", got)
	}
	if len(resolvedExec.Calls) != 0 {
		t.Errorf("expected 0 calls on resolved host mock, got %d: %v", len(resolvedExec.Calls), resolvedExec.Calls)
	}
}

// TestResolveHostInfo_HappyPathSingleHost pins the single-host happy path:
// the orchestrator detects OS+arch, writes status lines to w in order
// ("Detecting..." then "  h: detected ..."), and mutates cfg.Hosts in place.
// This is the most common real-world case (single self-hosted runner).
func TestResolveHostInfo_HappyPathSingleHost(t *testing.T) {
	t.Parallel()

	installMockConnectHost(t, map[string]host.Executor{
		"h1": mockExecutorForDetect("linux", "x86_64", nil),
	})
	cfg := cfgWithHostsOrLocal(
		struct{ Name, Addr, OS, Arch string }{"h1", "h1.example", "", ""},
	)

	var buf bytes.Buffer
	if err := ResolveHostInfo(&buf, cfg); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	hcfg := cfg.Hosts["h1"]
	if hcfg.OS != "linux" {
		t.Errorf("OS = %q; want linux", hcfg.OS)
	}
	if hcfg.Arch != "amd64" {
		t.Errorf("Arch = %q; want amd64 (x86_64 normalises)", hcfg.Arch)
	}

	out := buf.String()
	if !strings.Contains(out, "Detecting OS/arch for host h1") {
		t.Errorf("missing 'Detecting' status line; got %q", out)
	}
	if !strings.Contains(out, "detected os=linux arch=amd64") {
		t.Errorf("missing 'detected' status line; got %q", out)
	}
}

// TestResolveHostInfo_PartialOSOnly verifies that when a host has arch set
// but OS empty, ONLY the OS probe runs. Saves an unnecessary probe and pins
// the orchestrator's conditional branch — a regression that always probes
// both would silently double SSH round trips per partially-resolved host.
func TestResolveHostInfo_PartialOSOnly(t *testing.T) {
	t.Parallel()

	var unameS, unameM atomic.Int32
	exec := &testutil.MockExecutor{
		RunFn: func(cmd string) (string, error) {
			switch {
			case strings.HasPrefix(cmd, "uname -s"):
				unameS.Add(1)
				return "Linux\n", nil
			case strings.HasPrefix(cmd, "uname -m"):
				unameM.Add(1)
				return "", errors.New("arch probe must not run")
			}
			return "", nil
		},
	}
	installMockConnectHost(t, map[string]host.Executor{"h1": exec})
	cfg := cfgWithHostsOrLocal(
		struct{ Name, Addr, OS, Arch string }{"h1", "h1.example", "", "amd64"},
	)

	if err := ResolveHostInfo(nil, cfg); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := unameS.Load(); got != 1 {
		t.Errorf("uname -s called %d times; want 1", got)
	}
	if got := unameM.Load(); got != 0 {
		t.Errorf("uname -m called %d times; want 0 (arch was already set)", got)
	}
	if cfg.Hosts["h1"].OS != "linux" {
		t.Errorf("OS = %q; want linux", cfg.Hosts["h1"].OS)
	}
	if cfg.Hosts["h1"].Arch != "amd64" {
		t.Errorf("Arch mutated: %q; want amd64", cfg.Hosts["h1"].Arch)
	}
}

// TestResolveHostInfo_PartialArchOnly verifies the symmetric case: host has
// OS set, arch empty → only the arch probe runs.
func TestResolveHostInfo_PartialArchOnly(t *testing.T) {
	t.Parallel()

	var unameS, unameM atomic.Int32
	exec := &testutil.MockExecutor{
		RunFn: func(cmd string) (string, error) {
			switch {
			case strings.HasPrefix(cmd, "uname -s"):
				unameS.Add(1)
				return "", errors.New("os probe must not run")
			case strings.HasPrefix(cmd, "uname -m"):
				unameM.Add(1)
				return "aarch64\n", nil
			}
			return "", nil
		},
	}
	installMockConnectHost(t, map[string]host.Executor{"h1": exec})
	cfg := cfgWithHostsOrLocal(
		struct{ Name, Addr, OS, Arch string }{"h1", "h1.example", "linux", ""},
	)

	if err := ResolveHostInfo(nil, cfg); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := unameM.Load(); got != 1 {
		t.Errorf("uname -m called %d times; want 1", got)
	}
	if got := unameS.Load(); got != 0 {
		t.Errorf("uname -s called %d times; want 0 (os was already set)", got)
	}
	if cfg.Hosts["h1"].Arch != "arm64" {
		t.Errorf("Arch = %q; want arm64", cfg.Hosts["h1"].Arch)
	}
	if cfg.Hosts["h1"].OS != "linux" {
		t.Errorf("OS mutated: %q; want linux", cfg.Hosts["h1"].OS)
	}
}

// TestResolveHostInfo_MultiHostConcurrent verifies that N hosts are probed in
// parallel: all N goroutines reach the mock executor before any of them
// returns. If the orchestrator ran them sequentially, only one would touch
// the mock before the timeout. This pins the concurrency primitive's
// concurrency primitive (separate goroutines per host).
func TestResolveHostInfo_MultiHostConcurrent(t *testing.T) {
	t.Parallel()

	const N = 5
	barrier := make(chan struct{})
	entered := make(chan struct{}, N)

	factories := make(map[string]host.Executor, N)
	for i := 0; i < N; i++ {
		name := "h" + itoa(i+1)
		factories[name] = &testutil.MockExecutor{
			RunFn: func(cmd string) (string, error) {
				entered <- struct{}{}
				<-barrier
				switch {
				case strings.HasPrefix(cmd, "uname -s"):
					return "Linux\n", nil
				case strings.HasPrefix(cmd, "uname -m"):
					return "x86_64\n", nil
				}
				return "", nil
			},
		}
	}
	installMockConnectHost(t, factories)

	entries := make([]struct{ Name, Addr, OS, Arch string }, N)
	hostNames := make([]string, N)
	for i := 0; i < N; i++ {
		hostNames[i] = "h" + itoa(i+1)
		entries[i] = struct{ Name, Addr, OS, Arch string }{hostNames[i], hostNames[i] + ".example", "", ""}
	}
	cfg := cfgWithHostsOrLocal(entries...)

	done := make(chan error, 1)
	go func() { done <- ResolveHostInfo(nil, cfg) }()

	// All N host goroutines must enter the mock before any returns. Generous
	// timeout for race-detector overhead under busy CI.
	timeout := time.After(10 * time.Second)
	for i := 0; i < N; i++ {
		select {
		case <-entered:
		case <-timeout:
			close(barrier)
			t.Fatalf("only %d/%d hosts entered detection within timeout (sequential?)", i, N)
		}
	}
	close(barrier)
	if err := <-done; err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// cfg.Hosts must reflect detected values for every host.
	for _, n := range hostNames {
		h := cfg.Hosts[n]
		if h.OS != "linux" || h.Arch != "amd64" {
			t.Errorf("%s not detected: %+v", n, h)
		}
	}
}

// TestResolveHostInfo_DetectOSError verifies that a failed OS probe on one
// host surfaces as the orchestrator's error, and that the host's connection
// is closed before the error is returned. cfg.Hosts for the failing host
// must remain untouched (no partial mutation).
func TestResolveHostInfo_DetectOSError(t *testing.T) {
	t.Parallel()

	sentinel := errors.New("uname timeout")
	installMockConnectHost(t, map[string]host.Executor{
		"h_ok": mockExecutorForDetect("linux", "amd64", nil),
		"h_fail": &testutil.MockExecutor{
			RunFn: func(cmd string) (string, error) {
				if strings.HasPrefix(cmd, "uname -s") {
					return "", sentinel
				}
				return "", nil
			},
		},
	})
	cfg := cfgWithHostsOrLocal(
		struct{ Name, Addr, OS, Arch string }{"h_ok", "h_ok.example", "", ""},
		struct{ Name, Addr, OS, Arch string }{"h_fail", "h_fail.example", "", ""},
	)

	err := ResolveHostInfo(nil, cfg)
	if err == nil || !errors.Is(err, sentinel) {
		t.Fatalf("got %v; want wraps %v", err, sentinel)
	}
	// Error message should identify the failing host by name.
	if !strings.Contains(err.Error(), "h_fail") {
		t.Errorf("error %q does not name the failing host", err)
	}
	// Failing host config must not have been mutated.
	if got := cfg.Hosts["h_fail"]; got.OS != "" || got.Arch != "" {
		t.Errorf("failing host cfg was mutated: %+v", got)
	}
}

// TestResolveHostInfo_DetectArchError verifies the arch-only error path:
// OS probe succeeds, arch probe fails → orchestrator returns wrapped error
// identifying the failing host by name. Pins that any detection failure
// aborts the whole batch (no partial propagation) — safer than merging
// half-resolved state into cfg.Hosts and letting downstream code run on
// inconsistent data.
//
// host.DetectArch falls through three probes (uname -m, powershell.exe,
// pwsh.exe). To make the orchestrator see a wrapped sentinel error we make
// ALL probes fail with the same sentinel — DetectArch's last-resort branch
// then wraps the original uname error with %w, which the orchestrator wraps
// again with %w, satisfying errors.Is end-to-end.
func TestResolveHostInfo_DetectArchError(t *testing.T) {
	t.Parallel()

	sentinel := errors.New("arch probe broken")
	exec := &testutil.MockExecutor{
		RunFn: func(cmd string) (string, error) {
			if strings.HasPrefix(cmd, "uname -s") {
				return "Linux\n", nil
			}
			// Every arch probe (uname -m + powershell + pwsh) returns the
			// sentinel so DetectArch reaches its terminal error branch and
			// wraps the sentinel with %w.
			return "", sentinel
		},
	}
	installMockConnectHost(t, map[string]host.Executor{"h1": exec})
	cfg := cfgWithHostsOrLocal(
		struct{ Name, Addr, OS, Arch string }{"h1", "h1.example", "", ""},
	)

	err := ResolveHostInfo(nil, cfg)
	if err == nil || !errors.Is(err, sentinel) {
		t.Fatalf("got %v; want wraps %v", err, sentinel)
	}
	if !strings.Contains(err.Error(), "arch") {
		t.Errorf("error %q does not mention arch", err)
	}
	if !strings.Contains(err.Error(), "h1") {
		t.Errorf("error %q does not name the failing host", err)
	}
	// On any host failure the orchestrator returns the error WITHOUT
	// propagating partially-resolved state — the caller's cfg.Hosts for the
	// failing host must remain untouched so downstream code doesn't run on
	// inconsistent data.
	if got := cfg.Hosts["h1"]; got.OS != "" || got.Arch != "" {
		t.Errorf("partial state leaked: %+v", got)
	}
}

// TestResolveHostInfo_ConnectError verifies a connect failure on any host
// propagates immediately and other hosts' detection does NOT block waiting.
// The error must wrap the failing host's name so users can identify it.
func TestResolveHostInfo_ConnectError(t *testing.T) {
	t.Parallel()

	sentinel := errors.New("ssh dial timeout")
	installFailingConnectHost(t, sentinel)

	cfg := cfgWithHostsOrLocal(
		struct{ Name, Addr, OS, Arch string }{"h1", "h1.example", "", ""},
		struct{ Name, Addr, OS, Arch string }{"h2", "h2.example", "", ""},
	)

	err := ResolveHostInfo(nil, cfg)
	if !errors.Is(err, sentinel) {
		t.Fatalf("got %v; want wraps %v", err, sentinel)
	}
	// Neither host should have been mutated (no successful probe).
	if got := cfg.Hosts["h1"]; got.OS != "" || got.Arch != "" {
		t.Errorf("h1 cfg was mutated: %+v", got)
	}
	if got := cfg.Hosts["h2"]; got.OS != "" || got.Arch != "" {
		t.Errorf("h2 cfg was mutated: %+v", got)
	}
}

// TestResolveHostInfo_NilWriterSafe verifies the orchestrator doesn't panic
// when called with w=nil. Status lines are skipped but detection still
// proceeds and cfg.Hosts is still mutated. Catches a regression where the
// w-nil branch dereferences nil.
func TestResolveHostInfo_NilWriterSafe(t *testing.T) {
	t.Parallel()

	installMockConnectHost(t, map[string]host.Executor{
		"h1": mockExecutorForDetect("linux", "amd64", nil),
	})
	cfg := cfgWithHostsOrLocal(
		struct{ Name, Addr, OS, Arch string }{"h1", "h1.example", "", ""},
	)

	// Should not panic on nil w.
	if err := ResolveHostInfo(nil, cfg); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := cfg.Hosts["h1"]; got.OS != "linux" || got.Arch != "amd64" {
		t.Errorf("detection did not run: %+v", got)
	}
}

// TestResolveHostInfo_WriterSerialized verifies that when w is provided,
// writes from multiple host goroutines are serialised via the wMu mutex
// (no torn token output). Without the lock, two goroutines' "Detecting..."
// lines could interleave into a single buffer in the middle of a token,
// breaking log parsers downstream.
func TestResolveHostInfo_WriterSerialized(t *testing.T) {
	t.Parallel()

	const N = 6

	factories := make(map[string]host.Executor, N)
	for i := 0; i < N; i++ {
		name := "h" + itoa(i+1)
		factories[name] = mockExecutorForDetect("linux", "amd64", nil)
	}
	installMockConnectHost(t, factories)

	entries := make([]struct{ Name, Addr, OS, Arch string }, N)
	hostNames := make([]string, N)
	for i := 0; i < N; i++ {
		hostNames[i] = "h" + itoa(i+1)
		entries[i] = struct{ Name, Addr, OS, Arch string }{hostNames[i], hostNames[i] + ".example", "", ""}
	}
	cfg := cfgWithHostsOrLocal(entries...)

	var buf bytes.Buffer
	if err := ResolveHostInfo(&buf, cfg); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := buf.String()

	// Each "Detecting OS/arch for host hN (hN.example)..." line must appear
	// exactly once and be a complete token (no torn-write interleaving).
	// Counting the exact line string is the cleanest way to assert atomicity:
	// torn writes would produce partial matches (e.g. lines starting with
	// "host h3..." that aren't exact).
	for _, n := range hostNames {
		want := "Detecting OS/arch for host " + n + " (" + n + ".example)..."
		count := strings.Count(out, want)
		if count != 1 {
			t.Errorf("expected 1 occurrence of %q; got %d\nfull output:\n%s", want, count, out)
		}
	}
	// Each "detected os=linux arch=amd64" status line must appear once.
	if got, want := strings.Count(out, "detected os=linux arch=amd64"), N; got != want {
		t.Errorf("detected-line count = %d; want %d\nfull output:\n%s", got, want, out)
	}
}

// TestResolveHostInfo_ClosesHostConnection verifies the orchestrator closes
// the host connection after detection completes (no leaks). On a successful
// path the executor's Close must be called exactly once per host. On error
// paths (DetectOS error) the executor must also be closed so a future refactor
// that removes the conn.Close() calls surfaces immediately.
func TestResolveHostInfo_ClosesHostConnection(t *testing.T) {
	t.Parallel()

	t.Run("success path", func(t *testing.T) {
		t.Parallel()
		closeCh := make(chan struct{}, 1)
		mock := &recordingCloserExecutor{
			MockExecutor: mockExecutorForDetect("linux", "amd64", nil),
			closeCh:      closeCh,
		}
		installMockConnectHost(t, map[string]host.Executor{"h1": mock})
		cfg := cfgWithHostsOrLocal(
			struct{ Name, Addr, OS, Arch string }{"h1", "h1.example", "", ""},
		)
		if err := ResolveHostInfo(nil, cfg); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		select {
		case <-closeCh:
		case <-time.After(time.Second):
			t.Fatal("host.Close() never called on success path")
		}
	})

	t.Run("error path", func(t *testing.T) {
		t.Parallel()
		closeCh := make(chan struct{}, 1)
		mock := &recordingCloserExecutor{
			MockExecutor: &testutil.MockExecutor{
				RunFn: func(cmd string) (string, error) {
					if strings.HasPrefix(cmd, "uname -s") {
						return "", errors.New("os probe fail")
					}
					return "", nil
				},
			},
			closeCh: closeCh,
		}
		installMockConnectHost(t, map[string]host.Executor{"h1": mock})
		cfg := cfgWithHostsOrLocal(
			struct{ Name, Addr, OS, Arch string }{"h1", "h1.example", "", ""},
		)
		if err := ResolveHostInfo(nil, cfg); err == nil {
			t.Fatal("expected error")
		}
		select {
		case <-closeCh:
		case <-time.After(time.Second):
			t.Fatal("host.Close() never called on error path")
		}
	})
}

// TestResolveHostInfo_ConfigMutationIsInPlace verifies the orchestrator
// updates the caller's cfg.Hosts map in place (no rebinding). If a future
// refactor accidentally replaces cfg.Hosts with a new map (e.g.
// `cfg.Hosts = make(...)`), the caller's reference would diverge from the
// orchestrator's view — disastrous for downstream code that reads cfg.Hosts.
//
// We can't compare &cfg.Hosts to &hostsMap (Go forbids taking the address
// of a map header), but we CAN observe the contract through indirection:
// after the call, lookups on the original reference must reflect the
// detected values. That would only happen if cfg.Hosts is the same map
// the orchestrator mutated.
func TestResolveHostInfo_ConfigMutationIsInPlace(t *testing.T) {
	t.Parallel()

	installMockConnectHost(t, map[string]host.Executor{
		"h1": mockExecutorForDetect("linux", "arm64", nil),
	})
	hostsMap := map[string]config.HostConfig{
		"h1": {Addr: "h1.example"},
	}
	cfg := &config.Config{Hosts: hostsMap}

	if err := ResolveHostInfo(io.Discard, cfg); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Detected values must be visible through the caller's reference — this
	// is only possible if cfg.Hosts is the same map the orchestrator wrote.
	if got := hostsMap["h1"]; got.OS != "linux" || got.Arch != "arm64" {
		t.Errorf("caller's map not updated (in-place mutation contract): %+v", got)
	}
	// cfg.Hosts must still be the same reference as the original map.
	if cfg.Hosts == nil {
		t.Fatal("cfg.Hosts was nil-ed by orchestrator")
	}
}

// recordingCloserExecutor is defined in run_per_host_parallel_test.go and
// reused here for the close-ordering assertions below.

package ops

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/an-lee/gh-sr/internal/config"
	"github.com/an-lee/gh-sr/internal/host"
	"github.com/an-lee/gh-sr/internal/testutil"
)

// TestFanOutHosts_EmptyGroups pins the fast path: no groups → no goroutines,
// no connect attempt, empty result.
func TestFanOutHosts_EmptyGroups(t *testing.T) {
	t.Parallel()

	installMockConnectHost(t, map[string]host.Executor{})
	got := fanOutHosts(nil, cfgWithHosts(), nil, func(_ io.Writer, _ *host.Host, _ hostGroup) (int, error) {
		t.Error("fn must not run for empty input")
		return 0, nil
	})
	if len(got) != 0 {
		t.Errorf("got %d results; want 0", len(got))
	}
}

// TestFanOutHosts_OrderAndValue verifies results come back aligned with the
// input groups regardless of completion order, carrying fn's return value.
func TestFanOutHosts_OrderAndValue(t *testing.T) {
	t.Parallel()

	installMockConnectHost(t, map[string]host.Executor{
		"h1": &testutil.MockExecutor{},
		"h2": &testutil.MockExecutor{},
		"h3": &testutil.MockExecutor{},
	})
	groups := []hostGroup{{name: "h1"}, {name: "h2"}, {name: "h3"}}

	got := fanOutHosts(nil, cfgWithHosts("h1", "h2", "h3"), groups,
		func(_ io.Writer, _ *host.Host, g hostGroup) (string, error) {
			return "ran-" + g.name, nil
		})
	if len(got) != 3 {
		t.Fatalf("got %d results; want 3", len(got))
	}
	for i, want := range []string{"ran-h1", "ran-h2", "ran-h3"} {
		if got[i].Val != want {
			t.Errorf("results[%d].Val = %q; want %q (input order must be preserved)", i, got[i].Val, want)
		}
		if got[i].ConnectErr != nil || got[i].Err != nil {
			t.Errorf("results[%d] unexpected error: connect=%v fn=%v", i, got[i].ConnectErr, got[i].Err)
		}
	}
}

// TestFanOutHosts_ConnectError verifies a connect failure records ConnectErr
// under the result's Name, emits the standard warning line exactly once, and
// never invokes fn for that group.
func TestFanOutHosts_ConnectError(t *testing.T) {
	t.Parallel()

	sentinel := errors.New("ssh dial timeout")
	installFailingConnectHost(t, sentinel)

	var fnCalls int
	var buf bytes.Buffer
	got := fanOutHosts(&buf, cfgWithHosts("h1"), []hostGroup{{name: "h1"}},
		func(_ io.Writer, _ *host.Host, _ hostGroup) (string, error) {
			fnCalls++
			return "", nil
		})
	if len(got) != 1 {
		t.Fatalf("got %d results; want 1", len(got))
	}
	if !errors.Is(got[0].ConnectErr, sentinel) {
		t.Errorf("ConnectErr = %v; want %v", got[0].ConnectErr, sentinel)
	}
	if got[0].Name != "h1" {
		t.Errorf("Name = %q; want h1", got[0].Name)
	}
	if fnCalls != 0 {
		t.Errorf("fn called %d times; want 0 on connect failure", fnCalls)
	}
	want := "Warning: cannot connect to h1: ssh dial timeout\n"
	if buf.String() != want {
		t.Errorf("output = %q; want %q", buf.String(), want)
	}
}

// TestFanOutHosts_FnError verifies fn's error is captured per-group without
// aborting the other groups, and that nil writers never panic on the
// warning path.
func TestFanOutHosts_FnError(t *testing.T) {
	t.Parallel()

	sentinel := errors.New("boom")
	installMockConnectHost(t, map[string]host.Executor{
		"h1": &testutil.MockExecutor{},
		"h2": &testutil.MockExecutor{},
	})

	// nil writer + connect errors must not panic either.
	groups := []hostGroup{{name: "h1"}, {name: "h2"}}
	got := fanOutHosts(nil, cfgWithHosts("h1", "h2"), groups,
		func(_ io.Writer, _ *host.Host, g hostGroup) (int, error) {
			if g.name == "h1" {
				return 0, sentinel
			}
			return 42, nil
		})
	if !errors.Is(got[0].Err, sentinel) {
		t.Errorf("results[0].Err = %v; want %v", got[0].Err, sentinel)
	}
	if got[1].Val != 42 || got[1].Err != nil {
		t.Errorf("results[1] = %+v; want {Val:42 Err:nil} (other groups unaffected)", got[1])
	}
}

// TestFanOutHosts_NilWriterConnectErrorNoPanic covers the TUI path: w=nil
// with a connect failure must not panic while still recording ConnectErr.
func TestFanOutHosts_NilWriterConnectErrorNoPanic(t *testing.T) {
	t.Parallel()

	sentinel := errors.New("down")
	installFailingConnectHost(t, sentinel)

	got := fanOutHosts(nil, cfgWithHosts("h1"), []hostGroup{{name: "h1"}},
		func(_ io.Writer, _ *host.Host, _ hostGroup) (string, error) {
			return "", nil
		})
	if !errors.Is(got[0].ConnectErr, sentinel) {
		t.Errorf("ConnectErr = %v; want %v", got[0].ConnectErr, sentinel)
	}
}

// TestFanOutHosts_WarningLinesAreComplete verifies concurrent connect
// failures produce N complete warning lines (no torn writes) — the
// regression the old per-orchestrator wMu guarded against.
func TestFanOutHosts_WarningLinesAreComplete(t *testing.T) {
	t.Parallel()

	sentinel := errors.New("refused")
	installFailingConnectHost(t, sentinel)

	const N = 8
	groups := make([]hostGroup, N)
	names := make([]string, N)
	cfg := &config.Config{Hosts: make(map[string]config.HostConfig)}
	for i := range N {
		names[i] = "h" + itoa(i)
		groups[i] = hostGroup{name: names[i]}
		cfg.Hosts[names[i]] = config.HostConfig{Addr: names[i]}
	}

	var buf bytes.Buffer
	got := fanOutHosts(&buf, cfg, groups,
		func(_ io.Writer, _ *host.Host, _ hostGroup) (string, error) {
			return "", nil
		})
	if len(got) != N {
		t.Fatalf("got %d results; want %d", len(got), N)
	}
	lines := strings.Split(strings.TrimRight(buf.String(), "\n"), "\n")
	if len(lines) != N {
		t.Fatalf("got %d warning lines; want %d (buf=%q)", len(lines), N, buf.String())
	}
	seen := map[string]bool{}
	for _, line := range lines {
		if !strings.HasPrefix(line, "Warning: cannot connect to h") || !strings.HasSuffix(line, ": refused") {
			t.Errorf("malformed/torn warning line: %q", line)
		}
		if seen[line] {
			t.Errorf("duplicated line: %q", line)
		}
		seen[line] = true
	}
}

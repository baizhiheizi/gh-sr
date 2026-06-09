package runner

import (
	"strings"
	"testing"

	"github.com/an-lee/gh-sr/internal/config"
	"github.com/an-lee/gh-sr/internal/host"
)

type diskMockExecutor struct {
	output string
	err    error
	calls  []string
}

func (m *diskMockExecutor) Run(cmd string) (string, error) {
	m.calls = append(m.calls, cmd)
	if m.err != nil {
		return "", m.err
	}
	return m.output, nil
}

func (m *diskMockExecutor) Upload(_, _ string) error { return nil }
func (m *diskMockExecutor) Close() error             { return nil }

func diskMockHost(os string, mock host.Executor) *host.Host {
	h := host.NewHost("test", config.HostConfig{OS: os, Addr: "local"})
	h.SetConn(mock)
	return h
}

type sequentialMock struct {
	responses []string
	idx       int
	calls     []string
}

func (m *sequentialMock) Run(cmd string) (string, error) {
	m.calls = append(m.calls, cmd)
	if m.idx >= len(m.responses) {
		return "", nil
	}
	out := m.responses[m.idx]
	m.idx++
	return out, nil
}

func (m *sequentialMock) Upload(_, _ string) error { return nil }
func (m *sequentialMock) Close() error             { return nil }

func TestParseFourInt64s(t *testing.T) {
	t.Parallel()
	a, b, c, d, err := parseFourInt64s("100 20 10 70\n")
	if err != nil {
		t.Fatal(err)
	}
	if a != 100 || b != 20 || c != 10 || d != 70 {
		t.Fatalf("got %d %d %d %d", a, b, c, d)
	}
}

func TestMeasureDiskUsage_linux(t *testing.T) {
	t.Parallel()
	h := diskMockHost("linux", &sequentialMock{
		responses: []string{"/home/u", "1000000 500000 100000 300000\n"},
	})
	rc := config.RunnerConfig{Name: "ci", Count: 1}
	entry := MeasureDiskUsage(h, "host1", "ci-1", &rc)
	if entry.Err != nil {
		t.Fatal(entry.Err)
	}
	if entry.TotalBytes != 1000000 {
		t.Fatalf("total=%d", entry.TotalBytes)
	}
	if entry.WorkBytes != 500000 || entry.TempBytes != 100000 || entry.DockerDataBytes != 300000 {
		t.Fatalf("breakdown work=%d temp=%d docker=%d", entry.WorkBytes, entry.TempBytes, entry.DockerDataBytes)
	}
	if entry.Mode != "native" {
		t.Fatalf("mode=%q", entry.Mode)
	}
}

func TestPruneInstance_skipsBusy(t *testing.T) {
	t.Parallel()
	m := NewManager("")
	h := diskMockHost("linux", &diskMockExecutor{})
	rc := config.RunnerConfig{Name: "ci", Count: 1}
	res := m.PruneInstance(h, "host1", "ci-1", &rc, true, PruneOptions{})
	if !res.Skipped || res.Reason != "busy" {
		t.Fatalf("got %+v", res)
	}
}

func TestPruneInstance_clearWorkTemp_dryRun(t *testing.T) {
	t.Parallel()
	m := NewManager("")
	mock := &sequentialMock{responses: []string{"/home/u"}}
	h := diskMockHost("linux", mock)
	rc := config.RunnerConfig{Name: "ci", Count: 1}
	res := m.PruneInstance(h, "host1", "ci-1", &rc, false, PruneOptions{DryRun: true})
	if res.Skipped {
		t.Fatalf("unexpected skip: %+v", res)
	}
	if len(res.Actions) == 0 {
		t.Fatal("expected actions")
	}
	if len(mock.calls) > 0 {
		t.Fatalf("dry-run should not run remote commands, got %d calls", len(mock.calls))
	}
}

func TestPruneInstance_defaultKeepsDockerCache(t *testing.T) {
	t.Parallel()
	m := NewManager("")
	mock := &sequentialMock{responses: []string{"/home/u"}}
	h := diskMockHost("linux", mock)
	rc := config.RunnerConfig{Name: "ci", Count: 1, Profile: "agentic"}
	res := m.PruneInstance(h, "host1", "ci-1", &rc, false, PruneOptions{DryRun: true})
	for _, a := range res.Actions {
		if strings.Contains(a, "docker cache") {
			t.Fatalf("default should keep docker cache: %q", a)
		}
	}
}

func TestPruneInstance_pruneCacheIncludesDockerPrune(t *testing.T) {
	t.Parallel()
	m := NewManager("")
	mock := &sequentialMock{responses: []string{"/home/u"}}
	h := diskMockHost("linux", mock)
	rc := config.RunnerConfig{Name: "ci", Count: 1, Profile: "agentic"}
	res := m.PruneInstance(h, "host1", "ci-1", &rc, false, PruneOptions{DryRun: true, PruneCache: true})
	found := false
	for _, a := range res.Actions {
		if strings.Contains(a, "docker cache") {
			found = true
		}
	}
	if !found {
		t.Fatal("expected inner docker cache prune action with --prune-cache")
	}
}

func TestPruneInstance_neverTouchesRunnerRegistration(t *testing.T) {
	t.Parallel()
	m := NewManager("")
	mock := &sequentialMock{responses: []string{"/home/u", ""}}
	h := diskMockHost("linux", mock)
	rc := config.RunnerConfig{Name: "ci", Count: 1}
	_ = m.PruneInstance(h, "host1", "ci-1", &rc, false, PruneOptions{})
	for _, cmd := range mock.calls {
		if strings.Contains(cmd, ".runner") {
			t.Fatalf("prune must not touch .runner: %q", cmd)
		}
	}
}

func TestFormatBytesHuman(t *testing.T) {
	t.Parallel()
	if got := FormatBytesHuman(2 * 1024 * 1024 * 1024); got != "2.0 GiB" {
		t.Fatalf("got %q", got)
	}
}

func TestDiskWarnThresholdBytes(t *testing.T) {
	t.Parallel()
	want := int64(50) * 1024 * 1024 * 1024
	if got := DiskWarnThresholdBytes(); got != want {
		t.Fatalf("got %d want %d", got, want)
	}
}

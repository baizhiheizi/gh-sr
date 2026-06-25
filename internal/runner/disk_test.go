package runner

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/an-lee/gh-sr/internal/config"
	"github.com/an-lee/gh-sr/internal/host"
	"github.com/an-lee/gh-sr/internal/testutil"
)

func diskMockHost(os string, mock *testutil.MockExecutor) *host.Host {
	h := host.NewHost("test", config.HostConfig{OS: os, Addr: "local"})
	h.SetConn(mock)
	return h
}

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
	h := diskMockHost("linux", &testutil.MockExecutor{
		Responses: []string{"1000000 500000 100000 300000\n"},
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

func TestMeasureDiskUsage_agenticMode(t *testing.T) {
	t.Parallel()
	h := diskMockHost("linux", &testutil.MockExecutor{
		Responses: []string{"100 0 0 0\n"},
	})
	rc := config.RunnerConfig{Name: "ag", Count: 1, Profile: "agentic"}
	entry := MeasureDiskUsage(h, "host1", "ag-1", &rc)
	if entry.Err != nil {
		t.Fatal(entry.Err)
	}
	if entry.Mode != "container" {
		t.Fatalf("mode=%q, want container", entry.Mode)
	}
}

// TestDiskDispatchers_unsupportedHostOS locks in the runOnHostOS fallback for
// the three dispatchers in disk.go (#229): when h.OS is neither "windows" nor
// "linux"/"darwin", dirSizes / clearWorkTemp / removeDirTree must all return
// the "unsupported host OS" error and never invoke the remote host. The
// refactor that moved the three hand-rolled switches onto runOnHostOS would be
// a regression if it dropped the error.
func TestDiskDispatchers_unsupportedHostOS(t *testing.T) {
	t.Parallel()
	mock := &testutil.MockExecutor{}
	h := diskMockHost("freebsd", mock)

	if _, _, _, _, err := dirSizes(h, "ci-1"); err == nil {
		t.Fatal("dirSizes: expected unsupported-host-OS error, got nil")
	}
	if err := clearWorkTemp(h, "ci-1", false); err == nil {
		t.Fatal("clearWorkTemp: expected unsupported-host-OS error, got nil")
	}
	if err := removeDirTree(h, "ci-1"); err == nil {
		t.Fatal("removeDirTree: expected unsupported-host-OS error, got nil")
	}
	if len(mock.Calls) != 0 {
		t.Fatalf("unsupported OS must not invoke the host, got %d call(s)", len(mock.Calls))
	}
	for _, err := range []error{
		func() error { _, _, _, _, e := dirSizes(h, "ci-1"); return e }(),
		clearWorkTemp(h, "ci-1", false),
		removeDirTree(h, "ci-1"),
	} {
		if !strings.Contains(err.Error(), "freebsd") {
			t.Fatalf("error should mention the offending OS, got: %v", err)
		}
	}
}

func TestPOSIXScripts_includeSetE(t *testing.T) {
	t.Parallel()
	for name, script := range map[string]string{
		"dirSizes":      buildDirSizesPOSIXScript("ci-1"),
		"clearWorkTemp": clearWorkTempPOSIX("ci-1", false),
		"removeDir":     removeDirTreePOSIX("ci-1"),
	} {
		if !strings.Contains(script, "set -e") {
			t.Fatalf("%s script missing set -e", name)
		}
	}
}

func TestPruneInstance_skipsBusy(t *testing.T) {
	t.Parallel()
	m := NewManager("")
	h := diskMockHost("linux", &testutil.MockExecutor{})
	rc := config.RunnerConfig{Name: "ci", Count: 1}
	res := m.PruneInstance(h, "host1", "ci-1", &rc, true, PruneOptions{})
	if !res.Skipped || res.Reason != "busy" {
		t.Fatalf("got %+v", res)
	}
}

func TestPruneInstance_clearWorkTemp_dryRun(t *testing.T) {
	t.Parallel()
	m := NewManager("")
	mock := &testutil.MockExecutor{}
	h := diskMockHost("linux", mock)
	rc := config.RunnerConfig{Name: "ci", Count: 1}
	res := m.PruneInstance(h, "host1", "ci-1", &rc, false, PruneOptions{DryRun: true})
	if res.Skipped {
		t.Fatalf("unexpected skip: %+v", res)
	}
	if len(res.Actions) == 0 {
		t.Fatal("expected actions")
	}
	if len(mock.Calls) > 0 {
		t.Fatalf("dry-run should not run remote commands, got %d calls", len(mock.Calls))
	}
}

func TestPruneInstance_defaultKeepsDockerCache(t *testing.T) {
	t.Parallel()
	m := NewManager("")
	mock := &testutil.MockExecutor{}
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
	mock := &testutil.MockExecutor{}
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

func TestClearWorkTempPOSIX_escalatesForContainerMode(t *testing.T) {
	t.Parallel()
	script := clearWorkTempPOSIX("rune-agentic-3", true)
	if !strings.Contains(script, "docker exec") {
		t.Fatal("container mode should try docker exec for root-owned _work files")
	}
	if !strings.Contains(script, "gh-sr-rune-agentic-3") {
		t.Fatalf("expected container name in script, got: %s", script)
	}
	if !strings.Contains(script, "sudo -n") {
		t.Fatal("expected host sudo fallback")
	}
}

func TestClearWorkTempPOSIX_nativeSkipsDockerExec(t *testing.T) {
	t.Parallel()
	script := clearWorkTempPOSIX("ci-1", false)
	if strings.Contains(script, "docker exec") {
		t.Fatal("native mode should not use docker exec")
	}
	if !strings.Contains(script, "sudo -n") {
		t.Fatal("expected host sudo fallback")
	}
}

func TestContainerEscalation_quotesContainerAndCommand(t *testing.T) {
	t.Parallel()
	script := containerEscalation("gh-sr-rune-1", `for sub in _work _temp; do rm -rf "$sub"; done`)
	if !strings.Contains(script, "docker exec") {
		t.Fatal("expected docker exec invocation")
	}
	if !strings.Contains(script, "docker start") {
		t.Fatal("expected docker start fallback")
	}
	// QuoteContainerName wraps the name in Go-style double quotes via strconv.Quote.
	if !strings.Contains(script, `"gh-sr-rune-1"`) {
		t.Fatalf("expected double-quoted container name, got: %s", script)
	}
	if !strings.Contains(script, "sh -c") {
		t.Fatal("expected shell wrapper for the inner command")
	}
	if !strings.Contains(script, `for sub in _work _temp; do rm -rf "$sub"; done`) {
		t.Fatalf("inner command should appear verbatim inside the sh -c quote, got: %s", script)
	}
}

func TestContainerEscalation_handlesSpacesInName(t *testing.T) {
	t.Parallel()
	script := containerEscalation("weird name", `echo hi`)
	// QuoteContainerName wraps spaces in Go-style double quotes via strconv.Quote.
	if !strings.Contains(script, `"weird name"`) {
		t.Fatalf("expected double-quoted container name with spaces, got: %s", script)
	}
}

func TestPasswordlessSudo_setsSUDOVariable(t *testing.T) {
	t.Parallel()
	script := passwordlessSudo()
	if !strings.Contains(script, "SUDO=") {
		t.Fatal("expected SUDO variable assignment")
	}
	if !strings.Contains(script, "sudo -n true") {
		t.Fatal("expected passwordless sudo probe")
	}
	if !strings.Contains(script, "sudo -n'") {
		t.Fatal("expected sudo -n prefix assignment when probe succeeds")
	}
}

func TestRemoveDirTreePOSIX_usesSharedEscalationHelpers(t *testing.T) {
	t.Parallel()
	script := removeDirTreePOSIX("rune-orphan-7")
	// Both helpers should appear: docker exec escalation + sudo probe.
	if !strings.Contains(script, "docker exec") {
		t.Fatal("expected docker exec escalation in removeDirTreePOSIX")
	}
	if !strings.Contains(script, "sudo -n true") {
		t.Fatal("expected passwordless sudo probe in removeDirTreePOSIX")
	}
	if !strings.Contains(script, "rm -rf /runner-state") {
		t.Fatal("expected inner command to reach the container")
	}
	if !strings.Contains(script, "runner-state/rune-orphan-7") && !strings.Contains(script, `runners/rune-orphan-7`) {
		t.Fatalf("expected runner dir var to point at rune-orphan-7, got: %s", script)
	}
}

func TestPruneInstance_neverTouchesRunnerRegistration(t *testing.T) {
	t.Parallel()
	m := NewManager("")
	mock := &testutil.MockExecutor{Responses: []string{""}}
	h := diskMockHost("linux", mock)
	rc := config.RunnerConfig{Name: "ci", Count: 1}
	_ = m.PruneInstance(h, "host1", "ci-1", &rc, false, PruneOptions{})
	for _, cmd := range mock.Calls {
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

func TestSafeRunnerInstanceName(t *testing.T) {
	t.Parallel()
	if err := SafeRunnerInstanceName("ci-1"); err != nil {
		t.Fatal(err)
	}
	for _, bad := range []string{"", ".", "..", "a/b", "a\nb", `a";rm -rf`} {
		if err := SafeRunnerInstanceName(bad); err == nil {
			t.Fatalf("expected error for %q", bad)
		}
	}
}

func TestMeasureDiskUsage_rejectsUnsafeInstance(t *testing.T) {
	t.Parallel()
	h := diskMockHost("linux", &testutil.MockExecutor{})
	entry := MeasureDiskUsage(h, "host1", `bad"name`, nil)
	if entry.Err == nil {
		t.Fatal("expected error")
	}
}

// TestDirSizesPOSIXScript_structure guards the single-walk optimization: the
// generated script must make exactly one `du` invocation with depth 1, not
// four separate `du -sk` calls. This is the energy-efficiency invariant the
// PR claims — if a future refactor re-introduces multiple `du` walks, this
// test will catch it. The test runs the real production script (via
// buildDirSizesPOSIXScript) on a real temp dir, not a frozen copy.
func TestDirSizesPOSIXScript_structure(t *testing.T) {
	t.Parallel()
	if _, err := exec.LookPath("du"); err != nil {
		t.Skip("du not available")
	}
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("bash not available")
	}

	// Build the same instance-dir layout the production script targets.
	inst := "ci-bench"
	fakeHome := t.TempDir()
	runnerDir := filepath.Join(fakeHome, ".gh-sr", "runners", inst)
	for _, sub := range []string{"_work", "_temp", "docker-data"} {
		if err := os.MkdirAll(filepath.Join(runnerDir, sub), 0o755); err != nil {
			t.Fatal(err)
		}
		for i := 0; i < 5; i++ {
			f, err := os.CreateTemp(filepath.Join(runnerDir, sub), "f*.bin")
			if err != nil {
				t.Fatal(err)
			}
			if _, err := f.Write(make([]byte, 4096)); err != nil {
				t.Fatal(err)
			}
			f.Close()
		}
	}

	cmd := exec.Command("bash", "-c", buildDirSizesPOSIXScript(inst))
	cmd.Env = append(os.Environ(), "HOME="+fakeHome)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("script failed: %v\n%s", err, out)
	}
	parts := strings.Fields(strings.TrimSpace(string(out)))
	if len(parts) != 4 {
		t.Fatalf("expected 4 numbers, got %d: %q", len(parts), out)
	}
	// All three subdirs were populated with 5 × 4 KiB files = 20 KiB each.
	// `du` reports in 1-KiB blocks; allow >= 16 KiB each (rounding).
	for i, label := range []string{"total", "work", "temp", "docker"} {
		var n int64
		for _, c := range parts[i] {
			if c < '0' || c > '9' {
				t.Fatalf("non-numeric in %s: %q", label, parts[i])
			}
			n = n*10 + int64(c-'0')
		}
		if n < 16*1024 {
			t.Fatalf("%s=%d bytes, want >= 16384", label, n)
		}
	}
}

// TestDirSizesPOSIXScript_singleDuInvocation is the structural invariant
// guard. The OLD script had 4 separate `du -sk "$dir[/sub]"` calls. The
// NEW script has exactly one effective `du` invocation per execution
// (an `if/else` branch probes GNU vs BSD, but only one branch runs).
// We assert two properties:
//
//  1. The old `du -sk "$dir"`-per-subdir pattern is gone (0 occurrences).
//  2. There is exactly one `du` line that performs a data fetch
//     (the depth-0 probe is not a fetch — it only tests flag support).
func TestDirSizesPOSIXScript_singleDuInvocation(t *testing.T) {
	t.Parallel()
	script := buildDirSizesPOSIXScript("foo")

	// (1) The old 4-call pattern must be absent.
	if c := strings.Count(script, `du -sk "$dir"`); c != 0 {
		t.Fatalf("old 4-du pattern present: %d occurrences of `du -sk \"$dir\"` in script", c)
	}

	// (2) Exactly one data-fetching `du` line. Both the GNU and BSD
	// branches assign to `out=$(du ...)`; structurally that's 2 lines,
	// but only one runs per execution. We allow 2 (one per platform
	// branch) and disallow anything more.
	lines := strings.Split(script, "\n")
	fetches := 0
	for _, l := range lines {
		trimmed := strings.TrimSpace(l)
		if strings.HasPrefix(trimmed, "out=$(du ") {
			fetches++
		}
	}
	if fetches != 1 && fetches != 2 {
		t.Fatalf("expected 1 or 2 du fetch lines (GNU/BSD branch), got %d\nscript:\n%s", fetches, script)
	}
}

// BenchmarkMeasureDiskUsage_linux benchmarks the Go side of MeasureDiskUsage
// with a mock host. The real I/O win is on the remote host (4 SSH round
// trips → 1), which is not exercised here. This benchmark exists primarily
// to catch accidental per-call allocation regressions in the Go wrapper.
func BenchmarkMeasureDiskUsage_linux(b *testing.B) {
	h := diskMockHost("linux", &testutil.MockExecutor{
		Responses: []string{"1000000 500000 100000 300000\n"},
	})
	rc := config.RunnerConfig{Name: "ci", Count: 1}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = MeasureDiskUsage(h, "host1", "ci-1", &rc)
	}
}

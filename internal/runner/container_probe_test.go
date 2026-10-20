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

func TestBuildContainerProbeScript(t *testing.T) {
	t.Parallel()
	script := buildContainerProbeScript("$HOME/.gh-sr/runners", []string{"ci-1", "ci-2"})

	for _, want := range []string{
		`for i in "ci-1" "ci-2"; do`,
		`c="gh-sr-$i"`,
		`docker inspect --format '{{.State.Status}}' "$c"`,
		`docker exec "$c" sh -c '`,
		`GH_SR_DOCKERD_OK`,
		`GH_SR_RUNNER_OK`,
		`test -f "$HOME/.gh-sr/runners/$i/bootstrap-failed"`,
		`printf '%s|%s|%s|%s|%s\n'`,
	} {
		if !strings.Contains(script, want) {
			t.Errorf("script missing %q:\n%s", want, script)
		}
	}
}

func TestBuildContainerProbeScriptEscapesInstanceNames(t *testing.T) {
	t.Parallel()
	script := buildContainerProbeScript("$HOME/.gh-sr/runners", []string{`a"b`, "c$d"})
	if !strings.Contains(script, `"a\"b"`) {
		t.Errorf("expected escaped quote in script:\n%s", script)
	}
	if !strings.Contains(script, `"c\$d"`) {
		t.Errorf("expected escaped dollar in script:\n%s", script)
	}
}

func TestParseContainerProbeOutput(t *testing.T) {
	t.Parallel()
	out := strings.Join([]string{
		"running|yes|yes|no|ci-1",
		"missing|no|no|no|ci-2",
		"restarting|yes|no|yes|ci-3",
		"exited|no|no|no|ci-4",
		"",
		"garbage line",
	}, "\n")

	probes := parseContainerProbeOutput(out)
	if len(probes) != 4 {
		t.Fatalf("expected 4 probes, got %d: %+v", len(probes), probes)
	}

	p := probes["ci-1"]
	if p.State != "running" || !p.InnerDockerdOK || !p.Registered || p.BootstrapFailed {
		t.Errorf("ci-1 = %+v", p)
	}
	if p := probes["ci-2"]; p.State != "missing" || p.InnerDockerdOK || p.Registered || p.BootstrapFailed {
		t.Errorf("ci-2 = %+v", p)
	}
	if p := probes["ci-3"]; p.State != "restarting" || !p.InnerDockerdOK || p.Registered || !p.BootstrapFailed {
		t.Errorf("ci-3 = %+v", p)
	}
	if p := probes["ci-4"]; p.State != "exited" || p.InnerDockerdOK || p.Registered || p.BootstrapFailed {
		t.Errorf("ci-4 = %+v", p)
	}
}

// An instance name that contains the field separator must not corrupt the
// flags of its own line: the name is the last field, so the parser keeps the
// remainder verbatim.
func TestParseContainerProbeOutputKeepsSeparatorInInstanceName(t *testing.T) {
	t.Parallel()
	probes := parseContainerProbeOutput("running|yes|no|no|we|ird\n")
	p, ok := probes["we|ird"]
	if !ok {
		t.Fatalf("expected probe for %q, got %+v", "we|ird", probes)
	}
	if p.State != "running" || !p.InnerDockerdOK || p.Registered {
		t.Errorf("probe = %+v", p)
	}
}

// The whole point of the batched probe: the number of SSH round-trips must
// not grow with the number of instances on the host.
func TestProbeContainersBatchUsesOneRoundTrip(t *testing.T) {
	t.Parallel()
	for _, n := range []int{1, 4, 12} {
		instances := make([]string, 0, n)
		for i := range n {
			instances = append(instances, "ci-"+string(rune('a'+i)))
		}
		mock := &testutil.MockExecutor{
			RunFn: func(cmd string) (string, error) {
				var b strings.Builder
				for _, inst := range instances {
					b.WriteString("running|yes|yes|no|" + inst + "\n")
				}
				return b.String(), nil
			},
		}
		h := host.NewHost("linux", config.HostConfig{OS: "linux", Addr: "local"})
		h.SetConn(mock)

		probes, err := ProbeContainersBatch(h, instances)
		if err != nil {
			t.Fatalf("n=%d: %v", n, err)
		}
		if len(probes) != n {
			t.Fatalf("n=%d: expected %d probes, got %d", n, n, len(probes))
		}
		if len(mock.Calls) != 1 {
			t.Fatalf("n=%d: expected 1 round-trip, got %d: %v", n, len(mock.Calls), mock.Calls)
		}
	}
}

func TestProbeContainersBatchEmpty(t *testing.T) {
	t.Parallel()
	mock := &testutil.MockExecutor{}
	h := host.NewHost("linux", config.HostConfig{OS: "linux", Addr: "local"})
	h.SetConn(mock)

	probes, err := ProbeContainersBatch(h, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(probes) != 0 {
		t.Fatalf("expected no probes, got %+v", probes)
	}
	if len(mock.Calls) != 0 {
		t.Fatalf("expected no round-trips, got %v", mock.Calls)
	}
}

func TestProbeContainersBatchWindowsUnsupported(t *testing.T) {
	t.Parallel()
	h := host.NewHost("win", config.HostConfig{OS: "windows", Addr: "local"})
	h.SetConn(&testutil.MockExecutor{})
	if _, err := ProbeContainersBatch(h, []string{"ci-1"}); err == nil {
		t.Fatal("expected an error for a windows host")
	}
}

// TestContainerProbeScriptRunsUnderPOSIXShell executes the generated probe
// script against a stubbed `docker` so the shell logic itself — the state
// fallback, the exec token matching, the marker test and the output framing
// — is verified, not just the Go-side parser.
func TestContainerProbeScriptRunsUnderPOSIXShell(t *testing.T) {
	t.Parallel()
	sh, err := exec.LookPath("sh")
	if err != nil {
		t.Skip("sh not available")
	}
	dir := t.TempDir()
	fake := `#!/bin/sh
case "$1" in
inspect)
  # docker inspect --format '<fmt>' <container>
  case "$4" in
  gh-sr-ci-1) echo running; exit 0 ;;
  esac
  echo "Error: No such object" >&2
  exit 1
  ;;
exec)
  # docker exec <container> sh -c '<probe>'
  case "$2" in
  gh-sr-ci-1) echo GH_SR_DOCKERD_OK; echo GH_SR_RUNNER_OK; exit 0 ;;
  esac
  exit 1
  ;;
esac
exit 1
`
	if err := os.WriteFile(filepath.Join(dir, "docker"), []byte(fake), 0o755); err != nil {
		t.Fatal(err)
	}
	home := t.TempDir()
	// ci-1 carries the bootstrap-failed marker; ci-2's container is absent.
	markerDir := filepath.Join(home, ".gh-sr", "runners", "ci-1")
	if err := os.MkdirAll(markerDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(markerDir, bootstrapFailedMarker), nil, 0o644); err != nil {
		t.Fatal(err)
	}

	script := buildContainerProbeScript("$HOME/.gh-sr/runners", []string{"ci-1", "ci-2"})
	cmd := exec.Command(sh, "-c", script)
	cmd.Env = append(os.Environ(), "PATH="+dir+":"+os.Getenv("PATH"), "HOME="+home)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("probe script failed: %v\n%s", err, out)
	}

	probes := parseContainerProbeOutput(string(out))
	p1, ok := probes["ci-1"]
	if !ok {
		t.Fatalf("no probe for ci-1, output:\n%s", out)
	}
	if p1.State != "running" || !p1.InnerDockerdOK || !p1.Registered || !p1.BootstrapFailed {
		t.Errorf("ci-1 = %+v", p1)
	}
	p2, ok := probes["ci-2"]
	if !ok {
		t.Fatalf("no probe for ci-2, output:\n%s", out)
	}
	if p2.State != "missing" || p2.InnerDockerdOK || p2.Registered || p2.BootstrapFailed {
		t.Errorf("ci-2 = %+v", p2)
	}
}

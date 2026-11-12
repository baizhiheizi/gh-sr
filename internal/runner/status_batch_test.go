package runner

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/an-lee/gh-sr/internal/config"
	"github.com/an-lee/gh-sr/internal/host"
	"github.com/an-lee/gh-sr/internal/testutil"
)

// batchTestHost returns a Linux host wired to mock.
func batchTestHost(mock *testutil.MockExecutor) *host.Host {
	h := host.NewHost("h", config.HostConfig{OS: "linux", Arch: "amd64", Addr: "local"})
	h.SetConn(mock)
	return h
}

func batchTestRC(count int) config.RunnerConfig {
	return config.RunnerConfig{
		Name:       "ci",
		Repo:       "owner/repo",
		Host:       "h",
		Count:      count,
		RunnerMode: config.RunnerModeNative,
	}
}

// TestNativeStatusBatchScriptShape pins the fragments the state machine
// depends on: one elevation gate, the version collapse, the svc.sh →
// autostart → PID-file branch order, and the tagged emits. The full state
// machine is exercised end-to-end by TestNativeStatusBatchScriptShell.
func TestNativeStatusBatchScriptShape(t *testing.T) {
	h := batchTestHost(&testutil.MockExecutor{})
	script, ok := buildNativeStatusBatchScript(h, []string{"ci-1", "ci-2"})
	if !ok {
		t.Fatal("build failed for valid instance names")
	}

	checks := []string{
		statusBatchScriptHeader,
		"command -v sudo", // elevation gate, evaluated once
		"sudo -n true",    // passwordless-sudo probe
		"set -f",          // noglob for the version collapse
		"cat $HOME/.gh-sr/runners/ci-1/.runner-version",
		"cat $HOME/.gh-sr/runners/ci-2/.runner-version",
		"[ -f $HOME/.gh-sr/runners/ci-1/svc.sh ]",
		"systemctl is-active",        // svc.sh branch
		"systemctl --user is-active", // user unit branch
		"$SUDO systemctl is-active",  // system unit branch
		"[ -f \"$HOME/.config/systemd/user/ghsr-runner-ci-1.service\" ]",
		"[ -f /etc/systemd/system/ghsr-runner-ci-2.service ]",
		"kill -0 \"$pid\"", // PID-file fallback
		"rstate='not installed'",
		"rstate='service error'",
		nativeStatusBatchTag + "1|$rstate|$v",
		nativeStatusBatchTag + "2|$rstate|$v",
	}
	for _, want := range checks {
		if !strings.Contains(script, want) {
			t.Errorf("script missing %q", want)
		}
	}
	if strings.Contains(script, "echo D") {
		t.Error("status batch must not emit the orphan-cleanup D marker")
	}
	if got := strings.Count(script, "id -u"); got != 1 {
		t.Errorf("elevation gate evaluated %d times, want 1", got)
	}
}

// TestNativeStatusBatchScriptRejectsUnsanitizedName covers the fall-through:
// an instance name that fails autostart sanitization returns ok=false so
// Status uses the per-instance probes (which own that error path).
func TestNativeStatusBatchScriptRejectsUnsanitizedName(t *testing.T) {
	h := batchTestHost(&testutil.MockExecutor{})
	if _, ok := buildNativeStatusBatchScript(h, []string{"ok-1", "   "}); ok {
		t.Fatal("expected ok=false for an unsanitizable instance name")
	}
}

func TestParseNativeStatusBatch(t *testing.T) {
	out := strings.Join([]string{
		"warning: some stray remote output",
		nativeStatusBatchTag + "1|running|2.320.0",
		nativeStatusBatchTag + "2|stopped|",
		nativeStatusBatchTag + "3|not installed|-",
		"",
	}, "\n")

	results, ok := parseStatusBatch(out, nativeStatusBatchTag, 3, parseNativeStatusTail)
	if !ok {
		t.Fatal("parse failed on well-formed output")
	}
	want := []statusBatchResult{
		{local: "running", version: "2.320.0"},
		{local: "stopped"},
		{local: "not installed", version: "-"},
	}
	for i, w := range want {
		if results[i] != w {
			t.Errorf("results[%d] = %+v, want %+v", i, results[i], w)
		}
	}

	// Missing index → whole batch fails so Status falls back.
	if _, ok := parseStatusBatch(nativeStatusBatchTag+"1|running|1\n", nativeStatusBatchTag, 2, parseNativeStatusTail); ok {
		t.Error("missing index 2 must fail the batch")
	}
	// Out-of-range and duplicate indices are ignored, not corrupted rows.
	out = nativeStatusBatchTag + "1|running|1\n" + nativeStatusBatchTag + "9|ghost|\n" + nativeStatusBatchTag + "1|stopped|\n"
	results, ok = parseStatusBatch(out, nativeStatusBatchTag, 2, parseNativeStatusTail)
	if ok {
		t.Error("duplicate 1 with missing 2 must fail the batch")
	}
	if results != nil {
		t.Errorf("failed batch must return nil results, got %+v", results)
	}
}

func TestParseContainerStatusBatch(t *testing.T) {
	out := containerStatusBatchTag + "1|running|gh-sr/agentic-runner:v1|sha256:abc|cafebabe\n" +
		containerStatusBatchTag + "2|not installed|||\n" +
		containerStatusBatchTag + "3|failed|img|sha|rev\n"

	results, ok := parseStatusBatch(out, containerStatusBatchTag, 3, parseContainerStatusTail)
	if !ok {
		t.Fatal("parse failed on well-formed output")
	}
	if results[0].local != "running" || results[0].image != "gh-sr/agentic-runner:v1" || results[0].imageRev != "cafebabe" {
		t.Errorf("results[0] = %+v", results[0])
	}
	if results[1].local != "not installed" || results[1].image != "" || results[1].imageRev != "" {
		t.Errorf("results[1] = %+v", results[1])
	}
	if results[2].local != "failed" || results[2].image != "img" || results[2].imageRev != "rev" {
		t.Errorf("results[2] = %+v", results[2])
	}
}

// TestManagerStatusBatchNative runs Status against the batch answer and
// asserts both the round-trip count (1 for any instance count) and the
// per-instance values.
func TestManagerStatusBatchNative(t *testing.T) {
	for _, count := range []int{1, 4} {
		mock := &testutil.MockExecutor{RunFn: func(cmd string) (string, error) {
			if strings.Contains(cmd, statusBatchScriptHeader) {
				return composeNativeStatusBatchOutput(count), nil
			}
			t.Errorf("per-instance probe issued alongside batch: %q", cmd[:min(len(cmd), 60)])
			return "", nil
		}}
		h := batchTestHost(mock)
		mgr := NewManager("")

		statuses, err := mgr.Status(h, batchTestRC(count))
		if err != nil {
			t.Fatal(err)
		}
		if len(mock.Calls) != 1 {
			t.Errorf("count=%d: %d round trips, want 1 (batch only)", count, len(mock.Calls))
		}
		for i, s := range statuses {
			if s.Local != "running" {
				t.Errorf("count=%d statuses[%d].Local = %q, want running", count, i, s.Local)
			}
			if s.ContainerImageBuild != statusScenarioVersion {
				t.Errorf("count=%d statuses[%d] version = %q, want %q", count, i, s.ContainerImageBuild, statusScenarioVersion)
			}
		}
	}
}

// TestManagerStatusBatchNativeVersionDash covers the missing-version contract:
// an empty version field renders "-" in the BUILD cell, matching
// statusNativeAndVersion.
func TestManagerStatusBatchNativeVersionDash(t *testing.T) {
	mock := &testutil.MockExecutor{RunFn: func(string) (string, error) {
		return nativeStatusBatchTag + "1|stopped|\n", nil
	}}
	h := batchTestHost(mock)
	mgr := NewManager("")

	statuses, err := mgr.Status(h, batchTestRC(1))
	if err != nil {
		t.Fatal(err)
	}
	if statuses[0].ContainerImageBuild != "-" {
		t.Errorf("version = %q, want -", statuses[0].ContainerImageBuild)
	}
}

// TestManagerStatusBatchFallback preserves the pre-batch contract: a batch
// transport failure falls back to the per-instance probes and still fills
// every row with the same values.
func TestManagerStatusBatchFallback(t *testing.T) {
	sentinel := errors.New("ssh down")
	mock := &testutil.MockExecutor{RunFn: func(cmd string) (string, error) {
		if strings.Contains(cmd, statusBatchScriptHeader) {
			return "", sentinel
		}
		return nativeStatusMock(cmd)
	}}
	h := batchTestHost(mock)
	mgr := NewManager("")

	statuses, err := mgr.Status(h, batchTestRC(2))
	if err != nil {
		t.Fatal(err)
	}
	// Fallback shape: 1 batch attempt + 2 probes per instance.
	if len(mock.Calls) != 1+2*2 {
		t.Errorf("%d round trips, want 5 (1 failed batch + 2 per instance)", len(mock.Calls))
	}
	for i, s := range statuses {
		if s.Local != "running" || s.ContainerImageBuild != statusScenarioVersion {
			t.Errorf("statuses[%d] = {%q %q}, want {running %s}", i, s.Local, s.ContainerImageBuild, statusScenarioVersion)
		}
	}
}

// TestManagerStatusBatchContainer covers the container batch: one round trip,
// statuses identical to the per-instance shape, and the truncated-output
// fallback.
func TestManagerStatusBatchContainer(t *testing.T) {
	rc := batchTestRC(3)
	rc.RunnerMode = config.RunnerModeContainer

	mock := &testutil.MockExecutor{RunFn: func(cmd string) (string, error) {
		if strings.Contains(cmd, statusBatchScriptHeader) {
			return composeContainerStatusBatchOutput(3), nil
		}
		t.Errorf("per-instance container probe issued alongside batch: %q", cmd[:min(len(cmd), 60)])
		return "", nil
	}}
	h := batchTestHost(mock)
	mgr := NewManager("")

	statuses, err := mgr.Status(h, rc)
	if err != nil {
		t.Fatal(err)
	}
	if len(mock.Calls) != 1 {
		t.Errorf("%d round trips, want 1 (batch only)", len(mock.Calls))
	}
	for i, s := range statuses {
		if s.Local != "running" || s.ContainerImage != "gh-sr/agentic-runner:v1" || s.ContainerImageRevision != "cafebabe" {
			t.Errorf("statuses[%d] = {%q %q %q}", i, s.Local, s.ContainerImage, s.ContainerImageRevision)
		}
		if s.ContainerImageBuild == "" {
			t.Errorf("statuses[%d].ContainerImageBuild empty — formatContainerImageBuild not applied", i)
		}
	}

	// Truncated batch output (instance 3 missing) → fallback to per-instance.
	mock = &testutil.MockExecutor{RunFn: func(cmd string) (string, error) {
		if strings.Contains(cmd, statusBatchScriptHeader) {
			return composeContainerStatusBatchOutput(2), nil // instance 3 missing
		}
		return containerStatusMock(cmd)
	}}
	h = batchTestHost(mock)
	statuses, err = mgr.Status(h, rc)
	if err != nil {
		t.Fatal(err)
	}
	if len(statuses) != 3 || statuses[2].Local != "running" {
		t.Errorf("fallback statuses[2] = %+v, want running", statuses[2])
	}
}

// TestManagerStatusNonLinuxSkipsBatch pins that non-Linux hosts never issue
// the Linux-only batch script.
func TestManagerStatusNonLinuxSkipsBatch(t *testing.T) {
	h := host.NewHost("h", config.HostConfig{OS: "darwin", Arch: "arm64", Addr: "local"})
	mock := &testutil.MockExecutor{RunFn: func(cmd string) (string, error) {
		if strings.Contains(cmd, statusBatchScriptHeader) {
			t.Errorf("batch script issued on darwin: %q", cmd[:min(len(cmd), 60)])
		}
		return "S\nU\nV" + statusScenarioVersion + "\n", nil
	}}
	h.SetConn(mock)
	mgr := NewManager("")

	if _, err := mgr.Status(h, batchTestRC(1)); err != nil {
		t.Fatal(err)
	}
}

// TestNativeStatusBatchScriptShell runs the generated script under a real
// POSIX shell with shimmed systemctl/sudo/id binaries and asserts the
// emitted markers for the state-machine scenarios reachable without root
// (the /etc system-unit branch is covered by the script-shape and parse
// tests; its shell needs /etc/systemd/system writes).
func TestNativeStatusBatchScriptShell(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("POSIX-shell scenario test needs a Linux host")
	}
	shPath := "/bin/sh"
	if _, err := os.Stat(shPath); err != nil {
		t.Skipf("no %s: %v", shPath, err)
	}

	h := batchTestHost(&testutil.MockExecutor{})
	script, ok := buildNativeStatusBatchScript(h, []string{"ci-1"})
	if !ok {
		t.Fatal("build failed")
	}

	scenarios := []struct {
		name      string
		files     []string // paths relative to the instance dir; dirs created as needed
		unit      string   // ".service" contents (svc.sh branch)
		svcSh     bool
		userUnit  bool
		svcState  string // systemctl answer for the svc.sh branch
		userState string // systemctl --user answer
		sudoOK    string // "1" → passwordless sudo works
		want      string // "<local>|<version>" tail
	}{
		{
			name: "svc active", svcSh: true, unit: "ghsr-runner-ci-1.service", svcState: "active",
			files: []string{".runner-version:2.320.0"}, want: "running|2.320.0",
		},
		{
			name: "svc failed maps to stopped", svcSh: true, unit: "ghsr-runner-ci-1.service", svcState: "failed",
			files: []string{".runner-version:2.320.0"}, want: "stopped|2.320.0",
		},
		{
			name: "svc inactive maps to stopped", svcSh: true, unit: "ghsr-runner-ci-1.service", svcState: "inactive",
			files: []string{".runner-version:2.320.0"}, want: "stopped|2.320.0",
		},
		{
			name: "svc.sh without .service falls to pid file", svcSh: true,
			files: []string{".runner", ".runner_pid:999999999"}, want: "stopped|",
		},
		{
			name: "user unit failed maps to service error", userUnit: true, userState: "failed",
			files: []string{".runner-version:2.320.0"}, want: "service error|2.320.0",
		},
		{
			name: "user unit activating maps to service error", userUnit: true, userState: "activating",
			files: []string{".runner-version:2.320.0"}, want: "service error|2.320.0",
		},
		{
			name: "user unit inactive falls to live pid", userUnit: true, userState: "inactive",
			files: []string{".runner", ".runner_pid:self", ".runner-version:2.320.0"}, want: "running|2.320.0",
		},
		{
			name: "user unit inactive with dead pid", userUnit: true, userState: "inactive",
			files: []string{".runner", ".runner_pid:999999999"}, want: "stopped|",
		},
		{
			name: "no pid file is stopped", userUnit: true, userState: "inactive",
			files: []string{".runner", ".runner-version:2.320.0"}, want: "stopped|2.320.0",
		},
		{
			name: "nothing installed", want: "not installed|",
		},
		{
			name: "multiline version collapses", userUnit: true, userState: "inactive",
			files: []string{".runner", ".runner_pid:999999999", ".runner-version:a\nb"}, want: "stopped|a b",
		},
	}

	for _, sc := range scenarios {
		t.Run(sc.name, func(t *testing.T) {
			home := t.TempDir()
			dir := filepath.Join(home, ".gh-sr", "runners", "ci-1")
			if err := os.MkdirAll(dir, 0o755); err != nil {
				t.Fatal(err)
			}
			for _, f := range sc.files {
				name, content, _ := strings.Cut(f, ":")
				if content == "self" {
					continue // pid written by the wrapper below
				}
				if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			if sc.svcSh {
				if err := os.WriteFile(filepath.Join(dir, "svc.sh"), []byte("#!/bin/sh\n"), 0o755); err != nil {
					t.Fatal(err)
				}
				if sc.unit != "" {
					if err := os.WriteFile(filepath.Join(dir, ".service"), []byte(sc.unit), 0o644); err != nil {
						t.Fatal(err)
					}
				}
			}
			if sc.userUnit {
				up := filepath.Join(home, ".config", "systemd", "user")
				if err := os.MkdirAll(up, 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(up, "ghsr-runner-ci-1.service"), []byte(""), 0o644); err != nil {
					t.Fatal(err)
				}
			}

			bin := t.TempDir()
			writeShim(t, bin, "systemctl", `#!/bin/sh
case " $* " in
  *" --user "*) [ -n "$SHIM_USER_STATE" ] && printf '%s\n' "$SHIM_USER_STATE"; exit 0 ;;
esac
[ -n "$SHIM_SVC_STATE" ] && printf '%s\n' "$SHIM_SVC_STATE"
exit 0
`)
			writeShim(t, bin, "sudo", `#!/bin/sh
if [ "$1" = "-n" ] && [ "$2" = "true" ]; then
  [ "$SHIM_SUDO_OK" = 1 ] && exit 0 || exit 1
fi
exec systemctl "$@"
`)
			writeShim(t, bin, "id", `#!/bin/sh
[ "$1" = "-u" ] && printf '%s\n' "${SHIM_UID:-1000}"
`)

			// The wrapper optionally records the shell's own PID so the
			// "live pid" scenario asserts the kill -0 branch against a real
			// process; every other scenario manages its own pid file.
			self := "0"
			for _, f := range sc.files {
				if strings.HasPrefix(f, ".runner_pid:self") {
					self = "1"
				}
			}
			wrapper := "if [ \"$WRITE_SELF_PID\" = 1 ]; then echo $$ > " +
				shQuote(dir+"/.runner_pid") + "; fi\n" + script

			out, err := runScript(t, shPath, bin, home, wrapper, map[string]string{
				"SHIM_SVC_STATE":  sc.svcState,
				"SHIM_USER_STATE": sc.userState,
				"SHIM_SUDO_OK":    sc.sudoOK,
				"WRITE_SELF_PID":  self,
			})
			if err != nil {
				t.Fatalf("script run failed: %v\noutput:\n%s", err, out)
			}
			if !strings.Contains(out, nativeStatusBatchTag+"1|"+sc.want+"\n") {
				t.Errorf("emitted %q, want %q\nfull output:\n%s", strings.TrimSpace(out), nativeStatusBatchTag+"1|"+sc.want, out)
			}
		})
	}
}

// writeShim installs a shell script on the shim PATH.
func writeShim(t *testing.T, dir, name, body string) {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
}

func shQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
}

// runScript executes wrapper under sh with the shim dir prepended to PATH.
func runScript(t *testing.T, sh, shimDir, home, wrapper string, env map[string]string) (string, error) {
	t.Helper()
	cmd := exec.Command(sh, "-c", wrapper)
	cmd.Env = append(os.Environ(),
		"HOME="+home,
		"PATH="+shimDir+string(os.PathListSeparator)+os.Getenv("PATH"),
	)
	for k, v := range env {
		cmd.Env = append(cmd.Env, k+"="+v)
	}
	out, err := cmd.CombinedOutput()
	return string(out), err
}

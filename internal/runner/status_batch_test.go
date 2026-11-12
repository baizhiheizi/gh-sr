package runner

import (
	"errors"
	"fmt"
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
// depends on: one elevation gate, one copy of the state machine driven by a
// remote loop, the version collapse, the svc.sh → autostart → PID-file branch
// order, the bounded heredoc rows, and the tagged emit. The full state
// machine is exercised end-to-end by TestNativeStatusBatchScriptShell.
func TestNativeStatusBatchScriptShape(t *testing.T) {
	h := batchTestHost(&testutil.MockExecutor{})
	rows, ok := nativeStatusBatchRows([]string{"ci-1", "ci-2"})
	if !ok {
		t.Fatal("row build failed for valid instance names")
	}
	script, count := buildNativeStatusBatchScript(h, rows)
	if count != 2 {
		t.Fatalf("chunk carries %d rows, want 2", count)
	}
	if len(script) > statusBatchScriptMaxBytes {
		t.Errorf("script is %d bytes, cap %d", len(script), statusBatchScriptMaxBytes)
	}

	checks := []string{
		statusBatchScriptHeader,
		"command -v sudo",                     // elevation gate, evaluated once
		"sudo -n true",                        // passwordless-sudo probe
		"set -f",                              // noglob for the version collapse
		"while IFS='|' read -r inst base; do", // one state machine, remote loop
		"n=$((n+1))",
		"dir=\"$HOME/.gh-sr/runners/$inst\"",
		"cat \"$dir/.runner-version\"",
		"[ -f \"$dir/svc.sh\" ]",
		"systemctl is-active",        // svc.sh branch
		"systemctl --user is-active", // user unit branch
		"$SUDO systemctl is-active",  // system unit branch
		"\"$HOME/.config/systemd/user/$base.service\"",
		"\"/etc/systemd/system/$base.service\"",
		"kill -0 \"$pid\"", // PID-file fallback
		"rstate='not installed'",
		"rstate='service error'",
		nativeStatusBatchTag + "$n|$rstate|$v",
		"ci-1|ghsr-runner-ci-1",
		"ci-2|ghsr-runner-ci-2",
		"done <<'" + statusBatchRowsDelim + "'",
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
	// The state machine must be rendered exactly once, not once per instance.
	for _, fragment := range []string{"rstate=''", "st=''; src=''", nativeStatusBatchTag} {
		if got := strings.Count(script, fragment); got != 1 {
			t.Errorf("fragment %q rendered %d times, want 1 (script must not grow per instance)", fragment, got)
		}
	}
	// The state machine itself has two systemctl probes (svc.sh + autostart),
	// but that count is fixed — it must not scale with the instance list.
	if got := strings.Count(script, "systemctl is-active"); got != 2 {
		t.Errorf("systemctl probe rendered %d times, want 2 (independent of instance count)", got)
	}
}

// TestNativeStatusBatchScriptRejectsUnsanitizedName covers the fall-through:
// an instance name that fails autostart sanitization (or would break the
// heredoc framing) returns ok=false so Status uses the per-instance probes.
func TestNativeStatusBatchScriptRejectsUnsanitizedName(t *testing.T) {
	if _, ok := nativeStatusBatchRows([]string{"ok-1", "   "}); ok {
		t.Fatal("expected ok=false for an unsanitizable instance name")
	}
	for _, bad := range []string{"a|b", "a\nb", ""} {
		if _, ok := nativeStatusBatchRows([]string{bad}); ok {
			t.Errorf("nativeStatusBatchRows(%q) = ok, want false", bad)
		}
		if _, ok := containerStatusBatchRows([]string{bad}); ok {
			t.Errorf("containerStatusBatchRows(%q) = ok, want false", bad)
		}
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
	// Out-of-range index → whole batch fails, even when every row is present.
	out = nativeStatusBatchTag + "1|running|1\n" + nativeStatusBatchTag + "2|stopped|\n" + nativeStatusBatchTag + "9|ghost|\n"
	results, ok = parseStatusBatch(out, nativeStatusBatchTag, 2, parseNativeStatusTail)
	if ok {
		t.Error("out-of-range index must fail the batch")
	}
	if results != nil {
		t.Errorf("failed batch must return nil results, got %+v", results)
	}
	// Duplicate index → whole batch fails, even when every row is present.
	// The remote emitted two different answers for instance 1; guessing the
	// first one is exactly the silent corruption the PR contract forbids.
	out = nativeStatusBatchTag + "1|running|1\n" + nativeStatusBatchTag + "2|stopped|\n" + nativeStatusBatchTag + "1|stopped|\n"
	if _, ok := parseStatusBatch(out, nativeStatusBatchTag, 2, parseNativeStatusTail); ok {
		t.Error("duplicate index must fail the batch")
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

// TestStatusBatchScriptLengthGrowsOnlyWithNames is the regression guard for
// the SSH max-packet finding: the old script inlined ~1.3 KB of shell per
// instance, so 24 runners exceeded a 32 KB packet and silently fell back to
// 2N+1 round trips. With one remote loop the only per-instance cost is the
// heredoc row, which is exactly what this delta asserts.
func TestStatusBatchScriptLengthGrowsOnlyWithNames(t *testing.T) {
	h := batchTestHost(&testutil.MockExecutor{})

	instances := make([]string, 64)
	for i := range instances {
		instances[i] = fmt.Sprintf("ci-%d", i+1)
	}
	one := instances[:1]

	nativeRows1, _ := nativeStatusBatchRows(one)
	nativeRows64, _ := nativeStatusBatchRows(instances)
	s1, n1 := buildNativeStatusBatchScript(h, nativeRows1)
	s64, n64 := buildNativeStatusBatchScript(h, nativeRows64)
	if n1 != 1 || n64 != 64 {
		t.Fatalf("chunk sizes = %d, %d; want 1, 64", n1, n64)
	}
	assertScriptGrowthOnlyFromRows(t, "native", s1, nativeRows1, s64, nativeRows64)

	containerRows1, _ := containerStatusBatchRows(one)
	containerRows64, _ := containerStatusBatchRows(instances)
	c1, cn1 := buildContainerStatusBatchScript(h, containerRows1)
	c64, cn64 := buildContainerStatusBatchScript(h, containerRows64)
	if cn1 != 1 || cn64 != 64 {
		t.Fatalf("container chunk sizes = %d, %d; want 1, 64", cn1, cn64)
	}
	assertScriptGrowthOnlyFromRows(t, "container", c1, containerRows1, c64, containerRows64)

	if len(s64) >= statusBatchScriptMaxBytes || len(c64) >= statusBatchScriptMaxBytes {
		t.Fatalf("64-instance script hit the cap (native %d, container %d, cap %d)",
			len(s64), len(c64), statusBatchScriptMaxBytes)
	}
}

func assertScriptGrowthOnlyFromRows(t *testing.T, label, small string, smallRows []string, large string, largeRows []string) {
	t.Helper()
	want := 0
	for _, r := range largeRows {
		want += len(r) + 1
	}
	for _, r := range smallRows {
		want -= len(r) + 1
	}
	if got := len(large) - len(small); got != want {
		t.Errorf("%s: 1→%d instances grew the script by %d bytes, want %d (rows only)",
			label, len(largeRows), got, want)
	}
}

// TestStatusBatchChunksWithinScriptLimit forces the instance-name list past
// the cap and asserts the batch slices into several bounded execs instead of
// emitting an oversized request (or failing wholesale).
func TestStatusBatchChunksWithinScriptLimit(t *testing.T) {
	const total = 240
	instances := make([]string, total)
	for i := range instances {
		instances[i] = fmt.Sprintf("ci-%03d-%s", i, strings.Repeat("x", 180))
	}

	mock := &testutil.MockExecutor{RunFn: func(cmd string) (string, error) {
		if !strings.Contains(cmd, statusBatchScriptHeader) {
			return "", errors.New("unexpected per-instance probe")
		}
		if len(cmd) > statusBatchScriptMaxBytes {
			t.Errorf("chunk script is %d bytes, cap %d", len(cmd), statusBatchScriptMaxBytes)
		}
		return composeNativeStatusBatchOutput(batchScriptRowCount(cmd)), nil
	}}
	h := batchTestHost(mock)

	results, ok := statusNativeBatch(h, instances)
	if !ok {
		t.Fatal("statusNativeBatch failed instead of chunking")
	}
	if len(results) != total {
		t.Fatalf("got %d results, want %d", len(results), total)
	}
	for i, r := range results {
		if r.local != "running" {
			t.Fatalf("results[%d].local = %q, want running", i, r.local)
		}
	}
	if len(mock.Calls) < 2 {
		t.Errorf("expected the name list to split across multiple bounded execs, got %d", len(mock.Calls))
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

// TestManagerStatusBatchMalformedOutputFallsBack covers the strict parse
// contract end-to-end: a duplicate marker from the remote must trigger the
// per-instance fallback rather than a silently guessed dashboard row.
func TestManagerStatusBatchMalformedOutputFallsBack(t *testing.T) {
	mock := &testutil.MockExecutor{RunFn: func(cmd string) (string, error) {
		if strings.Contains(cmd, statusBatchScriptHeader) {
			return nativeStatusBatchTag + "1|running|2.320.0\n" +
				nativeStatusBatchTag + "2|stopped|\n" +
				nativeStatusBatchTag + "1|stopped|\n", nil
		}
		return nativeStatusMock(cmd)
	}}
	h := batchTestHost(mock)
	mgr := NewManager("")

	statuses, err := mgr.Status(h, batchTestRC(2))
	if err != nil {
		t.Fatal(err)
	}
	if len(mock.Calls) != 1+2*2 {
		t.Errorf("%d round trips, want 5 (1 rejected batch + 2 per instance)", len(mock.Calls))
	}
	for i, s := range statuses {
		if s.Local != "running" {
			t.Errorf("statuses[%d].Local = %q, want running", i, s.Local)
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
// emitted markers for every state-machine branch, including the
// system-level systemd unit path reached through `sudo -n`.
func TestNativeStatusBatchScriptShell(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("POSIX-shell scenario test needs a Linux host")
	}
	shPath := "/bin/sh"
	if _, err := os.Stat(shPath); err != nil {
		t.Skipf("no %s: %v", shPath, err)
	}

	h := batchTestHost(&testutil.MockExecutor{})
	rows, ok := nativeStatusBatchRows([]string{"ci-1"})
	if !ok {
		t.Fatal("row build failed")
	}
	prefix := nativeStatusBatchPrefix(h)

	scenarios := []struct {
		name       string
		files      []string // paths relative to the instance dir; dirs created as needed
		unit       string   // ".service" contents (svc.sh branch)
		svcSh      bool
		userUnit   bool
		sysUnit    bool   // install a system-level unit (path injected into the script)
		svcState   string // systemctl answer for the svc.sh branch
		userState  string // systemctl --user answer
		sysState   string // systemctl answer on the `$SUDO systemctl` path
		sudoOK     string // "1" → passwordless sudo works
		want       string // "<local>|<version>" tail
		wantSudo   string // assert the sudo shim forwarded this exact command
		wantNoSudo bool   // assert the sudo shim never forwarded a command
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
			name: "empty pid file is stopped not unknown", userUnit: true, userState: "inactive",
			files: []string{".runner", ".runner_pid:", ".runner-version:2.320.0"}, want: "stopped|2.320.0",
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
		{
			name: "system unit active uses sudo -n", sysUnit: true, sudoOK: "1", sysState: "active",
			files:      []string{".runner-version:2.320.0"},
			want:       "running|2.320.0",
			wantSudo:   "systemctl is-active ghsr-runner-ci-1.service",
			wantNoSudo: false,
		},
		{
			name: "system unit failed maps to service error via sudo -n", sysUnit: true, sudoOK: "1", sysState: "failed",
			files:    []string{".runner-version:2.320.0"},
			want:     "service error|2.320.0",
			wantSudo: "systemctl is-active ghsr-runner-ci-1.service",
		},
		{
			name: "system unit without elevation falls back to pid file", sysUnit: true, sudoOK: "",
			files:      []string{".runner", ".runner_pid:999999999"},
			want:       "stopped|",
			wantNoSudo: true,
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
			// The system-unit path is fixed to /etc/systemd/system in
			// production. Inject a writable stand-in so the shell can really
			// execute the `$SUDO systemctl is-active` branch without root.
			sysDir := filepath.Join(home, "etc", "systemd", "system")
			if sc.sysUnit {
				if err := os.MkdirAll(sysDir, 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(sysDir, "ghsr-runner-ci-1.service"), []byte(""), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			script := strings.ReplaceAll(
				renderStatusBatchScript(prefix, rows),
				"/etc/systemd/system/", sysDir+"/",
			)

			bin := t.TempDir()
			trace := filepath.Join(t.TempDir(), "sudo.trace")
			writeShim(t, bin, "systemctl", `#!/bin/sh
case " $* " in
  *" --user "*) [ -n "$SHIM_USER_STATE" ] && printf '%s\n' "$SHIM_USER_STATE"; exit 0 ;;
esac
if [ -n "$SHIM_SYSTEMCTL_STATE" ]; then printf '%s\n' "$SHIM_SYSTEMCTL_STATE"; exit 0; fi
[ -n "$SHIM_SVC_STATE" ] && printf '%s\n' "$SHIM_SVC_STATE"
exit 0
`)
			// The sudo shim must forward `sudo -n <cmd> [args...]` verbatim.
			// The old shim did `exec systemctl "$@"`, which turned
			// `sudo -n systemctl is-active …` into
			// `systemctl -n systemctl is-active …` and hid the whole branch.
			writeShim(t, bin, "sudo", `#!/bin/sh
if [ "$1" = "-n" ] && [ "$2" = "true" ]; then
  [ "$SHIM_SUDO_OK" = 1 ] && exit 0 || exit 1
fi
[ "$1" = "-n" ] && shift
if [ -n "$SHIM_SUDO_TRACE" ]; then printf '%s\n' "$*" >> "$SHIM_SUDO_TRACE"; fi
SHIM_SYSTEMCTL_STATE="$SHIM_SYS_STATE"
export SHIM_SYSTEMCTL_STATE
exec "$@"
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
				"SHIM_SYS_STATE":  sc.sysState,
				"SHIM_SUDO_OK":    sc.sudoOK,
				"SHIM_SUDO_TRACE": trace,
				"WRITE_SELF_PID":  self,
			})
			if err != nil {
				t.Fatalf("script run failed: %v\noutput:\n%s", err, out)
			}
			if !strings.Contains(out, nativeStatusBatchTag+"1|"+sc.want+"\n") {
				t.Errorf("emitted %q, want %q\nfull output:\n%s", strings.TrimSpace(out), nativeStatusBatchTag+"1|"+sc.want, out)
			}
			traceOut, _ := os.ReadFile(trace)
			if sc.wantSudo != "" && !strings.Contains(string(traceOut), sc.wantSudo) {
				t.Errorf("$SUDO branch not exercised: trace %q, want %q\nfull output:\n%s", traceOut, sc.wantSudo, out)
			}
			if sc.wantNoSudo && len(traceOut) != 0 {
				t.Errorf("elevation failed but a command was forwarded through sudo: %q", traceOut)
			}
		})
	}
}

// TestContainerStatusProbeBody_IsSingleSourceOfTruth pins the shared shell
// state machine: both the one-shot probe and the Status batch must embed the
// exact containerStatusProbeBody fragment, exactly once. This is what stops
// an IMAGE / REVISION payload change from drifting between the two paths.
func TestContainerStatusProbeBody_IsSingleSourceOfTruth(t *testing.T) {
	body := containerStatusProbeBody()
	h := batchTestHost(&testutil.MockExecutor{})
	rows, ok := containerStatusBatchRows([]string{"ci-1"})
	if !ok {
		t.Fatal("row build failed")
	}
	batch, count := buildContainerStatusBatchScript(h, rows)
	if count != 1 {
		t.Fatalf("chunk carries %d rows, want 1", count)
	}
	for name, script := range map[string]string{
		"one-shot": containerLocalStatusOneShotScript("ci-1"),
		"batch":    batch,
	} {
		if got := strings.Count(script, body); got != 1 {
			t.Errorf("%s script embeds the probe body %d times, want 1\nscript:\n%s", name, got, script)
		}
	}
}

// TestContainerStatusBatchScriptShell executes both script shapes under a
// real POSIX shell with a docker shim, asserting that the batch payload is
// byte-for-byte the one-shot payload for the bootstrap-failed, missing
// container, and running-container cases.
func TestContainerStatusBatchScriptShell(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("POSIX-shell scenario test needs a Linux host")
	}
	shPath := "/bin/sh"
	if _, err := os.Stat(shPath); err != nil {
		t.Skipf("no %s: %v", shPath, err)
	}

	h := batchTestHost(&testutil.MockExecutor{})
	rows, _ := containerStatusBatchRows([]string{"ci-1"})
	batch, count := buildContainerStatusBatchScript(h, rows)
	if count != 1 {
		t.Fatalf("chunk carries %d rows, want 1", count)
	}
	oneShot := containerLocalStatusOneShotScript("ci-1")

	scenarios := []struct {
		name      string
		marker    bool   // bootstrap-failed marker present
		inspect   string // docker inspect answer; empty = container absent
		rev       string // docker image inspect answer
		wantState string
	}{
		{name: "container missing", wantState: "not installed"},
		{name: "bootstrap failed without container", marker: true, wantState: "failed"},
		{name: "running container", inspect: "running|gh-sr/agentic-runner:v1|sha256:abc", rev: "cafebabe", wantState: "running"},
		{name: "bootstrap failed with container", marker: true, inspect: "exited|gh-sr/agentic-runner:v1|sha256:abc", rev: "cafebabe", wantState: "failed"},
	}

	for _, sc := range scenarios {
		t.Run(sc.name, func(t *testing.T) {
			home := t.TempDir()
			dir := filepath.Join(home, ".gh-sr", "runners", "ci-1")
			if err := os.MkdirAll(dir, 0o755); err != nil {
				t.Fatal(err)
			}
			if sc.marker {
				if err := os.WriteFile(filepath.Join(dir, "bootstrap-failed"), []byte(""), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			bin := t.TempDir()
			writeShim(t, bin, "docker", `#!/bin/sh
case "$1" in
  inspect)
    [ -n "$SHIM_DOCKER_INSPECT" ] && printf '%s\n' "$SHIM_DOCKER_INSPECT" || exit 1 ;;
  image)
    printf '%s\n' "$SHIM_DOCKER_REV" ;;
esac
`)
			env := map[string]string{
				"SHIM_DOCKER_INSPECT": sc.inspect,
				"SHIM_DOCKER_REV":     sc.rev,
			}
			oneOut, err := runScript(t, shPath, bin, home, oneShot, env)
			if err != nil {
				t.Fatalf("one-shot script failed: %v\noutput:\n%s", err, oneOut)
			}
			batchOut, err := runScript(t, shPath, bin, home, batch, env)
			if err != nil {
				t.Fatalf("batch script failed: %v\noutput:\n%s", err, batchOut)
			}
			payload := strings.TrimSpace(oneOut)
			if want := containerStatusBatchTag + "1|" + payload; !strings.Contains(batchOut, want) {
				t.Errorf("batch emitted %q, want %q", strings.TrimSpace(batchOut), want)
			}
			local, _, _ := parseContainerStatusInspectOutput(payload)
			if local != sc.wantState {
				t.Errorf("state = %q, want %q (payload %q)", local, sc.wantState, payload)
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

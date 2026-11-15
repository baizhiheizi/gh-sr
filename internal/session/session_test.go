package session

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/an-lee/gh-sr/internal/doctor"
)

// TestLoad_resolvesPathsAndConfig pins the happy path: resolved paths, loaded
// config, and the version stamp carried for the derived Manager.
func TestLoad_resolvesPathsAndConfig(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	p := filepath.Join(dir, "runners.yml")
	if err := os.WriteFile(p, []byte("hosts:\n  h1:\n    addr: user@10.0.0.1\nrunners:\n  - name: r1\n    repo: o/r\n    host: h1\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	s, err := Load(p, "test-version")
	if err != nil {
		t.Fatal(err)
	}
	if s.ConfigErr != nil {
		t.Errorf("valid config must not record ConfigErr: %v", s.ConfigErr)
	}
	if s.Config == nil {
		t.Error("valid config must populate Config")
	}
	if s.ConfigPath == "" || s.EnvPath == "" {
		t.Errorf("paths must resolve, got cfg=%q env=%q", s.ConfigPath, s.EnvPath)
	}
	if want, _ := filepath.Abs(p); s.ConfigPath != want {
		t.Errorf("ConfigPath = %q, want %q", s.ConfigPath, want)
	}
	if s.GhSrVersion != "test-version" {
		t.Errorf("GhSrVersion = %q, want test-version", s.GhSrVersion)
	}
	// Token resolution is environment-dependent (a live `gh auth token` on
	// dev machines) — exercise it once without asserting the outcome, so a
	// panic or hard failure in the wiring still shows up.
	s.Token()
}

// TestLoad_recordsConfigError pins the doctor-degradation contract: a broken
// config is NOT fatal — Load succeeds with ConfigErr recorded and Config nil,
// so doctor can still run and report the failure as a FAIL row.
func TestLoad_recordsConfigError(t *testing.T) {
	t.Parallel()
	s, err := Load(filepath.Join(t.TempDir(), "missing.yml"), "test")
	if err != nil {
		t.Fatalf("missing config must not abort Load: %v", err)
	}
	if s.ConfigErr == nil {
		t.Error("missing config must record ConfigErr")
	}
	if s.Config != nil {
		t.Error("missing config must leave Config nil")
	}
}

// TestDoctorFixLoop pins the doctor --fix state machine through injected
// doctor/ops steps: when the fix flag is set and the first run failed with a
// loadable config, setup runs once, doctor re-runs, and the exit code comes
// from the re-run. Any other combination never touches setup.
func TestDoctorFixLoop(t *testing.T) {
	t.Parallel()
	fail := doctor.Result{Fail: 2}
	warn := doctor.Result{Warn: 1}
	pass := doctor.Result{}

	type step struct {
		doctorRuns int
		setupRuns  int
	}

	run := func(t *testing.T, strict, cfgReady, fix bool, first, rerun doctor.Result, setupErr error) (int, step, string) {
		t.Helper()
		var st step
		var buf bytes.Buffer
		var last doctor.Result
		code := doctorFixLoop(&buf, strict, cfgReady, fix,
			func() doctor.Result {
				st.doctorRuns++
				if st.doctorRuns == 1 {
					last = first
				} else {
					last = rerun
				}
				return last
			},
			func() error {
				st.setupRuns++
				return setupErr
			},
		)
		return code, st, buf.String()
	}

	t.Run("fix disabled never attempts setup", func(t *testing.T) {
		code, st, _ := run(t, false, true, false, fail, pass, nil)
		if code != 1 || st.doctorRuns != 1 || st.setupRuns != 0 {
			t.Errorf("code=%d steps=%+v", code, st)
		}
	})

	t.Run("fix on failure runs setup then re-runs doctor", func(t *testing.T) {
		code, st, out := run(t, false, true, true, fail, pass, nil)
		if code != 0 {
			t.Errorf("re-run passing must exit 0, got %d", code)
		}
		if st.doctorRuns != 2 || st.setupRuns != 1 {
			t.Errorf("want 2 doctor runs + 1 setup, got %+v", st)
		}
		for _, want := range []string{
			"--- Running `gh sr setup` to attempt fixes ---",
			"fix attempt completed successfully.",
			"--- Re-running doctor after fix ---",
		} {
			if !bytes.Contains([]byte(out), []byte(want)) {
				t.Errorf("output missing %q:\n%s", want, out)
			}
		}
	})

	t.Run("fix with loadable-config gate skips setup", func(t *testing.T) {
		code, st, _ := run(t, false, false, true, fail, pass, nil)
		if code != 1 || st.setupRuns != 0 {
			t.Errorf("broken config must skip setup: code=%d steps=%+v", code, st)
		}
	})

	t.Run("fix with no failures skips setup", func(t *testing.T) {
		code, st, _ := run(t, false, true, true, pass, pass, nil)
		if code != 0 || st.doctorRuns != 1 || st.setupRuns != 0 {
			t.Errorf("no failures must skip setup: code=%d steps=%+v", code, st)
		}
	})

	t.Run("setup errors still re-run doctor and take the re-run exit code", func(t *testing.T) {
		code, st, out := run(t, false, true, true, fail, fail, errString("boom"))
		if code != 1 {
			t.Errorf("re-run failing must exit 1, got %d", code)
		}
		if st.doctorRuns != 2 || st.setupRuns != 1 {
			t.Errorf("want 2 doctor runs + 1 setup, got %+v", st)
		}
		if !bytes.Contains([]byte(out), []byte("fix attempt completed with errors: boom")) {
			t.Errorf("setup error must be reported:\n%s", out)
		}
	})

	t.Run("strict escalates warnings without attempting setup", func(t *testing.T) {
		code, st, _ := run(t, true, true, true, warn, warn, nil)
		if code != 1 || st.setupRuns != 0 {
			t.Errorf("strict warn-only must exit 1 without setup: code=%d steps=%+v", code, st)
		}
	})
}

// errString is a minimal error for the injected setup step.
type errString string

func (e errString) Error() string { return string(e) }

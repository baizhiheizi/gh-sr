package session

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/an-lee/gh-sr/internal/doctor"
)

const validConfigYAML = "hosts:\n  h1:\n    addr: user@10.0.0.1\nrunners:\n  - name: r1\n    repo: o/r\n    host: h1\n"

// writeValidConfig writes a minimal valid config and returns its path.
func writeValidConfig(t *testing.T) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "runners.yml")
	if err := os.WriteFile(p, []byte(validConfigYAML), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

// TestLoad_resolvesPathsAndConfig pins the happy path: resolved paths, loaded
// config, and the version stamp carried for the derived Manager.
func TestLoad_resolvesPathsAndConfig(t *testing.T) {
	t.Parallel()
	p := writeValidConfig(t)
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

// TestLoad_bootstrapFailureIsFatal pins the fatal half of the error policy:
// when the bootstrap (env-file) step cannot even resolve the home directory,
// Load aborts instead of returning a degraded Session.
func TestLoad_bootstrapFailureIsFatal(t *testing.T) {
	t.Setenv("HOME", "")
	if _, err := Load("", "test"); err == nil {
		t.Fatal("bootstrap failure (no resolvable home dir) must abort Load")
	}
}

const fakeToken = "fake-session-test-token"

// isolateHostEnv points HOME at a temp dir so session.Load's BootstrapEnv
// cannot apply the host's real ~/.gh-sr/env. ApplyEnvFile sets variables
// unconditionally, so a host file carrying GH_TOKEN would override the
// token a test set — and print the real token into the test log when an
// assertion fails.
func isolateHostEnv(t *testing.T) {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
}

// fakeTokenEnv pins the resolved token to a fixed value and cuts the host's
// env file out of the picture, so token resolution never depends on how the
// host is authenticated: GH_TOKEN wins in go-gh's lookup order, and the
// isolated HOME keeps ~/.gh-sr/env from overriding it.
func fakeTokenEnv(t *testing.T) {
	t.Helper()
	t.Setenv("GH_TOKEN", fakeToken)
	isolateHostEnv(t)
}

// noTokenEnv makes "no token is resolvable" a property of the test rather
// than of the host. go-gh's TokenForHost tries three sources in order: the
// token env vars, the gh config file, and — as a last resort — exec'ing the
// real `gh auth token --secure-storage`, which reads the OS keyring and is
// not covered by GH_CONFIG_DIR. The blanked vars and the empty config dir
// cut off the first two; GH_PATH pointing at a missing binary cuts off the
// third, so a host with a live `gh auth login` still resolves no token.
// t.Setenv forbids parallel tests, so every test using it runs serially.
func noTokenEnv(t *testing.T) {
	t.Helper()
	t.Setenv("GH_TOKEN", "")
	t.Setenv("GITHUB_TOKEN", "")
	t.Setenv("GH_CONFIG_DIR", t.TempDir())
	t.Setenv("GH_PATH", filepath.Join(t.TempDir(), "missing-gh"))
	isolateHostEnv(t)
}

// TestToken_memoizesFirstOutcome pins the process-lifetime contract: the
// second call returns exactly the first result — a mid-session `gh auth
// logout` is not re-probed.
func TestToken_memoizesFirstOutcome(t *testing.T) {
	fakeTokenEnv(t)
	s, err := Load(filepath.Join(t.TempDir(), "missing.yml"), "test")
	if err != nil {
		t.Fatal(err)
	}
	tok1, err1 := s.Token()
	tok2, err2 := s.Token()
	if tok1 != fakeToken || tok2 != tok1 || err1 != nil || err1 != err2 {
		t.Fatalf("Token must memoize the first outcome: first (%q, %v), second (%q, %v)", tok1, err1, tok2, err2)
	}
}

// TestGitHubAndManager_errorWithoutToken pins the failure half of the
// documented error split: GitHub() returns nil (doctor degrades) while
// Manager() surfaces the error (runner operations are load-bearing).
func TestGitHubAndManager_errorWithoutToken(t *testing.T) {
	noTokenEnv(t)
	s, err := Load(writeValidConfig(t), "test-version")
	if err != nil {
		t.Fatal(err)
	}
	if gh := s.GitHub(); gh != nil {
		t.Error("GitHub() must return nil when no token can be resolved")
	}
	m, mgrErr := s.Manager(&bytes.Buffer{})
	if m != nil || mgrErr == nil {
		t.Errorf("Manager must fail with a nil client when no token resolves, got (%v, %v)", m, mgrErr)
	}
}

// TestGitHubAndManager_wireFromToken pins the success half: both clients
// build from a resolvable token, the Manager carries the session's writer
// and version stamp, and a nil config still yields a Manager — the
// native-only shape with zero config-derived fields.
func TestGitHubAndManager_wireFromToken(t *testing.T) {
	fakeTokenEnv(t)
	s, err := Load(writeValidConfig(t), "test-version")
	if err != nil {
		t.Fatal(err)
	}
	if gh := s.GitHub(); gh == nil {
		t.Fatal("GitHub() must build a client when a token resolves")
	}

	var buf bytes.Buffer
	m, mgrErr := s.Manager(&buf)
	if mgrErr != nil || m == nil {
		t.Fatalf("Manager must build when the token resolves, got (%v, %v)", m, mgrErr)
	}
	if m.GhSrVersion != "test-version" {
		t.Errorf("GhSrVersion = %q, want session stamp test-version", m.GhSrVersion)
	}
	var wantOut io.Writer = &buf
	if m.Out != wantOut {
		t.Error("Manager.Out must be the writer passed to Manager()")
	}

	// The nil-config shape: a session whose config failed to load must still
	// produce a Manager, with no config-derived fields.
	broken, err := Load(filepath.Join(t.TempDir(), "missing.yml"), "v2")
	if err != nil {
		t.Fatal(err)
	}
	m2, m2Err := broken.Manager(io.Discard)
	if m2Err != nil || m2 == nil {
		t.Fatalf("Manager must build from a nil config, got (%v, %v)", m2, m2Err)
	}
	if m2.ContainerImageBaseImage != "" {
		t.Errorf("nil config must yield zero derived fields, got image base %q", m2.ContainerImageBaseImage)
	}
	if m2.GhSrVersion != "v2" {
		t.Errorf("GhSrVersion = %q, want session stamp v2", m2.GhSrVersion)
	}
}

// TestDoctorInput_assemblesRunInput pins the single-assembly-point contract:
// DoctorInput maps the session's fields and the caller's flags onto
// doctor.RunInput verbatim, so no call site re-assembles it by hand.
func TestDoctorInput_assemblesRunInput(t *testing.T) {
	fakeTokenEnv(t)
	s, err := Load(writeValidConfig(t), "test-version")
	if err != nil {
		t.Fatal(err)
	}
	in := s.DoctorInput("h1", "o/r", true, true)
	if in.ConfigPath != s.ConfigPath || in.EnvPath != s.EnvPath {
		t.Errorf("paths not carried: cfg=%q env=%q", in.ConfigPath, in.EnvPath)
	}
	if in.Config != s.Config || in.ConfigErr != s.ConfigErr {
		t.Error("config (and its recorded error) must be carried by reference")
	}
	if in.FilterHost != "h1" || in.FilterRepo != "o/r" || !in.Strict || !in.CheckLockfiles {
		t.Errorf("flags not carried: host=%q repo=%q strict=%v lockfiles=%v",
			in.FilterHost, in.FilterRepo, in.Strict, in.CheckLockfiles)
	}
	if in.GitHub == nil {
		t.Error("GitHub field must carry the built client")
	}
}

// TestDoctorFix_wiring pins the untested shell around doctorFixLoop: the
// closures hand doctor.Run the session's assembled input (filters included)
// and the cfgReady gate is s.ConfigErr == nil. Both scenarios stay offline
// on every machine: the filtered-out config warns out of doctor.Run before
// the API section, and the broken config fails before it.
func TestDoctorFix_wiring(t *testing.T) {
	noTokenEnv(t)
	t.Run("valid config with no matching filter: exit 0, setup untouched", func(t *testing.T) {
		var buf bytes.Buffer
		s, err := Load(writeValidConfig(t), "test-version")
		if err != nil {
			t.Fatal(err)
		}
		code := DoctorFix(&buf, s, false, "no-such-host", "", false, false)
		if code != 0 {
			t.Fatalf("warn-only doctor run must exit 0, got %d:\n%s", code, buf.String())
		}
		if !bytes.Contains(buf.Bytes(), []byte("no runners match")) {
			t.Errorf("filters must flow through DoctorInput into doctor.Run:\n%s", buf.String())
		}
		if bytes.Contains(buf.Bytes(), []byte("gh sr setup")) {
			t.Errorf("fix=false must never attempt setup:\n%s", buf.String())
		}
	})

	t.Run("broken config with fix on: gate skips setup, config error drives exit 1", func(t *testing.T) {
		var buf bytes.Buffer
		s, err := Load(filepath.Join(t.TempDir(), "missing.yml"), "test")
		if err != nil {
			t.Fatal(err)
		}
		code := DoctorFix(&buf, s, true, "", "", false, false)
		if code != 1 {
			t.Fatalf("config failure must exit 1, got %d:\n%s", code, buf.String())
		}
		if !bytes.Contains(buf.Bytes(), []byte("config file not found")) {
			t.Errorf("recorded ConfigErr must surface as a doctor FAIL row:\n%s", buf.String())
		}
		if bytes.Contains(buf.Bytes(), []byte("gh sr setup")) {
			t.Errorf("a broken config must gate setup off even with fix=true:\n%s", buf.String())
		}
	})
}

// errString is a minimal error for the injected setup step.
type errString string

func (e errString) Error() string { return string(e) }

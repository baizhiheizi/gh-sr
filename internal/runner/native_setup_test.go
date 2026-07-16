package runner

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/an-lee/gh-sr/internal/config"
	"github.com/an-lee/gh-sr/internal/host"
	"github.com/an-lee/gh-sr/internal/testutil"
)

// setupNativeGitHubServer stands up a tiny httptest server that satisfies the
// two API calls setupNative performs: GetLatestRunnerVersion
// (GET /repos/actions/runner/releases/latest) and GetRegistrationTokenScoped
// (POST /repos/<repo>/actions/runners/registration-token). The returned Manager
// already has GitHub wired to that server. The registration endpoint always
// returns the configured token; the latest-version endpoint can be customised
// to surface error/empty-tag paths from the test.
func setupNativeGitHubServer(t *testing.T, version string, versionStatus int) (*Manager, *httptest.Server) {
	t.Helper()
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/repos/actions/runner/releases/latest"):
			if versionStatus != http.StatusOK {
				http.Error(w, "boom", versionStatus)
				return
			}
			_ = json.NewEncoder(w).Encode(releaseResponse{TagName: "v" + version})
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/actions/runners/registration-token"):
			_ = json.NewEncoder(w).Encode(tokenResponse{Token: "TEST-TOKEN"})
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(ts.Close)
	m := NewManager("")
	m.GitHub = NewGitHubClientWithHTTP("p", ts.Client(), ts.URL)
	return m, ts
}

// setupNativeLinuxHost wires a Linux host backed by a fresh MockExecutor.
func setupNativeLinuxHost(t *testing.T, mock *testutil.MockExecutor) *host.Host {
	t.Helper()
	h := host.NewHost("h", config.HostConfig{Addr: "runner@vps", OS: "linux", Arch: "amd64"})
	h.SetConn(mock)
	return h
}

// TestSetupNative_linuxDownloadsAndConfiguresRunner covers the happy path of
// setupNative on Linux for a fresh host: each per-instance directory is
// created, the runner tarball is downloaded and extracted, the .runner-version
// marker is written, the systemd svc.sh files are deployed, and config.sh is
// invoked once with the registration token from GitHub.
func TestSetupNative_linuxDownloadsAndConfiguresRunner(t *testing.T) {
	t.Parallel()

	m, _ := setupNativeGitHubServer(t, "2.320.0", http.StatusOK)
	var buf bytes.Buffer
	m.Out = &buf

	var sawDownload, sawExtract, sawSvcShInstall, sawRegister, sawVersionWrite bool
	mock := &testutil.MockExecutor{
		RunFn: func(cmd string) (string, error) {
			switch {
			// Per-instance presence probe from NativeRunnerConfigPresent.
			case strings.Contains(cmd, ".gh-sr/runners/ci-1") && strings.Contains(cmd, "test -f") && strings.Contains(cmd, probeTailYes):
				return "no\n", nil
			// Tarball download.
			case strings.Contains(cmd, "curl -fSL") && strings.Contains(cmd, "ghsr-runner-ci-1-2.320.0.tar.gz"):
				sawDownload = true
				return "", nil
			// Extract the downloaded tarball.
			case strings.Contains(cmd, "tar xzf") && strings.Contains(cmd, "ghsr-runner-ci-1-2.320.0.tar.gz"):
				sawExtract = true
				return "", nil
			// svc.sh install command from setupNative's systemd branch.
			case strings.Contains(cmd, "svc.sh install"):
				sawSvcShInstall = true
				return "", nil
			// config.sh registration call carries the test token and --replace.
			case strings.Contains(cmd, "./config.sh --unattended") && strings.Contains(cmd, "TEST-TOKEN"):
				sawRegister = true
				return "", nil
			default:
				return "", nil
			}
		},
		UploadCalled: false,
	}
	// WriteRemoteBytes goes through Run (not Upload) on POSIX hosts; assert it.
	defer func() {
		// Allow the test to inspect Upload as a no-op surface.
	}()
	h := setupNativeLinuxHost(t, mock)
	rc := config.RunnerConfig{Name: "ci", Repo: "o/r", Host: "h", Count: 1, Labels: []string{"self-hosted", "linux"}}

	if err := m.setupNative(h, rc); err != nil {
		t.Fatalf("setupNative: %v", err)
	}

	for _, c := range mock.Calls {
		// Detect the WriteRemoteBytes call: it's the only one piping base64-decoded
		// data into the .runner-version marker.
		if strings.Contains(c, "base64 -d") && strings.Contains(c, ".gh-sr/runners/ci-1/.runner-version") {
			sawVersionWrite = true
		}
	}
	if !sawDownload {
		t.Errorf("runner tarball was not downloaded; calls=%v", mock.Calls)
	}
	if !sawExtract {
		t.Errorf("runner tarball was not extracted; calls=%v", mock.Calls)
	}
	if !sawSvcShInstall {
		t.Errorf("svc.sh install was not invoked; calls=%v", mock.Calls)
	}
	if !sawRegister {
		t.Errorf("./config.sh was not invoked with the registration token; calls=%v", mock.Calls)
	}
	if !sawVersionWrite {
		t.Errorf(".runner-version marker was not written via WriteRemoteBytes; calls=%v", mock.Calls)
	}
	if !strings.Contains(buf.String(), "installing runner v2.320.0") {
		t.Errorf("missing install log line: %q", buf.String())
	}
}

// TestSetupNative_skipsAlreadyInstalled verifies the early "already installed,
// skipping" branch: when NativeRunnerConfigPresent reports yes for an instance,
// setupNative must not download the tarball or re-register that instance.
func TestSetupNative_skipsAlreadyInstalled(t *testing.T) {
	t.Parallel()

	m, _ := setupNativeGitHubServer(t, "2.320.0", http.StatusOK)
	var buf bytes.Buffer
	m.Out = &buf

	mock := &testutil.MockExecutor{
		RunFn: func(cmd string) (string, error) {
			// Per-instance presence probe returns yes → skip.
			if strings.Contains(cmd, ".gh-sr/runners/ci-1") && strings.Contains(cmd, "test -f") && strings.Contains(cmd, probeTailYes) {
				return "yes\n", nil
			}
			return "", nil
		},
	}
	h := setupNativeLinuxHost(t, mock)
	rc := config.RunnerConfig{Name: "ci", Repo: "o/r", Host: "h", Count: 1}

	if err := m.setupNative(h, rc); err != nil {
		t.Fatalf("setupNative: %v", err)
	}
	for _, c := range mock.Calls {
		if strings.Contains(c, "curl -fSL") {
			t.Errorf("should not download tarball when instance already installed; calls=%v", mock.Calls)
		}
		if strings.Contains(c, "./config.sh --unattended") {
			t.Errorf("should not re-register when instance already installed; calls=%v", mock.Calls)
		}
	}
	if !strings.Contains(buf.String(), "already installed, skipping") {
		t.Errorf("missing 'already installed, skipping' log line: %q", buf.String())
	}
}

// TestSetupNative_propagatesVersionError covers the early-return path when
// GetLatestRunnerVersion fails (network outage / API error). The error must
// propagate without any install commands being issued.
func TestSetupNative_propagatesVersionError(t *testing.T) {
	t.Parallel()

	m, _ := setupNativeGitHubServer(t, "", http.StatusInternalServerError)
	var buf bytes.Buffer
	m.Out = &buf

	mock := &testutil.MockExecutor{}
	h := setupNativeLinuxHost(t, mock)
	rc := config.RunnerConfig{Name: "ci", Repo: "o/r", Host: "h", Count: 1}

	err := m.setupNative(h, rc)
	if err == nil {
		t.Fatalf("expected error from GetLatestRunnerVersion failure, got nil; calls=%v", mock.Calls)
	}
	if !strings.Contains(err.Error(), "fetching latest runner version") &&
		!strings.Contains(err.Error(), "500") {
		t.Errorf("unexpected error: %v", err)
	}
	if len(mock.Calls) != 0 {
		t.Errorf("no install commands should run when version lookup fails; calls=%v", mock.Calls)
	}
}

// TestSetupNative_unsupportedOSArch_returnsError covers the unsupported OS/arch
// short-circuit: when runnerTarballURL returns "" (e.g. freebsd), setupNative
// must return a descriptive error before any install commands are issued.
func TestSetupNative_unsupportedOSArch_returnsError(t *testing.T) {
	t.Parallel()

	m, _ := setupNativeGitHubServer(t, "2.320.0", http.StatusOK)
	var buf bytes.Buffer
	m.Out = &buf

	mock := &testutil.MockExecutor{}
	h := host.NewHost("bsd", config.HostConfig{Addr: "runner@vps", OS: "freebsd", Arch: "amd64"})
	h.SetConn(mock)
	rc := config.RunnerConfig{Name: "ci", Repo: "o/r", Host: "h", Count: 1}

	err := m.setupNative(h, rc)
	if err == nil {
		t.Fatalf("expected unsupported OS/arch error, got nil; calls=%v", mock.Calls)
	}
	if !strings.Contains(err.Error(), "unsupported OS/arch") {
		t.Errorf("error should mention unsupported OS/arch: %v", err)
	}
	if len(mock.Calls) != 0 {
		t.Errorf("no install commands should run for unsupported OS/arch; calls=%v", mock.Calls)
	}
}

// TestStartNativeOnce_callsSetupWhenMissing covers the auto-setup branch of
// startNativeOnce: when NativeRunnerConfigPresent returns false (no instance
// directory or no .runner file), startNativeOnce must invoke setupNative and
// then issue the start command. The probe must NOT be repeated once setup has
// installed the runner.
func TestStartNativeOnce_callsSetupWhenMissing(t *testing.T) {
	t.Parallel()

	m, _ := setupNativeGitHubServer(t, "2.320.0", http.StatusOK)
	var buf bytes.Buffer
	m.Out = &buf

	presenceChecks := 0
	sawStart := false
	sawDownload := false
	mock := &testutil.MockExecutor{
		RunFn: func(cmd string) (string, error) {
			switch {
			// First (and only) presence probe: report not installed → setupNative fires.
			case strings.Contains(cmd, ".gh-sr/runners/ci-1") && strings.Contains(cmd, "test -f") && strings.Contains(cmd, probeTailYes):
				presenceChecks++
				return "no\n", nil
			case strings.Contains(cmd, "curl -fSL") && strings.Contains(cmd, "ghsr-runner-ci-1-2.320.0.tar.gz"):
				sawDownload = true
				return "", nil
			case strings.Contains(cmd, "nohup ./run.sh"):
				sawStart = true
				return "started PID 1234\n", nil
			default:
				return "", nil
			}
		},
	}
	h := setupNativeLinuxHost(t, mock)
	rc := config.RunnerConfig{Name: "ci", Repo: "o/r", Host: "h", Count: 1}

	if err := m.startNativeOnce(h, rc, "ci-1", false); err != nil {
		t.Fatalf("startNativeOnce: %v", err)
	}
	// startNativeOnce probes once (returns "no"), then setupNative probes once
	// more to decide whether to install. Two probes total is expected.
	if presenceChecks != 2 {
		t.Errorf("presence probe count = %d, want 2 (startNativeOnce + setupNative); calls=%v", presenceChecks, mock.Calls)
	}
	if !sawDownload {
		t.Errorf("setupNative download step was not invoked; calls=%v", mock.Calls)
	}
	if !sawStart {
		t.Errorf("start command was not invoked after setup; calls=%v", mock.Calls)
	}
	if !strings.Contains(buf.String(), "not installed, running setup") {
		t.Errorf("missing auto-setup log line: %q", buf.String())
	}
}

// TestStartNativeOnce_runsStartCmdWhenInstalled covers the steady-state branch
// of startNativeOnce: when NativeRunnerConfigPresent returns true, the function
// must skip setupNative entirely and only run the start command.
func TestStartNativeOnce_runsStartCmdWhenInstalled(t *testing.T) {
	t.Parallel()

	m, _ := setupNativeGitHubServer(t, "2.320.0", http.StatusOK)
	var buf bytes.Buffer
	m.Out = &buf

	mock := &testutil.MockExecutor{
		RunFn: func(cmd string) (string, error) {
			switch {
			case strings.Contains(cmd, ".gh-sr/runners/ci-1") && strings.Contains(cmd, "test -f") && strings.Contains(cmd, probeTailYes):
				return "yes\n", nil
			case strings.Contains(cmd, "nohup ./run.sh"):
				return "started PID 9999\n", nil
			default:
				return "", nil
			}
		},
	}
	h := setupNativeLinuxHost(t, mock)
	rc := config.RunnerConfig{Name: "ci", Repo: "o/r", Host: "h", Count: 1}

	if err := m.startNativeOnce(h, rc, "ci-1", false); err != nil {
		t.Fatalf("startNativeOnce: %v", err)
	}
	for _, c := range mock.Calls {
		if strings.Contains(c, "curl -fSL") || strings.Contains(c, "./config.sh --unattended") {
			t.Errorf("setupNative must not run when instance is already installed; offending call: %q", c)
		}
	}
	if !strings.Contains(buf.String(), "started PID 9999") {
		t.Errorf("start log missing; buf=%q", buf.String())
	}
}

// TestHandleStaleRegistration_clearsCredentialsAndReconfigures covers the
// stale-registration recovery path on Linux: when the stale check returns
// "stale", handleStaleRegistration must rm the credential files, re-run
// setupNative, and call startNativeOnce with retryOnStale=false to avoid an
// infinite stale-recovery loop. The second start must NOT trigger another
// stale check.
func TestHandleStaleRegistration_clearsCredentialsAndReconfigures(t *testing.T) {
	t.Parallel()

	m, _ := setupNativeGitHubServer(t, "2.320.0", http.StatusOK)
	var buf bytes.Buffer
	m.Out = &buf

	const instance = "ci-1"
	var presenceChecks, staleChecks, credRmCalls, downloads, starts int
	mock := &testutil.MockExecutor{
		RunFn: func(cmd string) (string, error) {
			switch {
			// Probes — the 2nd probe (setupNative) sees the .runner file just
			// deleted by handleStaleRegistration and reports not installed.
			case strings.Contains(cmd, ".gh-sr/runners/"+instance) && strings.Contains(cmd, "test -f") && strings.Contains(cmd, probeTailYes):
				presenceChecks++
				if presenceChecks == 2 {
					return "no\n", nil
				}
				return "yes\n", nil
			// Stale check: emits "stale" the first time (triggers handleStaleRegistration)
			// and "ok" on any subsequent call so we can detect a runaway loop.
			case strings.Contains(cmd, "grep -q") && strings.Contains(cmd, "runner.log"):
				staleChecks++
				if staleChecks == 1 {
					return "stale\n", nil
				}
				return "ok\n", nil
			// Credential scrub: handleStaleRegistration's first action.
			case strings.Contains(cmd, "rm -f") && strings.Contains(cmd, ".runner") && strings.Contains(cmd, ".credentials"):
				credRmCalls++
				return "", nil
			// setupNative download path runs again after credential scrub.
			case strings.Contains(cmd, "curl -fSL") && strings.Contains(cmd, "ghsr-runner-"+instance+"-2.320.0.tar.gz"):
				downloads++
				return "", nil
			// start command: must run twice (initial start + retry from handleStaleRegistration).
			case strings.Contains(cmd, "nohup ./run.sh"):
				starts++
				return "started PID 4242\n", nil
			default:
				return "", nil
			}
		},
	}
	h := setupNativeLinuxHost(t, mock)
	rc := config.RunnerConfig{Name: "ci", Repo: "o/r", Host: "h", Count: 1}

	// Call startNativeOnce with retryOnStale=true to exercise the recovery path.
	if err := m.startNativeOnce(h, rc, instance, true); err != nil {
		t.Fatalf("startNativeOnce: %v", err)
	}

	// Three probes total — see the switch-case comment above for the order.
	if presenceChecks != 3 {
		t.Errorf("presence probe count = %d, want 3 (initial + setupNative + retry); calls=%v", presenceChecks, mock.Calls)
	}
	// Critical: the retry must NOT trigger another stale check.
	if staleChecks != 1 {
		t.Errorf("stale check count = %d, want 1 (initial only — retry must skip stale check); calls=%v", staleChecks, mock.Calls)
	}
	if credRmCalls != 1 {
		t.Errorf("credential rm count = %d, want 1; calls=%v", credRmCalls, mock.Calls)
	}
	if downloads != 1 {
		t.Errorf("re-setup download count = %d, want 1; calls=%v", downloads, mock.Calls)
	}
	if starts != 2 {
		t.Errorf("start count = %d, want 2 (initial + post-stale retry); calls=%v", starts, mock.Calls)
	}
	if !strings.Contains(buf.String(), "registration expired on GitHub, re-configuring") {
		t.Errorf("missing stale-recovery log line; buf=%q", buf.String())
	}
}

// TestEnsureSetup_skipsWhenAlreadyInstalled covers the wrapper branch that
// gates Setup: when NeedsSetup returns false (every instance fully installed),
// EnsureSetup must short-circuit and never invoke Setup. This protects against
// re-registering an existing runner during `gh sr up`.
func TestEnsureSetup_skipsWhenAlreadyInstalled(t *testing.T) {
	t.Parallel()

	m, _ := setupNativeGitHubServer(t, "2.320.0", http.StatusOK)
	var buf bytes.Buffer
	m.Out = &buf

	mock := &testutil.MockExecutor{
		RunFn: answerInstancePresence(map[string]bool{"ci-1": true, "ci-2": true}),
	}
	h := needsSetupMockHost(t, mock)
	rc := config.RunnerConfig{Name: "ci", Repo: "o/r", Host: "h", Count: 2}

	if err := m.EnsureSetup(h, rc); err != nil {
		t.Fatalf("EnsureSetup: %v", err)
	}
	for _, c := range mock.Calls {
		if strings.Contains(c, "curl -fSL") || strings.Contains(c, "./config.sh --unattended") {
			t.Errorf("EnsureSetup must skip setup when every instance is present; offending call: %q", c)
		}
	}
}

// TestEnsureSetup_runsSetupWhenMissing covers the wrapper's fall-through:
// when NeedsSetup returns true (any instance missing), EnsureSetup must invoke
// Setup, which in turn runs setupNative. This guards the `gh sr up` recovery
// path on hosts where the runner was partially installed.
func TestEnsureSetup_runsSetupWhenMissing(t *testing.T) {
	t.Parallel()

	m, _ := setupNativeGitHubServer(t, "2.320.0", http.StatusOK)
	var buf bytes.Buffer
	m.Out = &buf

	// ci-1 already installed, ci-2 missing → NeedsSetup returns true → setup runs.
	mock := &testutil.MockExecutor{
		RunFn: func(cmd string) (string, error) {
			switch {
			case strings.Contains(cmd, ".gh-sr/runners/ci-1") && strings.Contains(cmd, probeTailYes):
				return "yes\n", nil
			case strings.Contains(cmd, ".gh-sr/runners/ci-2") && strings.Contains(cmd, probeTailYes):
				return "no\n", nil
			default:
				return "", nil
			}
		},
	}
	h := needsSetupMockHost(t, mock)
	rc := config.RunnerConfig{Name: "ci", Repo: "o/r", Host: "h", Count: 2}

	if err := m.EnsureSetup(h, rc); err != nil {
		t.Fatalf("EnsureSetup: %v", err)
	}
	var sawCi2Download bool
	for _, c := range mock.Calls {
		if strings.Contains(c, "curl -fSL") && strings.Contains(c, "ghsr-runner-ci-2-2.320.0.tar.gz") {
			sawCi2Download = true
		}
		if strings.Contains(c, "curl -fSL") && strings.Contains(c, "ghsr-runner-ci-1-2.320.0.tar.gz") {
			t.Errorf("EnsureSetup should not re-download already-installed ci-1; calls=%v", mock.Calls)
		}
	}
	if !sawCi2Download {
		t.Errorf("EnsureSetup should download tarball for missing ci-2; calls=%v", mock.Calls)
	}
	if !strings.Contains(buf.String(), fmt.Sprintf("installing runner v2.320.0")) {
		t.Errorf("missing install log line; buf=%q", buf.String())
	}
}

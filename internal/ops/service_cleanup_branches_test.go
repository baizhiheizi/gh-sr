package ops

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/an-lee/gh-sr/internal/config"
	"github.com/an-lee/gh-sr/internal/host"
	"github.com/an-lee/gh-sr/internal/testutil"
)

// serviceCleanupMock builds a MockExecutor wired so ServiceCleanup's
// happy-path discovery probes succeed:
//   - ls -1 "$HOME/.gh-sr/runners"               → dirs list
//   - "for f in ...ghsr-runner-" systemd probe    → units list
//   - systemd-user/system probe                   → "user" / "system" / ""
//   - combined Linux orphan-plan probe            → D/S/U/Y markers
//     (PlanOrphanCleanup collapses the three per-instance probes into one
//     shell call on Linux; see orphanLinuxPlanProbe.)
//   - test -d DIR                                 → instanceDirectoryExists result
//   - test -f .../svc.sh                          → svc.sh presence (yes/no)
//   - systemctl/systemctl --user disable ...     → no-op for dryRun, must succeed for real cleanup
//   - rm -rf / Remove-Item                        → no-op for real cleanup
//
// Pass opts to override default behaviour per test.
func serviceCleanupMock(t *testing.T, opts cleanupMockOpts) *testutil.MockExecutor {
	t.Helper()
	return &testutil.MockExecutor{
		RunFn: func(cmd string) (string, error) {
			switch {
			case strings.Contains(cmd, `ls -1 "$HOME/.gh-sr/runners"`):
				if opts.lsErr != nil {
					return "", opts.lsErr
				}
				return strings.Join(opts.dirs, "\n") + "\n", nil
			case strings.Contains(cmd, "for f in \"$HOME/.config/systemd/user/ghsr-runner-\""):
				if opts.systemdList != "" {
					return opts.systemdList, nil
				}
				return "", nil
			case strings.Contains(cmd, "echo D") && strings.Contains(cmd, "echo S"):
				// Combined Linux orphan-plan probe (orphanLinuxPlanProbe).
				// Returns D/S/U/Y markers per the configured cleanupMockOpts.
				var out string
				if opts.dirExists {
					out += "D\n"
				}
				if opts.svcShPresent {
					out += "S\n"
				}
				switch opts.detectKind {
				case "user":
					out += "U\n"
				case "system":
					out += "Y\n"
				}
				return out, nil
			case strings.Contains(cmd, ".config/systemd/user/") && strings.Contains(cmd, "/etc/systemd/system/"):
				return opts.detectKind, nil
			case strings.Contains(cmd, "test -d"):
				if opts.dirExists {
					return "yes\n", nil
				}
				return "no\n", nil
			case strings.Contains(cmd, "test -f") && strings.Contains(cmd, "svc.sh"):
				if opts.svcShPresent {
					return "yes\n", nil
				}
				return "no\n", nil
			case strings.Contains(cmd, "systemctl --user daemon-reload"):
				// daemon-reload is the LAST line of systemdDisableUserScript and has
				// no `|| true` wrapper, so an error here propagates out of h.Run and
				// thus out of Uninstall. The earlier `disable --now 2>/dev/null || true`
				// is intentionally swallowed by the production script, so we cannot
				// inject an error there.
				if opts.disableErr != nil {
					return "", opts.disableErr
				}
				return "", nil
			case strings.Contains(cmd, "rm -rf"):
				if opts.rmErr != nil {
					return "", opts.rmErr
				}
				return "", nil
			default:
				return "", nil
			}
		},
	}
}

// cleanupMockOpts controls the responses emitted by serviceCleanupMock.
// Zero values map to "no orphans, no autostart installed" semantics.
type cleanupMockOpts struct {
	dirs         []string // runner instance dirs returned by `ls -1 ~/.gh-sr/runners`
	systemdList  string   // output of the systemd ListInstalled for-loop
	detectKind   string   // systemd-user/system probe result: "user", "system", or "" for KindNone
	dirExists    bool     // test -d response for instanceDirectoryExists
	svcShPresent bool     // test -f svc.sh response
	lsErr        error    // if set, ls -1 returns this error
	disableErr   error    // if set, systemctl disable returns this error
	rmErr        error    // if set, rm -rf returns this error
}

// TestServiceCleanup_NoHostsConfigured covers the empty-hosts branch:
// len(names) == 0 and filterHost == "" → "no hosts configured" error.
// ServiceCleanup must never call connectHostFn in this branch (the host
// mock is intentionally not registered).
func TestServiceCleanup_NoHostsConfigured(t *testing.T) {
	t.Parallel()
	installMockConnectHost(t, map[string]host.Executor{})

	cfg := &config.Config{Hosts: map[string]config.HostConfig{}}
	var buf bytes.Buffer
	err := ServiceCleanup(&buf, cfg, "", false)
	if err == nil {
		t.Fatalf("expected error for empty hosts; got nil")
	}
	if !strings.Contains(err.Error(), "no hosts configured") {
		t.Errorf("got %v; want error containing 'no hosts configured'", err)
	}
	if out := buf.String(); out != "" {
		t.Errorf("expected no output, got:\n%s", out)
	}
}

// TestServiceCleanup_UnknownHostFilter covers the filterHost="unknown" branch:
// len(names) == 0 but filterHost != "" → "unknown host" error.
func TestServiceCleanup_UnknownHostFilter(t *testing.T) {
	t.Parallel()
	installMockConnectHost(t, map[string]host.Executor{})

	cfg := &config.Config{Hosts: map[string]config.HostConfig{
		"h1": {Addr: "local", OS: "linux"},
	}}
	var buf bytes.Buffer
	err := ServiceCleanup(&buf, cfg, "nonexistent", false)
	if err == nil {
		t.Fatalf("expected error for unknown host filter; got nil")
	}
	if !strings.Contains(err.Error(), `unknown host "nonexistent"`) {
		t.Errorf("got %v; want error containing 'unknown host \"nonexistent\"'", err)
	}
}

// TestServiceCleanup_ConnectError pins the contract that a connect failure
// surfaces with the host-name prefix and the underlying error wrapped. No
// per-host banner must appear (the orchestrator fails before writing).
//
// The host entry must have OS/Arch pre-resolved so ResolveHostInfo (called
// first) is a no-op — otherwise the connect failure happens inside
// ResolveHostInfo and surfaces with "auto-detect" prefix instead of
// "connect". This test specifically exercises the per-host connect call in
// the orphan-detection loop.
func TestServiceCleanup_ConnectError(t *testing.T) {
	t.Parallel()
	sentinel := errors.New("ssh dial timeout")
	installFailingConnectHost(t, sentinel)

	cfg := &config.Config{Hosts: map[string]config.HostConfig{
		"h1": {Addr: "user@10.0.0.1", OS: "linux", Arch: "x86_64"},
	}}
	var buf bytes.Buffer
	err := ServiceCleanup(&buf, cfg, "", false)
	if err == nil {
		t.Fatalf("expected error for connect failure; got nil")
	}
	if !errors.Is(err, sentinel) {
		t.Errorf("got %v; want wrap of %v", err, sentinel)
	}
	if !strings.Contains(err.Error(), "h1:") {
		t.Errorf("got %v; want error containing host name 'h1:'", err)
	}
	if !strings.Contains(err.Error(), "connect") {
		t.Errorf("got %v; want error containing 'connect'", err)
	}
}

// TestServiceCleanup_OrphanInstancesError covers the path where the host
// directory listing fails. The orchestrator must wrap the host name + a
// "list orphans" label and call h.Close() (the host mock's Close is a
// no-op so this is just a contract assertion).
func TestServiceCleanup_OrphanInstancesError(t *testing.T) {
	t.Parallel()
	lsSentinel := errors.New("permission denied")
	exec := serviceCleanupMock(t, cleanupMockOpts{lsErr: lsSentinel})
	installMockConnectHost(t, map[string]host.Executor{"h1": exec})

	cfg := &config.Config{Hosts: map[string]config.HostConfig{
		"h1": {Addr: "local", OS: "linux"},
	}}
	var buf bytes.Buffer
	err := ServiceCleanup(&buf, cfg, "", true)
	if err == nil {
		t.Fatalf("expected error from ls failure; got nil")
	}
	if !errors.Is(err, lsSentinel) {
		t.Errorf("got %v; want wrap of %v", err, lsSentinel)
	}
	if !strings.Contains(err.Error(), "h1:") || !strings.Contains(err.Error(), "list orphans") {
		t.Errorf("got %v; want error containing 'h1:' and 'list orphans'", err)
	}
}

// TestServiceCleanup_NoOrphans pins the "all runners accounted for" footer:
// every dir + every autostart unit is in the configured set, so the
// orchestrator emits "No orphan runner services or directories found."
// after iterating all hosts. This branch is NOT triggered by the dryRun
// branch (which always prints the "Found X" line, even for X=0).
func TestServiceCleanup_NoOrphans(t *testing.T) {
	t.Parallel()
	exec := serviceCleanupMock(t, cleanupMockOpts{
		dirs:         []string{"ci-1"}, // matches configured
		systemdList:  "",               // no autostart installed
		detectKind:   "",               // Detect returns KindNone
		dirExists:    false,            // not reached (no orphans)
		svcShPresent: false,
	})
	installMockConnectHost(t, map[string]host.Executor{"h1": exec})

	cfg := &config.Config{Hosts: map[string]config.HostConfig{
		"h1": {Addr: "local", OS: "linux"},
	},
		Runners: []config.RunnerConfig{
			{Name: "ci", Host: "h1", Repo: "o/r", Count: 1},
		},
	}
	var buf bytes.Buffer
	if err := ServiceCleanup(&buf, cfg, "", false); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "No orphan runner services or directories found.") {
		t.Errorf("expected 'No orphan' footer; got:\n%s", out)
	}
	if strings.Contains(out, "Cleaned up") {
		t.Errorf("did not expect 'Cleaned up' footer for zero orphans; got:\n%s", out)
	}
}

// TestServiceCleanup_DryRunNoOrphans pins the dryRun=true + zero-orphans
// footer: the orchestrator still prints "Found 0 orphan instance(s)"
// (zero is not omitted — operators want explicit confirmation of "no
// orphans found" in dry-run too).
func TestServiceCleanup_DryRunNoOrphans(t *testing.T) {
	t.Parallel()
	exec := serviceCleanupMock(t, cleanupMockOpts{
		dirs:        []string{"ci-1"},
		systemdList: "",
		detectKind:  "",
	})
	installMockConnectHost(t, map[string]host.Executor{"h1": exec})

	cfg := &config.Config{Hosts: map[string]config.HostConfig{
		"h1": {Addr: "local", OS: "linux"},
	},
		Runners: []config.RunnerConfig{
			{Name: "ci", Host: "h1", Repo: "o/r", Count: 1},
		},
	}
	var buf bytes.Buffer
	if err := ServiceCleanup(&buf, cfg, "", true); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "Found 0 orphan instance(s)") {
		t.Errorf("expected 'Found 0' footer for dry-run with no orphans; got:\n%s", out)
	}
	if strings.Contains(out, "would remove") {
		t.Errorf("did not expect 'would remove' lines; got:\n%s", out)
	}
}

// TestServiceCleanup_RealCleanup covers the dryRun=false happy path with at
// least one orphan: each orphan's autostart is disabled and its directory is
// removed. The "Cleaned up X orphan instance(s)" footer counts only orphans
// where plan.Autostart || plan.Directory.
func TestServiceCleanup_RealCleanup(t *testing.T) {
	t.Parallel()
	exec := serviceCleanupMock(t, cleanupMockOpts{
		dirs:         []string{"stale-1", "active-1"},
		systemdList:  "ghsr-runner-stale-1\n", // basename strips .service; we simulate the post-strip output.
		detectKind:   "user",
		dirExists:    true,
		svcShPresent: false,
	})
	installMockConnectHost(t, map[string]host.Executor{"h1": exec})

	cfg := &config.Config{Hosts: map[string]config.HostConfig{
		"h1": {Addr: "local", OS: "linux"},
	},
		Runners: []config.RunnerConfig{
			{Name: "active", Host: "h1", Repo: "o/r", Count: 1},
		},
	}
	var buf bytes.Buffer
	if err := ServiceCleanup(&buf, cfg, "", false); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "Cleaned up 1 orphan instance(s)") {
		t.Errorf("expected 'Cleaned up 1' footer; got:\n%s", out)
	}
	if !strings.Contains(out, "stale-1: orphan directory removed") {
		t.Errorf("expected 'orphan directory removed' line; got:\n%s", out)
	}
	// Real cleanup must have invoked systemctl disable for the autostart.
	var sawDisable, sawRmRf bool
	for _, call := range exec.Calls {
		if strings.Contains(call, "systemctl") && strings.Contains(call, "disable") {
			sawDisable = true
		}
		if strings.Contains(call, "rm -rf") {
			sawRmRf = true
		}
	}
	if !sawDisable {
		t.Errorf("expected systemctl disable command during real cleanup; calls=%v", exec.Calls)
	}
	if !sawRmRf {
		t.Errorf("expected rm -rf during real cleanup; calls=%v", exec.Calls)
	}
}

// TestServiceCleanup_DryRunAutostartOnly pins the case where the orphan has
// plan.Autostart=true but plan.Directory=false. The orchestrator prints
// "would remove autostart" but NOT "would remove orphan directory".
func TestServiceCleanup_DryRunAutostartOnly(t *testing.T) {
	t.Parallel()
	exec := serviceCleanupMock(t, cleanupMockOpts{
		dirs:         []string{},
		systemdList:  "ghsr-runner-stale-1\n",
		detectKind:   "user", // Detect returns KindSystemdUser
		dirExists:    false,  // no instance dir
		svcShPresent: false,
	})
	installMockConnectHost(t, map[string]host.Executor{"h1": exec})

	cfg := &config.Config{Hosts: map[string]config.HostConfig{
		"h1": {Addr: "local", OS: "linux"},
	},
		Runners: []config.RunnerConfig{},
	}
	var buf bytes.Buffer
	if err := ServiceCleanup(&buf, cfg, "", true); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "stale-1: would remove autostart") {
		t.Errorf("expected 'would remove autostart' line; got:\n%s", out)
	}
	if strings.Contains(out, "would remove orphan directory") {
		t.Errorf("did not expect 'would remove orphan directory'; got:\n%s", out)
	}
	if !strings.Contains(out, "Found 1 orphan instance(s)") {
		t.Errorf("expected 'Found 1' footer; got:\n%s", out)
	}
	if !strings.Contains(out, "1 autostart, 0 directories") {
		t.Errorf("expected footer breakdown '1 autostart, 0 directories'; got:\n%s", out)
	}
}

// TestServiceCleanup_DryRunDirectoryOnly pins the case where the orphan has
// plan.Directory=true but plan.Autostart=false (the systemd unit was
// uninstalled manually but the runner dir lingers).
func TestServiceCleanup_DryRunDirectoryOnly(t *testing.T) {
	t.Parallel()
	exec := serviceCleanupMock(t, cleanupMockOpts{
		dirs:         []string{"stale-1"},
		systemdList:  "",   // no autostart unit
		detectKind:   "",   // Detect returns KindNone
		dirExists:    true, // dir exists
		svcShPresent: false,
	})
	installMockConnectHost(t, map[string]host.Executor{"h1": exec})

	cfg := &config.Config{Hosts: map[string]config.HostConfig{
		"h1": {Addr: "local", OS: "linux"},
	},
		Runners: []config.RunnerConfig{},
	}
	var buf bytes.Buffer
	if err := ServiceCleanup(&buf, cfg, "", true); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := buf.String()
	if strings.Contains(out, "would remove autostart") {
		t.Errorf("did not expect 'would remove autostart'; got:\n%s", out)
	}
	if !strings.Contains(out, "stale-1: would remove orphan directory") {
		t.Errorf("expected 'would remove orphan directory'; got:\n%s", out)
	}
	if !strings.Contains(out, "Found 1 orphan instance(s) (0 autostart, 1 directories)") {
		t.Errorf("expected footer breakdown '0 autostart, 1 directories'; got:\n%s", out)
	}
}

// TestServiceCleanup_PlanHasNothing covers the case where the orphan is in
// neither dirs nor units (e.g., the add() in OrphanInstances dedup found it
// via SafeRunnerInstanceName and it survived both lists). Actually OrphanInstances
// only adds from dirs or installed, so this branch is hard to reach directly;
// the closest contract is: an orphan in both lists with neither autostart
// installed nor dir present → continue with no increment.
//
// We exercise it by making the orphan appear in the systemd list but Detect
// returning KindNone and instanceDirectoryExists returning false.
func TestServiceCleanup_PlanHasNothing(t *testing.T) {
	t.Parallel()
	exec := serviceCleanupMock(t, cleanupMockOpts{
		dirs:         []string{},
		systemdList:  "ghsr-runner-stale-1\n",
		detectKind:   "", // Detect returns KindNone despite the unit file (the unit list is informational)
		dirExists:    false,
		svcShPresent: false,
	})
	installMockConnectHost(t, map[string]host.Executor{"h1": exec})

	cfg := &config.Config{Hosts: map[string]config.HostConfig{
		"h1": {Addr: "local", OS: "linux"},
	},
		Runners: []config.RunnerConfig{},
	}
	var buf bytes.Buffer
	if err := ServiceCleanup(&buf, cfg, "", true); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "Found 0 orphan instance(s)") {
		t.Errorf("expected 'Found 0' footer; got:\n%s", out)
	}
}

// TestServiceCleanup_MultipleHosts exercises the loop over multiple host
// names: each host gets its own "Checking orphan runners on X" banner and
// its own SSH connection. With one host configured + one orphan each, the
// banner appears exactly N times.
func TestServiceCleanup_MultipleHosts(t *testing.T) {
	t.Parallel()
	installMockConnectHost(t, map[string]host.Executor{
		"h1": serviceCleanupMock(t, cleanupMockOpts{
			dirs: []string{"stale-a"}, systemdList: "", detectKind: "user",
			dirExists: true,
		}),
		"h2": serviceCleanupMock(t, cleanupMockOpts{
			dirs: []string{"stale-b"}, systemdList: "", detectKind: "user",
			dirExists: true,
		}),
	})

	cfg := &config.Config{Hosts: map[string]config.HostConfig{
		"h1": {Addr: "local", OS: "linux"},
		"h2": {Addr: "local", OS: "linux"},
	},
		Runners: []config.RunnerConfig{},
	}
	var buf bytes.Buffer
	if err := ServiceCleanup(&buf, cfg, "", true); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := buf.String()
	for _, want := range []string{
		"Checking orphan runners on h1",
		"Checking orphan runners on h2",
		"stale-a: would remove",
		"stale-b: would remove",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q; got:\n%s", want, out)
		}
	}
	if !strings.Contains(out, "Found 2 orphan instance(s)") {
		t.Errorf("expected 'Found 2' footer; got:\n%s", out)
	}
}

// TestServiceCleanup_FilterByHost pins the filterHost integration: with two
// hosts configured, filterHost="h1" narrows the loop so only h1's orphans
// are checked. h2's banner must NOT appear.
func TestServiceCleanup_FilterByHost(t *testing.T) {
	t.Parallel()
	h1Exec := serviceCleanupMock(t, cleanupMockOpts{
		dirs: []string{"stale-h1"}, systemdList: "", detectKind: "user",
		dirExists: true,
	})
	h2Exec := serviceCleanupMock(t, cleanupMockOpts{}) // empty mock
	installMockConnectHost(t, map[string]host.Executor{
		"h1": h1Exec,
		"h2": h2Exec,
	})

	cfg := &config.Config{Hosts: map[string]config.HostConfig{
		"h1": {Addr: "local", OS: "linux"},
		"h2": {Addr: "local", OS: "linux"},
	},
		Runners: []config.RunnerConfig{},
	}
	var buf bytes.Buffer
	if err := ServiceCleanup(&buf, cfg, "h1", true); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "Checking orphan runners on h1") {
		t.Errorf("expected h1 banner; got:\n%s", out)
	}
	if strings.Contains(out, "Checking orphan runners on h2") {
		t.Errorf("did not expect h2 banner (filtered out); got:\n%s", out)
	}
	if !strings.Contains(out, "stale-h1: would remove") {
		t.Errorf("expected h1's orphan to appear; got:\n%s", out)
	}
	if len(h2Exec.Calls) > 0 {
		t.Errorf("h2's mock should not have been called; calls=%v", h2Exec.Calls)
	}
}

// TestServiceCleanup_AutostartUninstallFailureDoesNotAbort pins the contract
// that a failure to remove an autostart unit during real cleanup is treated
// as a soft warning, not a hard error. This matches the policy of
// removeNativeServices: orphaned autostart units may be in a wedged state
// (unit file present but unit not loaded), and continuing to remove the
// runner directory is still the right behaviour. The cleanup continues and
// reports the directory removal as successful.
//
// Verifies:
//   - ServiceCleanup returns nil (no error propagation from uninstall failure).
//   - Output includes the warning line containing the sentinel message.
//   - The orphan directory is still removed (cleanup is not aborted).
func TestServiceCleanup_AutostartUninstallFailureDoesNotAbort(t *testing.T) {
	t.Parallel()
	disableSentinel := errors.New("unit not loaded")
	exec := serviceCleanupMock(t, cleanupMockOpts{
		dirs:         []string{"stale-1"},
		systemdList:  "ghsr-runner-stale-1\n",
		detectKind:   "user",
		dirExists:    true,
		svcShPresent: false,
		disableErr:   disableSentinel,
	})
	installMockConnectHost(t, map[string]host.Executor{"h1": exec})

	cfg := &config.Config{Hosts: map[string]config.HostConfig{
		"h1": {Addr: "local", OS: "linux"},
	},
		Runners: []config.RunnerConfig{},
	}
	var buf bytes.Buffer
	if err := ServiceCleanup(&buf, cfg, "", false); err != nil {
		t.Fatalf("expected nil error (autostart uninstall failure is a warning); got %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "warning: failed to remove autostart") {
		t.Errorf("expected autostart warning line; got:\n%s", out)
	}
	if !strings.Contains(out, "unit not loaded") {
		t.Errorf("expected sentinel message in warning; got:\n%s", out)
	}
	// The orphan directory must still be removed — uninstall failure does
	// not abort cleanup.
	if !strings.Contains(out, "stale-1: orphan directory removed") {
		t.Errorf("expected directory to be removed despite autostart failure; got:\n%s", out)
	}
	// The cleanup counter still counts the orphan (plan.Directory=true), but
	// autostart count is 0 since the unit was not actually removed.
	if !strings.Contains(out, "Cleaned up 1 orphan instance(s)") {
		t.Errorf("expected 'Cleaned up 1' footer; got:\n%s", out)
	}
}

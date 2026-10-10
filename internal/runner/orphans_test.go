package runner

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/an-lee/gh-sr/internal/autostart"
	"github.com/an-lee/gh-sr/internal/config"
	"github.com/an-lee/gh-sr/internal/host"
	"github.com/an-lee/gh-sr/internal/testutil"
)

func TestOrphanInstances(t *testing.T) {
	t.Parallel()
	mock := &testutil.MockExecutor{
		RunFn: func(cmd string) (string, error) {
			switch {
			case strings.Contains(cmd, `ls -1 "$HOME/.gh-sr/runners"`):
				return "active-1\nold-1\n", nil
			case strings.Contains(cmd, "for f in \"$HOME/.config/systemd/user/ghsr-runner-\""):
				return "ghsr-runner-old-2\n", nil
			default:
				return "", nil
			}
		},
	}
	h := host.NewHost("linux", config.HostConfig{OS: "linux"})
	h.SetConn(mock)

	configured := map[string]struct{}{"active-1": {}}
	orphans, err := OrphanInstances(h, configured)
	if err != nil {
		t.Fatal(err)
	}
	if len(orphans) != 2 || orphans[0] != "old-1" || orphans[1] != "old-2" {
		t.Fatalf("got %v", orphans)
	}
}

func TestConfiguredInstanceSet(t *testing.T) {
	t.Parallel()
	runners := []config.RunnerConfig{
		{Name: "ci", Host: "h1", Repo: "o/r", Count: 2},
		{Name: "web", Host: "h2", Repo: "o/r", Count: 1},
	}
	set := ConfiguredInstanceSet(runners, "h1")
	if len(set) != 2 {
		t.Fatalf("got %d entries", len(set))
	}
	if _, ok := set["ci-1"]; !ok {
		t.Fatal("missing ci-1")
	}
	if _, ok := set["ci-2"]; !ok {
		t.Fatal("missing ci-2")
	}
}

func TestCleanupOrphanInstanceDryRun(t *testing.T) {
	t.Parallel()
	mock := &testutil.MockExecutor{
		RunFn: func(cmd string) (string, error) {
			switch {
			case strings.Contains(cmd, "echo D") && strings.Contains(cmd, "echo S"):
				// Combined Linux orphan-plan probe: dir present, no svc.sh, user systemd unit.
				return "D\nU\n", nil
			default:
				return "", nil
			}
		},
	}
	h := host.NewHost("linux", config.HostConfig{OS: "linux"})
	h.SetConn(mock)
	m := NewManager("")
	plan, err := m.CleanupOrphanInstance(h, "old-1", true)
	if err != nil {
		t.Fatal(err)
	}
	if !plan.Autostart || !plan.Directory {
		t.Fatalf("plan = %+v", plan)
	}
}

// TestInstanceDirectoryExists_Windows pins the Windows branch of
// instanceDirectoryExists. The runner directory is rooted at $env:USERPROFILE,
// so the probe must embed h.RunnerDirPS(instance) as a PowerShell expression
// instead of single-quoting h.RunnerDir(instance) as a literal path. The Linux
// branch is exercised end-to-end via TestServiceCleanup_OrphanWithDirectoryInOps
// (test -d probe).
//
// Addr is set to "local" so config.IsLocalAddr reports true and
// host.Host.wrapCommand is a no-op — the mock sees the literal PowerShell
// script that instanceDirectoryExists emitted.
func TestInstanceDirectoryExists_Windows(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name     string
		instance string
		reply    string
		want     bool
	}{
		{"dir present", "r1", "yes\r\n", true},
		{"dir absent", "gone", "no\r\n", false},
		{"dir absent whitespace", "gone", "   \r\n", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			h := host.NewHost("windows", config.HostConfig{OS: "windows", Addr: "local"})
			mock := &testutil.MockExecutor{
				RunFn: func(cmd string) (string, error) {
					if !strings.Contains(cmd, "Test-Path") || !strings.Contains(cmd, "-PathType Container") {
						t.Errorf("expected Test-Path -PathType Container probe, got: %q", cmd)
					}
					if strings.Contains(cmd, "'$env:USERPROFILE") {
						t.Errorf("probe must not single-quote $env:USERPROFILE, got: %q", cmd)
					}
					wantExpr := h.RunnerDirPS(tc.instance)
					if !strings.Contains(cmd, "-LiteralPath ("+wantExpr+")") {
						t.Errorf("probe should use RunnerDirPS expression %q, got: %q", wantExpr, cmd)
					}
					return tc.reply, nil
				},
			}
			h.SetConn(mock)
			got, err := instanceDirectoryExists(h, tc.instance)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Errorf("got %v, want %v", got, tc.want)
			}
		})
	}
}

// TestOrphanLinuxPlanProbe pins the combined-probe parsing for the Linux
// orphan-plan path: D (dir), S (svc.sh), U (user systemd unit), Y (system
// systemd unit) markers are mapped to the matching flags/kind. Asserts a
// single SSH round-trip per call (the whole point of the consolidation).
// TestInstanceDirectoryExists_Linux pins the Linux branch of
// instanceDirectoryExists (the Windows sibling is covered by
// TestInstanceDirectoryExists_Windows). The runner directory is rooted at
// $HOME, so the probe deliberately embeds the unquoted "$HOME/..." prefix
// in the `test -d` argument — RemoteDirExists would have single-quoted it
// and frozen $HOME. RemoteBoolCheck wraps the probe as
// `test -d <dir> && echo yes || echo no`; the mock returns "yes" / "no"
// accordingly.
//
// Addr is "local" so wrapCommand is a no-op (no base64 PowerShell wrapper);
// the mock sees the literal POSIX command and string-matches against it.
func TestInstanceDirectoryExists_Linux(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name     string
		instance string
		reply    string
		want     bool
	}{
		{"dir present", "ci-1", "yes\n", true},
		{"dir absent", "gone-1", "no\n", false},
		{"dir absent whitespace", "gone-2", "  \n", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			h := host.NewHost("linux", config.HostConfig{OS: "linux", Addr: "local"})
			mock := &testutil.MockExecutor{
				RunFn: func(cmd string) (string, error) {
					wantDir := h.RunnerDir(tc.instance)
					if !strings.Contains(cmd, "test -d "+wantDir) {
						t.Errorf("probe should test -d %q (with unquoted $HOME), got: %q", wantDir, cmd)
					}
					if !strings.Contains(cmd, "&& echo yes || echo no") {
						t.Errorf("probe should be wrapped in `&& echo yes || echo no`: %q", cmd)
					}
					return tc.reply, nil
				},
			}
			h.SetConn(mock)
			got, err := instanceDirectoryExists(h, tc.instance)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Errorf("got %v, want %v", got, tc.want)
			}
		})
	}
}

// TestPlanOrphanCleanup_RejectsInvalidInstance verifies the SafeRunnerInstanceName
// guard fires before any SSH call. The manager should never invoke host.Executor
// for an obviously unsafe name (empty, traversal, or shell-meta).
func TestPlanOrphanCleanup_RejectsInvalidInstance(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name     string
		instance string
	}{
		{"empty", ""},
		{"dot", "."},
		{"dotdot", ".."},
		{"contains slash", "ci/1"},
		{"contains backslash", `ci\1`},
		{"contains semicolon", "ci;1"},
		{"contains backtick", "ci`1"},
		{"contains dollar", "ci$1"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			called := false
			mock := &testutil.MockExecutor{
				RunFn: func(string) (string, error) {
					called = true
					return "", nil
				},
			}
			h := host.NewHost("linux", config.HostConfig{OS: "linux", Addr: "local"})
			h.SetConn(mock)
			m := NewManager("")
			_, err := m.PlanOrphanCleanup(h, tc.instance)
			if err == nil {
				t.Fatalf("expected error for %q, got nil", tc.instance)
			}
			if called {
				t.Errorf("SafeRunnerInstanceName must reject before any host call for %q", tc.instance)
			}
		})
	}
}

// TestPlanOrphanCleanup_Windows pins the non-Linux fallback path for Windows:
// PlanOrphanCleanup calls autostart.Detect (ScheduledTaskExists wrapped in
// `if (...) { 'yes' } else { 'no' }`) and then instanceDirectoryExists
// (Test-Path -LiteralPath wrapper). Both probes reach the mock via Host.Run,
// which is wrapCommand-aware — Addr="local" keeps the PowerShell scripts
// literal so the mock's string-matches are stable.
//
// The plan must combine the two facts: Autostart = "task installed",
// Directory = "runner dir exists". Each case asserts the exact Boolean pair
// the manager should produce from the mock's two replies.
func TestPlanOrphanCleanup_Windows(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name      string
		taskReply string
		dirReply  string
		wantAuto  bool
		wantDir   bool
	}{
		{"task + dir both present", "yes\n", "yes\n", true, true},
		{"task absent + dir present", "no\n", "yes\n", false, true},
		{"task present + dir absent", "yes\n", "no\n", true, false},
		{"both absent", "no\n", "no\n", false, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			mock := &testutil.MockExecutor{
				RunFn: func(cmd string) (string, error) {
					switch {
					case strings.Contains(cmd, "Get-ScheduledTask"):
						return tc.taskReply, nil
					case strings.Contains(cmd, "Test-Path"):
						return tc.dirReply, nil
					default:
						return "", fmt.Errorf("unexpected probe: %q", cmd)
					}
				},
			}
			h := host.NewHost("win", config.HostConfig{OS: "windows", Addr: "local"})
			h.SetConn(mock)
			m := NewManager("")
			plan, err := m.PlanOrphanCleanup(h, "ci-win-1")
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if plan.Autostart != tc.wantAuto {
				t.Errorf("Autostart = %v, want %v", plan.Autostart, tc.wantAuto)
			}
			if plan.Directory != tc.wantDir {
				t.Errorf("Directory = %v, want %v", plan.Directory, tc.wantDir)
			}
			if plan.Instance != "ci-win-1" {
				t.Errorf("Instance = %q, want %q", plan.Instance, "ci-win-1")
			}
		})
	}
}

// TestPlanOrphanCleanup_Darwin pins the non-Linux fallback path for Darwin.
// autostart.Detect does a `test -f "$HOME/Library/LaunchAgents/<label>.plist"`
// probe and instanceDirectoryExists uses hostshell.RemoteBoolCheck for the
// `test -d` probe (Linux path, by design — Darwin runners are stored under
// $HOME/.gh-sr/runners/<instance> just like Linux ones).
func TestPlanOrphanCleanup_Darwin(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name     string
		plistRep string
		dirRep   string
		wantAuto bool
		wantDir  bool
	}{
		{"plist + dir present", "yes\n", "yes\n", true, true},
		{"plist absent + dir present", "", "yes\n", false, true},
		{"plist present + dir absent", "yes\n", "no\n", true, false},
		{"both absent", "", "no\n", false, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			mock := &testutil.MockExecutor{
				RunFn: func(cmd string) (string, error) {
					switch {
					case strings.Contains(cmd, "Library/LaunchAgents"):
						return tc.plistRep, nil
					case strings.HasPrefix(cmd, "test -d "):
						return tc.dirRep, nil
					default:
						return "", fmt.Errorf("unexpected probe: %q", cmd)
					}
				},
			}
			h := host.NewHost("mac", config.HostConfig{OS: "darwin", Addr: "local"})
			h.SetConn(mock)
			m := NewManager("")
			plan, err := m.PlanOrphanCleanup(h, "ci-mac-1")
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if plan.Autostart != tc.wantAuto {
				t.Errorf("Autostart = %v, want %v", plan.Autostart, tc.wantAuto)
			}
			if plan.Directory != tc.wantDir {
				t.Errorf("Directory = %v, want %v", plan.Directory, tc.wantDir)
			}
		})
	}
}

// TestPlanOrphanCleanup_NonLinux_PropagatesErrors verifies the non-Linux
// fallback surfaces errors from autostart.Detect and instanceDirectoryExists
// instead of swallowing them — both probes run on the user-supplied SSH
// transport and the manager must not silently render an empty plan when the
// host probe fails.
func TestPlanOrphanCleanup_NonLinux_PropagatesErrors(t *testing.T) {
	t.Parallel()
	t.Run("Detect error", func(t *testing.T) {
		t.Parallel()
		sentinel := errors.New("scheduled-task probe failed")
		mock := &testutil.MockExecutor{
			RunFn: func(cmd string) (string, error) {
				if strings.Contains(cmd, "Get-ScheduledTask") {
					return "", sentinel
				}
				return "", fmt.Errorf("unexpected probe: %q", cmd)
			},
		}
		h := host.NewHost("win", config.HostConfig{OS: "windows", Addr: "local"})
		h.SetConn(mock)
		m := NewManager("")
		_, err := m.PlanOrphanCleanup(h, "ci-win-1")
		if !errors.Is(err, sentinel) {
			t.Fatalf("err = %v, want %v", err, sentinel)
		}
	})
	t.Run("instanceDirectoryExists error", func(t *testing.T) {
		t.Parallel()
		sentinel := errors.New("Test-Path failed")
		mock := &testutil.MockExecutor{
			RunFn: func(cmd string) (string, error) {
				switch {
				case strings.Contains(cmd, "Get-ScheduledTask"):
					return "no\n", nil
				case strings.Contains(cmd, "Test-Path"):
					return "", sentinel
				default:
					return "", fmt.Errorf("unexpected probe: %q", cmd)
				}
			},
		}
		h := host.NewHost("win", config.HostConfig{OS: "windows", Addr: "local"})
		h.SetConn(mock)
		m := NewManager("")
		_, err := m.PlanOrphanCleanup(h, "ci-win-1")
		if !errors.Is(err, sentinel) {
			t.Fatalf("err = %v, want %v", err, sentinel)
		}
	})
}

// TestCleanupOrphanInstance_NothingToDo pins the early-return path when the
// plan reports no artifacts to remove — neither autostart nor directory is
// set, so the manager must skip the would-remove prints AND skip the actual
// remove calls. The mock would echo "unexpected" if any SSH call landed.
func TestCleanupOrphanInstance_NothingToDo(t *testing.T) {
	t.Parallel()
	mock := &testutil.MockExecutor{
		RunFn: func(string) (string, error) {
			// Orphan plan probe reports neither svc.sh/D marker nor U/Y, and
			// the directory test -d probe (only used on Linux non-folded path)
			// is also absent — the combined Linux probe alone is called and
			// returns "no autostart, no dir".
			return "", nil
		},
	}
	h := host.NewHost("linux", config.HostConfig{OS: "linux", Addr: "local"})
	h.SetConn(mock)
	m := NewManager("")
	plan, err := m.CleanupOrphanInstance(h, "ci-clean-1", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if plan.Autostart || plan.Directory {
		t.Fatalf("plan = %+v (want both false)", plan)
	}
}

// TestCleanupOrphanInstance_RemovesAutostartAndDirectory pins the
// non-dryRun success path that drives `gh sr cleanup` on a live host.
// The Linux-fast-path plan reports both Autostart and Directory set; the
// manager must invoke removeNativeServices (which itself issues one SSH
// round-trip on Linux) and removeDirTree (one SSH round-trip on Linux),
// plus the success print "  <inst>: orphan directory removed". Anything
// the manager does is observable through the mock's call counter.
func TestCleanupOrphanInstance_RemovesAutostartAndDirectory(t *testing.T) {
	t.Parallel()
	var calls []string
	mock := &testutil.MockExecutor{
		RunFn: func(cmd string) (string, error) {
			calls = append(calls, cmd)
			switch {
			case strings.Contains(cmd, "echo D") && strings.Contains(cmd, "echo S"):
				// Combined Linux orphan-plan probe: dir present, svc.sh present, user systemd unit.
				return "D\nS\nU\n", nil
			case strings.Contains(cmd, "rm -rf") || strings.Contains(cmd, "disk prune"):
				// removeDirTreePOSIX is a multi-line shell script — the
				// actual command does not start with "rm -rf" but does
				// contain it (the success branch). Match anywhere.
				return "", nil
			default:
				return "", nil
			}
		},
	}
	h := host.NewHost("linux", config.HostConfig{OS: "linux", Addr: "local"})
	h.SetConn(mock)
	m := NewManager("")
	plan, err := m.CleanupOrphanInstance(h, "ci-clean-1", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !plan.Autostart || !plan.Directory {
		t.Fatalf("plan = %+v (want both true)", plan)
	}
	// Sanity: the manager issued at least one call containing the
	// removeDirTreePOSIX marker — without it, neither removeNativeServices
	// nor removeDirTree ran.
	var sawDirRemove bool
	for _, c := range calls {
		if strings.Contains(c, "rm -rf") {
			sawDirRemove = true
			break
		}
	}
	if !sawDirRemove {
		t.Errorf("expected a `rm -rf` call for the orphan directory; got calls=%v", calls)
	}
}

// TestCleanupOrphanInstance_PropagatesRemoveDirTreeError pins the error
// wrap on removeDirTree failure. The plan says "directory is here"; the
// mock returns an SSH error for the removeDirTreePOSIX script; the
// manager must wrap the error so callers can tell whether the autostart
// removal succeeded and the directory removal did not.
func TestCleanupOrphanInstance_PropagatesRemoveDirTreeError(t *testing.T) {
	t.Parallel()
	sentinel := errors.New("rm failed")
	mock := &testutil.MockExecutor{
		RunFn: func(cmd string) (string, error) {
			switch {
			case strings.Contains(cmd, "echo D") && strings.Contains(cmd, "echo S"):
				// dir present, svc.sh present, user systemd unit.
				return "D\nS\nU\n", nil
			case strings.Contains(cmd, "rm -rf") || strings.Contains(cmd, "disk prune"):
				// removeDirTreePOSIX dispatch — fail the whole script.
				return "", sentinel
			default:
				return "", nil
			}
		},
	}
	h := host.NewHost("linux", config.HostConfig{OS: "linux", Addr: "local"})
	h.SetConn(mock)
	m := NewManager("")
	_, err := m.CleanupOrphanInstance(h, "ci-clean-1", false)
	if err == nil {
		t.Fatalf("expected error from removeDirTree, got nil")
	}
	if !strings.Contains(err.Error(), "removing orphan directory") {
		t.Errorf("err = %v; want wrap containing %q", err, "removing orphan directory")
	}
	if !errors.Is(err, sentinel) {
		t.Errorf("err should wrap sentinel via %%w; got %v", err)
	}
}

// TestCleanupOrphanInstance_RejectsInvalidInstance verifies the
// SafeRunnerInstanceName guard fires at the entry point and short-circuits
// before the PlanOrphanCleanup SSH call lands.
func TestCleanupOrphanInstance_RejectsInvalidInstance(t *testing.T) {
	t.Parallel()
	called := false
	mock := &testutil.MockExecutor{
		RunFn: func(string) (string, error) {
			called = true
			return "", nil
		},
	}
	h := host.NewHost("linux", config.HostConfig{OS: "linux", Addr: "local"})
	h.SetConn(mock)
	m := NewManager("")
	_, err := m.CleanupOrphanInstance(h, "ci;1", false)
	if err == nil {
		t.Fatalf("expected error for invalid instance, got nil")
	}
	if called {
		t.Errorf("SafeRunnerInstanceName must reject before any host call")
	}
}

// TestOrphanInstances_PropagatesListErrors pins the two early-error
// returns: ListRunnerInstanceDirs failure (host directory listing error)
// and autostart.ListInstalled failure. Neither should fall through to
// the dedup loop.
func TestOrphanInstances_PropagatesListErrors(t *testing.T) {
	t.Parallel()
	t.Run("ListRunnerInstanceDirs error", func(t *testing.T) {
		t.Parallel()
		sentinel := errors.New("ls failed")
		mock := &testutil.MockExecutor{
			RunFn: func(cmd string) (string, error) {
				if strings.Contains(cmd, `ls -1 "$HOME/.gh-sr/runners"`) {
					return "", sentinel
				}
				return "", nil
			},
		}
		h := host.NewHost("linux", config.HostConfig{OS: "linux", Addr: "local"})
		h.SetConn(mock)
		_, err := OrphanInstances(h, map[string]struct{}{})
		if !errors.Is(err, sentinel) {
			t.Fatalf("err = %v, want %v", err, sentinel)
		}
	})
	t.Run("ListInstalled error", func(t *testing.T) {
		t.Parallel()
		sentinel := errors.New("systemctl failed")
		mock := &testutil.MockExecutor{
			RunFn: func(cmd string) (string, error) {
				switch {
				case strings.Contains(cmd, `ls -1 "$HOME/.gh-sr/runners"`):
					return "ci-clean-1\n", nil
				case strings.Contains(cmd, `for f in "$HOME/.config/systemd/user/ghsr-runner-"`):
					return "", sentinel
				default:
					return "", nil
				}
			},
		}
		h := host.NewHost("linux", config.HostConfig{OS: "linux", Addr: "local"})
		h.SetConn(mock)
		_, err := OrphanInstances(h, map[string]struct{}{})
		if !errors.Is(err, sentinel) {
			t.Fatalf("err = %v, want %v", err, sentinel)
		}
	})
}

// TestOrphanInstances_SkipsInvalidDirEntries pins the dedup loop's
// rejection of instance names that pass through the listing but fail
// SafeRunnerInstanceName (e.g. a previous run left a directory with a
// shell-meta character). The orphan slice must not contain them, and
// the manager must never reach for them in a cleanup pass.
func TestOrphanInstances_SkipsInvalidDirEntries(t *testing.T) {
	t.Parallel()
	mock := &testutil.MockExecutor{
		RunFn: func(cmd string) (string, error) {
			switch {
			case strings.Contains(cmd, `ls -1 "$HOME/.gh-sr/runners"`):
				// "..;rm -rf /" is included to assert SafeRunnerInstanceName
				// filters it; legitimate names are kept.
				return "ci-clean-1\n..;rm -rf /\nci-good\n", nil
			case strings.Contains(cmd, `for f in "$HOME/.config/systemd/user/ghsr-runner-"`):
				return "", nil
			default:
				return "", nil
			}
		},
	}
	h := host.NewHost("linux", config.HostConfig{OS: "linux", Addr: "local"})
	h.SetConn(mock)
	got, err := OrphanInstances(h, map[string]struct{}{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []string{"ci-clean-1", "ci-good"}
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Errorf("got %v, want %v (the `..;rm -rf /` entry must be filtered)", got, want)
	}
}

// TestConfiguredInstanceSet_EmptyWhenNoHostMatch pins the empty-set case
// for runners whose Host field does not equal the queried hostName. Used
// by cleanup to tell which instances are "still wanted" — a missing
// host match must yield an empty set, not panic.
func TestConfiguredInstanceSet_EmptyWhenNoHostMatch(t *testing.T) {
	t.Parallel()
	runners := []config.RunnerConfig{
		{Name: "ci", Host: "host-a", Repo: "o/r", Count: 2},
	}
	set := ConfiguredInstanceSet(runners, "host-b")
	if len(set) != 0 {
		t.Fatalf("expected empty set, got %v", set)
	}
}

func TestOrphanLinuxPlanProbe(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name     string
		out      string
		wantKind autostart.Kind
		wantSvc  bool
		wantDir  bool
	}{
		{"nothing", "", autostart.KindNone, false, false},
		{"dir only", "D\n", autostart.KindNone, false, true},
		{"svc only", "S\n", autostart.KindNone, true, false},
		{"dir+svc", "S\nD\n", autostart.KindNone, true, true},
		{"user only", "U\n", autostart.KindSystemdUser, false, false},
		{"system only", "Y\n", autostart.KindSystemdSystem, false, false},
		{"all three", "D\nS\nU\n", autostart.KindSystemdUser, true, true},
		{"crlf", "D\r\nS\r\nY\r\n", autostart.KindSystemdSystem, true, true},
		{"all three system", "D\nS\nY\n", autostart.KindSystemdSystem, true, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var calls int
			mock := &testutil.MockExecutor{
				RunFn: func(cmd string) (string, error) {
					if strings.Contains(cmd, "echo D") && strings.Contains(cmd, "echo S") {
						calls++
						if !strings.Contains(cmd, "[ -d $HOME/") {
							t.Errorf("directory probe must allow remote $HOME expansion: %q", cmd)
						}
						return tc.out, nil
					}
					return "", nil
				},
			}
			h := host.NewHost("linux", config.HostConfig{OS: "linux"})
			h.SetConn(mock)
			kind, svc, dir, err := orphanLinuxPlanProbe(h, "old-1")
			if err != nil {
				t.Fatal(err)
			}
			if calls != 1 {
				t.Errorf("got %d SSH calls; want 1", calls)
			}
			if kind != tc.wantKind {
				t.Errorf("kind = %q; want %q", kind, tc.wantKind)
			}
			if svc != tc.wantSvc {
				t.Errorf("svc = %v; want %v", svc, tc.wantSvc)
			}
			if dir != tc.wantDir {
				t.Errorf("dir = %v; want %v", dir, tc.wantDir)
			}
		})
	}
}

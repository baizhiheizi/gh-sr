package tui

import (
	"strings"
	"testing"

	"github.com/an-lee/gh-sr/internal/config"
	"github.com/an-lee/gh-sr/internal/runner"
)

func Test_formatGitHubStatus(t *testing.T) {
	t.Parallel()
	if got := formatGitHubStatus(runner.RunnerStatus{}); got != "-" {
		t.Errorf("empty remote: got %q", got)
	}
	if got := formatGitHubStatus(runner.RunnerStatus{Remote: "online", Busy: false}); got != "online" {
		t.Errorf("online: got %q", got)
	}
	if got := formatGitHubStatus(runner.RunnerStatus{Remote: "online", Busy: true}); got != "busy" {
		t.Errorf("busy: got %q", got)
	}
}

// TestColorizeImageBuild_branches covers every branch of colorizeImageBuild:
// the "ok" / "stale" prefix paths, the literal "?" sentinel, and the default
// fallback that returns the cell unchanged.
func TestColorizeImageBuild_branches(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		in   string
		// contains: substring that must appear in the styled output. lipgloss
		// wraps the cell with ANSI codes; we only assert the visible payload.
		contains string
		// plain: when true the function must return the cell unchanged
		// (the default branch — neither "ok", "stale", nor "?").
		plain bool
	}{
		{name: "ok prefix", in: "ok 2m ago", contains: "ok 2m ago"},
		{name: "stale prefix", in: "stale (5d)", contains: "stale (5d)"},
		{name: "question sentinel", in: "?", contains: "?"},
		{name: "unknown token falls through", in: "weird-build", plain: true},
		{name: "empty string falls through", in: "", plain: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := colorizeImageBuild(tc.in)
			if tc.plain {
				if got != tc.in {
					t.Errorf("colorizeImageBuild(%q) should return cell unchanged, got %q", tc.in, got)
				}
				return
			}
			if !strings.Contains(got, tc.contains) {
				t.Errorf("colorizeImageBuild(%q) should contain %q, got %q", tc.in, tc.contains, got)
			}
		})
	}
}

// TestColorizeLocalStatus_branches covers every documented status string plus
// the unknown default (which renders through statusUnknown to keep the cell
// non-empty — never blank — in the dashboard view).
func TestColorizeLocalStatus_branches(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name     string
		in       string
		contains string
	}{
		{"running", "running", "running"},
		{"stopped", "stopped", "stopped"},
		{"failed", "failed", "failed"},
		{"restarting", "restarting", "restarting"},
		{"service error", "service error", "service error"},
		{"not installed", "not installed", "not installed"},
		{"unreachable", "unreachable", "unreachable"},
		{"unknown falls through", "garbage", "garbage"},
		{"empty falls through", "", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := colorizeLocalStatus(tc.in)
			if !strings.Contains(got, tc.contains) {
				t.Errorf("colorizeLocalStatus(%q) should contain %q, got %q", tc.in, tc.contains, got)
			}
		})
	}
}

// TestColorizeGitHubStatus_branches covers the three known GitHub runner
// statuses (online / offline / busy) plus the default unknown branch.
func TestColorizeGitHubStatus_branches(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name     string
		in       string
		contains string
	}{
		{"online", "online", "online"},
		{"offline", "offline", "offline"},
		{"busy", "busy", "busy"},
		{"unknown falls through", "weird", "weird"},
		{"empty falls through", "", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := colorizeGitHubStatus(tc.in)
			if !strings.Contains(got, tc.contains) {
				t.Errorf("colorizeGitHubStatus(%q) should contain %q, got %q", tc.in, tc.contains, got)
			}
		})
	}
}

// TestRunnerStatusColorize_dispatchesByColumn verifies that runnerStatusColorize
// only styles the BUILD/LOCAL/GITHUB columns (5/6/7) and passes every other
// column through unchanged. The column layout matches runnerStatusHeaders, so
// any column reorder that breaks the dispatch will surface here.
func TestRunnerStatusColorize_dispatchesByColumn(t *testing.T) {
	t.Parallel()
	// Columns outside the BUILD/LOCAL/GITHUB range must pass through unchanged.
	for col := 0; col < 9; col++ {
		if col == 5 || col == 6 || col == 7 {
			continue
		}
		got := runnerStatusColorize(col, "payload")
		if got != "payload" {
			t.Errorf("col %d should pass through unchanged, got %q", col, got)
		}
	}
	// BUILD column (5) routes through colorizeImageBuild.
	if got := runnerStatusColorize(5, "ok 1m"); !strings.Contains(got, "ok 1m") {
		t.Errorf("col=5 (BUILD) should style via colorizeImageBuild, got %q", got)
	}
	// LOCAL column (6) routes through colorizeLocalStatus.
	if got := runnerStatusColorize(6, "running"); !strings.Contains(got, "running") {
		t.Errorf("col=6 (LOCAL) should style via colorizeLocalStatus, got %q", got)
	}
	// GITHUB column (7) routes through colorizeGitHubStatus.
	if got := runnerStatusColorize(7, "online"); !strings.Contains(got, "online") {
		t.Errorf("col=7 (GITHUB) should style via colorizeGitHubStatus, got %q", got)
	}
}

// TestRunnerStatusCells_defaultFillers pins the "-" fallback for the two
// optional image/build fields so an empty container image renders as "-" in
// the dashboard (not a blank cell). The remaining columns pass through
// unchanged and the column count matches runnerStatusHeaders.
func TestRunnerStatusCells_defaultFillers(t *testing.T) {
	t.Parallel()
	s := runner.RunnerStatus{
		Instance: "i1", Host: "h1", Repo: "o/r", Mode: "container",
		ContainerImage: "", ContainerImageBuild: "",
		Local: "running", Remote: "online", Labels: "self-hosted",
	}
	cells := runnerStatusCells(s)
	if len(cells) != len(runnerStatusHeaders) {
		t.Fatalf("cells length %d != headers length %d", len(cells), len(runnerStatusHeaders))
	}
	want := []string{"i1", "h1", "o/r", "container", "-", "-", "running", "online", "self-hosted"}
	for i, w := range want {
		if cells[i] != w {
			t.Errorf("cells[%d] = %q, want %q", i, cells[i], w)
		}
	}
	// When the image/build fields are populated, they pass through (no "-").
	s.ContainerImage = "ghcr.io/foo/bar"
	s.ContainerImageBuild = "ok 5m ago"
	cells = runnerStatusCells(s)
	if cells[4] != "ghcr.io/foo/bar" {
		t.Errorf("image should pass through, got %q", cells[4])
	}
	if cells[5] != "ok 5m ago" {
		t.Errorf("build should pass through, got %q", cells[5])
	}
	// GitHub status uses formatGitHubStatus — pin the populated path here.
	if cells[7] != "online" {
		t.Errorf("Remote=\"online\" should pass through, got %q", cells[7])
	}
	// Separately verify the empty-Remote -> "-" substitution that hides blank
	// GITHUB cells in the dashboard (otherwise the column shifts visually).
	emptyRemote := runnerStatusCells(runner.RunnerStatus{
		Instance: "i1", Host: "h1", Repo: "o/r", Mode: "container",
	})
	if emptyRemote[7] != "-" {
		t.Errorf("empty Remote should render as \"-\", got %q", emptyRemote[7])
	}
}

func TestFormatConfig_containsHostsAndRunners(t *testing.T) {
	t.Parallel()
	cfg := &config.Config{
		GitHub: config.GitHubConfig{},
		Hosts: map[string]config.HostConfig{
			"h1": {Addr: "local", OS: "linux", Arch: "amd64"},
		},
		Runners: []config.RunnerConfig{
			{Name: "r1", Repo: "o/r", Host: "h1", Count: 1, Labels: []string{"self-hosted"}},
		},
	}
	out := FormatConfig(cfg)
	if !strings.Contains(out, "h1") || !strings.Contains(out, "r1") || !strings.Contains(out, "target=o/r") {
		t.Fatalf("unexpected FormatConfig output:\n%s", out)
	}
	if strings.Contains(out, "github_pat_") {
		t.Fatal("FormatConfig should not echo raw tokens")
	}
	if !strings.Contains(out, "Token:") || (!strings.Contains(out, "(from gh CLI)") && !strings.Contains(out, "(none)")) {
		t.Fatalf("expected Token line with gh or none, got:\n%s", out)
	}
}

func TestFormatConfigShowsEffectiveRunnerMode(t *testing.T) {
	t.Parallel()
	cfg := &config.Config{
		Hosts: map[string]config.HostConfig{
			"h1": {Addr: "local", OS: "linux", Arch: "amd64"},
		},
		Runners: []config.RunnerConfig{
			{Name: "native", Repo: "o/native", Host: "h1"},
			{Name: "container", Repo: "o/container", Host: "h1", RunnerMode: config.RunnerModeContainer},
		},
	}

	out := FormatConfig(cfg)
	if !strings.Contains(out, "native  target=o/native  host=h1  count=0  mode=native") {
		t.Fatalf("expected native runner to show native mode, got:\n%s", out)
	}
	if !strings.Contains(out, "container  target=o/container  host=h1  count=0  mode=container") {
		t.Fatalf("expected container runner to show container mode, got:\n%s", out)
	}
}

func TestFormatConfig_orgRunnerDisplayTarget(t *testing.T) {
	t.Parallel()
	cfg := &config.Config{
		Hosts: map[string]config.HostConfig{
			"h1": {Addr: "local", OS: "linux", Arch: "amd64"},
		},
		Runners: []config.RunnerConfig{
			{Name: "org-ci", Org: "my-org", Group: "ci-pool", Host: "h1", Count: 2},
		},
	}
	out := FormatConfig(cfg)
	if !strings.Contains(out, "target=org:my-org group=ci-pool") {
		t.Fatalf("expected org display target with group, got:\n%s", out)
	}
}

func TestClampCursor(t *testing.T) {
	t.Parallel()
	if got := clampCursor(3, 2); got != 1 {
		t.Fatalf("clamp 3 with n=2: got %d", got)
	}
	if got := clampCursor(0, 0); got != 0 {
		t.Fatalf("empty: got %d", got)
	}
}

func TestNonTTYHint_nonempty(t *testing.T) {
	t.Parallel()
	if NonTTYHint == "" {
		t.Fatal("NonTTYHint empty")
	}
}

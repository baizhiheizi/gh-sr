package runner

import (
	"strings"
	"testing"

	"github.com/an-lee/gh-sr/internal/config"
	"github.com/an-lee/gh-sr/internal/host"
)

// TestFormatContainerImageBuild pins the BUILD-cell formatter used by the TUI
// Status table. The contract is:
//
//   - local == "not installed" → "-" (the cell has no image to report)
//   - local != "not installed", actual == "" → "?" (runner present but no
//     image-revision label visible, e.g. older image without the marker)
//   - actual == expected → "ok (<short>)" where <short> is the first 8 chars
//     of the revision (or the whole string if it is shorter than or equal to 8
//     chars).
//   - actual != expected → "stale (<short>)" using the actual (running) rev.
//
// The "short" truncation matches the TUI column width — long hex SHAs would
// overflow the cell. The branch ordering (local first, then empty-actual, then
// equality) matters: re-ordering the early-returns would change the output for
// the "not installed" + non-empty actual combination, which Status reports
// after a `Stop`.
func TestFormatContainerImageBuild(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name              string
		local, expected   string
		actual            string
		want              string
		wantShortRevision bool // asserts the output contains an 8-char prefix of expected/actual
	}{
		{
			name:     "not installed short-circuits before equality check",
			local:    "not installed",
			expected: "deadbeefcafebabe1234567890abcdef",
			actual:   "deadbeefcafebabe1234567890abcdef",
			want:     "-",
		},
		{
			name:     "not installed wins over empty actual",
			local:    "not installed",
			expected: "deadbeef",
			actual:   "",
			want:     "-",
		},
		{
			name:     "empty actual returns question mark",
			local:    "running",
			expected: "deadbeef",
			actual:   "",
			want:     "?",
		},
		{
			name:     "matching revisions show full revision when short",
			local:    "running",
			expected: "abc1234",
			actual:   "abc1234",
			want:     "ok (abc1234)",
		},
		{
			name:              "matching revisions truncated to 8 chars when long",
			local:             "running",
			expected:          "deadbeefcafebabe1234567890abcdef",
			actual:            "deadbeefcafebabe1234567890abcdef",
			want:              "ok (deadbeef)",
			wantShortRevision: true,
		},
		{
			name:              "stale uses actual revision when long",
			local:             "running",
			expected:          "deadbeefcafebabe0000000000000000",
			actual:            "1234567890abcdef1234567890abcdef",
			want:              "stale (12345678)",
			wantShortRevision: true,
		},
		{
			name:     "stale with short actual revision uses full string",
			local:    "running",
			expected: "v1",
			actual:   "v0",
			want:     "stale (v0)",
		},
		{
			name:     "stopped local still formats the build match",
			local:    "stopped",
			expected: "abcdef",
			actual:   "abcdef",
			want:     "ok (abcdef)",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := formatContainerImageBuild(tc.local, tc.expected, tc.actual)
			if got != tc.want {
				t.Errorf("formatContainerImageBuild(%q, %q, %q) = %q, want %q",
					tc.local, tc.expected, tc.actual, got, tc.want)
			}
			if tc.wantShortRevision {
				rev := tc.actual
				if tc.want == "ok (deadbeef)" {
					rev = tc.expected
				}
				if !strings.Contains(got, rev[:8]) {
					t.Errorf("output %q should contain first 8 chars %q of revision", got, rev[:8])
				}
			}
		})
	}
}

// TestContainerImageExtraApt_nilReceiver pins the nil-safety branch. Every
// other entry point in the package goes through (*Manager).containerImageExtraApt,
// so a nil receiver can only happen if a caller holds the helper directly via
// a typed nil interface; the contract is "return nil, do not panic".
func TestContainerImageExtraApt_nilReceiver(t *testing.T) {
	t.Parallel()
	var m *Manager
	if got := m.containerImageExtraApt(); got != nil {
		t.Errorf("containerImageExtraApt on nil receiver = %v, want nil", got)
	}
}

// TestContainerImageExtraApt_returnsConfiguredSlice pins the non-nil path:
// when Manager has a configured ContainerImageExtraApt, the helper returns
// it verbatim. The slice is shared (not copied) — callers must not mutate it.
func TestContainerImageExtraApt_returnsConfiguredSlice(t *testing.T) {
	t.Parallel()
	want := []string{"git", "jq", "curl"}
	m := &Manager{ContainerImageExtraApt: want}
	got := m.containerImageExtraApt()
	if got == nil || len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("got[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

// TestWindowsNativeConfigScript_runnerGroupFlag pins the optional
// `--runnergroup <Group>` flag added by windowsNativeConfigScript when
// rc.Group is non-empty. The flag is appended after `--replace` and before
// any subsequent flag, so re-ordering the build steps would scramble the
// flag position and break the contract on Windows.
func TestWindowsNativeConfigScript_runnerGroupFlag(t *testing.T) {
	t.Parallel()
	h := host.NewHost("win", config.HostConfig{Addr: "u@h", OS: "windows", Arch: "amd64"})
	rc := config.RunnerConfig{
		Repo:   "an-lee/gh-sr",
		Labels: []string{"windows", "native"},
		Group:  "ops-runners",
	}

	script := windowsNativeConfigScript(h, rc, "unwx-1", "token-value", 0)

	if !strings.Contains(script, "--runnergroup 'ops-runners'") {
		t.Errorf("expected --runnergroup 'ops-runners' in script, got: %q", script)
	}
	// --runnergroup must come after --replace (config.cmd ordering
	// requirement) and before any ephemeral / further flags.
	replaceIdx := strings.Index(script, "--replace")
	groupIdx := strings.Index(script, "--runnergroup")
	if replaceIdx < 0 || groupIdx < 0 || replaceIdx >= groupIdx {
		t.Errorf("--replace must precede --runnergroup: replaceIdx=%d groupIdx=%d script=%q",
			replaceIdx, groupIdx, script)
	}
}

// TestWindowsNativeConfigScript_ephemeralFlag pins the optional `--ephemeral`
// flag added by windowsNativeConfigScript when rc.Ephemeral is true. Ephemeral
// runners self-deregister after a single job, which is the contract for the
// CI `self-hosted` runner pool.
func TestWindowsNativeConfigScript_ephemeralFlag(t *testing.T) {
	t.Parallel()
	h := host.NewHost("win", config.HostConfig{Addr: "u@h", OS: "windows", Arch: "amd64"})
	rc := config.RunnerConfig{
		Repo:      "an-lee/gh-sr",
		Labels:    []string{"windows", "native"},
		Ephemeral: true,
	}

	script := windowsNativeConfigScript(h, rc, "unwx-1", "token-value", 0)

	if !strings.Contains(script, "--ephemeral") {
		t.Errorf("expected --ephemeral in script, got: %q", script)
	}
	// Sanity: the Group is empty, so --runnergroup must not appear.
	if strings.Contains(script, "--runnergroup") {
		t.Errorf("did not expect --runnergroup when Group is empty: %q", script)
	}
}

// TestWindowsNativeConfigScript_groupAndEphemeralCombined pins both optional
// flags together. config.cmd accepts --ephemeral after --runnergroup, so both
// must appear when both opts are set.
func TestWindowsNativeConfigScript_groupAndEphemeralCombined(t *testing.T) {
	t.Parallel()
	h := host.NewHost("win", config.HostConfig{Addr: "u@h", OS: "windows", Arch: "amd64"})
	rc := config.RunnerConfig{
		Repo:      "an-lee/gh-sr",
		Labels:    []string{"windows", "native"},
		Group:     "ci-runners",
		Ephemeral: true,
	}

	script := windowsNativeConfigScript(h, rc, "unwx-1", "token-value", 0)

	if !strings.Contains(script, "--runnergroup 'ci-runners'") {
		t.Errorf("expected --runnergroup, got: %q", script)
	}
	if !strings.Contains(script, "--ephemeral") {
		t.Errorf("expected --ephemeral, got: %q", script)
	}
}

// TestWindowsNativeConfigScript_noOptionalFlags pins the default path: when
// Group is empty and Ephemeral is false, neither flag appears. This guards
// against both being appended unconditionally (which would break
// non-ephemeral, no-group Windows runners).
func TestWindowsNativeConfigScript_noOptionalFlags(t *testing.T) {
	t.Parallel()
	h := host.NewHost("win", config.HostConfig{Addr: "u@h", OS: "windows", Arch: "amd64"})
	rc := config.RunnerConfig{
		Repo:   "an-lee/gh-sr",
		Labels: []string{"windows", "native"},
	}

	script := windowsNativeConfigScript(h, rc, "unwx-1", "token-value", 0)

	if strings.Contains(script, "--runnergroup") {
		t.Errorf("did not expect --runnergroup when Group is empty: %q", script)
	}
	if strings.Contains(script, "--ephemeral") {
		t.Errorf("did not expect --ephemeral when Ephemeral is false: %q", script)
	}
}

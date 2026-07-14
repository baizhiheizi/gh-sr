package tui

import (
	"strings"
	"testing"
)

// TestFooterMain_idleAndLoading pins the two pre-built footer strings the
// dashboard renders. The fix replaced a per-call fmt.Sprintf (reflection
// overhead + buffer alloc per View()) with two static consts; this test
// guards against accidental edits to either literal.
func TestFooterMain_idleAndLoading(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		loading bool
		want    string
	}{
		{
			name:    "idle footer contains the key hints and trailing-newline marker",
			loading: false,
			want:    "\n  j/k: move  enter: runner actions  g: global menu  h: host metrics  f: filters  r: refresh  ?: help  q: quit",
		},
		{
			name:    "loading footer appends the refreshing indicator",
			loading: true,
			want:    "\n  j/k: move  enter: runner actions  g: global menu  h: host metrics  f: filters  r: refresh  ?: help  q: quit  (refreshing…)",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			m := &dashboardModel{loading: tc.loading}
			got := m.footerMain()
			// helpStyle.Render wraps the literal in ANSI escapes when a TTY
			// is detected; for a unit test we only assert the inner literal
			// is present, since lipgloss strips/normalises the prefix.
			if !strings.Contains(got, tc.want) {
				t.Errorf("footerMain(loading=%v) missing literal %q; got: %q", tc.loading, tc.want, got)
			}
		})
	}
}

// TestFooterMain_constantsMatch guards the relationship between the two
// pre-built strings: the loading variant must equal the idle variant plus
// the indicator suffix. If a maintainer adds a new keybinding to one and
// not the other, this test fails before any visual drift ships.
func TestFooterMain_constantsMatch(t *testing.T) {
	t.Parallel()
	if !strings.HasPrefix(footerMainLoading, footerMainIdle) {
		t.Errorf("footerMainLoading (%q) must start with footerMainIdle (%q)", footerMainLoading, footerMainIdle)
	}
	const loadingSuffix = "  (refreshing…)"
	if !strings.HasSuffix(footerMainLoading, loadingSuffix) {
		t.Errorf("footerMainLoading (%q) must end with %q", footerMainLoading, loadingSuffix)
	}
}
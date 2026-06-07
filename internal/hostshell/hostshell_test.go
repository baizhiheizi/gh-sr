package hostshell

import (
	"strings"
	"testing"
)

func TestPosixSingleQuote(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name  string
		input string
		want  string
	}{
		// Empty string produces empty single-quoted string.
		{"empty", "", "''"},
		// Simple strings pass through unchanged inside quotes.
		{"simple", "hello", "'hello'"},
		{"no_spaces", "no spaces", "'no spaces'"},
		{"path", "path/to/file", "'path/to/file'"},
		// Single quotes are escaped via the POSIX idiom ' → '\''.
		{"apos_one", "it's", "'it'\\''s'"},
		{"apos_multi", "a'b'c", "'a'\\''b'\\''c'"},
		// Two consecutive single quotes.
		{"two_apos", "''", "''\\'''\\'''"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := PosixSingleQuote(tc.input)
			if got != tc.want {
				t.Errorf("PosixSingleQuote(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

func TestPowerShellSingleQuote(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name  string
		input string
		want  string
	}{
		{"simple", "hello", "'hello'"},
		{"single_apos", "didn't", "'didn''t'"},
		{"double_quote_inside", `it's "quoted"`, `'it''s "quoted"'`},
		{"apos_between", "a'b", "'a''b'"},
		{"empty", "", "''"},
		{"no_quotes", "no quotes", "'no quotes'"},
		{"trailing_apos", "trailing'", "'trailing'''"},
		{"leading_apos", "'leading'", "'''leading'''"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := PowerShellSingleQuote(tc.input)
			if got != tc.want {
				t.Errorf("PowerShellSingleQuote(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

func TestLinuxElevatePrelude(t *testing.T) {
	t.Parallel()
	// Both callers produce shells that:
	//   * initialise $SUDO to ''
	//   * detect root via id -u
	//   * try passwordless sudo (sudo -n true)
	//   * print the supplied failureMsg and exit 1 on failure
	// The literal failureMsg is inlined into the script via PosixSingleQuote
	// so we assert on its presence and the surrounding shell skeleton.
	cases := []struct {
		name       string
		failureMsg string
		wantSubs   []string
	}{
		{
			name:       "runner_message",
			failureMsg: "gh sr: remote Linux commands need root SSH or passwordless sudo (non-interactive); SSH has no TTY for sudo passwords. Use NOPASSWD, connect as root, or install software manually. Run: gh sr doctor",
			wantSubs: []string{
				`SUDO=''`,
				`if [ "$(id -u)" -ne 0 ]`,
				`command -v sudo >/dev/null 2>&1`,
				`sudo -n true 2>/dev/null`,
				`SUDO='sudo -n'`,
				`gh sr: remote Linux commands`,
				`exit 1`,
			},
		},
		{
			name:       "autostart_message",
			failureMsg: "gh sr: system-level autostart needs root SSH or passwordless sudo (non-interactive)",
			wantSubs: []string{
				`SUDO=''`,
				`if [ "$(id -u)" -ne 0 ]`,
				`command -v sudo >/dev/null 2>&1`,
				`sudo -n true 2>/dev/null`,
				`SUDO='sudo -n'`,
				`gh sr: system-level autostart`,
				`exit 1`,
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := LinuxElevatePrelude(tc.failureMsg)
			for _, sub := range tc.wantSubs {
				if !strings.Contains(got, sub) {
					t.Errorf("LinuxElevatePrelude missing substring %q in output:\n%s", sub, got)
				}
			}
		})
	}
}

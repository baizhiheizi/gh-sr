package autostart

import "testing"

func Test_posixSingleQuote(t *testing.T) {
	t.Parallel()
	cases := []struct {
		input string
		want  string
	}{
		// Empty string produces empty single-quoted string
		{"", "''"},
		// Simple strings pass through unchanged inside quotes
		{"hello", "'hello'"},
		{"no spaces", "'no spaces'"},
		{"path/to/file", "'path/to/file'"},
		// Single quotes are escaped via the POSIX idiom ' → '\''
		{"it's", "'it'\\''s'"},
		{"a'b'c", "'a'\\''b'\\''c'"},
		// Two consecutive single quotes
		{"''", "''\\'''\\'''"},
	}
	for _, tc := range cases {
		t.Run(tc.input, func(t *testing.T) {
			t.Parallel()
			got := posixSingleQuote(tc.input)
			if got != tc.want {
				t.Errorf("posixSingleQuote(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

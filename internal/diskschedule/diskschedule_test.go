package diskschedule

import (
	"testing"

	"github.com/an-lee/gh-sr/internal/hostshell"
)

// TestParseAtTime pins the HH:MM parser used by Install to validate AtTime
// before any platform-specific work runs. The contract is:
//   - trim leading/trailing whitespace
//   - require exactly one colon
//   - hour ∈ [0,23], minute ∈ [0,59]
//   - both parts must be base-10 integers
func TestParseAtTime(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		in      string
		wantH   int
		wantM   int
		wantErr bool
	}{
		{"typical morning", "03:00", 3, 0, false},
		{"midnight", "00:00", 0, 0, false},
		{"late evening", "23:59", 23, 59, false},
		{"single-digit hour", "9:30", 9, 30, false},
		{"single-digit minute", "12:5", 12, 5, false},
		{"no leading zero on hour", "9:00", 9, 0, false},
		{"surrounding whitespace", "  03:00  ", 3, 0, false},
		{"tab whitespace", "\t12:34\t", 12, 34, false},
		{"no colon", "bad", 0, 0, true},
		{"trailing colon only", "12:", 0, 0, true},
		{"leading colon only", ":34", 0, 0, true},
		{"three parts", "12:34:56", 0, 0, true},
		{"non-numeric hour", "ab:00", 0, 0, true},
		{"non-numeric minute", "12:xy", 0, 0, true},
		{"negative hour", "-1:00", 0, 0, true},
		{"negative minute", "12:-1", 0, 0, true},
		{"hour too large", "24:00", 0, 0, true},
		{"hour far too large", "99:00", 0, 0, true},
		{"minute too large", "12:60", 0, 0, true},
		{"minute far too large", "12:99", 0, 0, true},
		{"empty string", "", 0, 0, true},
		{"whitespace only", "   ", 0, 0, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			h, m, err := parseAtTime(tc.in)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("parseAtTime(%q) = (%d, %d, nil); want error", tc.in, h, m)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseAtTime(%q): unexpected error %v", tc.in, err)
			}
			if h != tc.wantH || m != tc.wantM {
				t.Fatalf("parseAtTime(%q) = (%d, %d); want (%d, %d)", tc.in, h, m, tc.wantH, tc.wantM)
			}
		})
	}
}

// TestSystemdQuoteArg pins the systemd ExecStart argument-quoting contract
// used by installSystemdUser when embedding GhPath / ConfigPath into the
// generated .service unit. The contract is:
//   - safe chars [A-Za-z0-9_/.-] pass through unquoted
//   - anything containing space, tab, double-quote, single-quote, or
//     backslash is wrapped in double quotes
//   - inside the wrapper, backslash and double-quote are themselves
//     backslash-escaped; other chars (including single-quote) pass through
//     verbatim
func TestSystemdQuoteArg(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"plain path passes through", "/usr/bin/gh", "/usr/bin/gh"},
		{"path with safe punctuation", "/usr/local/bin/gh-1.2.3", "/usr/local/bin/gh-1.2.3"},
		{"single space wrapped", "/home/me/my config.yml", "\"/home/me/my config.yml\""},
		{"tab wrapped", "/path/with\ttab", "\"/path/with\ttab\""},
		{"double quote escaped", `/path/with"quote`, `"/path/with\"quote"`},
		{"backslash escaped", `/path/with\backslash`, `"/path/with\\backslash"`},
		{"single quote is not special", `/path/with'squote`, `"/path/with'squote"`},
		{"multiple special chars", `a b"c\d`, `"a b\"c\\d"`},
		{"empty string passes through", "", ""},
		{"only space wrapped", " ", "\" \""},
		{"only double quote wrapped and escaped", `"`, `"\""`},
		{"only backslash wrapped and escaped", `\`, `"\\"`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := systemdQuoteArg(tc.in); got != tc.want {
				t.Errorf("systemdQuoteArg(%q) = %q; want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestPlistEscape(t *testing.T) {
	t.Parallel()
	got := hostshell.PlistEscape(`a&b"c<d>e`)
	want := `a&amp;b&quot;c&lt;d&gt;e`
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestDefaultAtTime(t *testing.T) {
	t.Parallel()
	if DefaultAtTime != "03:00" {
		t.Fatalf("got %q", DefaultAtTime)
	}
}

// TestEscapePS pins the PowerShell single-quote escape contract used by
// installWindowsTask (diskschedule.go:314) to embed GhPath / ConfigPath into
// the `powershell -Command` string. The escape rule is `'` → `”` — doubling
// the apostrophe — which is how PowerShell escapes a single quote inside an
// already-single-quoted literal. A future change to use backslash-escape
// (or to skip escaping) would break the Windows task install path.
func TestPowerShellSingleQuote_forDiskSchedule(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"empty string", "", "''"},
		{"no apostrophes", `C:\Users\me\bin\gh.exe`, `'C:\Users\me\bin\gh.exe'`},
		{"single apostrophe doubles", `O'Brien`, `'O''Brien'`},
		{"consecutive apostrophes all double", `it's a 'test'`, `'it''s a ''test'''`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := hostshell.PowerShellSingleQuote(tc.in); got != tc.want {
				t.Errorf("PowerShellSingleQuote(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

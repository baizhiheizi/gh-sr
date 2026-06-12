package diskschedule

import (
	"testing"

	"github.com/an-lee/gh-sr/internal/hostshell"
)

func TestParseAtTime(t *testing.T) {
	t.Parallel()
	h, m, err := parseAtTime("03:00")
	if err != nil {
		t.Fatal(err)
	}
	if h != 3 || m != 0 {
		t.Fatalf("got %d:%d", h, m)
	}
	_, _, err = parseAtTime("bad")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestSystemdQuoteArg(t *testing.T) {
	t.Parallel()
	if got := systemdQuoteArg("/usr/bin/gh"); got != "/usr/bin/gh" {
		t.Fatalf("got %q", got)
	}
	got := systemdQuoteArg(`/home/me/my config.yml`)
	if got != `"/home/me/my config.yml"` {
		t.Fatalf("got %q", got)
	}
	got = systemdQuoteArg(`/path/with"quote`)
	if got != `"/path/with\"quote"` {
		t.Fatalf("got %q", got)
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

package diskschedule

import "testing"

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

func TestXMLEscapePlist(t *testing.T) {
	t.Parallel()
	got := xmlEscapePlist(`a&b"c<d>e`)
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

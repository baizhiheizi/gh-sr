package probe

import (
	"fmt"
	"strings"
	"testing"
)

func TestWriterEcho(t *testing.T) {
	t.Parallel()
	cases := []struct {
		tag, index, tail string
		want             string
	}{
		{"@@m:", "1", "", "echo \"@@m:1\"\n"},
		{"@@st:", "1", "$out", "echo \"@@st:1|$out\"\n"},
		{"@@begin:", "$n", "", "echo \"@@begin:$n\"\n"},
		{"@@hyg:", "$n", "0", "echo \"@@hyg:$n|0\"\n"},
	}
	for _, tc := range cases {
		var b strings.Builder
		NewWriter(&b).Echo(tc.tag, tc.index, tc.tail)
		if got := b.String(); got != tc.want {
			t.Errorf("Echo(%q,%q,%q) = %q, want %q", tc.tag, tc.index, tc.tail, got, tc.want)
		}
	}
}

func TestWriterEchoInline(t *testing.T) {
	t.Parallel()
	var b strings.Builder
	NewWriter(&b).EchoInline("@@health:", "2", "ok")
	if got, want := b.String(), "echo \"@@health:2|ok\""; got != want {
		t.Errorf("EchoInline = %q, want %q", got, want)
	}
}

func TestWriterPrintf(t *testing.T) {
	t.Parallel()
	cases := []struct {
		tag, format, arg string
		want             string
	}{
		{"S:", "%s", "$st", "printf 'S:%s\\n' \"$st\"\n"},
		{"H:", "%s", "$HOME", "printf 'H:%s\\n' \"$HOME\"\n"},
		{"K:", "1", "", "printf 'K:1\\n'\n"},
	}
	for _, tc := range cases {
		var b strings.Builder
		NewWriter(&b).Printf(tc.tag, tc.format, tc.arg)
		if got := b.String(); got != tc.want {
			t.Errorf("Printf(%q,%q,%q) = %q, want %q", tc.tag, tc.format, tc.arg, got, tc.want)
		}
	}
}

func TestWriterPrintfInline(t *testing.T) {
	t.Parallel()
	var b strings.Builder
	NewWriter(&b).PrintfInline("D:", "%s", "$(du -sb /x)")
	if got, want := b.String(), "printf 'D:%s\\n' \"$(du -sb /x)\""; got != want {
		t.Errorf("PrintfInline = %q, want %q", got, want)
	}
}

func TestDecodeIndexedMarkers(t *testing.T) {
	t.Parallel()
	cases := []struct {
		raw         string
		tag         string
		index       int
		body, tail  string
		hasTail, ok bool
	}{
		{"@@m:1", "@@m:", 1, "1", "", false, true},
		{"@@st:3|running|image:1", "@@st:", 3, "3|running|image:1", "running|image:1", true, true},
		{"@@health:2|ok", "@@health:", 2, "2|ok", "ok", true, true},
		{"@@state:1", "@@state:", 1, "1", "", false, true},
		{"@@begin:1", "@@begin:", 1, "1", "", false, true},
		{"@@begin:1|junk", "@@begin:", 1, "1|junk", "junk", true, true},
		{"@@begin:0", "@@begin:", 0, "0", "", false, true},
		{"@@begin:x", "@@begin:", 0, "x", "", false, true},
		{"@@st: 4 ", "@@st:", 4, " 4 ", "", false, true},
		{"@@hyg:2|1", "@@hyg:", 2, "2|1", "1", true, true},
		{"@@st:5|", "@@st:", 5, "5|", "", true, true},
	}
	for _, tc := range cases {
		t.Run(fmt.Sprintf("%q", tc.raw), func(t *testing.T) {
			t.Parallel()
			d, ok := Decode(tc.raw)
			if ok != tc.ok || d.Tag != tc.tag || d.Index != tc.index ||
				d.Body != tc.body || d.Tail != tc.tail || d.HasTail != tc.hasTail || d.Raw != tc.raw {
				t.Fatalf("Decode(%q) = (%+v, %v), want tag=%q index=%d body=%q tail=%q hasTail=%v ok=%v",
					tc.raw, d, ok, tc.tag, tc.index, tc.body, tc.tail, tc.hasTail, tc.ok)
			}
		})
	}
}

func TestDecodeFlatAndPayload(t *testing.T) {
	t.Parallel()
	// Flat tags with a pipe in the payload are NOT index markers; the adapter
	// reads the payload from Body.
	d, ok := Decode("S:running|image:1")
	if !ok || d.Tag != "S:" || d.Index != 0 || d.Body != "running|image:1" || d.HasTail {
		t.Fatalf("flat tag decoded wrong: %+v ok=%v", d, ok)
	}
	d, ok = Decode("G:2: docker0    inet 172.17.0.1/16")
	if !ok || d.Tag != "G:" || d.Index != 0 || d.Body != "2: docker0    inet 172.17.0.1/16" {
		t.Fatalf("gateway line decoded wrong: %+v ok=%v", d, ok)
	}
	// CR trimming.
	d, ok = Decode("K:1\r")
	if !ok || d.Raw != "K:1" || d.Body != "1" {
		t.Fatalf("CR trim failed: %+v ok=%v", d, ok)
	}
	// Plain payload line.
	d, ok = Decode("awmg-mcpg-stale")
	if ok || d.Raw != "awmg-mcpg-stale" || d.OK {
		t.Fatalf("payload line should not be a marker: %+v ok=%v", d, ok)
	}
}

func TestLines(t *testing.T) {
	t.Parallel()
	out := "@@begin:1\r\n@@state:1|running\r\npayload line\r\n@@end:1\r\n"
	var got []string
	for d := range Lines(out) {
		got = append(got, d.Raw+"/"+d.Tag)
	}
	want := []string{"@@begin:1/@@begin:", "@@state:1|running/@@state:", "payload line/", "@@end:1/@@end:", "/"}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("Lines = %v, want %v", got, want)
	}
}

func TestAtoiOrNeg(t *testing.T) {
	t.Parallel()
	cases := []struct {
		in   string
		want int
	}{
		{"1", 1}, {"42", 42}, {"  7  ", 7}, {"0", 0}, {"-3", -3},
		{"", -1}, {"   ", -1}, {"5x", -1}, {"x5", -1}, {"5|x", -1},
		{"9999999999999999999999", -1},
	}
	for _, tc := range cases {
		if got := AtoiOrNeg(tc.in); got != tc.want {
			t.Errorf("AtoiOrNeg(%q) = %d, want %d", tc.in, got, tc.want)
		}
	}
}

package strfmt

import (
	"fmt"
	"testing"
)

func TestAtoiOrNeg(t *testing.T) {
	t.Parallel()
	cases := []struct {
		in   string
		want int
	}{
		{"1", 1},
		{"42", 42},
		{"  7  ", 7},
		{"0", 0},
		{"-3", -3},
		{"", -1},
		{"   ", -1},
		{"5x", -1},
		{"x5", -1},
		{"5|x", -1},
		{"9999999999999999999999", -1},
	}
	for _, tc := range cases {
		if got := AtoiOrNeg(tc.in); got != tc.want {
			t.Errorf("AtoiOrNeg(%q) = %d, want %d", tc.in, got, tc.want)
		}
	}
}

func TestSplitMarker(t *testing.T) {
	t.Parallel()
	cases := []struct {
		line, prefix string
		wantN        int
		wantTail     string
		wantOK       bool
	}{
		// Well-formed markers, with and without a tail.
		{"@@st:3|running|image:1", "@@st:", 3, "running|image:1", true},
		{"@@health:2|ok", "@@health:", 2, "ok", true},
		{"@@du:9|1024", "@@du:", 9, "1024", true},
		{"@@state:1", "@@state:", 1, "", true},
		{"@@m: 4 ", "@@m:", 4, "", true},

		// Missing prefix / empty number.
		{"other:5", "@@st:", -1, "", false},
		{"@@st:", "@@st:", -1, "", false},
		{"", "@@st:", -1, "", false},

		// Invalid numbers (including the trimmed-whitespace variant).
		{"@@st:x", "@@st:", -1, "", false},
		{"@@st:  ", "@@st:", -1, "", false},
		{"@@st:-1|ok", "@@st:", -1, "", false},
		{"@@st:1.5|ok", "@@st:", -1, "", false},

		// Empty tail after the separator is fine; only the first '|'
		// separates n from the tail.
		{"@@st:5|", "@@st:", 5, "", true},
		{"@@st:5||extra", "@@st:", 5, "|extra", true},
	}
	for _, tc := range cases {
		t.Run(fmt.Sprintf("%s/%q", tc.prefix, tc.line), func(t *testing.T) {
			t.Parallel()
			n, tail, ok := SplitMarker(tc.line, tc.prefix)
			if !tc.wantOK {
				if ok {
					t.Fatalf("SplitMarker(%q, %q) = ok, want !ok", tc.line, tc.prefix)
				}
				return
			}
			if !ok || n != tc.wantN || tail != tc.wantTail {
				t.Fatalf("SplitMarker(%q, %q) = (%d, %q, %v), want (%d, %q, true)",
					tc.line, tc.prefix, n, tail, ok, tc.wantN, tc.wantTail)
			}
		})
	}
}

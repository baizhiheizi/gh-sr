package strfmt

import (
	"strconv"
	"testing"
)

func TestFmtFloat_matchesStrconv(t *testing.T) {
	t.Parallel()
	cases := []struct {
		v    float64
		prec int
	}{
		{0, 0},
		{0, 1},
		{1.5, 1},
		{99.99, 2},
		{-3.14159, 4},
		{1e10, 1},
		{1.234567890123456e15, 6},
	}
	for _, tc := range cases {
		tc := tc
		t.Run("", func(t *testing.T) {
			t.Parallel()
			var dst [32]byte
			got := FmtFloat(dst[:0], tc.v, tc.prec)
			want := strconv.AppendFloat(dst[:0], tc.v, 'f', tc.prec, 64)
			if string(got) != string(want) {
				t.Fatalf("FmtFloat(%v, %d) = %q, want %q", tc.v, tc.prec, got, want)
			}
		})
	}
}

func TestFmtFloat_zeroAllocs(t *testing.T) {
	// AllocsPerRun cannot be combined with t.Parallel() (panics with
	// "AllocsPerRun called during parallel test"), so this test is
	// deliberately sequential.
	allocs := testing.AllocsPerRun(1000, func() {
		var dst [32]byte
		_ = FmtFloat(dst[:0], 12.345, 2)
	})
	if allocs != 0 {
		t.Fatalf("FmtFloat allocated %v allocs/op (want 0)", allocs)
	}
}

package table

import (
	"reflect"
	"testing"
)

func TestFitColumns_fitsWithoutChange(t *testing.T) {
	t.Parallel()
	natural := []int{5, 10, 3}
	mins := []int{4, 4, 3}
	got := FitColumns(natural, mins, 100, 2)
	if !reflect.DeepEqual(got, natural) {
		t.Errorf("FitColumns with generous maxTotal = %v, want natural %v", got, natural)
	}
}

func TestFitColumns_exactFitUnchanged(t *testing.T) {
	t.Parallel()
	natural := []int{5, 10, 3}
	mins := []int{4, 4, 3}
	// total = 5+10+3 + 3*2 = 24
	got := FitColumns(natural, mins, 24, 2)
	if !reflect.DeepEqual(got, natural) {
		t.Errorf("FitColumns at exact fit = %v, want natural %v", got, natural)
	}
}

// TestFitColumns_widestAbsorbsDeficit pins the widest-first policy: the
// widest column takes the whole deficit (down to its floor) before any
// other column shrinks. This keeps narrow status-like columns intact
// while long text columns truncate.
func TestFitColumns_widestAbsorbsDeficit(t *testing.T) {
	t.Parallel()
	natural := []int{5, 30, 7}
	mins := []int{5, 4, 7}
	// total = 42+6 = 48; maxTotal = 30 → deficit 18 → col 1 shrinks by 18.
	got := FitColumns(natural, mins, 30, 2)
	want := []int{5, 12, 7}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("FitColumns = %v, want %v (widest column absorbs the deficit)", got, want)
	}
}

// TestFitColumns_floorStopsShrink verifies a column never shrinks below
// its floor and the reduction continues on the next-widest column.
func TestFitColumns_floorStopsShrink(t *testing.T) {
	t.Parallel()
	natural := []int{5, 10, 7}
	mins := []int{5, 6, 7}
	// total = 22+6 = 28; maxTotal = 20 → deficit 8. Col 1 (10) hits its
	// floor 6 (cut 4); remaining deficit 4 cannot come from the pinned
	// columns → result overflows by design with col 1 at its floor.
	got := FitColumns(natural, mins, 20, 2)
	want := []int{5, 6, 7}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("FitColumns = %v, want %v (floor respected, pinned columns intact)", got, want)
	}
}

// TestFitColumns_pinnedColumnsNeverShrink documents that mins[i] =
// natural[i] pins a column: the shrink load lands entirely on flex
// columns even when multiple columns are wider than the average.
func TestFitColumns_pinnedColumnsNeverShrink(t *testing.T) {
	t.Parallel()
	natural := []int{9, 12, 6, 20}
	mins := []int{9, 12, 6, 4} // only the last column is flex
	// total = 47+8 = 55; maxTotal = 43 → deficit 12 from col 3.
	got := FitColumns(natural, mins, 43, 2)
	want := []int{9, 12, 6, 8}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("FitColumns = %v, want %v", got, want)
	}
}

// TestFitColumns_floorAboveNaturalClamped verifies a nonsensical floor
// above the natural width is clamped instead of growing the column.
func TestFitColumns_floorAboveNaturalClamped(t *testing.T) {
	t.Parallel()
	natural := []int{5, 3}
	mins := []int{10, 20}
	got := FitColumns(natural, mins, 5, 2)
	want := []int{5, 3}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("FitColumns = %v, want %v (floors clamped to natural)", got, want)
	}
}

// TestFitColumns_mismatchedInputReturnedAsIs documents the defensive
// branch for mismatched natural/mins lengths.
func TestFitColumns_mismatchedInputReturnedAsIs(t *testing.T) {
	t.Parallel()
	natural := []int{5, 10}
	got := FitColumns(natural, []int{1}, 3, 2)
	if !reflect.DeepEqual(got, natural) {
		t.Errorf("FitColumns with mismatched mins = %v, want natural %v", got, natural)
	}
}

func TestFitColumns_empty(t *testing.T) {
	t.Parallel()
	got := FitColumns(nil, nil, 10, 2)
	if len(got) != 0 {
		t.Errorf("FitColumns on empty input = %v, want empty", got)
	}
}

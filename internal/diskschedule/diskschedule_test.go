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

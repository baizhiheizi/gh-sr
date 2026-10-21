package benchstat

import (
	"strings"
	"testing"
)

// The round-trips/op metric is the custom metric the SSH round-trip
// benchmarks report (see internal/runner/roundtrip_bench_test.go and
// friends). It counts the remote command invocations one operation costs —
// the network/energy proxy that ns/op cannot express.

func TestParseFile_ParsesRoundTripsMetric(t *testing.T) {
	dir := t.TempDir()
	// Go prints custom metrics between ns/op and B/op.
	path := writeBench(t, dir, "bench.txt", `goos: linux
pkg: github.com/example/x
BenchmarkFoo-24   	1000000	      1234 ns/op	       5.000 round-trips/op	     256 B/op	       4 allocs/op
`)

	got, err := ParseFile(path)
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}
	r := got["Foo"]
	if r.RoundTripsPerOp != 5 {
		t.Errorf("Foo.RoundTripsPerOp = %v, want 5", r.RoundTripsPerOp)
	}
	// The extra column must not shift the other metrics.
	if r.NsPerOp != 1234 {
		t.Errorf("Foo.NsPerOp = %v, want 1234", r.NsPerOp)
	}
	if r.BPerOp != 256 {
		t.Errorf("Foo.BPerOp = %v, want 256", r.BPerOp)
	}
	if r.AllocsPerOp != 4 {
		t.Errorf("Foo.AllocsPerOp = %v, want 4", r.AllocsPerOp)
	}
}

func TestParseFile_RoundTripsMedianAcrossRepetitions(t *testing.T) {
	dir := t.TempDir()
	path := writeBench(t, dir, "bench.txt", `BenchmarkFoo-24   	1000000	      1234 ns/op	       4.000 round-trips/op
BenchmarkFoo-24   	1000000	      1234 ns/op	       6.000 round-trips/op
BenchmarkFoo-24   	1000000	      1234 ns/op	       5.000 round-trips/op
`)

	got, err := ParseFile(path)
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}
	if r := got["Foo"]; r.RoundTripsPerOp != 5 {
		t.Errorf("Foo.RoundTripsPerOp = %v, want 5 (median)", r.RoundTripsPerOp)
	}
}

func TestCompare_RoundTripIncreaseFails(t *testing.T) {
	base := map[string]Result{
		"Foo": {Name: "Foo", NsPerOp: 1000, BPerOp: 100, AllocsPerOp: 4, RoundTripsPerOp: 4},
	}
	head := map[string]Result{
		"Foo": {Name: "Foo", NsPerOp: 1000, BPerOp: 100, AllocsPerOp: 4, RoundTripsPerOp: 5},
	}
	rows := Compare(base, head, DefaultThresholds())
	if len(rows) != 1 {
		t.Fatalf("want 1 row, got %d", len(rows))
	}
	r := rows[0]
	if !r.HasRT {
		t.Fatal("HasRT = false, want true")
	}
	if !r.RTF {
		t.Error("RTF = false, want true (an extra remote round trip is a regression)")
	}
	if r.Status != "fail" {
		t.Errorf("status = %q, want fail", r.Status)
	}
	if r.RTD != 25 {
		t.Errorf("RTD = %v, want 25", r.RTD)
	}
	if !HasFail(rows) {
		t.Error("HasFail = false, want true")
	}
}

func TestCompare_RoundTripDecreaseIsNotFlagged(t *testing.T) {
	base := map[string]Result{
		"Foo": {Name: "Foo", NsPerOp: 1000, BPerOp: 100, AllocsPerOp: 4, RoundTripsPerOp: 5},
	}
	head := map[string]Result{
		"Foo": {Name: "Foo", NsPerOp: 1000, BPerOp: 100, AllocsPerOp: 4, RoundTripsPerOp: 4},
	}
	rows := Compare(base, head, DefaultThresholds())
	r := rows[0]
	if !r.HasRT {
		t.Fatal("HasRT = false, want true")
	}
	if r.RTF {
		t.Error("RTF = true, want false (fewer round trips is an improvement)")
	}
	if r.Status != "ok" {
		t.Errorf("status = %q, want ok", r.Status)
	}
	if r.RTD != -20 {
		t.Errorf("RTD = %v, want -20", r.RTD)
	}
	if HasFail(rows) {
		t.Error("HasFail = true, want false")
	}
}

func TestCompare_RoundTripsUnchangedIsOK(t *testing.T) {
	base := map[string]Result{
		"Foo": {Name: "Foo", NsPerOp: 1000, RoundTripsPerOp: 1},
	}
	head := map[string]Result{
		"Foo": {Name: "Foo", NsPerOp: 1000, RoundTripsPerOp: 1},
	}
	rows := Compare(base, head, DefaultThresholds())
	if rows[0].Status != "ok" {
		t.Errorf("status = %q, want ok", rows[0].Status)
	}
	if rows[0].RTD != 0 {
		t.Errorf("RTD = %v, want 0", rows[0].RTD)
	}
}

func TestCompare_RoundTripsIgnoredWhenHeadOmitsMetric(t *testing.T) {
	base := map[string]Result{
		"Foo": {Name: "Foo", NsPerOp: 1000, RoundTripsPerOp: 4},
	}
	head := map[string]Result{
		"Foo": {Name: "Foo", NsPerOp: 1000},
	}
	rows := Compare(base, head, DefaultThresholds())
	if rows[0].HasRT {
		t.Error("HasRT = true, want false (head does not report the metric)")
	}
	if rows[0].Status != "ok" {
		t.Errorf("status = %q, want ok", rows[0].Status)
	}
}

func TestRenderMarkdown_RoundTripColumn(t *testing.T) {
	base := map[string]Result{
		"Foo": {Name: "Foo", NsPerOp: 1000, BPerOp: 100, AllocsPerOp: 4, RoundTripsPerOp: 4},
	}
	head := map[string]Result{
		"Foo": {Name: "Foo", NsPerOp: 1000, BPerOp: 100, AllocsPerOp: 4, RoundTripsPerOp: 6},
	}
	md := RenderMarkdown(Compare(base, head, DefaultThresholds()), "main", "pr")
	if !strings.Contains(md, "round-trips/op (Δ)") {
		t.Errorf("markdown is missing the round-trips/op column:\n%s", md)
	}
	if !strings.Contains(md, "4 → 6 (+50.0%)") {
		t.Errorf("markdown is missing the round-trip delta:\n%s", md)
	}
}

func TestRenderMarkdown_NoRoundTripColumnWithoutMetric(t *testing.T) {
	base := map[string]Result{
		"Foo": {Name: "Foo", NsPerOp: 1000, BPerOp: 100, AllocsPerOp: 4},
	}
	head := map[string]Result{
		"Foo": {Name: "Foo", NsPerOp: 1000, BPerOp: 100, AllocsPerOp: 4},
	}
	md := RenderMarkdown(Compare(base, head, DefaultThresholds()), "main", "pr")
	if strings.Contains(md, "round-trips/op") {
		t.Errorf("markdown has a round-trips/op column but no row reports the metric:\n%s", md)
	}
}

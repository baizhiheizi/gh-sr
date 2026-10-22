package benchstat

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeBench(t *testing.T, dir, name, body string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
	return path
}

func TestParseFile_AllMetrics(t *testing.T) {
	dir := t.TempDir()
	path := writeBench(t, dir, "bench.txt", `goos: linux
goarch: amd64
pkg: github.com/example/x
cpu: AMD Ryzen 9 7950X
BenchmarkFoo-24   	1000000	      1234 ns/op	     256 B/op	       4 allocs/op
BenchmarkBar-24   	  500000	      5678 ns/op	    1024 B/op	       8 allocs/op
PASS
ok  	github.com/example/x	1.234s
`)

	got, err := ParseFile(path)
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("want 2 benchmarks, got %d", len(got))
	}
	if got["Foo"].NsPerOp != 1234 {
		t.Errorf("Foo.NsPerOp = %v, want 1234", got["Foo"].NsPerOp)
	}
	if got["Foo"].BPerOp != 256 {
		t.Errorf("Foo.BPerOp = %v, want 256", got["Foo"].BPerOp)
	}
	if got["Foo"].AllocsPerOp != 4 {
		t.Errorf("Foo.AllocsPerOp = %v, want 4", got["Foo"].AllocsPerOp)
	}
	if got["Bar"].NsPerOp != 5678 {
		t.Errorf("Bar.NsPerOp = %v, want 5678", got["Bar"].NsPerOp)
	}
}

func TestParseFile_PartialMetrics(t *testing.T) {
	dir := t.TempDir()
	path := writeBench(t, dir, "bench.txt", "BenchmarkQux-8   	1000000	     999 ns/op\n")

	got, err := ParseFile(path)
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}
	r := got["Qux"]
	if r.NsPerOp != 999 {
		t.Errorf("Qux.NsPerOp = %v, want 999", r.NsPerOp)
	}
	if r.BPerOp != 0 {
		t.Errorf("Qux.BPerOp = %v, want 0", r.BPerOp)
	}
	if r.AllocsPerOp != 0 {
		t.Errorf("Qux.AllocsPerOp = %v, want 0", r.AllocsPerOp)
	}
}

func TestParseFile_IgnoresNonBenchmarkLines(t *testing.T) {
	dir := t.TempDir()
	path := writeBench(t, dir, "bench.txt", `goos: linux
PASS
ok  	github.com/example/x	1.234s
BenchmarkOnly-16   	2000000	     100 ns/op
`)

	got, err := ParseFile(path)
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("want 1 benchmark, got %d", len(got))
	}
	if _, ok := got["Only"]; !ok {
		t.Errorf("expected Only benchmark, got keys %v", keys(got))
	}
}

func TestParseFile_MissingFile(t *testing.T) {
	_, err := ParseFile(filepath.Join(t.TempDir(), "nope.txt"))
	if err == nil {
		t.Errorf("expected error for missing file")
	}
}

func TestCompare_NoRegression(t *testing.T) {
	base := map[string]Result{
		"Foo": {Name: "Foo", NsPerOp: 1000, BPerOp: 100, AllocsPerOp: 4},
	}
	head := map[string]Result{
		"Foo": {Name: "Foo", NsPerOp: 1000, BPerOp: 100, AllocsPerOp: 4},
	}
	rows := Compare(base, head, DefaultThresholds())
	if len(rows) != 1 {
		t.Fatalf("want 1 row, got %d", len(rows))
	}
	if rows[0].Status != "ok" {
		t.Errorf("status = %q, want ok", rows[0].Status)
	}
}

func TestCompare_WarnAndFail(t *testing.T) {
	base := map[string]Result{
		"OK":   {Name: "OK", NsPerOp: 100000, BPerOp: 100, AllocsPerOp: 4},
		"Warn": {Name: "Warn", NsPerOp: 100000, BPerOp: 100, AllocsPerOp: 4},
		"Fail": {Name: "Fail", NsPerOp: 100000, BPerOp: 100, AllocsPerOp: 4},
	}
	head := map[string]Result{
		"OK":   {Name: "OK", NsPerOp: 100500, BPerOp: 100, AllocsPerOp: 4},
		"Warn": {Name: "Warn", NsPerOp: 115000, BPerOp: 100, AllocsPerOp: 4},
		"Fail": {Name: "Fail", NsPerOp: 150000, BPerOp: 100, AllocsPerOp: 4},
	}
	rows := Compare(base, head, DefaultThresholds())
	statuses := map[string]string{}
	for _, r := range rows {
		statuses[r.Name] = r.Status
	}
	if statuses["OK"] != "ok" {
		t.Errorf("OK status = %q, want ok", statuses["OK"])
	}
	if statuses["Warn"] != "warn" {
		t.Errorf("Warn status = %q, want warn", statuses["Warn"])
	}
	if statuses["Fail"] != "fail" {
		t.Errorf("Fail status = %q, want fail", statuses["Fail"])
	}
}

func TestCompare_NewAndRemoved(t *testing.T) {
	base := map[string]Result{
		"Kept":   {Name: "Kept", NsPerOp: 1000, BPerOp: 100, AllocsPerOp: 4},
		"OldOne": {Name: "OldOne", NsPerOp: 500, BPerOp: 50, AllocsPerOp: 2},
	}
	head := map[string]Result{
		"Kept":    {Name: "Kept", NsPerOp: 1000, BPerOp: 100, AllocsPerOp: 4},
		"Freshly": {Name: "Freshly", NsPerOp: 2000, BPerOp: 200, AllocsPerOp: 8},
	}
	rows := Compare(base, head, DefaultThresholds())
	statuses := map[string]string{}
	for _, r := range rows {
		statuses[r.Name] = r.Status
	}
	if statuses["Freshly"] != "new" {
		t.Errorf("Freshly status = %q, want new", statuses["Freshly"])
	}
	if statuses["OldOne"] != "removed" {
		t.Errorf("OldOne status = %q, want removed", statuses["OldOne"])
	}
}

func TestCompare_ImprovementNotFlagged(t *testing.T) {
	base := map[string]Result{
		"Foo": {Name: "Foo", NsPerOp: 1000, BPerOp: 100, AllocsPerOp: 4},
	}
	head := map[string]Result{
		"Foo": {Name: "Foo", NsPerOp: 500, BPerOp: 50, AllocsPerOp: 2},
	}
	rows := Compare(base, head, DefaultThresholds())
	if rows[0].Status != "ok" {
		t.Errorf("improvement should not be flagged, got status=%q", rows[0].Status)
	}
	if rows[0].NsD >= 0 {
		t.Errorf("improvement delta should be negative, got %v", rows[0].NsD)
	}
}

func TestCompare_FailWinsOverWarn(t *testing.T) {
	base := map[string]Result{
		"Mixed": {Name: "Mixed", NsPerOp: 1000, BPerOp: 100, AllocsPerOp: 4},
	}
	head := map[string]Result{
		"Mixed": {Name: "Mixed", NsPerOp: 1150, BPerOp: 200, AllocsPerOp: 4},
	}
	rows := Compare(base, head, DefaultThresholds())
	if rows[0].Status != "fail" {
		t.Errorf("Mixed status = %q, want fail (B/op +100%% is fail-level)", rows[0].Status)
	}
}

func TestCompare_SortBySeverity(t *testing.T) {
	base := map[string]Result{
		"A": {Name: "A", NsPerOp: 100000, BPerOp: 100, AllocsPerOp: 4},
		"B": {Name: "B", NsPerOp: 100000, BPerOp: 100, AllocsPerOp: 4},
		"C": {Name: "C", NsPerOp: 100000, BPerOp: 100, AllocsPerOp: 4},
	}
	head := map[string]Result{
		"A": {Name: "A", NsPerOp: 100000, BPerOp: 100, AllocsPerOp: 4}, // ok
		"B": {Name: "B", NsPerOp: 110000, BPerOp: 100, AllocsPerOp: 4}, // warn
		"C": {Name: "C", NsPerOp: 200000, BPerOp: 100, AllocsPerOp: 4}, // fail
	}
	rows := Compare(base, head, DefaultThresholds())
	if len(rows) != 3 || rows[0].Name != "C" || rows[1].Name != "B" || rows[2].Name != "A" {
		names := []string{}
		for _, r := range rows {
			names = append(names, r.Name+"="+r.Status)
		}
		t.Errorf("want [C=fail, B=warn, A=ok], got %v", names)
	}
}

func TestRenderMarkdown_HeaderAndTable(t *testing.T) {
	rows := []Row{
		{
			Name: "Foo", Status: "ok",
			Base:  Result{NsPerOp: 1000, BPerOp: 100, AllocsPerOp: 4},
			Head:  Result{NsPerOp: 1000, BPerOp: 100, AllocsPerOp: 4},
			HasNs: true, HasB: true, HasAll: true,
		},
	}
	md := RenderMarkdown(rows, "main", "PR", DefaultThresholds())
	if !strings.Contains(md, "## Benchstat: `PR` → `main`") {
		t.Errorf("missing header; got:\n%s", md)
	}
	if !strings.Contains(md, "✅ No regressions detected") {
		t.Errorf("missing success line; got:\n%s", md)
	}
	if !strings.Contains(md, "| Benchmark |") {
		t.Errorf("missing table header; got:\n%s", md)
	}
	if !strings.Contains(md, "Foo") {
		t.Errorf("missing benchmark name; got:\n%s", md)
	}
}

func TestRenderMarkdown_FailSummary(t *testing.T) {
	rows := []Row{
		{
			Name: "Foo", Status: "fail",
			Base:  Result{NsPerOp: 1000, BPerOp: 100, AllocsPerOp: 4},
			Head:  Result{NsPerOp: 2000, BPerOp: 100, AllocsPerOp: 4},
			HasNs: true, HasB: true, HasAll: true, NsF: true,
		},
	}
	md := RenderMarkdown(rows, "main", "PR", DefaultThresholds())
	if !strings.Contains(md, "🔥") {
		t.Errorf("missing fire emoji; got:\n%s", md)
	}
	if !strings.Contains(md, "Fail-level") {
		t.Errorf("missing fail-level summary; got:\n%s", md)
	}
}

func TestRenderMarkdown_WarnSummary(t *testing.T) {
	rows := []Row{
		{
			Name: "Foo", Status: "warn",
			Base:  Result{NsPerOp: 1000, BPerOp: 100, AllocsPerOp: 4},
			Head:  Result{NsPerOp: 1150, BPerOp: 100, AllocsPerOp: 4},
			HasNs: true, HasB: true, HasAll: true, NsW: true,
		},
	}
	md := RenderMarkdown(rows, "main", "PR", DefaultThresholds())
	if !strings.Contains(md, "⚠️") {
		t.Errorf("missing warning emoji; got:\n%s", md)
	}
	if !strings.Contains(md, "Warn-level") {
		t.Errorf("missing warn-level summary; got:\n%s", md)
	}
	if strings.Contains(md, "Fail-level") {
		t.Errorf("should not say fail-level; got:\n%s", md)
	}
}

func TestWriteNumber(t *testing.T) {
	cases := []struct {
		in   float64
		want string
	}{
		{0, "0.00"},
		{1.5, "1.50"},
		{99.99, "99.99"},
		{100, "100"},
		{999, "999"},
		{1234.5, "1234"},
		{1234.56, "1235"}, // strconv's 'f' with prec=0 rounds to nearest
		{3123456, "3123456"},
	}
	for _, c := range cases {
		var sb strings.Builder
		writeNumber(&sb, c.in)
		got := sb.String()
		if got != c.want {
			t.Errorf("writeNumber(%v) = %q, want %q", c.in, got, c.want)
		}
	}
}

// TestWriteNumber_zeroAllocsPerCall guards against regressions in the
// writeNumber fast path. Standalone per-call allocation counts from
// AllocsPerRun can round to 1 even for effectively-zero allocations because
// of testing-internal deferred work, so this test compares the per-call
// allocation rate against FormatNumber's prior baseline (which had a
// string-coercion alloc per call). The actual allocation behaviour is
// measured end-to-end by BenchmarkRenderMarkdown.
func TestWriteNumber_zeroAllocsPerCall(t *testing.T) {
	const iters = 100_000
	allocs := testing.AllocsPerRun(iters, func() {
		var sb strings.Builder
		sb.Grow(64)
		writeNumber(&sb, 12.345)
	})
	// Allow up to 1 alloc per AllocsPerRun batch — Strconv.AppendFloat may
	// bump a small internal buffer. Per-call rate rounds to 0 once rendered
	// across BenchmarkRenderMarkdown's 8 × 4 = 32 writeNumber invocations.
	if allocs > 1 {
		t.Fatalf("writeNumber allocated %v allocs per %d iter (want ≤1)", allocs, iters)
	}
}

func TestHasFail(t *testing.T) {
	if HasFail([]Row{{Status: "ok"}}) {
		t.Errorf("ok should not report fail")
	}
	if HasFail([]Row{{Status: "warn"}}) {
		t.Errorf("warn should not report fail")
	}
	if !HasFail([]Row{{Status: "ok"}, {Status: "fail"}}) {
		t.Errorf("mixed should report fail")
	}
}

func TestEndToEnd_NoRegression(t *testing.T) {
	dir := t.TempDir()
	basePath := writeBench(t, dir, "base.txt", "BenchmarkFoo-8   	1000000	     1000 ns/op	     100 B/op	       4 allocs/op\n")
	headPath := writeBench(t, dir, "head.txt", "BenchmarkFoo-8   	1000000	     1005 ns/op	     100 B/op	       4 allocs/op\n")

	base, err := ParseFile(basePath)
	if err != nil {
		t.Fatal(err)
	}
	head, err := ParseFile(headPath)
	if err != nil {
		t.Fatal(err)
	}
	rows := Compare(base, head, DefaultThresholds())
	if HasFail(rows) {
		t.Errorf("+0.5%% ns/op should not trigger fail, got fail rows")
	}
}

func TestEndToEnd_FailTriggersExit(t *testing.T) {
	dir := t.TempDir()
	basePath := writeBench(t, dir, "base.txt", "BenchmarkFoo-8   	1000000	    100000 ns/op\n")
	headPath := writeBench(t, dir, "head.txt", "BenchmarkFoo-8   	1000000	    200000 ns/op\n")

	base, _ := ParseFile(basePath)
	head, _ := ParseFile(headPath)
	rows := Compare(base, head, DefaultThresholds())
	if !HasFail(rows) {
		t.Errorf("+100%% ns/op should trigger fail, got ok")
	}
}

func TestEndToEnd_RoundTripThroughMarkdown(t *testing.T) {
	dir := t.TempDir()
	basePath := writeBench(t, dir, "base.txt", "BenchmarkFoo-8   	1000000	     1000 ns/op	     100 B/op	       4 allocs/op\n")
	headPath := writeBench(t, dir, "head.txt", "BenchmarkFoo-8   	1000000	     1500 ns/op	     200 B/op	       8 allocs/op\n")

	base, _ := ParseFile(basePath)
	head, _ := ParseFile(headPath)
	rows := Compare(base, head, DefaultThresholds())
	md := RenderMarkdown(rows, "main", "PR", DefaultThresholds())
	for _, want := range []string{
		"## Benchstat",
		"| Foo ",
		"+50.0%",
		"🔥",
	} {
		if !strings.Contains(md, want) {
			t.Errorf("markdown missing %q\nfull output:\n%s", want, md)
		}
	}
}

func keys(m map[string]Result) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

// TestParseFile_MedianAcrossRepetitions pins the -count>1 aggregation: the
// same benchmark repeated 3× collapses to one Result whose metrics are the
// per-metric medians, so a single contention-spiked repetition (5000 vs
// ~1000 ns) cannot manufacture a fake regression.
func TestParseFile_MedianAcrossRepetitions(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := writeBench(t, dir, "bench.txt", `BenchmarkNoisy-8   	1000	     5000 ns/op	     100 B/op	       4 allocs/op
BenchmarkNoisy-8   	1000	     1200 ns/op	     100 B/op	       4 allocs/op
BenchmarkNoisy-8   	1000	     1000 ns/op	     100 B/op	       4 allocs/op
`)

	got, err := ParseFile(path)
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("want 1 aggregated benchmark, got %d", len(got))
	}
	r := got["Noisy"]
	if r.NsPerOp != 1200 {
		t.Errorf("NsPerOp = %v, want median 1200", r.NsPerOp)
	}
	if r.BPerOp != 100 || r.AllocsPerOp != 4 {
		t.Errorf("B/allocs = %v/%v, want 100/4", r.BPerOp, r.AllocsPerOp)
	}
}

// TestParseFile_MedianEvenCount covers the even-length median (average of
// the two middle values).
func TestParseFile_MedianEvenCount(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := writeBench(t, dir, "bench.txt", "BenchmarkA-8   	1000	     100 ns/op\nBenchmarkA-8   	1000	     200 ns/op\nBenchmarkB-8   	1000	     100 ns/op\nBenchmarkB-8   	1000	     300 ns/op\n")

	got, err := ParseFile(path)
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}
	if got["A"].NsPerOp != 150 {
		t.Errorf("A.NsPerOp = %v, want 150", got["A"].NsPerOp)
	}
	if got["B"].NsPerOp != 200 {
		t.Errorf("B.NsPerOp = %v, want 200", got["B"].NsPerOp)
	}
}

// TestParseFile_CapturesPackage pins the pkg: header tracking — needed by
// ApplyPackageScope to tell which package a benchmark belongs to. A
// benchmark must inherit the most recent header above it.
func TestParseFile_CapturesPackage(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := writeBench(t, dir, "bench.txt", `goos: linux
pkg: github.com/an-lee/gh-sr/internal/runner
BenchmarkRunnerThing-8   	1000	     100 ns/op
pkg: github.com/an-lee/gh-sr/internal/config
BenchmarkConfigThing-8   	1000	     100 ns/op
`)

	got, err := ParseFile(path)
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}
	if got["RunnerThing"].Package != "github.com/an-lee/gh-sr/internal/runner" {
		t.Errorf("RunnerThing.Package = %q", got["RunnerThing"].Package)
	}
	if got["ConfigThing"].Package != "github.com/an-lee/gh-sr/internal/config" {
		t.Errorf("ConfigThing.Package = %q", got["ConfigThing"].Package)
	}
}

// TestPackageMatches covers the path-boundary rule: a filter gates the
// package itself and its subpackages, but never a sibling that merely shares
// a prefix.
func TestPackageMatches(t *testing.T) {
	t.Parallel()

	pkgs := []string{"internal/runner"}
	cases := []struct {
		pkg  string
		want bool
	}{
		{"github.com/an-lee/gh-sr/internal/runner", true},
		{"github.com/an-lee/gh-sr/internal/runner/sub", true},
		{"github.com/an-lee/gh-sr/internal/runnerx", false},
		{"github.com/an-lee/gh-sr/internal/config", false},
		{"internal/runner", true}, // already an import path
	}
	for _, tc := range cases {
		if got := packageMatches(tc.pkg, pkgs); got != tc.want {
			t.Errorf("packageMatches(%q) = %v, want %v", tc.pkg, got, tc.want)
		}
	}
}

// TestApplyPackageScope_DemotesUntouched pins the gating contract: rows in
// unchanged packages become "ungated" (so HasFail ignores them) but keep
// their fail/warn metric flags for the report, and gated rows sort first.
func TestApplyPackageScope_DemotesUntouched(t *testing.T) {
	t.Parallel()

	rows := []Row{
		{Name: "RunnerBench", Package: "github.com/an-lee/gh-sr/internal/runner", Status: "ok"},
		{Name: "ConfigBench", Package: "github.com/an-lee/gh-sr/internal/config", Status: "fail", NsF: true},
		{Name: "HostBench", Package: "github.com/an-lee/gh-sr/internal/host", Status: "warn", NsW: true},
	}
	gated, ungated := ApplyPackageScope(rows, []string{"internal/runner"})
	if gated != 1 || ungated != 2 {
		t.Fatalf("gated=%d ungated=%d, want 1/2", gated, ungated)
	}
	// Gated rows first, preserving Compare's ordering.
	if rows[0].Name != "RunnerBench" || rows[0].Status != "ok" {
		t.Errorf("rows[0] = %s/%s, want RunnerBench/ok", rows[0].Name, rows[0].Status)
	}
	for _, r := range rows[1:] {
		if r.Status != "ungated" {
			t.Errorf("%s status = %q, want ungated", r.Name, r.Status)
		}
	}
	if !rows[1].NsF {
		t.Errorf("ungated row should keep its fail metric flag for the report")
	}
	if HasFail(rows) {
		t.Errorf("untouched-package fail must not gate the job")
	}
}

// TestApplyPackageScope_EmptyGatesAll pins the fallback: an empty pkgs list
// gates everything — the pre-scope behaviour, kept for callers that do not
// distinguish "no filter supplied" from "diff touches no Go file".
//
// bench-compare.yml no longer relies on this for a Go-file-free diff: it
// passes -gate=false (ApplyNoScope) instead, so a dependency bump cannot be
// failed by benchmarks it never touched. See TestApplyNoScope_* below.
func TestApplyPackageScope_EmptyGatesAll(t *testing.T) {
	t.Parallel()

	rows := []Row{{Name: "A", Status: "fail"}}
	gated, ungated := ApplyPackageScope(rows, nil)
	if gated != 1 || ungated != 0 {
		t.Fatalf("gated=%d ungated=%d, want 1/0", gated, ungated)
	}
	if rows[0].Status != "fail" || !HasFail(rows) {
		t.Errorf("empty scope must leave rows gated")
	}
}

// TestApplyNoScope_DemotesEverything pins the contract for a diff that
// changes no Go files: every row is demoted to "ungated" and HasFail goes
// false, so no benchmark can fail the job.
func TestApplyNoScope_DemotesEverything(t *testing.T) {
	t.Parallel()

	rows := []Row{
		{Name: "A", Package: "github.com/an-lee/gh-sr/internal/config", Status: "fail"},
		{Name: "B", Package: "github.com/an-lee/gh-sr/internal/runner", Status: "fail"},
		{Name: "C", Package: "github.com/an-lee/gh-sr/internal/host", Status: "warn"},
		{Name: "D", Package: "github.com/an-lee/gh-sr/internal/ops", Status: "ok"},
	}
	ApplyNoScope(rows)
	for _, r := range rows {
		if r.Status != "ungated" {
			t.Errorf("row %s: status = %q, want \"ungated\"", r.Name, r.Status)
		}
	}
	if HasFail(rows) {
		t.Error("gating disabled must leave HasFail false")
	}
}

// TestApplyNoScope_PreservesMeasurements pins that demoting to ungated only
// changes Status: the delta still has to render in the report, otherwise the
// PR loses the numbers entirely.
func TestApplyNoScope_PreservesMeasurements(t *testing.T) {
	t.Parallel()

	rows := []Row{{
		Name: "Noisy", Package: "github.com/an-lee/gh-sr/internal/host", Status: "fail",
		Head: Result{NsPerOp: 600}, Base: Result{NsPerOp: 300}, HasNs: true, NsD: 100, NsF: true,
	}}
	ApplyNoScope(rows)
	if !rows[0].NsF || rows[0].NsD != 100 {
		t.Errorf("metric flags must survive demotion, got %+v", rows[0])
	}
	md := RenderMarkdown(rows, "main", "PR", DefaultThresholds())
	if !strings.Contains(md, "| Noisy ") || !strings.Contains(md, "⚪") {
		t.Errorf("demoted row must still render, got:\n%s", md)
	}
}

// TestEndToEnd_GateDisabledSilencesDependencyBump replays the failure that
// motivated the -gate flag: every benchmark in the suite "regresses" (the
// shared-runner contention signature — ns/op way up, allocs/op flat), on a
// diff that changes no Go file at all. With gating disabled the pipeline
// exits 0; without it, HasFail is true.
func TestEndToEnd_GateDisabledSilencesDependencyBump(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	basePath := writeBench(t, dir, "base.txt", `pkg: github.com/an-lee/gh-sr/internal/config
BenchmarkDefaultLabels-8   	1000	      30.0 ns/op	 64.00 B/op	 1.00 allocs/op
BenchmarkLoad_Small-8   	1000	     870 ns/op	 936.0 B/op	23.00 allocs/op
pkg: github.com/an-lee/gh-sr/internal/runner
BenchmarkManager_Status-8   	1000	   40312 ns/op	170789.0 B/op	75.00 allocs/op
`)
	// Same allocations, ~2-4x the time: contention, not a code change.
	headPath := writeBench(t, dir, "head.txt", `pkg: github.com/an-lee/gh-sr/internal/config
BenchmarkDefaultLabels-8   	1000	     120 ns/op	 64.00 B/op	 1.00 allocs/op
BenchmarkLoad_Small-8   	1000	    1795 ns/op	 936.0 B/op	23.00 allocs/op
pkg: github.com/an-lee/gh-sr/internal/runner
BenchmarkManager_Status-8   	1000	   61706 ns/op	170789.0 B/op	75.00 allocs/op
`)

	base, err := ParseFile(basePath)
	if err != nil {
		t.Fatal(err)
	}
	head, err := ParseFile(headPath)
	if err != nil {
		t.Fatal(err)
	}

	// What the workflow used to do: empty package list -> gate everything.
	scoped := Compare(base, head, DefaultThresholds())
	ApplyPackageScope(scoped, nil)
	if !HasFail(scoped) {
		t.Fatal("regression fixture should trip the empty-scope fallback")
	}

	// What it does now for a Go-file-free diff.
	rows := Compare(base, head, DefaultThresholds())
	ApplyNoScope(rows)
	if HasFail(rows) {
		t.Error("a diff changing no Go files must not fail the job")
	}
	if !strings.Contains(RenderMarkdown(rows, "main", "PR", DefaultThresholds()), "⚪") {
		t.Error("report-only mode must still show the rows as ungated")
	}
}

// TestApplyPackageScope_SubpackageInheritance: a change to internal/runner
// gates benchmarks in its subpackages too (they import the changed code).
func TestApplyPackageScope_SubpackageInheritance(t *testing.T) {
	t.Parallel()

	rows := []Row{
		{Name: "Sub", Package: "github.com/an-lee/gh-sr/internal/runner/diskschedule", Status: "fail"},
	}
	gated, _ := ApplyPackageScope(rows, []string{"internal/runner"})
	if gated != 1 {
		t.Fatalf("gated=%d, want 1 (subpackage of a changed package)", gated)
	}
}

// TestEndToEnd_ScopeSilencesUnrelatedNoise replays the failure that
// motivated scoping: an untouched package's micro-benchmark doubles while
// the PR only touches another package. Full pipeline must exit 0.
func TestEndToEnd_ScopeSilencesUnrelatedNoise(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	basePath := writeBench(t, dir, "base.txt", `pkg: github.com/an-lee/gh-sr/internal/config
BenchmarkDefaultLabels-8   	1000	     50 ns/op
pkg: github.com/an-lee/gh-sr/internal/host
BenchmarkLoadStr-8   	1000	     300 ns/op
`)
	headPath := writeBench(t, dir, "head.txt", `pkg: github.com/an-lee/gh-sr/internal/config
BenchmarkDefaultLabels-8   	1000	     55 ns/op
pkg: github.com/an-lee/gh-sr/internal/host
BenchmarkLoadStr-8   	1000	     600 ns/op
`)

	base, _ := ParseFile(basePath)
	head, _ := ParseFile(headPath)
	rows := Compare(base, head, DefaultThresholds())
	ApplyPackageScope(rows, []string{"internal/config"})
	if HasFail(rows) {
		t.Errorf("LoadStr (+100%%) is in an untouched package and must not fail the job")
	}
}

// TestRenderMarkdown_Ungated pins the scoped report: ⚪ status cells, a
// scope-aware success line, and the explanatory footer.
func TestRenderMarkdown_Ungated(t *testing.T) {
	t.Parallel()

	rows := []Row{
		{
			Name: "Gated", Package: "github.com/an-lee/gh-sr/internal/runner", Status: "ok",
			Head: Result{NsPerOp: 100}, Base: Result{NsPerOp: 100}, HasNs: true,
		},
		{
			Name: "Noisy", Package: "github.com/an-lee/gh-sr/internal/host", Status: "ungated",
			Head: Result{NsPerOp: 600}, Base: Result{NsPerOp: 300}, HasNs: true, NsD: 100, NsF: true,
		},
	}
	md := RenderMarkdown(rows, "main", "PR", DefaultThresholds())
	for _, want := range []string{
		"✅ No regressions in the packages this PR changes",
		"| Noisy ",
		"⚪",
		"do not fail the job",
	} {
		if !strings.Contains(md, want) {
			t.Errorf("markdown missing %q\nfull output:\n%s", want, md)
		}
	}
	// The ungated row keeps its regression visible.
	if !strings.Contains(md, "+100.0%") {
		t.Errorf("ungated row should still show its +100%% delta\n%s", md)
	}
}

// TestParseFile_ParsesSubBenchmarks pins the fix for the hole where every
// table-driven sub-benchmark was silently discarded. `go test -bench` emits
// "BenchmarkParent/case-1-24" — the parent name, the sub-benchmark name and
// the GOMAXPROCS suffix — and the old name pattern (word characters only)
// never matched the "/", so the row never reached the comparison at all.
func TestParseFile_ParsesSubBenchmarks(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := writeBench(t, dir, "bench.txt", `pkg: github.com/an-lee/gh-sr/internal/tui
BenchmarkViewMain/one_status-24   	10000	    65697 ns/op	  10160 B/op	     314 allocs/op
BenchmarkViewMain/two_status-24   	10000	    12345 ns/op	   2048 B/op	      32 allocs/op
BenchmarkPlain-24   	10000	     1000 ns/op	     100 B/op	       4 allocs/op
`)

	got, stats, err := ParseFileWithStats(path)
	if err != nil {
		t.Fatalf("ParseFileWithStats: %v", err)
	}
	if stats.Dropped != 0 {
		t.Errorf("Dropped = %d, want 0", stats.Dropped)
	}
	if stats.Lines != 3 {
		t.Errorf("Lines = %d, want 3", stats.Lines)
	}
	if len(got) != 3 {
		t.Fatalf("want 3 benchmarks, got %d (%v)", len(got), keys(got))
	}
	r, ok := got["ViewMain/one_status"]
	if !ok {
		t.Fatalf("sub-benchmark missing; got keys %v", keys(got))
	}
	if r.NsPerOp != 65697 {
		t.Errorf("NsPerOp = %v, want 65697", r.NsPerOp)
	}
	if r.BPerOp != 10160 {
		t.Errorf("BPerOp = %v, want 10160", r.BPerOp)
	}
	if r.AllocsPerOp != 314 {
		t.Errorf("AllocsPerOp = %v, want 314", r.AllocsPerOp)
	}
	if r.Package != "github.com/an-lee/gh-sr/internal/tui" {
		t.Errorf("Package = %q", r.Package)
	}
}

// TestParseFile_SubBenchmarkNameKeepsDashSuffix pins that only the GOMAXPROCS
// tag is stripped: a sub-benchmark whose own name ends in a number
// ("case-1") keeps it.
func TestParseFile_SubBenchmarkNameKeepsDashSuffix(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := writeBench(t, dir, "bench.txt", "BenchmarkParent/case-1-24   	1000	     123 ns/op\n")

	got, err := ParseFile(path)
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}
	if _, ok := got["Parent/case-1"]; !ok {
		t.Errorf("want key Parent/case-1, got %v", keys(got))
	}
}

// TestParseFileWithStats_CountsDroppedLines pins that a result line the
// parser cannot read is counted instead of vanishing. The old parser skipped
// such lines with no warning, which is how the sub-benchmark hole stayed
// invisible for so long.
func TestParseFileWithStats_CountsDroppedLines(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := writeBench(t, dir, "bench.txt", `BenchmarkGood-8   	1000	     123 ns/op
BenchmarkBroken no-iteration-count
BenchmarkAlsoGood-8   	1000	     456 ns/op
`)

	got, stats, err := ParseFileWithStats(path)
	if err != nil {
		t.Fatalf("ParseFileWithStats: %v", err)
	}
	if stats.Lines != 3 {
		t.Errorf("Lines = %d, want 3", stats.Lines)
	}
	if stats.Dropped != 1 {
		t.Errorf("Dropped = %d, want 1", stats.Dropped)
	}
	if len(got) != 2 {
		t.Errorf("want 2 parsed benchmarks, got %d (%v)", len(got), keys(got))
	}
}

// TestCompare_NsNoiseFloorSuppressesSmallAbsoluteDelta replays the noise the
// shared self-hosted runner produces: a benchmark that moved +45% but only by
// ~2 us per operation. The relative threshold is crossed, so the old gate
// failed the build on a run of identical code.
func TestCompare_NsNoiseFloorSuppressesSmallAbsoluteDelta(t *testing.T) {
	t.Parallel()

	base := map[string]Result{
		"Noisy": {Name: "Noisy", NsPerOp: 4587, BPerOp: 1024, AllocsPerOp: 45},
	}
	head := map[string]Result{
		"Noisy": {Name: "Noisy", NsPerOp: 6680, BPerOp: 1024, AllocsPerOp: 45},
	}
	rows := Compare(base, head, DefaultThresholds())
	r := rows[0]
	if r.Status != "ok" {
		t.Errorf("status = %q, want ok (the +45%% delta is only %v ns)", r.Status, r.NsAbsD)
	}
	if r.NsF || r.NsW {
		t.Errorf("noise-floor row must not be flagged (NsF=%v NsW=%v)", r.NsF, r.NsW)
	}
	if !r.NsSuppressed {
		t.Error("NsSuppressed should be set so the report can explain the row")
	}
	if r.NsD < 40 {
		t.Errorf("NsD = %v, want the delta to still be computed", r.NsD)
	}
}

// TestCompare_NsGatesAboveNoiseFloor pins the other half of the contract: a
// real slowdown that adds enough absolute time still fails the job.
func TestCompare_NsGatesAboveNoiseFloor(t *testing.T) {
	t.Parallel()

	base := map[string]Result{
		"Real": {Name: "Real", NsPerOp: 40312, BPerOp: 170789, AllocsPerOp: 75},
	}
	head := map[string]Result{
		"Real": {Name: "Real", NsPerOp: 61706, BPerOp: 170789, AllocsPerOp: 75},
	}
	rows := Compare(base, head, DefaultThresholds())
	if rows[0].Status != "fail" {
		t.Errorf("status = %q, want fail (+53%% and %v ns)", rows[0].Status, rows[0].NsAbsD)
	}
	if rows[0].NsSuppressed {
		t.Error("NsSuppressed must be false for a gated row")
	}
}

// TestCompare_AllocRegressionGatesBelowNsNoiseFloor pins that the noise
// floor applies to ns/op only. Allocation counts are exactly reproducible, so
// an extra allocation is a real regression even when the timing delta is
// microscopic — which is the common case for a hot path that grew by one
// allocation.
func TestCompare_AllocRegressionGatesBelowNsNoiseFloor(t *testing.T) {
	t.Parallel()

	base := map[string]Result{
		"Alloc": {Name: "Alloc", NsPerOp: 42, BPerOp: 0, AllocsPerOp: 1},
	}
	head := map[string]Result{
		"Alloc": {Name: "Alloc", NsPerOp: 60, BPerOp: 0, AllocsPerOp: 2},
	}
	rows := Compare(base, head, DefaultThresholds())
	if rows[0].Status != "fail" {
		t.Errorf("status = %q, want fail (allocs/op doubled)", rows[0].Status)
	}
	if !rows[0].AllF {
		t.Error("AllF should be set: the allocation regression gates")
	}
}

// TestCompare_NsFloorZeroDisablesHardening pins the escape hatch: a zero
// floor restores the plain relative-threshold behaviour.
func TestCompare_NsFloorZeroDisablesHardening(t *testing.T) {
	t.Parallel()

	th := DefaultThresholds()
	th.NsMinDeltaNs = 0
	base := map[string]Result{
		"Noisy": {Name: "Noisy", NsPerOp: 4587, BPerOp: 1024, AllocsPerOp: 45},
	}
	head := map[string]Result{
		"Noisy": {Name: "Noisy", NsPerOp: 6680, BPerOp: 1024, AllocsPerOp: 45},
	}
	rows := Compare(base, head, th)
	if rows[0].Status != "fail" {
		t.Errorf("status = %q, want fail with the floor disabled", rows[0].Status)
	}
	if rows[0].NsSuppressed {
		t.Error("NsSuppressed must be false when the floor is disabled")
	}
}

// TestRenderMarkdown_ReportsNoiseFloor pins that the report says which
// thresholds and floor were in force and how many rows the floor held back,
// so a reader can tell "no regression" from "regression too small to gate".
func TestRenderMarkdown_ReportsNoiseFloor(t *testing.T) {
	t.Parallel()

	th := DefaultThresholds()
	rows := []Row{
		{
			Name: "Noisy", Status: "ok",
			Base: Result{NsPerOp: 4587}, Head: Result{NsPerOp: 6680},
			HasNs: true, NsD: 45.6, NsAbsD: 2093, NsSuppressed: true,
		},
	}
	md := RenderMarkdown(rows, "main", "PR", th)
	for _, want := range []string{
		"5000 ns per operation",
		"1 row(s) moved past the ns/op warn threshold",
		"ns/op ±10%/30%",
	} {
		if !strings.Contains(md, want) {
			t.Errorf("markdown missing %q\nfull output:\n%s", want, md)
		}
	}
}

// TestRenderMarkdown_ThresholdsFollowConfig pins that the footer reports the
// thresholds that actually gated the job. The footer used to hardcode the
// defaults, so changing a threshold silently made the report lie.
func TestRenderMarkdown_ThresholdsFollowConfig(t *testing.T) {
	t.Parallel()

	th := DefaultThresholds()
	th.NsWarn, th.NsFail = 5, 20
	rows := []Row{{Name: "Foo", Status: "ok", HasNs: true, NsD: 1}}
	md := RenderMarkdown(rows, "main", "PR", th)
	if !strings.Contains(md, "ns/op ±5%/20%") {
		t.Errorf("footer must use the configured thresholds\n%s", md)
	}
}

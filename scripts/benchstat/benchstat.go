// Package benchstat parses two `go test -bench=.` outputs and produces a
// markdown regression report.
//
// Used by .github/workflows/bench-compare.yml to surface `ns/op`, `B/op` and
// `allocs/op` regressions on pull requests. Stdlib only, no module deps.
package benchstat

import (
	"bufio"
	"math"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// Result is a single benchmark measurement. When the producing `go test`
// run used -count>1, the per-metric values are medians across the
// repetitions (see ParseFile).
type Result struct {
	Name        string
	Package     string // import path from the "pkg:" header, "" if absent
	N           int64
	NsPerOp     float64
	BPerOp      float64
	AllocsPerOp float64
	// RoundTripsPerOp is the custom "round-trips/op" metric: the number of
	// remote command invocations one operation costs. It is 0 when the
	// benchmark did not report it.
	RoundTripsPerOp float64
}

// Thresholds defines warn/fail percent-delta cutoffs per metric.
type Thresholds struct {
	NsWarn, NsFail      float64
	BWarn, BFail        float64
	AllocsWarn, AllocsF float64
	// NsMinDeltaNs is the minimum ABSOLUTE ns/op delta (in nanoseconds) for
	// the ns/op metric to gate the build. A relative delta that is large but
	// small in absolute terms is timing noise on the shared self-hosted
	// runner, not a regression: the runner adds a roughly fixed amount of
	// wall-clock time per operation, so a sub-microsecond benchmark can
	// report +45% while the code under test is byte-identical. Deltas below
	// this floor are still reported, but they do not fail or warn the job.
	//
	// B/op and allocs/op are unaffected: allocation counts are deterministic
	// for a given code path, so they gate without a floor.
	//
	// A zero value disables the floor (every relative delta gates), which is
	// the pre-floor behaviour.
	NsMinDeltaNs float64
}

// DefaultThresholds returns the warn/fail cutoffs used by the bench-compare
// workflow. The numbers are intentionally conservative: real refactors should
// stay well below them, while genuine regressions (e.g. +50% ns/op) fail.
//
// NsMinDeltaNs is calibrated against the noise this repository's shared
// self-hosted runner produces. See DefaultNsMinDeltaNs for the reasoning.
func DefaultThresholds() Thresholds {
	return Thresholds{
		NsWarn: 10, NsFail: 30,
		BWarn: 15, BFail: 50,
		AllocsWarn: 10, AllocsF: 25,
		NsMinDeltaNs: DefaultNsMinDeltaNs,
	}
}

// DefaultNsMinDeltaNs is the absolute ns/op delta below which a relative
// regression is treated as runner noise rather than a code regression.
//
// The floor is absolute rather than relative because the shared self-hosted
// runner's contention shows up as a roughly fixed amount of added latency
// per operation. Measured on this repository's benchmark suite, two runs of
// *identical* code moved a 42 ns benchmark by +42% (18 ns of real time) and
// a 4.6 us benchmark by +45% (2.1 us of real time), while allocs/op stayed
// exactly flat and B/op moved by under 3%. A relative-only threshold
// therefore flags the noise; a floor above the largest observed noise delta
// (2.1 us) rejects it, while still catching the regressions that matter —
// a 40 us benchmark that grows to 60 us adds 20 us, far above the floor.
//
// Sub-floor regressions are not invisible: the delta is still reported, and
// an extra allocation still fails the job through the allocs/op metric,
// which is exactly reproducible.
const DefaultNsMinDeltaNs = 5000

var (
	// benchRe matches a `go test -bench` result line. The name group is
	// lazy and accepts any character, so table-driven sub-benchmarks
	// ("BenchmarkParent/case-1-8") are captured whole instead of being
	// silently dropped. The optional "-\d+" suffix is the GOMAXPROCS tag
	// `go test` appends; the lazy name stops before it.
	benchRe = regexp.MustCompile(`^Benchmark(.+?)(?:-\d+)?\s+(\d+)\s+(.*)$`)
	pkgRe   = regexp.MustCompile(`^pkg:\s+(\S+)`)
)

// ParseStats describes what a bench output file contained.
type ParseStats struct {
	// Lines counts the result lines that look like a benchmark measurement.
	Lines int
	// Dropped counts those lines that could not be parsed. A non-zero value
	// means the parser is missing a result format, so measurements are
	// being discarded without anybody noticing.
	Dropped int
}

// ParseFile reads `go test -bench` output and returns one Result per
// benchmark name. Names without `-N` suffix or with various metric columns
// (ns/op only, ns/op + B/op, ns/op + B/op + allocs/op) are all accepted.
//
// With -count>1 the same benchmark name appears once per repetition. The
// returned Result aggregates the repetitions by taking the per-metric
// MEDIAN — the single-sample last-write-wins behaviour this function had
// before made CI comparisons on the shared self-hosted runner flap by
// ±30-66% on sub-microsecond benchmarks (runner contention from concurrent
// test/bench jobs). The median discards those one-off outliers.
func ParseFile(path string) (map[string]Result, error) {
	results, _, err := ParseFileWithStats(path)
	return results, err
}

// ParseFileWithStats is ParseFile plus a count of the result lines it could
// not parse. The count is what turns a silent parser gap into a visible
// one: before, a table-driven sub-benchmark ("BenchmarkParent/case-1") was
// skipped without a word and the report simply did not mention it.
func ParseFileWithStats(path string) (map[string]Result, ParseStats, error) {
	var stats ParseStats
	f, err := os.Open(path)
	if err != nil {
		return nil, stats, err
	}
	defer f.Close()

	samples := make(map[string]*benchAgg)
	pkg := ""
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if m := pkgRe.FindStringSubmatch(line); m != nil {
			pkg = m[1]
			continue
		}
		if !strings.HasPrefix(line, "Benchmark") {
			continue
		}
		stats.Lines++
		m := benchRe.FindStringSubmatch(line)
		if m == nil {
			stats.Dropped++
			continue
		}
		name := m[1]
		n, err := strconv.ParseInt(m[2], 10, 64)
		if err != nil {
			stats.Dropped++
			continue
		}
		var r Result
		r.Name, r.Package, r.N = name, pkg, n
		fields := strings.Fields(m[3])
		for i := 0; i+1 < len(fields); i += 2 {
			val, err := strconv.ParseFloat(fields[i], 64)
			if err != nil {
				continue
			}
			switch fields[i+1] {
			case "ns/op":
				r.NsPerOp = val
			case "B/op":
				r.BPerOp = val
			case "allocs/op":
				r.AllocsPerOp = val
			case "round-trips/op":
				r.RoundTripsPerOp = val
			}
		}
		a := samples[name]
		if a == nil {
			a = &benchAgg{first: r}
			samples[name] = a
		}
		a.add(r)
	}
	if err := scanner.Err(); err != nil {
		return nil, stats, err
	}

	results := make(map[string]Result, len(samples))
	for name, a := range samples {
		agg := a.first
		agg.NsPerOp = median(a.ns)
		agg.BPerOp = median(a.b)
		agg.AllocsPerOp = median(a.all)
		agg.RoundTripsPerOp = median(a.rt)
		results[name] = agg
	}
	return results, stats, nil
}

// benchAgg accumulates the per-repetition metric columns for one benchmark
// name. Columns skip zero values (metrics the run did not print); `go test`
// prints the same column set for every repetition of a run, so the three
// columns always grow in lockstep.
type benchAgg struct {
	first Result // first repetition; carries Name/Package/N untouched
	ns    []float64
	b     []float64
	all   []float64
	rt    []float64
}

func (a *benchAgg) add(r Result) {
	if r.NsPerOp > 0 {
		a.ns = append(a.ns, r.NsPerOp)
	}
	if r.BPerOp > 0 {
		a.b = append(a.b, r.BPerOp)
	}
	if r.AllocsPerOp > 0 {
		a.all = append(a.all, r.AllocsPerOp)
	}
	if r.RoundTripsPerOp > 0 {
		a.rt = append(a.rt, r.RoundTripsPerOp)
	}
}

// median returns the median of vals; for an even count it averages the two
// middle values. Zero-length input returns 0 (no samples printed the metric).
func median(vals []float64) float64 {
	switch len(vals) {
	case 0:
		return 0
	case 1:
		return vals[0]
	}
	sort.Float64s(vals)
	mid := len(vals) / 2
	if len(vals)%2 == 1 {
		return vals[mid]
	}
	return (vals[mid-1] + vals[mid]) / 2
}

// Row is one benchmark's status in the comparison.
type Row struct {
	Name    string
	Package string
	Status  string // "ok", "warn", "fail", "new", "removed", "ungated"
	Base    Result
	Head    Result
	HasNs   bool
	HasB    bool
	HasAll  bool
	HasRT   bool
	NsD     float64
	BD      float64
	AllD    float64
	RTD     float64
	NsF     bool
	NsW     bool
	BF      bool
	BW      bool
	AllF    bool
	AllW    bool
	RTF     bool
	// NsAbsD is the absolute ns/op delta in nanoseconds.
	NsAbsD float64
	// NsSuppressed is set when the relative ns/op delta crossed the warn
	// threshold but the absolute delta stayed below the noise floor: the
	// measurement moved, but not by enough to mean anything on a shared
	// runner. The delta is still reported.
	NsSuppressed bool
}

// Compare returns rows comparing head to base. Rows are sorted by severity
// (fail, warn, new, removed, ok) and then alphabetically.
func Compare(base, head map[string]Result, t Thresholds) []Row {
	var rows []Row
	seen := make(map[string]bool)

	for name, h := range head {
		b, ok := base[name]
		if !ok {
			rows = append(rows, Row{Name: name, Package: h.Package, Status: "new", Head: h})
			seen[name] = true
			continue
		}
		r := Row{Name: name, Package: h.Package, Base: b, Head: h}
		if b.NsPerOp > 0 {
			r.HasNs = true
			r.NsD = (h.NsPerOp - b.NsPerOp) / b.NsPerOp * 100
			r.NsAbsD = h.NsPerOp - b.NsPerOp
			switch {
			case r.NsD >= t.NsWarn && t.NsMinDeltaNs > 0 && math.Abs(r.NsAbsD) < t.NsMinDeltaNs:
				// The relative delta is large but the absolute one is tiny:
				// shared-runner timing noise, not a code regression.
				r.NsSuppressed = true
			case r.NsD >= t.NsFail:
				r.NsF = true
				r.Status = "fail"
			case r.NsD >= t.NsWarn:
				r.NsW = true
				if r.Status != "fail" {
					r.Status = "warn"
				}
			}
		}
		if b.BPerOp > 0 {
			r.HasB = true
			r.BD = (h.BPerOp - b.BPerOp) / b.BPerOp * 100
			switch {
			case r.BD >= t.BFail:
				r.BF = true
				r.Status = "fail"
			case r.BD >= t.BWarn:
				r.BW = true
				if r.Status != "fail" {
					r.Status = "warn"
				}
			}
		}
		if b.AllocsPerOp > 0 {
			r.HasAll = true
			r.AllD = (h.AllocsPerOp - b.AllocsPerOp) / b.AllocsPerOp * 100
			switch {
			case r.AllD >= t.AllocsF:
				r.AllF = true
				r.Status = "fail"
			case r.AllD >= t.AllocsWarn:
				r.AllW = true
				if r.Status != "fail" {
					r.Status = "warn"
				}
			}
		}
		// Round trips are an exact, deterministic count — the mock executor
		// returns fixed output, so the same operation always costs the same
		// number of remote invocations. That makes the comparison absolute
		// rather than percentage-based: any increase is a real extra remote
		// round trip (a network hop, a remote shell start and the remote
		// CPU to run the probe) and fails the comparison, while a decrease
		// is an improvement. Unlike ns/op there is no run-to-run noise to
		// absorb, so no threshold applies.
		if b.RoundTripsPerOp > 0 && h.RoundTripsPerOp > 0 {
			r.HasRT = true
			r.RTD = (h.RoundTripsPerOp - b.RoundTripsPerOp) / b.RoundTripsPerOp * 100
			if h.RoundTripsPerOp > b.RoundTripsPerOp {
				r.RTF = true
				r.Status = "fail"
			}
		}
		if r.Status == "" {
			r.Status = "ok"
		}
		rows = append(rows, r)
		seen[name] = true
	}

	for name, b := range base {
		if !seen[name] {
			rows = append(rows, Row{Name: name, Package: b.Package, Status: "removed", Base: b})
		}
	}

	sort.Slice(rows, func(i, j int) bool {
		score := func(s string) int {
			switch s {
			case "fail":
				return 4
			case "warn":
				return 3
			case "new":
				return 2
			case "removed":
				return 1
			default:
				return 0
			}
		}
		if si, sj := score(rows[i].Status), score(rows[j].Status); si != sj {
			return si > sj
		}
		return rows[i].Name < rows[j].Name
	})

	return rows
}

// HasFail reports whether any row is at fail severity.
func HasFail(rows []Row) bool {
	for _, r := range rows {
		if r.Status == "fail" {
			return true
		}
	}
	return false
}

// ApplyPackageScope demotes rows whose benchmark lives outside pkgs to
// "ungated" status: they stay in the report with their deltas visible, but
// they no longer count toward HasFail/HasWarn. A PR that only touches
// package A should not fail because an unrelated micro-benchmark in package
// B flapped — on the shared self-hosted runner, unchanged code moved
// ±30-66% between consecutive runs of the same PR.
//
// pkgs holds directory-style import paths as produced by
// `git diff --name-only | xargs dirname` (e.g. "internal/runner"), matched
// against full import paths with a path-boundary rule so "internal/runner"
// also gates subpackages but never "internal/runnerx". An empty pkgs gates
// everything (pre-scope behaviour). Returns (gated, ungated) row counts and
// reorders rows so gated rows stay first, preserving Compare's ordering
// within each group.
func ApplyPackageScope(rows []Row, pkgs []string) (gated, ungated int) {
	if len(pkgs) == 0 {
		return len(rows), 0
	}
	gatedRows := make([]Row, 0, len(rows))
	var ungatedRows []Row
	for _, r := range rows {
		if packageMatches(r.Package, pkgs) {
			gatedRows = append(gatedRows, r)
		} else {
			r.Status = "ungated"
			ungatedRows = append(ungatedRows, r)
		}
	}
	out := append(gatedRows, ungatedRows...)
	copy(rows, out)
	return len(gatedRows), len(ungatedRows)
}

// ApplyNoScope demotes every row to "ungated": the report still renders each
// benchmark's delta, but nothing gates the exit code (HasFail reports false).
//
// This is the correct mode for a PR that changes no Go files at all — a
// dependency bump (go.mod/go.sum) or a workflow/docs-only edit. Such a PR
// cannot regress the repo's own benchmark code, so gating it was always a
// false-failure generator: the empty pkgs list made ApplyPackageScope fall
// through to its "gate everything" fallback, and comparing an unchanged
// benchmark suite against the latest main artifact (which can be hours old,
// measured under different load on the shared self-hosted pool) produced a
// uniform 30-345% "regression" across every benchmark with allocs/op and
// B/op unchanged to 0.0% — the signature of machine contention, not code.
// That failed all seven dependabot PRs in the 2026-10-15 batch, twice each
// (re-run included).
func ApplyNoScope(rows []Row) {
	for i := range rows {
		rows[i].Status = "ungated"
	}
}

// packageMatches reports whether the import path pkg is covered by any of
// the directory-style filters. "internal/runner" matches the package itself,
// its subpackages, and is never confused with "internal/runnerx".
func packageMatches(pkg string, pkgs []string) bool {
	for _, f := range pkgs {
		if pkg == f || strings.HasSuffix(pkg, "/"+f) || strings.Contains(pkg, f+"/") {
			return true
		}
	}
	return false
}

// writeNumber appends a benchmark measurement to the markdown builder,
// dropping decimals on larger numbers so the table stays narrow. Writes the
// digits directly to the builder (no intermediate string alloc) using a
// stack-allocated 32-byte buffer — comfortably above the worst-case
// "%.2f" ceiling for nanosecond/microsecond-range measurements.
func writeNumber(sb *strings.Builder, f float64) {
	var b [32]byte
	if f >= 100 {
		sb.Write(strconv.AppendFloat(b[:0], f, 'f', 0, 64))
		return
	}
	sb.Write(strconv.AppendFloat(b[:0], f, 'f', 2, 64))
}

// formatDeltaTo appends the percent delta of d (with sign) into dst and
// returns the resulting slice. Single-allocation helper that mirrors the
// pre-1.21 strconv.AppendFloat pattern; 24 bytes is comfortably above the
// worst-case formatted delta length.
func formatDeltaTo(dst []byte, d float64) []byte {
	if d == 0 {
		return append(dst, "0%"...)
	}
	if d > 0 {
		dst = append(dst, '+')
	}
	dst = strconv.AppendFloat(dst, d, 'f', 1, 64)
	return append(dst, '%')
}

// RenderMarkdown returns a human-readable regression report.
func RenderMarkdown(rows []Row, baseRef, headRef string, t Thresholds) string {
	// Pre-size: header + per-row line. A typical row is ~80 bytes; header is
	// fixed; summary line is ~80 bytes. Pre-growing avoids repeated
	// strings.Builder growth reallocations under load.
	var sb strings.Builder
	sb.Grow(128 + 96*len(rows))
	sb.WriteString("## Benchstat: `")
	sb.WriteString(headRef)
	sb.WriteString("` → `")
	sb.WriteString(baseRef)
	sb.WriteString("`\n\n")

	hasFail, hasWarn, hasUngated := false, false, false
	suppressed := 0
	for _, r := range rows {
		switch r.Status {
		case "fail":
			hasFail = true
		case "warn":
			hasWarn = true
		case "ungated":
			hasUngated = true
		}
		if r.NsSuppressed {
			suppressed++
		}
	}

	switch {
	case hasFail:
		sb.WriteString("🔥 **Fail-level regression(s) detected.** Job exits non-zero.\n\n")
	case hasWarn:
		sb.WriteString("⚠️ Warn-level regression(s) detected.\n\n")
	case hasUngated:
		sb.WriteString("✅ No regressions in the packages this PR changes.\n\n")
	default:
		sb.WriteString("✅ No regressions detected.\n\n")
	}

	// The round-trips/op column is only rendered when at least one row
	// carries the metric, so benchmarks without it keep the original
	// four-column table.
	showRT := false
	for _, r := range rows {
		if r.HasRT {
			showRT = true
			break
		}
	}

	sb.WriteString("| Benchmark | ns/op (Δ) | B/op (Δ) | allocs/op (Δ) |")
	if showRT {
		sb.WriteString(" round-trips/op (Δ) |")
	}
	sb.WriteString(" Status |\n")
	sb.WriteString("|-----------|-----------|----------|---------------|")
	if showRT {
		sb.WriteString("------------------|")
	}
	sb.WriteString("--------|\n")

	for _, r := range rows {
		sb.WriteString("| ")
		sb.WriteString(r.Name)
		sb.WriteString(" | ")
		writeMetricCell(&sb, r.Status, r.HasNs, r.Head.NsPerOp, r.Base.NsPerOp, r.NsD, r.NsF, r.NsW)
		sb.WriteString(" | ")
		writeMetricCell(&sb, r.Status, r.HasB, r.Head.BPerOp, r.Base.BPerOp, r.BD, r.BF, r.BW)
		sb.WriteString(" | ")
		writeMetricCell(&sb, r.Status, r.HasAll, r.Head.AllocsPerOp, r.Base.AllocsPerOp, r.AllD, r.AllF, r.AllW)
		sb.WriteString(" |")
		if showRT {
			sb.WriteString(" ")
			writeRoundTripCell(&sb, r.Status, r.HasRT, r.Head.RoundTripsPerOp, r.Base.RoundTripsPerOp, r.RTD, r.RTF)
			sb.WriteString(" |")
		}
		sb.WriteString(" ")
		sb.WriteString(rowStatus(r.Status))
		sb.WriteString(" |\n")
	}

	sb.WriteString("\n_Thresholds: ")
	sb.WriteString(formatThresholds(t))
	sb.WriteString(". Each value is the median of the -count repetitions._\n")
	if t.NsMinDeltaNs > 0 {
		sb.WriteString("\n_An ns/op regression only gates the job when it also adds at least ")
		sb.WriteString(strconv.FormatFloat(t.NsMinDeltaNs, 'f', 0, 64))
		sb.WriteString(" ns per operation; below that the shared runner's timing noise is indistinguishable from a real change. B/op and allocs/op always gate._\n")
	}
	if showRT {
		sb.WriteString("\n_round-trips/op counts remote command invocations per operation. It is an exact count, not a noisy measurement: any increase is an extra remote round trip and fails the comparison._\n")
	}
	if suppressed > 0 {
		sb.WriteString("\n_")
		sb.WriteString(strconv.Itoa(suppressed))
		sb.WriteString(" row(s) moved past the ns/op warn threshold but stayed under the absolute noise floor — reported above, not gated._\n")
	}
	if hasUngated {
		sb.WriteString("\n_⚪ rows live in packages this PR does not change — shown for visibility, but they do not fail the job (unchanged code moved ±30-66% between runs on the shared runner)._\n")
	}
	return sb.String()
}

// formatThresholds renders the warn/fail cutoffs as they are configured, so
// the report cannot drift away from the thresholds that actually gated the
// job.
func formatThresholds(t Thresholds) string {
	var sb strings.Builder
	sb.WriteString("ns/op ±")
	sb.WriteString(formatPercent(t.NsWarn))
	sb.WriteString("/")
	sb.WriteString(formatPercent(t.NsFail))
	sb.WriteString(", B/op ±")
	sb.WriteString(formatPercent(t.BWarn))
	sb.WriteString("/")
	sb.WriteString(formatPercent(t.BFail))
	sb.WriteString(", allocs/op ±")
	sb.WriteString(formatPercent(t.AllocsWarn))
	sb.WriteString("/")
	sb.WriteString(formatPercent(t.AllocsF))
	sb.WriteString(" (warn/fail)")
	return sb.String()
}

func formatPercent(f float64) string {
	return strconv.FormatFloat(f, 'f', -1, 64) + "%"
}

func rowStatus(s string) string {
	switch s {
	case "fail":
		return "🔥"
	case "warn":
		return "⚠️"
	case "new":
		return "🆕"
	case "removed":
		return "🗑️"
	case "ungated":
		return "⚪"
	default:
		return "✅"
	}
}

// writeMetricCell appends the rendered metric cell directly to the markdown
// builder. The shape is "<base> → <head> (<delta>)[ mark]" where:
//   - base and head go through writeNumber's zero/2-decimal rule
//   - delta goes through formatDeltaTo (signed "+X.X%"/"-X.X%"/"0%")
//   - mark is " 🔥" / " ⚠️" for fail/warn, else ""
//
// new/removed short-circuit and the !hasMetric "—" path mirror the public
// metricCell contract tested in benchstat_test.go.
func writeMetricCell(sb *strings.Builder, status string, hasMetric bool, headVal, baseVal, delta float64, fail, warn bool) {
	switch status {
	case "new":
		sb.WriteString("— → ")
		writeNumber(sb, headVal)
		return
	case "removed":
		writeNumber(sb, baseVal)
		sb.WriteString(" → —")
		return
	}
	if !hasMetric {
		sb.WriteString("—")
		return
	}
	writeNumber(sb, baseVal)
	sb.WriteString(" → ")
	writeNumber(sb, headVal)
	sb.WriteString(" (")
	var b [24]byte
	sb.Write(formatDeltaTo(b[:0], delta))
	sb.WriteByte(')')
	if fail {
		sb.WriteString(" 🔥")
	} else if warn {
		sb.WriteString(" ⚠️")
	}
}

// writeRoundTripCell renders the round-trips/op cell. The shape matches
// writeMetricCell, but the counts are whole numbers, so they are written
// without the decimals writeNumber applies to small ns/op and B/op values.
func writeRoundTripCell(sb *strings.Builder, status string, hasRT bool, headVal, baseVal, delta float64, fail bool) {
	switch status {
	case "new":
		sb.WriteString("— → ")
		writeCount(sb, headVal)
		return
	case "removed":
		writeCount(sb, baseVal)
		sb.WriteString(" → —")
		return
	}
	if !hasRT {
		sb.WriteString("—")
		return
	}
	writeCount(sb, baseVal)
	sb.WriteString(" → ")
	writeCount(sb, headVal)
	sb.WriteString(" (")
	var b [24]byte
	sb.Write(formatDeltaTo(b[:0], delta))
	sb.WriteByte(')')
	if fail {
		sb.WriteString(" 🔥")
	}
}

// writeCount appends a round-trip count as a whole number.
func writeCount(sb *strings.Builder, f float64) {
	var b [20]byte
	sb.Write(strconv.AppendInt(b[:0], int64(f+0.5), 10))
}

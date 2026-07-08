// Package benchstat parses two `go test -bench=.` outputs and produces a
// markdown regression report.
//
// Used by .github/workflows/bench-compare.yml to surface `ns/op`, `B/op` and
// `allocs/op` regressions on pull requests. Stdlib only, no module deps.
package benchstat

import (
	"bufio"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// Result is a single benchmark measurement.
type Result struct {
	Name        string
	N           int64
	NsPerOp     float64
	BPerOp      float64
	AllocsPerOp float64
}

// Thresholds defines warn/fail percent-delta cutoffs per metric.
type Thresholds struct {
	NsWarn, NsFail      float64
	BWarn, BFail        float64
	AllocsWarn, AllocsF float64
}

// DefaultThresholds returns the warn/fail cutoffs used by the bench-compare
// workflow. The numbers are intentionally conservative: real refactors should
// stay well below them, while genuine regressions (e.g. +50% ns/op) fail.
func DefaultThresholds() Thresholds {
	return Thresholds{
		NsWarn: 10, NsFail: 30,
		BWarn: 15, BFail: 50,
		AllocsWarn: 10, AllocsF: 25,
	}
}

var benchRe = regexp.MustCompile(`^Benchmark(\w+)(?:-\d+)?\s+(\d+)\s+(.*)$`)

// ParseFile reads `go test -bench` output and returns one Result per
// benchmark name. Names without `-N` suffix or with various metric columns
// (ns/op only, ns/op + B/op, ns/op + B/op + allocs/op) are all accepted.
func ParseFile(path string) (map[string]Result, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	results := make(map[string]Result)
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		m := benchRe.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		name := m[1]
		n, err := strconv.ParseInt(m[2], 10, 64)
		if err != nil {
			continue
		}
		r := Result{Name: name, N: n}
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
			}
		}
		results[name] = r
	}
	return results, scanner.Err()
}

// Row is one benchmark's status in the comparison.
type Row struct {
	Name   string
	Status string // "ok", "warn", "fail", "new", "removed"
	Base   Result
	Head   Result
	HasNs  bool
	HasB   bool
	HasAll bool
	NsD    float64
	BD     float64
	AllD   float64
	NsF    bool
	NsW    bool
	BF     bool
	BW     bool
	AllF   bool
	AllW   bool
}

// Compare returns rows comparing head to base. Rows are sorted by severity
// (fail, warn, new, removed, ok) and then alphabetically.
func Compare(base, head map[string]Result, t Thresholds) []Row {
	var rows []Row
	seen := make(map[string]bool)

	for name, h := range head {
		b, ok := base[name]
		if !ok {
			rows = append(rows, Row{Name: name, Status: "new", Head: h})
			seen[name] = true
			continue
		}
		r := Row{Name: name, Base: b, Head: h}
		if b.NsPerOp > 0 {
			r.HasNs = true
			r.NsD = (h.NsPerOp - b.NsPerOp) / b.NsPerOp * 100
			switch {
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
		if r.Status == "" {
			r.Status = "ok"
		}
		rows = append(rows, r)
		seen[name] = true
	}

	for name, b := range base {
		if !seen[name] {
			rows = append(rows, Row{Name: name, Status: "removed", Base: b})
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

// FormatNumber renders a benchmark measurement, dropping decimals on larger
// numbers so the markdown table stays narrow.
func FormatNumber(f float64) string {
	if f >= 100 {
		return strconv.FormatFloat(f, 'f', 0, 64)
	}
	return strconv.FormatFloat(f, 'f', 2, 64)
}

// FormatDelta renders a percent delta with sign.
func FormatDelta(d float64) string {
	switch {
	case d > 0:
		return fmt.Sprintf("+%.1f%%", d)
	case d < 0:
		return fmt.Sprintf("%.1f%%", d)
	default:
		return "0%"
	}
}

// RenderMarkdown returns a human-readable regression report.
func RenderMarkdown(rows []Row, baseRef, headRef string) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "## Benchstat: `%s` → `%s`\n\n", headRef, baseRef)

	hasFail, hasWarn := false, false
	for _, r := range rows {
		if r.Status == "fail" {
			hasFail = true
		}
		if r.Status == "warn" {
			hasWarn = true
		}
	}

	switch {
	case hasFail:
		sb.WriteString("🔥 **Fail-level regression(s) detected.** Job exits non-zero.\n\n")
	case hasWarn:
		sb.WriteString("⚠️ Warn-level regression(s) detected.\n\n")
	default:
		sb.WriteString("✅ No regressions detected.\n\n")
	}

	sb.WriteString("| Benchmark | ns/op (Δ) | B/op (Δ) | allocs/op (Δ) | Status |\n")
	sb.WriteString("|-----------|-----------|----------|---------------|--------|\n")

	for _, r := range rows {
		status := rowStatus(r.Status)
		nsCol := metricCell(r, r.HasNs, r.Head.NsPerOp, r.Base.NsPerOp, r.NsD, r.NsF, r.NsW)
		bCol := metricCell(r, r.HasB, r.Head.BPerOp, r.Base.BPerOp, r.BD, r.BF, r.BW)
		allCol := metricCell(r, r.HasAll, r.Head.AllocsPerOp, r.Base.AllocsPerOp, r.AllD, r.AllF, r.AllW)
		fmt.Fprintf(&sb, "| %s | %s | %s | %s | %s |\n", r.Name, nsCol, bCol, allCol, status)
	}

	sb.WriteString("\n_Thresholds: ns/op ±10%/30%, B/op ±15%/50%, allocs/op ±10%/25% (warn/fail)._\n")
	return sb.String()
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
	default:
		return "✅"
	}
}

func metricCell(r Row, hasMetric bool, headVal, baseVal, delta float64, fail, warn bool) string {
	switch r.Status {
	case "new":
		return fmt.Sprintf("— → %s", FormatNumber(headVal))
	case "removed":
		return fmt.Sprintf("%s → —", FormatNumber(baseVal))
	}
	if !hasMetric {
		return "—"
	}
	mark := ""
	if fail {
		mark = " 🔥"
	} else if warn {
		mark = " ⚠️"
	}
	return fmt.Sprintf("%s → %s (%s)%s",
		FormatNumber(baseVal),
		FormatNumber(headVal),
		FormatDelta(delta),
		mark)
}

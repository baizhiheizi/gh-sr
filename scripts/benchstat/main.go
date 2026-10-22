//go:build ignore

// Command benchstat diffs two `go test -bench=.` outputs and renders a
// markdown regression report.
//
// Usage:
//
//	go run scripts/benchstat/main.go -base bench-main.txt -head bench-pr.txt \
//	    -base-ref main -head-ref pr -output bench-diff.md \
//	    -packages internal/runner,internal/host
//
// Pass -gate=false for a report-only comparison, for a PR that changes no Go
// files (dependency bump, workflow/docs-only edit) and therefore cannot
// regress the repo's own benchmark code.
//
// Exit codes:
//
//	0 — no fail-level regressions (warn-level OK)
//	1 — at least one fail-level regression
//	2 — usage or I/O error
package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/an-lee/gh-sr/scripts/benchstat"
)

func main() {
	basePath := flag.String("base", "", "path to baseline bench output")
	headPath := flag.String("head", "", "path to PR bench output")
	baseRef := flag.String("base-ref", "main", "base ref label")
	headRef := flag.String("head-ref", "PR", "head ref label")
	output := flag.String("output", "", "optional output file (default: stdout)")
	packages := flag.String("packages", "", "comma-separated directory-style import paths (e.g. internal/runner) whose benchmarks gate the exit code; empty gates everything")
	gate := flag.Bool("gate", true, "gate the exit code on fail-level regressions; false renders the report but never fails (for diffs that change no Go files)")
	nsMinDelta := flag.Float64("ns-min-delta", benchstat.DefaultNsMinDeltaNs, "minimum absolute ns/op delta (ns) for an ns/op regression to gate the job; below it the delta is reported but not gated (shared-runner timing noise)")
	flag.Parse()

	if *basePath == "" || *headPath == "" {
		fmt.Fprintln(os.Stderr, "usage: benchstat -base <file> -head <file> [-base-ref LABEL] [-head-ref LABEL] [-output FILE]")
		os.Exit(2)
	}

	base, baseStats, err := benchstat.ParseFileWithStats(*basePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error reading base: %v\n", err)
		os.Exit(2)
	}
	head, headStats, err := benchstat.ParseFileWithStats(*headPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error reading head: %v\n", err)
		os.Exit(2)
	}
	reportDropped("base", baseStats)
	reportDropped("head", headStats)

	thresholds := benchstat.DefaultThresholds()
	thresholds.NsMinDeltaNs = *nsMinDelta

	rows := benchstat.Compare(base, head, thresholds)
	var pkgs []string
	if *packages != "" {
		for _, p := range strings.Split(*packages, ",") {
			if p = strings.TrimSpace(p); p != "" {
				pkgs = append(pkgs, p)
			}
		}
	}
	if *gate {
		gated, ungated := benchstat.ApplyPackageScope(rows, pkgs)
		fmt.Fprintf(os.Stderr, "scope: gating %d benchmark(s) in changed packages, %d ungated\n", gated, ungated)
	} else {
		benchstat.ApplyNoScope(rows)
		fmt.Fprintf(os.Stderr, "scope: gating disabled (diff changes no Go files); report-only, exit code forced to 0\n")
	}
	md := benchstat.RenderMarkdown(rows, *baseRef, *headRef, thresholds)

	if *output != "" {
		if err := os.WriteFile(*output, []byte(md), 0o644); err != nil {
			fmt.Fprintf(os.Stderr, "error writing output: %v\n", err)
			os.Exit(2)
		}
	} else {
		fmt.Print(md)
	}

	if benchstat.HasFail(rows) {
		os.Exit(1)
	}
}

// reportDropped surfaces benchmark result lines the parser could not read.
// A non-zero count means measurements are being discarded, which used to
// happen silently: table-driven sub-benchmarks ("BenchmarkParent/case-1")
// were skipped without a word, so the gate never saw them.
func reportDropped(label string, stats benchstat.ParseStats) {
	if stats.Dropped == 0 {
		return
	}
	fmt.Fprintf(os.Stderr, "warning: %s: dropped %d unparsable benchmark result line(s) out of %d; those measurements are NOT compared\n", label, stats.Dropped, stats.Lines)
}

//go:build ignore

// Command benchstat diffs two `go test -bench=.` outputs and renders a
// markdown regression report.
//
// Usage:
//
//	go run scripts/benchstat/main.go -base bench-main.txt -head bench-pr.txt \
//	    -base-ref main -head-ref pr -output bench-diff.md
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

	"github.com/an-lee/gh-sr/scripts/benchstat"
)

func main() {
	basePath := flag.String("base", "", "path to baseline bench output")
	headPath := flag.String("head", "", "path to PR bench output")
	baseRef := flag.String("base-ref", "main", "base ref label")
	headRef := flag.String("head-ref", "PR", "head ref label")
	output := flag.String("output", "", "optional output file (default: stdout)")
	flag.Parse()

	if *basePath == "" || *headPath == "" {
		fmt.Fprintln(os.Stderr, "usage: benchstat -base <file> -head <file> [-base-ref LABEL] [-head-ref LABEL] [-output FILE]")
		os.Exit(2)
	}

	base, err := benchstat.ParseFile(*basePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error reading base: %v\n", err)
		os.Exit(2)
	}
	head, err := benchstat.ParseFile(*headPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error reading head: %v\n", err)
		os.Exit(2)
	}

	rows := benchstat.Compare(base, head, benchstat.DefaultThresholds())
	md := benchstat.RenderMarkdown(rows, *baseRef, *headRef)

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

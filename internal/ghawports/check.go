package ghawports

import (
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/an-lee/gh-sr/internal/config"
	"github.com/an-lee/gh-sr/internal/ghawfrontmatter"
)

// CheckOpts configures MCP port lint against gh-sr config and workflow sources.
type CheckOpts struct {
	WorkflowRoot string
	RepoFilter   string // optional: owner/repo to select runners
}

// Check writes human-readable findings to w. Returns counts of warnings and failures.
func Check(w io.Writer, cfg *config.Config, opts CheckOpts) (warns, fails int) {
	if cfg == nil {
		fmt.Fprintln(w, "WARN: no config loaded; only scanning workflow markdown")
		warns++
	}
	root := opts.WorkflowRoot
	if root == "" {
		root = "."
	}
	docs, err := ghawfrontmatter.ScanMarkdownWorkflows(root)
	if err != nil {
		fmt.Fprintf(w, "FAIL: scan workflows: %v\n", err)
		fails++
		return warns, fails
	}
	if len(docs) == 0 {
		fmt.Fprintf(w, "OK: no agentic-style workflow markdown under %s/.github/workflows\n", root)
		return warns, fails
	}

	portUsers := map[int][]string{}
	for _, d := range docs {
		p := d.EffectiveMCPPort()
		portUsers[p] = append(portUsers[p], d.Path)
	}

	var ports []int
	for p := range portUsers {
		ports = append(ports, p)
	}
	sort.Ints(ports)
	for _, p := range ports {
		paths := portUsers[p]
		if len(paths) > 1 {
			fmt.Fprintf(w, "WARN: MCP port %d used by %d workflows (concurrent jobs on one host may conflict):\n", p, len(paths))
			for _, path := range paths {
				fmt.Fprintf(w, "      - %s\n", path)
			}
			warns++
		}
	}

	if cfg != nil {
		warns += checkRunnerConcurrency(w, cfg, opts.RepoFilter, docs)
	}

	fmt.Fprintf(w, "OK: scanned %d agentic-style workflow(s) under %s/.github/workflows\n", len(docs), root)
	return warns, fails
}

func checkRunnerConcurrency(w io.Writer, cfg *config.Config, repoFilter string, docs []*ghawfrontmatter.Doc) (warns int) {
	byHost := map[string]int{}

	for i := range cfg.Runners {
		r := &cfg.Runners[i]
		if !r.IsAgentic() {
			continue
		}
		if repoFilter != "" && r.Repo != repoFilter {
			continue
		}
		byHost[r.Host] += r.InstanceCount()
	}

	selfHostedDocs := 0
	uniquePorts := map[int]bool{}
	for _, d := range docs {
		if d.HasSelfHostedRunsOn() {
			selfHostedDocs++
			uniquePorts[d.EffectiveMCPPort()] = true
		}
	}

	for host, n := range byHost {
		if n <= 1 {
			continue
		}
		if selfHostedDocs == 0 {
			continue
		}
		if len(uniquePorts) < n {
			fmt.Fprintf(w, "WARN: host %q has %d concurrent agentic runner instance(s) but only %d distinct MCP port(s) in self-hosted workflow frontmatter (need >= %d for parallel jobs)\n",
				host, n, len(uniquePorts), n)
			fmt.Fprintf(w, "      Same compiled workflow on one host still shares one port; duplicate workflow sources or limit concurrency.\n")
			warns++
		}
	}

	if repoFilter == "" {
		return warns
	}
	portSet := map[int]bool{}
	for i := range cfg.Runners {
		r := &cfg.Runners[i]
		if r.Repo != repoFilter || !r.IsAgentic() {
			continue
		}
		ports, ok := r.AgenticMCPPortsResolved()
		if !ok {
			continue
		}
		for _, p := range ports {
			portSet[p] = true
		}
	}
	if len(portSet) == 0 {
		return warns
	}
	for _, d := range docs {
		if !d.HasSelfHostedRunsOn() {
			continue
		}
		p := d.EffectiveMCPPort()
		if !portSet[p] {
			continue
		}
		if !d.HasMCPLabel(config.AgenticMCPLabelPrefix, p) {
			fmt.Fprintf(w, "WARN: %s uses MCP port %d with self-hosted runs-on; runners for repo %s use gh-sr MCP labels — add label %q to runs-on\n",
				d.Path, p, repoFilter, fmt.Sprintf("%s%d", config.AgenticMCPLabelPrefix, p))
			warns++
		}
	}

	return warns
}

// SuggestRunsOnSnippet prints a YAML snippet for runs-on including MCP label.
func SuggestRunsOnSnippet(w io.Writer, existing []string, port int) {
	label := fmt.Sprintf("%s%d", config.AgenticMCPLabelPrefix, port)
	var parts []string
	seen := map[string]bool{}
	for _, x := range existing {
		s := strings.TrimSpace(x)
		if s == "" {
			continue
		}
		if strings.HasPrefix(strings.ToLower(s), strings.ToLower(config.AgenticMCPLabelPrefix)) {
			continue
		}
		if !seen[s] {
			seen[s] = true
			parts = append(parts, s)
		}
	}
	parts = append(parts, label)
	fmt.Fprintf(w, "runs-on:\n")
	for _, p := range parts {
		fmt.Fprintf(w, "  - %s\n", p)
	}
}

package ghawports

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"

	"github.com/an-lee/gh-sr/internal/ghawfrontmatter"
)

// AssignOpts configures deterministic port assignment.
type AssignOpts struct {
	WorkflowRoot string
	BasePort     int
	Write        bool
}

// Assign assigns sandbox.mcp.port = base+i to each agentic-style workflow markdown (sorted by path).
// When Write is false, prints planned changes only.
func Assign(w io.Writer, opts AssignOpts) error {
	if opts.BasePort < 1 {
		return fmt.Errorf("base port must be >= 1")
	}
	root := opts.WorkflowRoot
	if root == "" {
		root = "."
	}
	docs, err := ghawfrontmatter.ScanMarkdownWorkflows(root)
	if err != nil {
		return err
	}
	if len(docs) == 0 {
		fmt.Fprintf(w, "no agentic-style workflow markdown under %s/.github/workflows\n", root)
		return nil
	}
	sort.Slice(docs, func(i, j int) bool { return docs[i].Path < docs[j].Path })
	for i, d := range docs {
		port := opts.BasePort + i
		if port > 65535 {
			return fmt.Errorf("port %d exceeds 65535", port)
		}
		data, err := os.ReadFile(d.Path)
		if err != nil {
			return err
		}
		newData, err := ghawfrontmatter.ApplyMCPPortPatch(data, port)
		if err != nil {
			return fmt.Errorf("%s: %w", d.Path, err)
		}
		rel, _ := filepath.Rel(root, d.Path)
		if string(newData) == string(data) {
			fmt.Fprintf(w, "unchanged: %s (already port %d)\n", rel, port)
			continue
		}
		if opts.Write {
			if err := os.WriteFile(d.Path, newData, 0o644); err != nil {
				return fmt.Errorf("write %s: %w", d.Path, err)
			}
			fmt.Fprintf(w, "wrote: %s -> sandbox.mcp.port %d\n", rel, port)
		} else {
			fmt.Fprintf(w, "would update: %s -> sandbox.mcp.port %d (dry-run; use --write)\n", rel, port)
		}
	}
	return nil
}

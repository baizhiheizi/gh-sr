package ghawports

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/an-lee/gh-sr/internal/config"
)

func TestCheck_duplicatePorts(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	wf := filepath.Join(dir, ".github", "workflows")
	if err := os.MkdirAll(wf, 0o755); err != nil {
		t.Fatal(err)
	}
	aw := "---\non: issues\ntools:\n  x: true\n---\n"
	if err := os.WriteFile(filepath.Join(wf, "a.md"), []byte(aw), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(wf, "b.md"), []byte(aw), 0o644); err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	warns, fails := Check(&buf, nil, CheckOpts{WorkflowRoot: dir})
	if fails != 0 {
		t.Fatalf("fails=%d out=%s", fails, buf.String())
	}
	if warns == 0 {
		t.Fatalf("expected duplicate port warning, out=%s", buf.String())
	}
}

func TestCheck_runnerConcurrencyWarn(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	wf := filepath.Join(dir, ".github", "workflows")
	if err := os.MkdirAll(wf, 0o755); err != nil {
		t.Fatal(err)
	}
	md := "---\non: issues\ntools:\n  x: true\nruns-on:\n  - self-hosted\n  - Linux\nsandbox:\n  mcp:\n    port: 9099\n---\n"
	if err := os.WriteFile(filepath.Join(wf, "w.md"), []byte(md), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := &config.Config{
		Hosts: map[string]config.HostConfig{"h": {Addr: "local", OS: "linux", Arch: "amd64"}},
		Runners: []config.RunnerConfig{{
			Name: "r", Repo: "o/r", Host: "h", Profile: "agentic", Count: 3,
		}},
	}
	var buf bytes.Buffer
	warns, fails := Check(&buf, cfg, CheckOpts{WorkflowRoot: dir, RepoFilter: "o/r"})
	if fails != 0 {
		t.Fatal(buf.String())
	}
	if warns == 0 {
		t.Fatalf("expected warn, out=%s", buf.String())
	}
}

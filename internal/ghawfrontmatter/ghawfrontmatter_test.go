package ghawfrontmatter

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSplitYAMLFrontmatter(t *testing.T) {
	t.Parallel()
	src := []byte("---\non: issues\nfoo: bar\n---\n\n# Body\n")
	yamlDoc, rest, ok := SplitYAMLFrontmatter(src)
	if !ok {
		t.Fatal("expected ok")
	}
	if !bytes.Equal(yamlDoc, []byte("on: issues\nfoo: bar")) {
		t.Fatalf("yaml: %q", yamlDoc)
	}
	if !bytes.HasPrefix(rest, []byte("\n# Body")) {
		t.Fatalf("rest: %q", rest)
	}
}

func TestApplyMCPPortPatch(t *testing.T) {
	t.Parallel()
	src := []byte("---\non: issues\nnetwork:\n  firewall: true\n---\n\nHello\n")
	out, err := ApplyMCPPortPatch(src, 9082)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.HasPrefix(out, []byte("---\n")) {
		t.Fatalf("prefix: %q", out[:20])
	}
	if !bytes.Contains(out, []byte("mcp-gateway: true")) {
		t.Fatalf("missing mcp-gateway: %s", out)
	}
	if !bytes.Contains(out, []byte("port: 9082")) {
		t.Fatalf("missing port: %s", out)
	}
	if !bytes.Contains(out, []byte("Hello")) {
		t.Fatalf("missing body")
	}
}

func TestApplyMCPPortPatch_preservesUnrelatedYAML(t *testing.T) {
	t.Parallel()
	src := []byte("---\non:\n  schedule: daily\ntools:\n  github:\n    toolsets: [default]\nsafe-outputs:\n  create-pull-request:\n    title-prefix: \"[ci-coach] \"\nsandbox:\n  mcp:\n    port: 80\n---\n\n# body\n")
	out, err := ApplyMCPPortPatch(src, 9080)
	if err != nil {
		t.Fatal(err)
	}
	s := string(out)
	if !strings.Contains(s, "toolsets: [default]") {
		t.Fatalf("expected flow sequence preserved under tools, got:\n%s", s)
	}
	if !strings.Contains(s, `title-prefix: "[ci-coach] "`) {
		t.Fatalf("expected title-prefix quoting preserved, got:\n%s", s)
	}
	if !strings.Contains(s, "on:\n  schedule: daily") {
		t.Fatalf("expected on block unchanged, got:\n%s", s)
	}
	if !strings.Contains(s, "mcp-gateway: true") || !strings.Contains(s, "port: 9080") {
		t.Fatalf("expected patch fields, got:\n%s", s)
	}
}

func TestApplyMCPPortPatch_errorsNonMappingFeatures(t *testing.T) {
	t.Parallel()
	src := []byte("---\nfeatures: []\non: issues\ntools:\n  bash: true\n---\n")
	_, err := ApplyMCPPortPatch(src, 9080)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "features") {
		t.Fatalf("error: %v", err)
	}
}

func TestApplyMCPPortPatch_errorsNonMappingSandbox(t *testing.T) {
	t.Parallel()
	src := []byte("---\non: issues\ntools:\n  bash: true\nsandbox: not-a-map\n---\n")
	_, err := ApplyMCPPortPatch(src, 9080)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "sandbox") {
		t.Fatalf("error: %v", err)
	}
}

func TestApplyMCPPortPatch_errorsNonMappingMCP(t *testing.T) {
	t.Parallel()
	src := []byte("---\non: issues\ntools:\n  bash: true\nsandbox:\n  mcp: 123\n---\n")
	_, err := ApplyMCPPortPatch(src, 9080)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "mcp") {
		t.Fatalf("error: %v", err)
	}
}

func TestParseDoc_runsOnAndPort(t *testing.T) {
	t.Parallel()
	raw := []byte("---\non: issues\nruns-on:\n  - self-hosted\n  - Linux\nsandbox:\n  mcp:\n    port: 9100\n---\n# x\n")
	d, err := ParseDoc("x.md", raw)
	if err != nil {
		t.Fatal(err)
	}
	if d.SandboxMCPPort == nil || *d.SandboxMCPPort != 9100 {
		t.Fatalf("port: %+v", d.SandboxMCPPort)
	}
	rs := d.RunsOnStrings()
	if len(rs) != 2 || rs[0] != "self-hosted" || rs[1] != "Linux" {
		t.Fatalf("runs-on: %v", rs)
	}
	if d.HasMCPLabel("gh-sr-mcp-", 9100) {
		t.Fatal("expected no gh-sr-mcp label in runs-on")
	}
}

func TestScanMarkdownWorkflows_skipsNonAW(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	wf := filepath.Join(dir, ".github", "workflows")
	if err := os.MkdirAll(wf, 0o755); err != nil {
		t.Fatal(err)
	}
	// No "on" key — skipped
	if err := os.WriteFile(filepath.Join(wf, "readme.md"), []byte("---\nfoo: bar\n---\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	docs, err := ScanMarkdownWorkflows(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(docs) != 0 {
		t.Fatalf("got %d docs", len(docs))
	}
}

func TestScanMarkdownWorkflows_findsAW(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	wf := filepath.Join(dir, ".github", "workflows")
	if err := os.MkdirAll(wf, 0o755); err != nil {
		t.Fatal(err)
	}
	md := []byte("---\non: issues\ntools:\n  bash: true\n---\n# Prompt\n")
	if err := os.WriteFile(filepath.Join(wf, "w.md"), md, 0o644); err != nil {
		t.Fatal(err)
	}
	docs, err := ScanMarkdownWorkflows(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(docs) != 1 {
		t.Fatalf("got %d", len(docs))
	}
	if !strings.HasSuffix(docs[0].Path, "w.md") {
		t.Fatalf("path %s", docs[0].Path)
	}
}

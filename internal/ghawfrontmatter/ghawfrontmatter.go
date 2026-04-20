// Package ghawfrontmatter parses GitHub Agentic Workflows markdown frontmatter
// (YAML between --- fences) for MCP gateway and runs-on settings.
package ghawfrontmatter

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// Doc describes one .github/workflows/*.md AW source file.
type Doc struct {
	Path string

	// RunsOn is the raw runs-on value (string or list of strings).
	RunsOn interface{}
	// RunsOnSlim is runs-on-slim if set.
	RunsOnSlim interface{}

	MCPGatewayFeature *bool
	SandboxMCPPort    *int

	// RawYAML is the frontmatter YAML bytes (excluding outer --- lines).
	RawYAML []byte
}

// EffectiveMCPPort returns the TCP port the compiled workflow would use for the MCP gateway.
// When sandbox.mcp.port is omitted, gh-aw defaults to 80.
func (d *Doc) EffectiveMCPPort() int {
	if d.SandboxMCPPort != nil && *d.SandboxMCPPort > 0 {
		return *d.SandboxMCPPort
	}
	return 80
}

// RunsOnStrings normalizes runs-on to a list of label tokens for matching.
func (d *Doc) RunsOnStrings() []string {
	return normalizeRunsOn(d.RunsOn)
}

// HasSelfHostedRunsOn reports whether runs-on mentions self-hosted routing.
func (d *Doc) HasSelfHostedRunsOn() bool {
	for _, s := range d.RunsOnStrings() {
		if strings.EqualFold(s, "self-hosted") {
			return true
		}
	}
	return false
}

// HasMCPLabel reports whether runs-on includes the gh-sr MCP routing label for port p.
func (d *Doc) HasMCPLabel(prefix string, port int) bool {
	want := fmt.Sprintf("%s%d", prefix, port)
	for _, s := range d.RunsOnStrings() {
		if strings.EqualFold(s, want) {
			return true
		}
	}
	return false
}

// IsLikelyAgenticMarkdown is a lightweight heuristic: YAML has "on" (triggers) and
// at least one of network, tools, safe-outputs, sandbox, engine — typical gh-aw docs.
func (d *Doc) IsLikelyAgenticMarkdown(root map[string]interface{}) bool {
	if root == nil {
		return false
	}
	if _, ok := root["on"]; !ok {
		return false
	}
	for _, k := range []string{"network", "tools", "safe-outputs", "sandbox", "engine"} {
		if _, ok := root[k]; ok {
			return true
		}
	}
	return false
}

// SplitYAMLFrontmatter returns the YAML document between the first pair of --- lines
// and the remainder of the file (markdown body).
func SplitYAMLFrontmatter(data []byte) (yamlDoc, rest []byte, ok bool) {
	if !bytes.HasPrefix(data, []byte("---")) {
		return nil, nil, false
	}
	i := 3
	if i < len(data) && data[i] == '\r' {
		i++
	}
	if i < len(data) && data[i] == '\n' {
		i++
	}
	restMarker := []byte("\n---")
	idx := bytes.Index(data[i:], restMarker)
	if idx < 0 {
		return nil, nil, false
	}
	end := i + idx
	yamlDoc = bytes.TrimSpace(data[i:end])
	restStart := end + len(restMarker)
	if restStart < len(data) && data[restStart] == '\r' {
		restStart++
	}
	if restStart < len(data) && data[restStart] == '\n' {
		restStart++
	}
	rest = data[restStart:]
	return yamlDoc, rest, true
}

// ParseDoc reads path and parses frontmatter into Doc.
func ParseDoc(path string, data []byte) (*Doc, error) {
	yamlBytes, _, ok := SplitYAMLFrontmatter(data)
	if !ok {
		return nil, fmt.Errorf("%s: no YAML frontmatter found", path)
	}
	var root map[string]interface{}
	if err := yaml.Unmarshal(yamlBytes, &root); err != nil {
		return nil, fmt.Errorf("%s: frontmatter YAML: %w", path, err)
	}
	d := &Doc{
		Path:    path,
		RawYAML: append([]byte(nil), yamlBytes...),
	}
	if v, ok := root["runs-on"]; ok {
		d.RunsOn = v
	}
	if v, ok := root["runs-on-slim"]; ok {
		d.RunsOnSlim = v
	}
	if feat, ok := root["features"].(map[string]interface{}); ok {
		if v, ok := feat["mcp-gateway"]; ok {
			switch t := v.(type) {
			case bool:
				d.MCPGatewayFeature = &t
			case string:
				low := strings.ToLower(strings.TrimSpace(t))
				if low == "true" || low == "yes" || low == "1" {
					b := true
					d.MCPGatewayFeature = &b
				}
				if low == "false" || low == "no" || low == "0" {
					b := false
					d.MCPGatewayFeature = &b
				}
			}
		}
	}
	if sb, ok := root["sandbox"].(map[string]interface{}); ok {
		if mcp, ok := sb["mcp"].(map[string]interface{}); ok {
			if p, ok := mcp["port"]; ok {
				switch t := p.(type) {
				case int:
					d.SandboxMCPPort = &t
				case int64:
					v := int(t)
					d.SandboxMCPPort = &v
				case float64:
					v := int(t)
					d.SandboxMCPPort = &v
				}
			}
		}
	}
	return d, nil
}

// ScanMarkdownWorkflows returns parsed docs for every *.md under root/.github/workflows.
func ScanMarkdownWorkflows(root string) ([]*Doc, error) {
	dir := filepath.Join(root, ".github", "workflows")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read workflows dir %q: %w", dir, err)
	}
	var paths []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if strings.EqualFold(filepath.Ext(e.Name()), ".md") {
			paths = append(paths, filepath.Join(dir, e.Name()))
		}
	}
	sort.Strings(paths)
	var out []*Doc
	for _, p := range paths {
		data, err := os.ReadFile(p)
		if err != nil {
			return nil, err
		}
		d, err := ParseDoc(p, data)
		if err != nil {
			// Skip non-frontmatter markdown (e.g. templates without ---).
			continue
		}
		var root map[string]interface{}
		_ = yaml.Unmarshal(d.RawYAML, &root)
		if !d.IsLikelyAgenticMarkdown(root) {
			continue
		}
		out = append(out, d)
	}
	return out, nil
}

// ApplyMCPPortPatch rewrites the YAML frontmatter to set sandbox.mcp.port only.
// It does not modify features.mcp-gateway (see upstream gh-aw Sandbox docs).
// The markdown body after the closing --- is preserved.
// It updates the YAML AST in place so unrelated keys keep order and style.
func ApplyMCPPortPatch(data []byte, port int) ([]byte, error) {
	if port < 1 || port > 65535 {
		return nil, fmt.Errorf("port must be 1..65535")
	}
	yamlDoc, rest, ok := SplitYAMLFrontmatter(data)
	if !ok {
		return nil, fmt.Errorf("no YAML frontmatter found")
	}
	var doc yaml.Node
	if err := yaml.Unmarshal(yamlDoc, &doc); err != nil {
		return nil, err
	}
	if doc.Kind != yaml.DocumentNode || len(doc.Content) == 0 {
		return nil, fmt.Errorf("frontmatter: empty YAML document")
	}
	top := doc.Content[0]
	if top.Kind != yaml.MappingNode {
		return nil, fmt.Errorf("frontmatter root must be a YAML mapping")
	}
	sandbox, err := getOrCreateMapping(top, "sandbox")
	if err != nil {
		return nil, fmt.Errorf("sandbox: %w", err)
	}
	mcp, err := getOrCreateMapping(sandbox, "mcp")
	if err != nil {
		return nil, fmt.Errorf("sandbox.mcp: %w", err)
	}
	setScalarKey(mcp, "port", &yaml.Node{
		Kind:  yaml.ScalarNode,
		Tag:   "!!int",
		Value: strconv.Itoa(port),
	})

	var buf bytes.Buffer
	buf.WriteString("---\n")
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(&doc); err != nil {
		return nil, err
	}
	if err := enc.Close(); err != nil {
		return nil, err
	}
	out := bytes.TrimSuffix(buf.Bytes(), []byte("\n"))
	var wb bytes.Buffer
	wb.Write(out)
	wb.WriteString("\n---\n")
	wb.Write(rest)
	return wb.Bytes(), nil
}

func findMapValueNode(mapping *yaml.Node, key string) *yaml.Node {
	for i := 0; i+1 < len(mapping.Content); i += 2 {
		k := mapping.Content[i]
		if k.Kind == yaml.ScalarNode && k.Value == key {
			return mapping.Content[i+1]
		}
	}
	return nil
}

// getOrCreateMapping returns the mapping under key, creating an empty mapping if absent.
// If the key exists but is not a mapping, it returns an error.
func getOrCreateMapping(mapping *yaml.Node, key string) (*yaml.Node, error) {
	if mapping.Kind != yaml.MappingNode {
		return nil, fmt.Errorf("expected a mapping")
	}
	v := findMapValueNode(mapping, key)
	if v == nil {
		newMap := &yaml.Node{Kind: yaml.MappingNode}
		mapping.Content = append(mapping.Content,
			&yaml.Node{Kind: yaml.ScalarNode, Value: key},
			newMap,
		)
		return newMap, nil
	}
	if v.Kind != yaml.MappingNode {
		return nil, fmt.Errorf("%q is not a mapping (kind %v)", key, v.Kind)
	}
	return v, nil
}

// setScalarKey sets or replaces key with the given scalar value node.
func setScalarKey(mapping *yaml.Node, key string, scalar *yaml.Node) {
	for i := 0; i+1 < len(mapping.Content); i += 2 {
		if mapping.Content[i].Kind == yaml.ScalarNode && mapping.Content[i].Value == key {
			mapping.Content[i+1] = scalar
			return
		}
	}
	mapping.Content = append(mapping.Content,
		&yaml.Node{Kind: yaml.ScalarNode, Value: key},
		scalar,
	)
}

func normalizeRunsOn(v interface{}) []string {
	if v == nil {
		return nil
	}
	switch t := v.(type) {
	case string:
		s := strings.TrimSpace(t)
		if s == "" {
			return nil
		}
		return []string{s}
	case []interface{}:
		var s []string
		for _, x := range t {
			if str, ok := x.(string); ok {
				s = append(s, strings.TrimSpace(str))
			}
		}
		return s
	case []string:
		return append([]string(nil), t...)
	default:
		return nil
	}
}

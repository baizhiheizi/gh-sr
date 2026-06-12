package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestAddHost(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "runners.yml")
	initial := `hosts:
  existing:
    addr: a@b
    os: linux
    arch: amd64
runners:
  - name: r1
    repo: o/r
    host: existing
`
	if err := os.WriteFile(cfgPath, []byte(initial), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := AddHost(cfgPath, "newhost", "user@10.0.0.1", "", ""); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)
	if !strings.Contains(content, "newhost") {
		t.Errorf("config should contain newhost: %s", content)
	}
	if !strings.Contains(content, "user@10.0.0.1") {
		t.Errorf("config should contain addr: %s", content)
	}

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("config should be loadable after AddHost: %v", err)
	}
	if _, ok := cfg.Hosts["newhost"]; !ok {
		t.Error("newhost not found in loaded config")
	}
}

func TestAddHost_duplicate(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "runners.yml")
	initial := `hosts:
  h1:
    addr: a@b
    os: linux
    arch: amd64
runners:
  - name: r1
    repo: o/r
    host: h1
`
	if err := os.WriteFile(cfgPath, []byte(initial), 0o600); err != nil {
		t.Fatal(err)
	}
	err := AddHost(cfgPath, "h1", "c@d", "", "")
	if err == nil {
		t.Fatal("expected error for duplicate host")
	}
	if !strings.Contains(err.Error(), "already exists") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestAddRunner(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "runners.yml")
	initial := `hosts:
  h1:
    addr: a@b
    os: linux
    arch: amd64
runners:
  - name: r1
    repo: o/r
    host: h1
`
	if err := os.WriteFile(cfgPath, []byte(initial), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := AddRunner(cfgPath, "r2", "o/r2", "h1", 2, nil); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("config should be loadable after AddRunner: %v", err)
	}
	if len(cfg.Runners) != 2 {
		t.Fatalf("expected 2 runners, got %d", len(cfg.Runners))
	}
	r2 := cfg.Runners[1]
	if r2.Name != "r2" || r2.Repo != "o/r2" || r2.Host != "h1" || r2.Count != 2 {
		t.Errorf("unexpected runner: %+v", r2)
	}
}

func TestRemoveRunner(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "runners.yml")
	initial := `hosts:
  h1:
    addr: a@b
    os: linux
    arch: amd64
runners:
  - name: r1
    repo: o/r
    host: h1
  - name: r2
    repo: o/r
    host: h1
`
	if err := os.WriteFile(cfgPath, []byte(initial), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := RemoveRunner(cfgPath, "r2"); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("config should be loadable after RemoveRunner: %v", err)
	}
	if len(cfg.Runners) != 1 {
		t.Fatalf("expected 1 runner, got %d", len(cfg.Runners))
	}
	if cfg.Runners[0].Name != "r1" {
		t.Errorf("unexpected remaining runner: %+v", cfg.Runners[0])
	}
}

func TestRemoveRunner_notFound(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "runners.yml")
	initial := `hosts:
  h1:
    addr: a@b
runners:
  - name: r1
    repo: o/r
    host: h1
`
	if err := os.WriteFile(cfgPath, []byte(initial), 0o600); err != nil {
		t.Fatal(err)
	}
	err := RemoveRunner(cfgPath, "missing")
	if err == nil {
		t.Fatal("expected error for missing runner")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestLoadYAMLRoot(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "runners.yml")
	initial := `hosts:
  h1:
    addr: a@b
runners:
  - name: r1
    repo: o/r
    host: h1
`
	if err := os.WriteFile(cfgPath, []byte(initial), 0o600); err != nil {
		t.Fatal(err)
	}

	top, err := loadYAMLRoot(cfgPath)
	if err != nil {
		t.Fatalf("loadYAMLRoot: %v", err)
	}
	if top.Kind != yaml.MappingNode {
		t.Fatalf("expected MappingNode, got %v", top.Kind)
	}
	if findMapValue(top, "hosts") == nil {
		t.Error("expected to find hosts mapping")
	}
	if findMapValue(top, "runners") == nil {
		t.Error("expected to find runners sequence")
	}
}

func TestLoadYAMLRoot_errors(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "runners.yml")

	// missing file
	if _, err := loadYAMLRoot(cfgPath); err == nil {
		t.Error("expected error for missing file")
	} else if !strings.Contains(err.Error(), "reading config") {
		t.Errorf("unexpected error: %v", err)
	}

	// malformed YAML
	if err := os.WriteFile(cfgPath, []byte(":\n  - : bad"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := loadYAMLRoot(cfgPath); err == nil {
		t.Error("expected error for malformed YAML")
	} else if !strings.Contains(err.Error(), "parsing config") {
		t.Errorf("unexpected error: %v", err)
	}

	// empty document
	if err := os.WriteFile(cfgPath, []byte(""), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := loadYAMLRoot(cfgPath); err == nil {
		t.Error("expected error for empty document")
	} else if !strings.Contains(err.Error(), "unexpected YAML structure") {
		t.Errorf("unexpected error: %v", err)
	}

	// root is a sequence, not a mapping
	if err := os.WriteFile(cfgPath, []byte("- a\n- b\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := loadYAMLRoot(cfgPath); err == nil {
		t.Error("expected error for non-mapping root")
	} else if !strings.Contains(err.Error(), "config root is not a mapping") {
		t.Errorf("unexpected error: %v", err)
	}
}

package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAddHost(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "runners.yml")
	initial := `github:
  pat: tok
hosts:
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
	initial := `github:
  pat: tok
hosts:
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
	initial := `github:
  pat: tok
hosts:
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
	if err := AddRunner(cfgPath, "r2", "o/r2", "h1", 2, nil, "docker"); err != nil {
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
	if r2.Name != "r2" || r2.Repo != "o/r2" || r2.Host != "h1" || r2.Count != 2 || r2.Mode != "docker" {
		t.Errorf("unexpected runner: %+v", r2)
	}
}

package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadFromPath_missingFileHint(t *testing.T) {
	dir := t.TempDir()
	missing := filepath.Join(dir, "does-not-exist.yml")

	_, err := LoadFromPath(missing)
	if err == nil {
		t.Fatal("expected error for missing config file")
	}
	msg := err.Error()
	if !strings.Contains(msg, "config file not found") {
		t.Errorf("error should describe missing file, got %q", msg)
	}
	if !strings.Contains(msg, "gh sr init") {
		t.Errorf("error should suggest running `gh sr init`, got %q", msg)
	}
	if !strings.Contains(msg, EnvVarConfigPath) {
		t.Errorf("error should mention %s, got %q", EnvVarConfigPath, msg)
	}
}

func TestLoadFromPath_delegatesToLoad(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "runners.yml")
	content := `
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
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadFromPath(path)
	if err != nil {
		t.Fatalf("LoadFromPath: %v", err)
	}
	if cfg == nil {
		t.Fatal("LoadFromPath returned nil config with no error")
	}
	if len(cfg.Runners) != 1 || cfg.Runners[0].Name != "r1" {
		t.Errorf("LoadFromPath did not delegate to Load correctly: %+v", cfg.Runners)
	}
}

func TestLoadFromPath_propagatesLoadError(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "runners.yml")
	// An empty `runners` list fails Validate's "at least one runner must be
	// defined" check, so Load returns an error even though the file exists.
	if err := os.WriteFile(path, []byte(`runners: []`), 0o600); err != nil {
		t.Fatal(err)
	}

	_, err := LoadFromPath(path)
	if err == nil {
		t.Fatal("expected Load validation error")
	}
	// The error should bubble up from Load, not the not-found branch.
	if strings.Contains(err.Error(), "config file not found") {
		t.Errorf("error came from stat branch, not Load: %q", err)
	}
}

func TestLoadFromPath_statErrorWraps(t *testing.T) {
	dir := t.TempDir()
	// A regular file used as a directory component makes os.Stat fail with
	// ENOTDIR, which is *not* os.IsNotExist — so this exercises the
	// "config file: %w" wrap rather than the gh sr init hint.
	blocker := filepath.Join(dir, "blocker")
	if err := os.WriteFile(blocker, []byte("not a directory"), 0o600); err != nil {
		t.Fatal(err)
	}

	_, err := LoadFromPath(filepath.Join(blocker, "runners.yml"))
	if err == nil {
		t.Fatal("expected error when stat fails with a non-ENOENT error")
	}
	msg := err.Error()
	if strings.Contains(msg, "config file not found") {
		t.Errorf("should not take the not-found branch, got %q", msg)
	}
	if !strings.Contains(msg, "config file:") {
		t.Errorf("error should wrap the stat failure, got %q", msg)
	}
}

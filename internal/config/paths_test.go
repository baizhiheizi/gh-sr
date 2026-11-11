package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestEnvFilePath(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	got, err := EnvFilePath()
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(home, ".gh-sr", "env")
	if got != want {
		t.Errorf("want %q got %q", want, got)
	}
}

func TestUserRunnersPath(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	got, err := UserRunnersPath()
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(home, ".gh-sr", "runners.yml")
	if got != want {
		t.Errorf("want %q got %q", want, got)
	}
}

func TestPaths_emptyHome(t *testing.T) {
	t.Setenv("HOME", "")
	if _, err := SrDir(); err == nil {
		t.Error("SrDir: expected error when HOME is empty")
	}
	if _, err := EnvFilePath(); err == nil {
		t.Error("EnvFilePath: expected error when HOME is empty")
	}
	if _, err := UserRunnersPath(); err == nil {
		t.Error("UserRunnersPath: expected error when HOME is empty")
	}
	if err := BootstrapEnv(); err == nil {
		t.Error("BootstrapEnv: expected error when HOME is empty")
	}
}

func TestBootstrapEnv_appliesEnvFile(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	srDir := filepath.Join(home, ".gh-sr")
	if err := os.MkdirAll(srDir, 0o700); err != nil {
		t.Fatal(err)
	}
	content := "GH_SR_BOOTSTRAP_PLAIN=hello\n# a comment\n\nexport GH_SR_BOOTSTRAP_EXPORTED=\"w\"\n"
	if err := os.WriteFile(filepath.Join(srDir, "env"), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GH_SR_BOOTSTRAP_PLAIN", "")
	t.Setenv("GH_SR_BOOTSTRAP_EXPORTED", "")

	if err := BootstrapEnv(); err != nil {
		t.Fatal(err)
	}
	if got := os.Getenv("GH_SR_BOOTSTRAP_PLAIN"); got != "hello" {
		t.Errorf("plain entry: got %q", got)
	}
	if got := os.Getenv("GH_SR_BOOTSTRAP_EXPORTED"); got != "w" {
		t.Errorf("exported+quoted entry: got %q", got)
	}
}

func TestBootstrapEnv_missingFileIsNoop(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	if err := BootstrapEnv(); err != nil {
		t.Fatalf("missing env file must be a no-op, got %v", err)
	}
}

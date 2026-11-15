package config

import (
	"os"
	"path/filepath"
	"testing"
)

// TestEnsureEnvFile_createsMissingFile pins the "first run" path: when the
// env file does not exist, EnsureEnvFile creates the parent directory
// (0o700) and writes the template (0o600). Moved here from the TUI, which relies on it when the user picks
// "Edit env file" from the global menu before the env file has been
// bootstrapped.
func TestEnsureEnvFile_createsMissingFile(t *testing.T) {
	// No t.Parallel(): touches a per-test tempdir.
	dir := t.TempDir()
	path := filepath.Join(dir, "nested", "env")
	created, err := EnsureEnvFile(path)
	if err != nil {
		t.Fatalf("EnsureEnvFile: %v", err)
	}
	if !created {
		t.Error("EnsureEnvFile must report created=true for a missing file")
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("env file not created: %v", err)
	}
	if info.Size() == 0 {
		t.Errorf("env file is empty; should contain EnvFileTemplate")
	}
	// Permissions: 0o600 (owner read/write only).
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Errorf("env file perm = %o, want 0o600", perm)
	}
	// Parent directory: 0o700 (owner full access only).
	parentInfo, err := os.Stat(filepath.Dir(path))
	if err != nil {
		t.Fatalf("parent dir not created: %v", err)
	}
	if perm := parentInfo.Mode().Perm(); perm != 0o700 {
		t.Errorf("parent dir perm = %o, want 0o700", perm)
	}
}

// TestEnsureEnvFile_preservesExistingFile pins the "not first run" path:
// when the env file already exists, EnsureEnvFile leaves
// the file untouched. Existing user's edits must be preserved across
// dashboard re-opens.
func TestEnsureEnvFile_preservesExistingFile(t *testing.T) {
	// No t.Parallel(): touches a per-test tempdir.
	dir := t.TempDir()
	path := filepath.Join(dir, "env")
	original := "GITHUB_TOKEN=secret\n"
	if err := os.WriteFile(path, []byte(original), 0o600); err != nil {
		t.Fatalf("seed: %v", err)
	}
	created, err := EnsureEnvFile(path)
	if err != nil {
		t.Fatalf("EnsureEnvFile: %v", err)
	}
	if created {
		t.Error("EnsureEnvFile must report created=false for an existing file")
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read after EnsureEnvFile: %v", err)
	}
	if string(got) != original {
		t.Errorf("EnsureEnvFile should not overwrite existing file; got %q, want %q", got, original)
	}
}

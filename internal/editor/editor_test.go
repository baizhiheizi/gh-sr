package editor

import (
	"errors"
	"os/exec"
	"runtime"
	"testing"
)

func TestPreferred(t *testing.T) {
	cases := []struct {
		name   string
		visual string
		editor string
		want   string
		// onlyOS is empty when the expectation holds on every platform;
		// "!windows" means the default is platform-dependent and we should
		// skip on Windows. Use "windows" to gate a Windows-only assertion.
		onlyOS string
	}{
		{
			name:   "VISUAL takes priority over EDITOR",
			visual: "emacs",
			editor: "nano",
			want:   "emacs",
		},
		{
			name:   "EDITOR used when VISUAL is unset",
			visual: "",
			editor: "nano",
			want:   "nano",
		},
		{
			name:   "default vim on non-Windows",
			visual: "",
			editor: "",
			want:   "vim",
			onlyOS: "!windows",
		},
		{
			name:   "default notepad on Windows",
			visual: "",
			editor: "",
			want:   "notepad",
			onlyOS: "windows",
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			switch tc.onlyOS {
			case "!windows":
				if runtime.GOOS == "windows" {
					t.Skip("default editor differs on Windows")
				}
			case "windows":
				if runtime.GOOS != "windows" {
					t.Skip("Windows-only default editor assertion")
				}
			}
			t.Setenv("VISUAL", tc.visual)
			t.Setenv("EDITOR", tc.editor)
			got := Preferred()
			if got != tc.want {
				t.Errorf("Preferred() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestCommand_usesPreferred(t *testing.T) {
	t.Setenv("VISUAL", "myvim")
	t.Setenv("EDITOR", "")
	cmd := Command("/some/file")
	if cmd == nil {
		t.Fatal("Command returned nil")
	}
	if cmd.Path == "" {
		t.Fatal("Command returned cmd with empty Path")
	}
	// Args[0] is the resolved binary path; Args[1] should be the file path
	if len(cmd.Args) < 2 || cmd.Args[1] != "/some/file" {
		t.Errorf("Command args = %v, want last arg to be /some/file", cmd.Args)
	}
}

// TestOpen_success exercises the happy path: when VISUAL points at a real
// binary that exits 0, Open returns nil without leaking stdin/stdout/stderr
// from the test runner's perspective.
func TestOpen_success(t *testing.T) {
	if runtime.GOOS == "windows" {
		// "true" is the POSIX path used below; Windows shells don't expose it
		// on PATH by default. Skipping here is safe — the Windows-only
		// notepad default is covered in TestPreferred.
		t.Skip("POSIX-only happy-path editor")
	}
	t.Setenv("VISUAL", "true")
	// t.TempDir guarantees a path that exists; true ignores its arg.
	if err := Open(t.TempDir() + "/unused"); err != nil {
		t.Errorf("Open() with VISUAL=true returned %v, want nil", err)
	}
}

// TestOpen_missingEditor verifies Open surfaces the executor's "executable
// not found" error verbatim so callers can distinguish a real failure from a
// zero exit. The error wraps the underlying *exec.Error from os/exec.LookPath.
func TestOpen_missingEditor(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX-only error-path editor")
	}
	t.Setenv("VISUAL", "gh-sr-nonexistent-editor-xyz")
	err := Open("/tmp/whatever")
	if err == nil {
		t.Fatal("Open() with missing editor = nil, want error")
	}
	// exec.LookPath wraps os/exec.ErrNotFound; the underlying sentinel is
	// matched directly so the test is robust against PathError framing.
	if !errors.Is(err, exec.ErrNotFound) {
		t.Errorf("Open() error chain = %v, want errors.Is(_, exec.ErrNotFound)", err)
	}
}

package editor

import (
	"runtime"
	"testing"
)

func TestPreferred(t *testing.T) {
	cases := []struct {
		name   string
		visual string
		editor string
		want   string
		// skip when the expected result depends on GOOS and we're not on that OS
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
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.onlyOS == "!windows" && runtime.GOOS == "windows" {
				t.Skip("default editor differs on Windows")
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

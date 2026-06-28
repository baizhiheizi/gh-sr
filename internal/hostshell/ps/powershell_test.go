package ps

import (
	"reflect"
	"testing"
)

func TestCommandArgs(t *testing.T) {
	t.Parallel()
	got := CommandArgs("echo hi")
	want := []string{"powershell.exe", "-NoProfile", "-NonInteractive", "-Command", "echo hi"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("CommandArgs() = %v, want %v", got, want)
	}
}

func TestCommandLine(t *testing.T) {
	t.Parallel()
	cases := []struct {
		script string
		want   string
	}{
		{
			script: "[Environment]::OSVersion.Platform",
			want:   `powershell.exe -NoProfile -NonInteractive -Command "[Environment]::OSVersion.Platform"`,
		},
		{
			script: "$env:PROCESSOR_ARCHITECTURE",
			want:   `powershell.exe -NoProfile -NonInteractive -Command "$env:PROCESSOR_ARCHITECTURE"`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.script, func(t *testing.T) {
			t.Parallel()
			if got := CommandLine(tc.script); got != tc.want {
				t.Fatalf("CommandLine(%q) = %q, want %q", tc.script, got, tc.want)
			}
		})
	}
}

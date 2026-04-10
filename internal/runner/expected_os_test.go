package runner

import "testing"

func TestExpectedGitHubRunnerOS(t *testing.T) {
	t.Parallel()
	tests := []struct {
		hostOS string
		want   string
	}{
		{hostOS: "linux", want: "Linux"},
		{hostOS: "darwin", want: "macOS"},
		{hostOS: "windows", want: "Windows"},
		{hostOS: "freebsd", want: ""},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.hostOS, func(t *testing.T) {
			t.Parallel()
			got := expectedGitHubRunnerOS(tc.hostOS)
			if got != tc.want {
				t.Errorf("expectedGitHubRunnerOS(%q) = %q; want %q", tc.hostOS, got, tc.want)
			}
		})
	}
}

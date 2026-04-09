package runner

import "testing"

func TestExpectedGitHubRunnerOS(t *testing.T) {
	t.Parallel()
	tests := []struct {
		mode   string
		hostOS string
		want   string
	}{
		// Explicit docker mode always returns Linux regardless of host OS.
		{mode: "docker", hostOS: "linux", want: "Linux"},
		{mode: "docker", hostOS: "darwin", want: "Linux"},
		{mode: "docker", hostOS: "windows", want: "Linux"},

		// Explicit native mode returns the host OS label.
		{mode: "native", hostOS: "linux", want: "Linux"},
		{mode: "native", hostOS: "darwin", want: "macOS"},
		{mode: "native", hostOS: "windows", want: "Windows"},
		{mode: "native", hostOS: "freebsd", want: ""},

		// Empty mode: linux host defaults to docker → Linux.
		{mode: "", hostOS: "linux", want: "Linux"},
		// Empty mode: non-linux host defaults to native.
		{mode: "", hostOS: "darwin", want: "macOS"},
		{mode: "", hostOS: "windows", want: "Windows"},
		{mode: "", hostOS: "freebsd", want: ""},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.mode+"/"+tc.hostOS, func(t *testing.T) {
			t.Parallel()
			got := expectedGitHubRunnerOS(tc.mode, tc.hostOS)
			if got != tc.want {
				t.Errorf("expectedGitHubRunnerOS(%q, %q) = %q; want %q", tc.mode, tc.hostOS, got, tc.want)
			}
		})
	}
}

package runner

import "testing"

func Test_containerName(t *testing.T) {
	t.Parallel()
	if got := containerName("my-runner"); got != "gh-runner-my-runner" {
		t.Errorf("got %q", got)
	}
}

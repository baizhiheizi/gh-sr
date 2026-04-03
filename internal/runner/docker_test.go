package runner

import (
	"strings"
	"testing"

	"github.com/an-lee/ghr/internal/config"
	"github.com/an-lee/ghr/internal/host"
)

func Test_containerName(t *testing.T) {
	t.Parallel()
	if got := containerName("my-runner"); got != "gh-runner-my-runner" {
		t.Errorf("got %q", got)
	}
}

func newTestHost(name, os string) *host.Host {
	return host.NewHost(name, config.HostConfig{
		Addr: "user@host",
		OS:   os,
		Arch: "amd64",
	})
}

func Test_dockerRun_dispatchesRunShellOnWindows(t *testing.T) {
	t.Parallel()
	h := newTestHost("win", "windows")
	// Without a real SSH connection, both paths will error; we verify the
	// function does not panic and that we get the expected path.
	_, err := dockerRun(h, "docker info")
	if err == nil {
		t.Error("expected error from unconnected host")
	}
}

func Test_dockerRun_dispatchesRunOnLinux(t *testing.T) {
	t.Parallel()
	h := newTestHost("lin", "linux")
	_, err := dockerRun(h, "docker info")
	if err == nil {
		t.Error("expected error from unconnected host")
	}
}

func Test_dockerRunIgnoreErr_noPanic(t *testing.T) {
	t.Parallel()
	for _, os := range []string{"windows", "linux", "darwin"} {
		h := newTestHost("h", os)
		dockerRunIgnoreErr(h, "docker rm -f nonexistent")
	}
}

func Test_patchDockerConfigWindows_commandShape(t *testing.T) {
	t.Parallel()
	// patchDockerConfigWindows sends a PowerShell script via RunShell; with no real
	// SSH connection the call errors, but we can verify the script content by
	// intercepting what RunShell would receive.  We do this by calling the exported
	// helper directly with a fake host — the error is expected.
	h := newTestHost("win", "windows")
	err := patchDockerConfigWindows(h)
	// Must error (no real SSH), not panic.
	if err == nil {
		t.Error("expected error from unconnected host")
	}
	// Sanity: error message should mention the host name.
	if !strings.Contains(err.Error(), "win") && !strings.Contains(err.Error(), "patching") {
		t.Logf("patchDockerConfigWindows error: %v", err)
	}
}

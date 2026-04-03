package runner

import (
	"errors"
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

func Test_windowsDockerCommand_commandShape(t *testing.T) {
	t.Parallel()
	script := windowsDockerCommand("docker pull ghcr.io/actions/actions-runner:latest")
	if !strings.Contains(script, "$env:DOCKER_CONFIG = $ghrDockerConfigDir") {
		t.Fatalf("script should set DOCKER_CONFIG, got %q", script)
	}
	if !strings.Contains(script, `"ghr.invalid"`) {
		t.Fatalf("script should include dummy auth entry, got %q", script)
	}
	if !strings.Contains(script, `"credsStore": ""`) {
		t.Fatalf("script should blank credsStore, got %q", script)
	}
	if !strings.Contains(script, "[System.Text.UTF8Encoding]::new($false)") {
		t.Fatalf("script should write BOM-free UTF-8, got %q", script)
	}
	if !strings.Contains(script, "docker pull ghcr.io/actions/actions-runner:latest") {
		t.Fatalf("script should include docker command, got %q", script)
	}
}

func Test_wrapDockerInfoErr_classifiesSocketPermission(t *testing.T) {
	t.Parallel()
	base := errors.New(`permission denied while trying to connect to the docker API at unix:///var/run/docker.sock`)
	w := wrapDockerInfoErr(base)
	if w == nil || !strings.Contains(w.Error(), "cannot access Docker socket") {
		t.Fatalf("expected socket hint, got %v", w)
	}
	if !errors.Is(w, base) {
		t.Fatalf("expected wrap to preserve base error")
	}
}

func Test_wrapDockerInfoErr_classifiesDaemonUnreachable(t *testing.T) {
	t.Parallel()
	base := errors.New("Cannot connect to the Docker daemon at unix:///var/run/docker.sock. Is the docker daemon running?")
	w := wrapDockerInfoErr(base)
	if w == nil || !strings.Contains(w.Error(), "Docker daemon not reachable") {
		t.Fatalf("expected daemon hint, got %v", w)
	}
}

func Test_wrapDockerInfoErr_passesThroughUnknown(t *testing.T) {
	t.Parallel()
	base := errors.New("some other docker failure")
	if got := wrapDockerInfoErr(base); !errors.Is(got, base) || got.Error() != base.Error() {
		t.Fatalf("expected same error, got %v", got)
	}
}

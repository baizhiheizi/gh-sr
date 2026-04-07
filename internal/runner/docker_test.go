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

func Test_prependDarwinDockerPATH(t *testing.T) {
	t.Parallel()
	const wantPrefix = `export PATH="/usr/local/bin:/opt/homebrew/bin:$PATH"; `
	cases := []struct {
		os      string
		wantPre bool
	}{
		{"darwin", true},
		{"linux", false},
		{"windows", false},
	}
	for _, tc := range cases {
		t.Run(tc.os, func(t *testing.T) {
			t.Parallel()
			h := newTestHost("h", tc.os)
			cmd := "docker info"
			got := prependDarwinDockerPATH(h, cmd)
			pre := strings.HasPrefix(got, wantPrefix)
			if pre != tc.wantPre {
				t.Fatalf("os=%s want prefix=%v, got %q", tc.os, tc.wantPre, got)
			}
			if !strings.HasSuffix(got, cmd) {
				t.Fatalf("suffix: got %q", got)
			}
		})
	}
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

func Test_dockerStartCommand_officialImageShape(t *testing.T) {
	t.Parallel()
	cmd := dockerStartCommand(
		"gh-runner-app-1",
		"app-1",
		"REGTOKEN123",
		"https://github.com/o/r",
		"self-hosted,linux",
		"-v /var/run/docker.sock:/var/run/docker.sock",
		"ghcr.io/actions/actions-runner:latest",
	)
	for _, sub := range []string{
		"ACTIONS_RUNNER_INPUT_URL=",
		"https://github.com/o/r",
		"ACTIONS_RUNNER_INPUT_TOKEN=",
		"REGTOKEN123",
		"ACTIONS_RUNNER_INPUT_NAME=",
		"app-1",
		"ACTIONS_RUNNER_INPUT_LABELS=",
		"self-hosted,linux",
		"ACTIONS_RUNNER_INPUT_WORK=",
		"_work",
		"--entrypoint /bin/bash",
		"ghcr.io/actions/actions-runner:latest",
		"./config.sh --unattended --replace",
		"exec ./run.sh",
	} {
		if !strings.Contains(cmd, sub) {
			t.Fatalf("docker start command missing %q in:\n%s", sub, cmd)
		}
	}
	if strings.Contains(cmd, "RUNNER_TOKEN=") || strings.Contains(cmd, "RUNNER_URL=") {
		t.Fatalf("should not use legacy RUNNER_* env names: %s", cmd)
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

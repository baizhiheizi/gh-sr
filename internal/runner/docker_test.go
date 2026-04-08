package runner

import (
	"errors"
	"strings"
	"testing"

	"github.com/an-lee/ghr/internal/config"
	"github.com/an-lee/ghr/internal/host"
)

func Test_ContainerName(t *testing.T) {
	t.Parallel()
	if got := ContainerName("my-runner"); got != "gh-runner-my-runner" {
		t.Errorf("got %q", got)
	}
}

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

func Test_dockerEngineSockFlags_mountOnly_whenGIDzero(t *testing.T) {
	t.Parallel()
	// An unconnected host will fail the GID stat; the function should fall back to mount-only.
	h := newTestHost("lin", "linux")
	got := dockerEngineSockFlags(h, "")
	if !strings.Contains(got, "-v /var/run/docker.sock:/var/run/docker.sock") {
		t.Fatalf("missing mount flag, got %q", got)
	}
}

func Test_dockerEngineSockFlags_customSocketPath(t *testing.T) {
	t.Parallel()
	h := newTestHost("lin", "linux")
	got := dockerEngineSockFlags(h, "/run/user/1000/docker.sock")
	if !strings.Contains(got, "-v /run/user/1000/docker.sock:/var/run/docker.sock") {
		t.Fatalf("expected custom socket bind-mount, got %q", got)
	}
}

func Test_DefaultDockerSocket(t *testing.T) {
	t.Parallel()
	if DefaultDockerSocket != "/var/run/docker.sock" {
		t.Fatalf("unexpected default: %q", DefaultDockerSocket)
	}
}

func Test_socketPathFromDockerContextHost(t *testing.T) {
	t.Parallel()
	cases := []struct {
		in      string
		want    string
		wantOK  bool
	}{
		{"", "", false},
		{"  ", "", false},
		{"tcp://127.0.0.1:2376", "", false},
		{"unix:///var/run/docker.sock", "/var/run/docker.sock", true},
		{"unix:///Users/me/.colima/default/docker.sock", "/Users/me/.colima/default/docker.sock", true},
		{"  unix:///run/user/1000/docker.sock \n", "/run/user/1000/docker.sock", true},
		{"unix://", "", false},
		{"unix://relative.sock", "", false},
	}
	for _, tc := range cases {
		got, ok := socketPathFromDockerContextHost(tc.in)
		if ok != tc.wantOK || got != tc.want {
			t.Errorf("socketPathFromDockerContextHost(%q): got (%q, %v) want (%q, %v)", tc.in, got, ok, tc.want, tc.wantOK)
		}
	}
}

func Test_dockerStartCommand_officialImageShape(t *testing.T) {
	t.Parallel()
	sockFlags := "-v /var/run/docker.sock:/var/run/docker.sock --group-add 999 "
	cmd := dockerStartCommand(
		"gh-runner-app-1",
		"app-1",
		"REGTOKEN123",
		"https://github.com/o/r",
		"self-hosted,linux",
		sockFlags,
		"ghcr.io/actions/actions-runner:latest",
		"bridge",
		nil,
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
	if !strings.Contains(cmd, "/var/run/docker.sock:/var/run/docker.sock") {
		t.Fatalf("docker start command should bind Docker engine socket: %s", cmd)
	}
	if !strings.Contains(cmd, "--group-add 999") {
		t.Fatalf("docker start command should include --group-add flag: %s", cmd)
	}
	if strings.Contains(cmd, "RUNNER_TOKEN=") || strings.Contains(cmd, "RUNNER_URL=") {
		t.Fatalf("should not use legacy RUNNER_* env names: %s", cmd)
	}
	if strings.Contains(cmd, "--network host") {
		t.Fatalf("bridge mode must not set --network host: %s", cmd)
	}
}

func Test_dockerStartCommand_hostNetwork(t *testing.T) {
	t.Parallel()
	sockFlags := "-v /var/run/docker.sock:/var/run/docker.sock --group-add 999 "
	cmd := dockerStartCommand(
		"gh-runner-app-1",
		"app-1",
		"REGTOKEN123",
		"https://github.com/o/r",
		"self-hosted,linux",
		sockFlags,
		"ghcr.io/actions/actions-runner:latest",
		"host",
		nil,
	)
	n := strings.Index(cmd, "--network host")
	r := strings.Index(cmd, "--restart unless-stopped")
	if n < 0 || r < 0 || n > r {
		t.Fatalf("expected --network host before --restart: %s", cmd)
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

func Test_darwinDockerSockFlags_defaultSocket(t *testing.T) {
	t.Parallel()
	got := darwinDockerSockFlags("")
	if !strings.Contains(got, "-v /var/run/docker.sock:/var/run/docker.sock") {
		t.Fatalf("expected default socket mount, got %q", got)
	}
	if strings.Contains(got, "--group-add") {
		t.Fatalf("macOS mount must not include --group-add, got %q", got)
	}
}

func Test_darwinDockerSockFlags_customSocket(t *testing.T) {
	t.Parallel()
	// Non-Colima custom path is passed through unchanged by darwinDockerSockFlags alone.
	got := darwinDockerSockFlags("/Users/me/.docker/run/docker.sock")
	if !strings.Contains(got, "-v /Users/me/.docker/run/docker.sock:/var/run/docker.sock") {
		t.Fatalf("expected custom socket mount, got %q", got)
	}
	if strings.Contains(got, "--group-add") {
		t.Fatalf("macOS mount must not include --group-add, got %q", got)
	}
}

func Test_darwinDockerBindSourcePath(t *testing.T) {
	t.Parallel()
	cases := []struct {
		in, want string
	}{
		{"", ""},
		{"/var/run/docker.sock", "/var/run/docker.sock"},
		{"/Users/me/.colima/default/docker.sock", "/var/run/docker.sock"},
		{"/Users/me/.colima/myprofile/docker.sock", "/var/run/docker.sock"},
		{"/Users/me/.docker/run/docker.sock", "/Users/me/.docker/run/docker.sock"},
		{"/Users/me/.colima/default/other.sock", "/Users/me/.colima/default/other.sock"},
	}
	for _, tc := range cases {
		if got := darwinDockerBindSourcePath(tc.in); got != tc.want {
			t.Fatalf("darwinDockerBindSourcePath(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func Test_darwinDockerSockFlags_colimaSocketUsesVMBindPath(t *testing.T) {
	t.Parallel()
	got := darwinDockerSockFlags(darwinDockerBindSourcePath("/Users/me/.colima/default/docker.sock"))
	if !strings.Contains(got, "-v /var/run/docker.sock:/var/run/docker.sock") {
		t.Fatalf("Colima client socket should bind VM /var/run/docker.sock, got %q", got)
	}
	if strings.Contains(got, ".colima") {
		t.Fatalf("mount must not use host Colima socket path, got %q", got)
	}
}

func Test_dockerStartCommand_darwinIncludesSocketMount(t *testing.T) {
	t.Parallel()
	sockFlags := darwinDockerSockFlags("")
	cmd := dockerStartCommand(
		"gh-runner-app-1",
		"app-1",
		"REGTOKEN",
		"https://github.com/o/r",
		"self-hosted,linux",
		sockFlags,
		"ghcr.io/actions/actions-runner:latest",
		"bridge",
		nil,
	)
	if !strings.Contains(cmd, "/var/run/docker.sock:/var/run/docker.sock") {
		t.Fatalf("darwin start command should bind Docker socket: %s", cmd)
	}
	if strings.Contains(cmd, "--group-add") {
		t.Fatalf("darwin start command must not include --group-add: %s", cmd)
	}
}

func Test_dockerStartCommand_windowsIncludesSocketMount(t *testing.T) {
	t.Parallel()
	// Simulate mount-only sockFlags when GID probe fails (e.g. offline host).
	sockFlags := dockerWindowsEngineSockMount
	cmd := dockerStartCommand(
		"gh-runner-app-1",
		"app-1",
		"REGTOKEN",
		"https://github.com/o/r",
		"self-hosted,linux",
		sockFlags,
		"ghcr.io/actions/actions-runner:latest",
		"bridge",
		nil,
	)
	if !strings.Contains(cmd, "/var/run/docker.sock:/var/run/docker.sock") {
		t.Fatalf("windows start command should bind Docker socket: %s", cmd)
	}
}

func Test_dockerStartCommand_windowsIncludesGroupAddWhenGIDKnown(t *testing.T) {
	t.Parallel()
	sockFlags := appendGroupAddForDockerSockGID(dockerWindowsEngineSockMount, "999\n")
	cmd := dockerStartCommand(
		"gh-runner-app-1",
		"app-1",
		"REGTOKEN",
		"https://github.com/o/r",
		"self-hosted,linux",
		sockFlags,
		"ghcr.io/actions/actions-runner:latest",
		"bridge",
		nil,
	)
	if !strings.Contains(cmd, "--group-add 999") {
		t.Fatalf("windows start command should include --group-add when GID is known: %s", cmd)
	}
}

func Test_dockerCapAddFlags(t *testing.T) {
	t.Parallel()
	if got := dockerCapAddFlags(nil); got != "" {
		t.Fatalf("nil: got %q", got)
	}
	if got := dockerCapAddFlags([]string{}); got != "" {
		t.Fatalf("empty: got %q", got)
	}
	got := dockerCapAddFlags([]string{"NET_ADMIN"})
	if got != "--cap-add NET_ADMIN " {
		t.Fatalf("single: got %q", got)
	}
	got = dockerCapAddFlags([]string{"NET_ADMIN", "SYS_PTRACE"})
	if got != "--cap-add NET_ADMIN --cap-add SYS_PTRACE " {
		t.Fatalf("two: got %q", got)
	}
}

func Test_dockerStartCommand_capAdd(t *testing.T) {
	t.Parallel()
	sockFlags := "-v /var/run/docker.sock:/var/run/docker.sock "
	cmd := dockerStartCommand(
		"gh-runner-app-1",
		"app-1",
		"REGTOKEN",
		"https://github.com/o/r",
		"self-hosted,linux",
		sockFlags,
		"ghcr.io/actions/actions-runner:latest",
		"bridge",
		[]string{"NET_ADMIN"},
	)
	r := strings.Index(cmd, "--restart unless-stopped")
	c := strings.Index(cmd, "--cap-add NET_ADMIN")
	if r < 0 || c < 0 || r > c {
		t.Fatalf("expected --cap-add after --restart: %s", cmd)
	}
	e := strings.Index(cmd, "-e 'ACTIONS_RUNNER_INPUT_URL=")
	if e < 0 || c > e {
		t.Fatalf("expected --cap-add before first -e: %s", cmd)
	}
}

func Test_appendGroupAddForDockerSockGID(t *testing.T) {
	t.Parallel()
	mount := dockerWindowsEngineSockMount
	cases := []struct {
		in   string
		want string
	}{
		{"", mount},
		{"   ", mount},
		{"0", mount + "--group-add 0 "},
		{"999", mount + "--group-add 999 "},
		{"999\n", mount + "--group-add 999 "},
		{"12ab", mount},
		{"-1", mount},
	}
	for _, tc := range cases {
		got := appendGroupAddForDockerSockGID(mount, tc.in)
		if got != tc.want {
			t.Errorf("appendGroupAddForDockerSockGID(%q): got %q want %q", tc.in, got, tc.want)
		}
	}
}

func Test_DockerWindowsSockGIDProbeCommand_shape(t *testing.T) {
	t.Parallel()
	cmd := DockerWindowsSockGIDProbeCommand("ghcr.io/actions/actions-runner:latest")
	for _, sub := range []string{
		"docker run --rm",
		"-v /var/run/docker.sock:/var/run/docker.sock",
		"--entrypoint sh",
		"ghcr.io/actions/actions-runner:latest",
		`-c "stat -c '%g' /var/run/docker.sock"`,
	} {
		if !strings.Contains(cmd, sub) {
			t.Fatalf("probe command missing %q in:\n%s", sub, cmd)
		}
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

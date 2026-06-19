package runner

import (
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/an-lee/gh-sr/internal/config"
	"github.com/an-lee/gh-sr/internal/host"
	"github.com/an-lee/gh-sr/internal/testutil"
)

func linuxDockerHost(t *testing.T, addr string) *host.Host {
	t.Helper()
	h := host.NewHost("h", config.HostConfig{Addr: addr, OS: "linux", Arch: "amd64"})
	return h
}

func TestEnsureHostDocker_alreadyAvailable(t *testing.T) {
	t.Parallel()
	mock := &testutil.MockExecutor{RunFn: func(cmd string) (string, error) {
		switch {
		case strings.Contains(cmd, "docker --version"):
			return "yes", nil
		case strings.Contains(cmd, "docker info"):
			return "ok", nil
		default:
			return "", nil
		}
	}}
	h := linuxDockerHost(t, "runner@vps")
	h.SetConn(mock)

	if err := EnsureHostDocker(h, io.Discard, "aw-runner"); err != nil {
		t.Fatalf("EnsureHostDocker: %v", err)
	}
	for _, c := range mock.Calls {
		if strings.Contains(c, "get.docker.com") {
			t.Fatalf("should not install when docker is available: %q", c)
		}
	}
}

func TestEnsureHostDocker_installInvokesGetDocker(t *testing.T) {
	t.Parallel()
	var installCalled bool
	mock := &testutil.MockExecutor{RunFn: func(cmd string) (string, error) {
		switch {
		case strings.Contains(cmd, "docker --version"):
			return "no", nil
		case strings.Contains(cmd, "get.docker.com"):
			installCalled = true
			return "", nil
		case strings.Contains(cmd, "id -u"):
			return "no", nil
		case strings.Contains(cmd, "usermod"):
			return "", nil
		default:
			return "", nil
		}
	}}
	h := linuxDockerHost(t, "runner@vps")
	h.SetConn(mock)

	err := EnsureHostDocker(h, io.Discard, "aw-runner")
	if !errors.Is(err, ErrDockerGroupPending) {
		t.Fatalf("expected ErrDockerGroupPending, got %v", err)
	}
	if !installCalled {
		t.Fatal("expected get.docker.com install script")
	}
}

func TestEnsureHostDocker_rootSSHNoGroupPending(t *testing.T) {
	t.Parallel()
	mock := &testutil.MockExecutor{RunFn: func(cmd string) (string, error) {
		switch {
		case strings.Contains(cmd, "docker --version"):
			return "no", nil
		case strings.Contains(cmd, "id -u"):
			return "yes", nil
		case strings.Contains(cmd, "docker info"):
			return "ok", nil
		default:
			return "", nil
		}
	}}
	h := linuxDockerHost(t, "root@vps")
	h.SetConn(mock)

	if err := EnsureHostDocker(h, io.Discard, "aw-runner"); err != nil {
		t.Fatalf("root install should succeed without group pending: %v", err)
	}
	for _, c := range mock.Calls {
		if strings.Contains(c, "usermod") {
			t.Fatalf("root should not need usermod: %q", c)
		}
	}
}

func TestEnsureHostDocker_daemonStartWhenCLIPresent(t *testing.T) {
	t.Parallel()
	var systemctlCalled bool
	infoCalls := 0
	mock := &testutil.MockExecutor{RunFn: func(cmd string) (string, error) {
		switch {
		case strings.Contains(cmd, "docker --version"):
			return "yes", nil
		case strings.Contains(cmd, "systemctl enable --now docker"):
			systemctlCalled = true
			return "", nil
		case strings.Contains(cmd, "docker info"):
			infoCalls++
			if infoCalls == 1 {
				return "Cannot connect to the Docker daemon", nil
			}
			return "ok", nil
		default:
			return "", nil
		}
	}}
	h := linuxDockerHost(t, "runner@vps")
	h.SetConn(mock)

	if err := EnsureHostDocker(h, io.Discard, "aw-runner"); err != nil {
		t.Fatalf("EnsureHostDocker: %v", err)
	}
	if !systemctlCalled {
		t.Fatal("expected systemctl start when daemon was down")
	}
	for _, c := range mock.Calls {
		if strings.Contains(c, "get.docker.com") {
			t.Fatalf("should not reinstall when CLI present: %q", c)
		}
	}
}

func TestEnsureHostDocker_permissionDeniedAutoUsermod(t *testing.T) {
	t.Parallel()
	var usermodCalled bool
	mock := &testutil.MockExecutor{RunFn: func(cmd string) (string, error) {
		switch {
		case strings.Contains(cmd, "docker --version"):
			return "yes", nil
		case strings.Contains(cmd, "docker info"):
			return "permission denied while trying to connect to the Docker daemon socket", nil
		case strings.Contains(cmd, "usermod"):
			usermodCalled = true
			return "", nil
		case strings.Contains(cmd, "id -u"):
			return "no", nil
		default:
			return "", nil
		}
	}}
	h := linuxDockerHost(t, "runner@vps")
	h.SetConn(mock)

	err := EnsureHostDocker(h, io.Discard, "aw-runner")
	if !errors.Is(err, ErrDockerGroupPending) {
		t.Fatalf("expected ErrDockerGroupPending after usermod, got %v", err)
	}
	if !usermodCalled {
		t.Fatal("expected usermod when permission denied")
	}
	for _, c := range mock.Calls {
		if strings.Contains(c, "get.docker.com") {
			t.Fatalf("should not reinstall on permission denied: %q", c)
		}
	}
}

func TestEnsureHostDocker_permissionDeniedNoReinstall(t *testing.T) {
	t.Parallel()
	mock := &testutil.MockExecutor{RunFn: func(cmd string) (string, error) {
		switch {
		case strings.Contains(cmd, "docker --version"):
			return "yes", nil
		case strings.Contains(cmd, "docker info"):
			return "permission denied while trying to connect to the Docker daemon socket", nil
		case strings.Contains(cmd, "usermod"):
			return "", errors.New("sudo: a password is required")
		default:
			return "", nil
		}
	}}
	h := linuxDockerHost(t, "runner@vps")
	h.SetConn(mock)

	err := EnsureHostDocker(h, io.Discard, "aw-runner")
	if err == nil || errors.Is(err, ErrDockerGroupPending) {
		t.Fatalf("expected permission error when usermod fails, got %v", err)
	}
	if !strings.Contains(err.Error(), "usermod") {
		t.Fatalf("expected docker group guidance, got %v", err)
	}
	for _, c := range mock.Calls {
		if strings.Contains(c, "get.docker.com") {
			t.Fatalf("should not reinstall on permission denied: %q", c)
		}
	}
}

func TestDockerGroupPendingMessage(t *testing.T) {
	t.Parallel()
	if got := dockerGroupPendingMessage("aw-runner"); got != "Re-run: gh sr setup aw-runner" {
		t.Fatalf("got %q", got)
	}
	if got := dockerGroupPendingMessage(""); got != "Re-run: gh sr setup" {
		t.Fatalf("got %q", got)
	}
}

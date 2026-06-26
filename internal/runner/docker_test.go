package runner

import (
	"bytes"
	"errors"
	"fmt"
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

func TestAddSSHTUserToDockerGroup_retryPathAddsAndAnnounces(t *testing.T) {
	t.Parallel()
	var usermodCalled bool
	mock := &testutil.MockExecutor{RunFn: func(cmd string) (string, error) {
		if strings.Contains(cmd, "usermod") {
			usermodCalled = true
			return "", nil
		}
		return "", nil
	}}
	h := linuxDockerHost(t, "runner@vps")
	h.SetConn(mock)

	var buf bytes.Buffer
	err := addSSHTUserToDockerGroup(h, &buf, "aw-runner",
		"  Added ", "to",
		func(string, error) error { return errors.New("should not be called") },
	)
	if !errors.Is(err, ErrDockerGroupPending) {
		t.Fatalf("expected ErrDockerGroupPending, got %v", err)
	}
	if !usermodCalled {
		t.Fatal("expected usermod to be called")
	}
	got := buf.String()
	if !strings.Contains(got, "  Added runner to the docker group.") {
		t.Fatalf("announcement line missing in: %q", got)
	}
	if !strings.Contains(got, "Re-run: gh sr setup aw-runner") {
		t.Fatalf("pending message missing in: %q", got)
	}
}

func TestAddSSHTUserToDockerGroup_freshInstallPathAddsAndAnnounces(t *testing.T) {
	t.Parallel()
	var usermodCalled bool
	mock := &testutil.MockExecutor{RunFn: func(cmd string) (string, error) {
		if strings.Contains(cmd, "usermod") {
			usermodCalled = true
			return "", nil
		}
		return "", nil
	}}
	h := linuxDockerHost(t, "runner@vps")
	h.SetConn(mock)

	var buf bytes.Buffer
	err := addSSHTUserToDockerGroup(h, &buf, "aw-runner",
		"  Docker installed and ", "added to",
		func(string, error) error { return errors.New("should not be called") },
	)
	if !errors.Is(err, ErrDockerGroupPending) {
		t.Fatalf("expected ErrDockerGroupPending, got %v", err)
	}
	if !usermodCalled {
		t.Fatal("expected usermod to be called")
	}
	got := buf.String()
	if !strings.Contains(got, "  Docker installed and runner added to the docker group.") {
		t.Fatalf("announcement line missing in: %q", got)
	}
}

func TestAddSSHTUserToDockerGroup_emptyUserCallsErrCallback(t *testing.T) {
	t.Parallel()
	var usermodCalled bool
	sentinelCalled := false
	mock := &testutil.MockExecutor{RunFn: func(cmd string) (string, error) {
		if strings.Contains(cmd, "usermod") {
			usermodCalled = true
			return "", nil
		}
		return "", nil
	}}
	// Empty ssh user: linuxDockerHost uses addr "h" (no '@'); SSHUser parses
	// the user@host format and returns "" when no '@' is present.
	h := linuxDockerHost(t, "noatsignhost")
	h.SetConn(mock)

	var buf bytes.Buffer
	err := addSSHTUserToDockerGroup(h, &buf, "aw-runner",
		"  Added ", "to",
		func(user string, e error) error {
			sentinelCalled = true
			if user != "" {
				t.Fatalf("expected empty sshUser, got %q", user)
			}
			if e == nil {
				t.Fatal("expected sentinel error")
			}
			return e
		},
	)
	if err == nil {
		t.Fatal("expected error from errOnUsermodFail")
	}
	if !strings.Contains(err.Error(), "ssh user is empty") {
		t.Fatalf("expected sentinel-bearing error, got %v", err)
	}
	if !sentinelCalled {
		t.Fatal("expected errOnUsermodFail to be called for empty sshUser")
	}
	if usermodCalled {
		t.Fatal("usermod must not be called when sshUser is empty")
	}
	if buf.Len() != 0 {
		t.Fatalf("expected no output, got %q", buf.String())
	}
}

func TestAddSSHTUserToDockerGroup_usermodFailurePropagatesWrappedError(t *testing.T) {
	t.Parallel()
	mock := &testutil.MockExecutor{RunFn: func(cmd string) (string, error) {
		if strings.Contains(cmd, "usermod") {
			return "", errors.New("sudo: a password is required")
		}
		return "", nil
	}}
	h := linuxDockerHost(t, "runner@vps")
	h.SetConn(mock)

	var buf bytes.Buffer
	err := addSSHTUserToDockerGroup(h, &buf, "aw-runner",
		"  Docker installed and ", "added to",
		func(sshUser string, e error) error {
			return fmt.Errorf("adding %s to docker group: %w", sshUser, e)
		},
	)
	if err == nil || errors.Is(err, ErrDockerGroupPending) {
		t.Fatalf("expected wrapped error, got %v", err)
	}
	if !strings.Contains(err.Error(), "adding runner to docker group") {
		t.Fatalf("expected wrapping to include sshUser, got %v", err)
	}
	if !strings.Contains(err.Error(), "password is required") {
		t.Fatalf("expected underlying error to wrap, got %v", err)
	}
	if buf.Len() != 0 {
		t.Fatalf("expected no output on failure, got %q", buf.String())
	}
}

// TestEnsureHostDocker_daemonDownUsesThreeSSHCalls pins the round-trip count
// for the daemon-down-then-started path: dockerInfoStatus (down) → systemctl
// start → dockerInfoStatus (ok). Previously 4 calls because the original code
// re-checked status twice after the start; the consolidation reuses both
// returns from the single dockerInfoStatus call. Saves 1 SSH round-trip per
// `gh sr setup` on hosts where the docker daemon is initially down.
func TestEnsureHostDocker_daemonDownUsesThreeSSHCalls(t *testing.T) {
	t.Parallel()
	var systemctlCalled bool
	var infoCalls, startCalls int
	mock := &testutil.MockExecutor{RunFn: func(cmd string) (string, error) {
		switch {
		case strings.Contains(cmd, "docker --version"):
			return "yes", nil
		case strings.Contains(cmd, "systemctl enable --now docker"):
			systemctlCalled = true
			startCalls++
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
	if infoCalls != 2 {
		t.Fatalf("expected 2 docker info calls (before+after systemctl), got %d", infoCalls)
	}
	if startCalls != 1 {
		t.Fatalf("expected 1 systemctl start, got %d", startCalls)
	}
}

// TestEnsureHostDocker_permissionDeniedSkipsSystemctlStart pins that the
// permission-denied path does not waste SSH round-trips on
// `systemctl enable --now docker` (which cannot fix a permission-denied error)
// and a redundant `docker info` re-check. Previously 4 calls; now 2 calls
// (docker info → usermod). Saves 2 SSH round-trips per `gh sr setup` on hosts
// where the SSH user is missing from the docker group.
func TestEnsureHostDocker_permissionDeniedSkipsSystemctlStart(t *testing.T) {
	t.Parallel()
	var usermodCalled bool
	var infoCalls, startCalls int
	mock := &testutil.MockExecutor{RunFn: func(cmd string) (string, error) {
		switch {
		case strings.Contains(cmd, "docker --version"):
			return "yes", nil
		case strings.Contains(cmd, "docker info"):
			infoCalls++
			return "permission denied while trying to connect to the Docker daemon socket", nil
		case strings.Contains(cmd, "systemctl enable --now docker"):
			startCalls++
			return "", nil
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
		t.Fatalf("expected ErrDockerGroupPending, got %v", err)
	}
	if !usermodCalled {
		t.Fatal("expected usermod when permission denied")
	}
	if startCalls != 0 {
		t.Fatalf("systemctl start cannot fix permission-denied; expected 0 calls, got %d", startCalls)
	}
	if infoCalls != 1 {
		t.Fatalf("expected 1 docker info call (initial check), got %d", infoCalls)
	}
}

package runner

import (
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/an-lee/gh-sr/internal/host"
	"github.com/an-lee/gh-sr/internal/hostshell"
)

const dockerGetURL = "https://get.docker.com"

// ErrDockerGroupPending indicates Docker was freshly installed and the SSH user
// was added to the docker group; setup must be re-run so a new SSH session picks
// up group membership before image build.
var ErrDockerGroupPending = errors.New("docker installed; re-run setup after docker group membership is active")

// EnsureHostDocker verifies Docker CLI and daemon access on Linux before container
// setup. When the CLI is missing it installs via get.docker.com. After a fresh
// install on a non-root SSH session it returns ErrDockerGroupPending (Option B).
func EnsureHostDocker(h *host.Host, w io.Writer, runnerName string) error {
	if h == nil || h.OS != "linux" {
		return nil
	}

	if dockerCLIInstalled(h) {
		return ensureDockerDaemonAccess(h, w, runnerName)
	}
	return installHostDocker(h, w, runnerName)
}

func dockerCLIInstalled(h *host.Host) bool {
	out, _ := h.Run(`sh -c 'docker --version 2>/dev/null | grep -q "Docker version" && echo yes || echo no'`)
	return strings.TrimSpace(out) == "yes"
}

func dockerInfoStatus(h *host.Host) (ok, permissionDenied bool) {
	out, _ := h.Run(`sh -c 'if docker info >/dev/null 2>&1; then echo ok; else docker info 2>&1; fi'`)
	trimmed := strings.TrimSpace(out)
	if trimmed == "ok" {
		return true, false
	}
	return false, strings.Contains(strings.ToLower(out), "permission denied")
}

func ensureDockerDaemonAccess(h *host.Host, w io.Writer, runnerName string) error {
	if ok, _ := dockerInfoStatus(h); ok {
		return nil
	}

	if _, err := h.Run(startDockerServiceScript()); err != nil {
		return fmt.Errorf("starting Docker service: %w", err)
	}
	if ok, _ := dockerInfoStatus(h); ok {
		return nil
	}

	if _, denied := dockerInfoStatus(h); denied {
		return permissionDeniedError(h)
	}

	return fmt.Errorf("docker daemon not reachable; try on the host: sudo systemctl start docker")
}

func installHostDocker(h *host.Host, w io.Writer, runnerName string) error {
	fmt.Fprintln(w, "  Docker not found, installing via get.docker.com (this may take several minutes)...")

	script := sudoPrelude() + ensureCurlForDockerScript() + installDockerScript()
	if _, err := h.Run(script); err != nil {
		return fmt.Errorf("installing Docker: %w", err)
	}
	if _, err := h.Run(startDockerServiceScript()); err != nil {
		return fmt.Errorf("starting Docker service after install: %w", err)
	}

	isRoot, _ := h.Run(`sh -c '[ "$(id -u)" -eq 0 ] && echo yes || echo no'`)
	if strings.TrimSpace(isRoot) == "yes" {
		if ok, _ := dockerInfoStatus(h); !ok {
			return fmt.Errorf("docker installed but daemon not reachable")
		}
		fmt.Fprintln(w, "  Docker installed.")
		return nil
	}

	sshUser := h.SSHUser()
	if sshUser != "" {
		usermod := sudoPrelude() + fmt.Sprintf("\n$SUDO usermod -aG docker %s\n", hostshell.PosixSingleQuote(sshUser))
		if _, err := h.Run(usermod); err != nil {
			return fmt.Errorf("adding %s to docker group: %w", sshUser, err)
		}
	}

	fmt.Fprint(w, "  Docker installed")
	if sshUser != "" {
		fmt.Fprintf(w, " and %s added to the docker group", sshUser)
	}
	fmt.Fprintln(w, ".")
	fmt.Fprintln(w, "  "+dockerGroupPendingMessage(runnerName))
	return ErrDockerGroupPending
}

func dockerGroupPendingMessage(runnerName string) string {
	if runnerName != "" {
		return fmt.Sprintf("Re-run: gh sr setup %s", runnerName)
	}
	return "Re-run: gh sr setup"
}

func ensureCurlForDockerScript() string {
	return `
if ! command -v curl >/dev/null 2>&1; then
  if command -v apt-get >/dev/null 2>&1; then $SUDO apt-get update && $SUDO apt-get install -y curl;
  elif command -v yum >/dev/null 2>&1; then $SUDO yum install -y curl;
  elif command -v apk >/dev/null 2>&1; then $SUDO apk add curl;
  fi
fi
`
}

func installDockerScript() string {
	return `
if ! command -v curl >/dev/null 2>&1; then
  echo "curl is required to install Docker" >&2
  exit 1
fi
curl -fsSL ` + hostshell.PosixSingleQuote(dockerGetURL) + ` | $SUDO sh
`
}

func startDockerServiceScript() string {
	return sudoPrelude() + `
if command -v systemctl >/dev/null 2>&1; then
  $SUDO systemctl enable --now docker 2>/dev/null || true
fi
`
}

func permissionDeniedError(h *host.Host) error {
	sshUser := h.SSHUser()
	if sshUser != "" {
		return fmt.Errorf(
			"docker CLI is installed but %s cannot access the Docker socket (run: sudo usermod -aG docker %s, then re-run gh sr setup)",
			sshUser, sshUser,
		)
	}
	return fmt.Errorf(
		"docker CLI is installed but the SSH user cannot access the Docker socket (add the user to the docker group, then re-run gh sr setup)",
	)
}

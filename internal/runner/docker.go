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
	// dockerInfoStatus returns (ok, permissionDenied) from a single `docker info`
	// call. Capture both — a permission-denied result means starting the service
	// can't help (the daemon is up but the socket is unreachable for this user),
	// so we can skip the start+recheck round-trips and go straight to the
	// docker-group remediation path. Saves 2 SSH round-trips on the
	// permission-denied path (4 → 2) and 1 on the daemon-down-then-started path
	// (4 → 3) per EnsureHostDocker call.
	ok, denied := dockerInfoStatus(h)
	if ok {
		return nil
	}
	if denied {
		return ensureDockerGroupAccess(h, w, runnerName)
	}

	if _, err := h.Run(startDockerServiceScript()); err != nil {
		return fmt.Errorf("starting Docker service: %w", err)
	}
	ok, denied = dockerInfoStatus(h)
	if ok {
		return nil
	}
	if denied {
		return ensureDockerGroupAccess(h, w, runnerName)
	}

	return fmt.Errorf("docker daemon not reachable; try on the host: sudo systemctl start docker")
}

// ensureDockerGroupAccess adds the SSH user to the docker group when they cannot
// access the socket, then returns ErrDockerGroupPending for a setup re-run.
func ensureDockerGroupAccess(h *host.Host, w io.Writer, runnerName string) error {
	isRoot, _ := h.Run(`sh -c '[ "$(id -u)" -eq 0 ] && echo yes || echo no'`)
	if strings.TrimSpace(isRoot) == "yes" {
		return fmt.Errorf("docker CLI is installed but docker info failed as root; check that the Docker daemon is running")
	}

	return addSSHTUserToDockerGroup(h, w, runnerName,
		"  Added ", "to",
		func(string, error) error { return permissionDeniedError(h) },
	)
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

	if h.SSHUser() == "" {
		fmt.Fprintln(w, "  Docker installed.")
		fmt.Fprintln(w, "  "+dockerGroupPendingMessage(runnerName))
		return ErrDockerGroupPending
	}
	return addSSHTUserToDockerGroup(h, w, runnerName,
		"  Docker installed and ", "added to",
		func(sshUser string, err error) error {
			return fmt.Errorf("adding %s to docker group: %w", sshUser, err)
		},
	)
}

func dockerGroupPendingMessage(runnerName string) string {
	if runnerName != "" {
		return fmt.Sprintf("Re-run: gh sr setup %s", runnerName)
	}
	return "Re-run: gh sr setup"
}

// addSSHTUserToDockerGroup runs `usermod -aG docker <sshUser>` on the host,
// prints the standard "<lead><user> <verb> the docker group." announcement,
// appends the pending-re-run message, and returns ErrDockerGroupPending.
//
// This is the canonical docker-group-add helper extracted from
// `ensureDockerGroupAccess` (the post-install permission-denied retry path)
// and the non-root tail of `installHostDocker` (the fresh-install path).
// The two sites previously duplicated the `sudoPrelude() + usermod` snippet,
// the announcement print, and the pending-message + ErrDockerGroupPending
// return. The duplication had already drifted — the empty-sshUser branch
// returned permissionDeniedError on the retry path and silently skipped on
// the fresh-install path; the usermod-failure error was wrapped differently.
//
// `lead` and `verb` together form the announcement prefix before "<user>":
//   - retry path: "  Added ", "to"  → "  Added runner to the docker group."
//   - fresh-install path: "  Docker installed and ", "added to"
//     → "  Docker installed and runner added to the docker group."
//
// `errOnUsermodFail` formats the error returned when sshUser is empty or
// when usermod fails. The retry path passes permissionDeniedError; the
// fresh-install path wraps with "adding %s to docker group: %w". sshUser is
// passed to the callback so the fresh-install path can include it in the
// wrapping error.
//
// The empty-sshUser case is treated as a usermod failure (errOnUsermodFail
// is called with an empty sshUser and an "ssh user is empty" sentinel).
// The fresh-install path's silent-skip behaviour is preserved by the caller
// checking `h.SSHUser() == ""` before calling this helper.
func addSSHTUserToDockerGroup(h *host.Host, w io.Writer, runnerName, lead, verb string, errOnUsermodFail func(sshUser string, err error) error) error {
	sshUser := h.SSHUser()
	if sshUser == "" {
		return errOnUsermodFail("", errors.New("ssh user is empty"))
	}
	usermod := sudoPrelude() + fmt.Sprintf("\n$SUDO usermod -aG docker %s\n", hostshell.PosixSingleQuote(sshUser))
	if _, err := h.Run(usermod); err != nil {
		return errOnUsermodFail(sshUser, err)
	}
	fmt.Fprintf(w, "%s%s %s the docker group.\n", lead, sshUser, verb)
	fmt.Fprintln(w, "  "+dockerGroupPendingMessage(runnerName))
	return ErrDockerGroupPending
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

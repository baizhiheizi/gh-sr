package runner

import (
	"fmt"
	"strings"

	"github.com/an-lee/ghr/internal/config"
	"github.com/an-lee/ghr/internal/host"
)

// RunnerDockerImage is the container image used for docker-mode runners.
const RunnerDockerImage = "ghcr.io/actions/actions-runner:latest"

// dockerRunnerEntryScript registers once (registration tokens are single-use) then runs the
// listener. Official ghcr.io/actions/actions-runner has no CMD; it expects config.sh then run.sh.
const dockerRunnerEntryScript = `cd /home/runner && if [ ! -f .runner ]; then ./config.sh --unattended --replace; fi && exec ./run.sh`

func containerName(instanceName string) string {
	return "gh-runner-" + instanceName
}

// shellSingleQuote wraps s in single quotes for a POSIX shell word (safe for docker -e on Linux/macOS SSH).
func shellSingleQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
}

// dockerEnvFlag renders one -e flag as a single shell word: -e 'NAME=value'
func dockerEnvFlag(name, value string) string {
	return "-e " + shellSingleQuote(name+"="+value)
}

// dockerStartCommand builds the docker run line for the official actions-runner image (config.sh + run.sh).
func dockerStartCommand(cname, instanceName, regToken, repoURL, labels, sockMount, image string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "docker run -d --name %s --restart unless-stopped ", cname)
	b.WriteString(dockerEnvFlag("ACTIONS_RUNNER_INPUT_URL", repoURL))
	b.WriteByte(' ')
	b.WriteString(dockerEnvFlag("ACTIONS_RUNNER_INPUT_TOKEN", regToken))
	b.WriteByte(' ')
	b.WriteString(dockerEnvFlag("ACTIONS_RUNNER_INPUT_NAME", instanceName))
	b.WriteByte(' ')
	b.WriteString(dockerEnvFlag("ACTIONS_RUNNER_INPUT_LABELS", labels))
	b.WriteByte(' ')
	b.WriteString(dockerEnvFlag("ACTIONS_RUNNER_INPUT_WORK", "_work"))
	b.WriteByte(' ')
	if s := strings.TrimSpace(sockMount); s != "" {
		b.WriteString(s)
		b.WriteByte(' ')
	}
	b.WriteString("--entrypoint /bin/bash ")
	b.WriteString(image)
	b.WriteString(" -c ")
	b.WriteString(shellSingleQuote(dockerRunnerEntryScript))
	return b.String()
}

// dockerEngineSockBindMount returns the docker run volume flag that exposes the host engine socket
// inside the Linux actions-runner container. On Docker Desktop for Windows, Linux containers run in
// the WSL2/Moby VM; this mount is the supported way for in-container `docker` to reach that engine.
func dockerEngineSockBindMount() string {
	return "-v /var/run/docker.sock:/var/run/docker.sock "
}

// prependDarwinDockerPATH prefixes a remote shell command on macOS so the Docker CLI is on PATH
// when SSH uses a minimal environment (missing /usr/local/bin and /opt/homebrew/bin).
func prependDarwinDockerPATH(h *host.Host, cmd string) string {
	if h.OS != "darwin" {
		return cmd
	}
	return `export PATH="/usr/local/bin:/opt/homebrew/bin:$PATH"; ` + cmd
}

// dockerRun executes a Docker CLI command on the host, using PowerShell
// wrapping on Windows and raw shell on Linux/macOS.
func dockerRun(h *host.Host, cmd string) (string, error) {
	if h.OS == "windows" {
		return h.RunShell(windowsDockerCommand(cmd))
	}
	return h.Run(prependDarwinDockerPATH(h, cmd))
}

// dockerRunIgnoreErr is like dockerRun but discards the error (for best-effort cleanup).
func dockerRunIgnoreErr(h *host.Host, cmd string) {
	if h.OS == "windows" {
		h.RunShell(windowsDockerCommand(cmd))
	} else {
		h.Run(prependDarwinDockerPATH(h, cmd))
	}
}

func (m *Manager) setupDocker(h *host.Host) error {
	if h.OS == "windows" {
		return m.setupDockerWindows(h)
	}
	return m.setupDockerUnix(h)
}

// windowsDockerCommand forces Docker to use an isolated config directory that
// avoids the Windows native credential helper in SSH sessions. The dummy auth
// entry keeps Docker CLI from auto-detecting wincred as the default store when
// no auth is configured.
func windowsDockerCommand(cmd string) string {
	return strings.TrimSpace(fmt.Sprintf(`
$ghrDockerConfigDir = Join-Path $env:TEMP 'ghr-docker-config'
New-Item -ItemType Directory -Force -Path $ghrDockerConfigDir | Out-Null
$ghrDockerConfigFile = Join-Path $ghrDockerConfigDir 'config.json'
$ghrDockerConfigJson = @'
{
  "auths": {
    "ghr.invalid": {
      "auth": "Z2hyOmdocg=="
    }
  },
  "credsStore": ""
}
'@
[System.IO.File]::WriteAllText($ghrDockerConfigFile, $ghrDockerConfigJson, [System.Text.UTF8Encoding]::new($false))
$env:DOCKER_CONFIG = $ghrDockerConfigDir
%s
`, cmd))
}

func (m *Manager) setupDockerWindows(h *host.Host) error {
	out, err := dockerRun(h, `docker info --format "{{.ServerVersion}}"`)
	if err != nil || strings.TrimSpace(out) == "" {
		return fmt.Errorf("docker not available on host %s: install Docker Desktop and ensure it is running", h.Name)
	}
	fmt.Fprintf(m.out(), "  %s: Docker %s available\n", h.Name, strings.TrimSpace(out))

	fmt.Fprintf(m.out(), "  %s: pulling runner image...\n", h.Name)
	if _, err := dockerRun(h, fmt.Sprintf("docker pull %s", RunnerDockerImage)); err != nil {
		return fmt.Errorf("pulling Docker image: %w", err)
	}

	return nil
}

// UnixDockerCLIInstalled reports whether a docker binary exists on PATH on the host.
func UnixDockerCLIInstalled(h *host.Host) (bool, error) {
	out, err := h.Run(prependDarwinDockerPATH(h, `if command -v docker >/dev/null 2>&1; then echo yes; else echo no; fi`))
	if err != nil {
		return false, err
	}
	switch strings.TrimSpace(out) {
	case "yes":
		return true, nil
	case "no":
		return false, nil
	default:
		return false, fmt.Errorf("unexpected output checking docker CLI: %q", out)
	}
}

func wrapDockerInfoErr(err error) error {
	if err == nil {
		return nil
	}
	lower := strings.ToLower(err.Error())
	switch {
	case strings.Contains(lower, "permission denied") && strings.Contains(lower, "docker.sock"):
		return fmt.Errorf("cannot access Docker socket (add the SSH user to the 'docker' group and reconnect SSH, or use root): %w", err)
	case strings.Contains(lower, "permission denied"):
		return fmt.Errorf("permission denied talking to Docker: %w", err)
	case strings.Contains(lower, "cannot connect to the docker daemon"),
		strings.Contains(lower, "is the docker daemon running"),
		strings.Contains(lower, "connection refused"):
		return fmt.Errorf("Docker daemon not reachable (start the Docker service, e.g. systemctl start docker; see README): %w", err)
	default:
		return err
	}
}

// UnixDockerServerVersion returns the Docker Engine server version from docker info, or an error
// that distinguishes socket permissions and daemon reachability from other failures.
func UnixDockerServerVersion(h *host.Host) (string, error) {
	out, err := h.Run(prependDarwinDockerPATH(h, "docker info --format '{{.ServerVersion}}'"))
	out = strings.TrimSpace(out)
	if err == nil {
		if out == "" {
			return "", fmt.Errorf("docker info returned empty server version")
		}
		return out, nil
	}
	return "", wrapDockerInfoErr(err)
}

func (m *Manager) setupDockerUnix(h *host.Host) error {
	hasCLI, err := UnixDockerCLIInstalled(h)
	if err != nil {
		return fmt.Errorf("checking docker on host %s: %w", h.Name, err)
	}

	if hasCLI {
		out, verr := UnixDockerServerVersion(h)
		if verr == nil {
			fmt.Fprintf(m.out(), "  %s: Docker %s available\n", h.Name, out)
			fmt.Fprintf(m.out(), "  %s: pulling runner image...\n", h.Name)
			if _, err := dockerRun(h, fmt.Sprintf("docker pull %s", RunnerDockerImage)); err != nil {
				return fmt.Errorf("pulling Docker image: %w", err)
			}
			return nil
		}
		if h.OS == "darwin" {
			return fmt.Errorf(
				"docker not available on host %s: install Docker Desktop, OrbStack, or Colima and ensure the Docker CLI works in your SSH session: %w",
				h.Name, verr,
			)
		}
		return fmt.Errorf("docker on host %s is not usable: %w", h.Name, verr)
	}

	if h.OS == "darwin" {
		return fmt.Errorf(
			"docker not available on host %s: install Docker Desktop, OrbStack, or Colima and ensure the Docker CLI works in your SSH session",
			h.Name,
		)
	}

	fmt.Fprintf(m.out(), "  %s: Docker not found, attempting to install...\n", h.Name)
	installCmd := linuxElevatePrelude + `
			if ! command -v curl >/dev/null 2>&1; then
				if command -v apt-get >/dev/null 2>&1; then $SUDO apt-get update && $SUDO apt-get install -y curl;
				elif command -v yum >/dev/null 2>&1; then $SUDO yum install -y curl;
				elif command -v apk >/dev/null 2>&1; then $SUDO apk add curl;
				fi
			fi &&
			curl -fsSL https://get.docker.com | $SUDO sh
		`
	if _, instErr := h.Run(installCmd); instErr != nil {
		return fmt.Errorf(
			"failed to install docker on host %s (need root SSH, passwordless sudo, or install Docker manually; see ghr doctor): %w",
			h.Name, instErr,
		)
	}

	hasCLI, err = UnixDockerCLIInstalled(h)
	if err != nil {
		return fmt.Errorf("rechecking docker on host %s: %w", h.Name, err)
	}
	if !hasCLI {
		return fmt.Errorf("docker still not installed on host %s after installation attempt", h.Name)
	}
	out, verr := UnixDockerServerVersion(h)
	if verr != nil {
		return fmt.Errorf("docker installed on host %s but still not usable (e.g. add user to 'docker' group and reconnect SSH): %w", h.Name, verr)
	}
	fmt.Fprintf(m.out(), "  %s: Docker %s available\n", h.Name, out)

	fmt.Fprintf(m.out(), "  %s: pulling runner image...\n", h.Name)
	if _, err := dockerRun(h, fmt.Sprintf("docker pull %s", RunnerDockerImage)); err != nil {
		return fmt.Errorf("pulling Docker image: %w", err)
	}

	return nil
}

func (m *Manager) startDocker(h *host.Host, rc config.RunnerConfig, instanceName string) error {
	cname := containerName(instanceName)

	if h.OS == "windows" {
		running, _ := dockerRun(h, fmt.Sprintf(`docker inspect -f "{{.State.Running}}" %s 2>$null`, cname))
		if strings.TrimSpace(running) == "true" {
			fmt.Fprintf(m.out(), "  %s: already running\n", instanceName)
			return nil
		}
		dockerRunIgnoreErr(h, fmt.Sprintf("docker rm -f %s 2>$null", cname))
	} else {
		running, _ := dockerRun(h, fmt.Sprintf("docker inspect -f '{{.State.Running}}' %s 2>/dev/null", cname))
		if strings.TrimSpace(running) == "true" {
			fmt.Fprintf(m.out(), "  %s: already running\n", instanceName)
			return nil
		}
		dockerRunIgnoreErr(h, fmt.Sprintf("docker rm -f %s 2>/dev/null || true", cname))
	}

	regToken, err := m.GitHub.GetRegistrationToken(rc.Repo)
	if err != nil {
		return err
	}

	labels := strings.Join(rc.Labels, ",")
	repoURL := fmt.Sprintf("https://github.com/%s", rc.Repo)

	cmd := dockerStartCommand(cname, instanceName, regToken, repoURL, labels, dockerEngineSockBindMount(), RunnerDockerImage)

	out, err := dockerRun(h, cmd)
	if err != nil {
		return fmt.Errorf("starting Docker container: %w", err)
	}

	containerID := strings.TrimSpace(out)
	if len(containerID) > 12 {
		containerID = containerID[:12]
	}
	fmt.Fprintf(m.out(), "  %s: started (container %s)\n", instanceName, containerID)
	return nil
}

func (m *Manager) stopDocker(h *host.Host, instanceName string) error {
	cname := containerName(instanceName)

	var inspectCmd string
	if h.OS == "windows" {
		inspectCmd = fmt.Sprintf(`docker inspect -f "{{.State.Running}}" %s 2>$null`, cname)
	} else {
		inspectCmd = fmt.Sprintf("docker inspect -f '{{.State.Running}}' %s 2>/dev/null", cname)
	}
	running, _ := dockerRun(h, inspectCmd)
	if strings.TrimSpace(running) != "true" {
		fmt.Fprintf(m.out(), "  %s: not running\n", instanceName)
		return nil
	}

	if _, err := dockerRun(h, fmt.Sprintf("docker stop -t 30 %s", cname)); err != nil {
		return fmt.Errorf("stopping container: %w", err)
	}

	fmt.Fprintf(m.out(), "  %s: stopped\n", instanceName)
	return nil
}

func (m *Manager) removeDocker(h *host.Host, instanceName string) error {
	cname := containerName(instanceName)

	_ = m.stopDocker(h, instanceName)

	var rmCmd string
	if h.OS == "windows" {
		rmCmd = fmt.Sprintf("docker rm -f %s 2>$null", cname)
	} else {
		rmCmd = fmt.Sprintf("docker rm -f %s 2>/dev/null || true", cname)
	}
	if _, err := dockerRun(h, rmCmd); err != nil {
		return fmt.Errorf("removing container: %w", err)
	}

	fmt.Fprintf(m.out(), "  %s: removed\n", instanceName)
	return nil
}

func (m *Manager) statusDocker(h *host.Host, instanceName string) string {
	cname := containerName(instanceName)

	var inspectCmd string
	if h.OS == "windows" {
		inspectCmd = fmt.Sprintf(`docker inspect -f "{{.State.Status}}" %s 2>$null`, cname)
	} else {
		inspectCmd = fmt.Sprintf("docker inspect -f '{{.State.Status}}' %s 2>/dev/null", cname)
	}
	out, err := dockerRun(h, inspectCmd)
	if err != nil {
		return "not installed"
	}

	status := strings.TrimSpace(out)
	switch status {
	case "running":
		return "running"
	case "exited", "dead", "created":
		return "stopped"
	default:
		return status
	}
}

func (m *Manager) logsDocker(h *host.Host, instanceName string) (string, error) {
	cname := containerName(instanceName)
	cmd := fmt.Sprintf("docker logs --tail 50 %s 2>&1", cname)
	return dockerRun(h, cmd)
}

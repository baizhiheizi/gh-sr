package runner

import (
	"fmt"
	"strings"

	"github.com/an-lee/ghr/internal/config"
	"github.com/an-lee/ghr/internal/host"
)

const dockerImage = "ghcr.io/actions/actions-runner:latest"

func containerName(instanceName string) string {
	return "gh-runner-" + instanceName
}

// dockerRun executes a Docker CLI command on the host, using PowerShell
// wrapping on Windows and raw shell on Linux/macOS.
func dockerRun(h *host.Host, cmd string) (string, error) {
	if h.OS == "windows" {
		return h.RunShell(cmd)
	}
	return h.Run(cmd)
}

// dockerRunIgnoreErr is like dockerRun but discards the error (for best-effort cleanup).
func dockerRunIgnoreErr(h *host.Host, cmd string) {
	if h.OS == "windows" {
		h.RunShell(cmd)
	} else {
		h.Run(cmd)
	}
}

func (m *Manager) setupDocker(h *host.Host) error {
	if h.OS == "windows" {
		return m.setupDockerWindows(h)
	}
	return m.setupDockerUnix(h)
}

func (m *Manager) setupDockerWindows(h *host.Host) error {
	out, err := h.RunShell(`docker info --format "{{.ServerVersion}}"`)
	if err != nil || strings.TrimSpace(out) == "" {
		return fmt.Errorf("docker not available on host %s: install Docker Desktop and ensure it is running", h.Name)
	}
	fmt.Printf("  %s: Docker %s available\n", h.Name, strings.TrimSpace(out))

	fmt.Printf("  %s: pulling runner image...\n", h.Name)
	if _, err := h.RunShell(fmt.Sprintf("docker pull %s", dockerImage)); err != nil {
		return fmt.Errorf("pulling Docker image: %w", err)
	}

	return nil
}

func (m *Manager) setupDockerUnix(h *host.Host) error {
	out, err := h.Run("docker info --format '{{.ServerVersion}}' 2>/dev/null || echo 'not found'")
	if err != nil || strings.Contains(out, "not found") {
		if h.OS == "darwin" {
			return fmt.Errorf(
				"docker not available on host %s: install Docker Desktop, OrbStack, or Colima and ensure the Docker CLI works in your SSH session",
				h.Name,
			)
		}
		fmt.Printf("  %s: Docker not found, attempting to install...\n", h.Name)
		installCmd := `
			SUDO=''; if command -v sudo >/dev/null 2>&1 && [ "$(id -u)" -ne 0 ]; then SUDO=sudo; fi;
			if ! command -v curl >/dev/null 2>&1; then
				if command -v apt-get >/dev/null 2>&1; then $SUDO apt-get update && $SUDO apt-get install -y curl;
				elif command -v yum >/dev/null 2>&1; then $SUDO yum install -y curl;
				elif command -v apk >/dev/null 2>&1; then $SUDO apk add curl;
				fi
			fi &&
			curl -fsSL https://get.docker.com | $SUDO sh
		`
		if _, instErr := h.Run(installCmd); instErr != nil {
			return fmt.Errorf("failed to install docker on host %s: %w", h.Name, instErr)
		}

		out, err = h.Run("docker info --format '{{.ServerVersion}}' 2>/dev/null || echo 'not found'")
		if err != nil || strings.Contains(out, "not found") {
			return fmt.Errorf("docker still not available on host %s after installation attempt", h.Name)
		}
	}
	fmt.Printf("  %s: Docker %s available\n", h.Name, strings.TrimSpace(out))

	fmt.Printf("  %s: pulling runner image...\n", h.Name)
	if _, err := h.Run(fmt.Sprintf("docker pull %s", dockerImage)); err != nil {
		return fmt.Errorf("pulling Docker image: %w", err)
	}

	return nil
}

func (m *Manager) startDocker(h *host.Host, rc config.RunnerConfig, instanceName string) error {
	cname := containerName(instanceName)

	if h.OS == "windows" {
		running, _ := h.RunShell(fmt.Sprintf(`docker inspect -f "{{.State.Running}}" %s 2>$null`, cname))
		if strings.TrimSpace(running) == "true" {
			fmt.Printf("  %s: already running\n", instanceName)
			return nil
		}
		dockerRunIgnoreErr(h, fmt.Sprintf("docker rm -f %s 2>$null", cname))
	} else {
		running, _ := h.Run(fmt.Sprintf("docker inspect -f '{{.State.Running}}' %s 2>/dev/null", cname))
		if strings.TrimSpace(running) == "true" {
			fmt.Printf("  %s: already running\n", instanceName)
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

	sockMount := "-v /var/run/docker.sock:/var/run/docker.sock "
	if h.OS == "windows" {
		sockMount = ""
	}

	cmd := fmt.Sprintf(
		"docker run -d --name %s --restart unless-stopped "+
			"-e RUNNER_NAME=%s "+
			"-e RUNNER_TOKEN=%s "+
			"-e RUNNER_URL=%s "+
			"-e RUNNER_LABELS=%s "+
			"-e RUNNER_WORKDIR=_work "+
			"%s%s",
		cname, instanceName, regToken, repoURL, labels, sockMount, dockerImage,
	)

	out, err := dockerRun(h, cmd)
	if err != nil {
		return fmt.Errorf("starting Docker container: %w", err)
	}

	containerID := strings.TrimSpace(out)
	if len(containerID) > 12 {
		containerID = containerID[:12]
	}
	fmt.Printf("  %s: started (container %s)\n", instanceName, containerID)
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
		fmt.Printf("  %s: not running\n", instanceName)
		return nil
	}

	if _, err := dockerRun(h, fmt.Sprintf("docker stop -t 30 %s", cname)); err != nil {
		return fmt.Errorf("stopping container: %w", err)
	}

	fmt.Printf("  %s: stopped\n", instanceName)
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

	fmt.Printf("  %s: removed\n", instanceName)
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

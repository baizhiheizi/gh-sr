package runner

import (
	"fmt"
	"strings"

	"github.com/an-lee/gh-runners/internal/config"
	"github.com/an-lee/gh-runners/internal/host"
)

const dockerImage = "ghcr.io/actions/actions-runner:latest"

func containerName(instanceName string) string {
	return "gh-runner-" + instanceName
}

func (m *Manager) setupDocker(h *host.Host) error {
	out, err := h.Run("docker info --format '{{.ServerVersion}}' 2>/dev/null || echo 'not found'")
	if err != nil || strings.Contains(out, "not found") {
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
		
		// Verify installation
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

	running, _ := h.Run(fmt.Sprintf("docker inspect -f '{{.State.Running}}' %s 2>/dev/null", cname))
	if strings.TrimSpace(running) == "true" {
		fmt.Printf("  %s: already running\n", instanceName)
		return nil
	}

	// Remove any stopped container with the same name
	h.Run(fmt.Sprintf("docker rm -f %s 2>/dev/null || true", cname))

	regToken, err := m.GitHub.GetRegistrationToken(rc.Repo)
	if err != nil {
		return err
	}

	labels := strings.Join(rc.Labels, ",")
	repoURL := fmt.Sprintf("https://github.com/%s", rc.Repo)

	cmd := fmt.Sprintf(
		"docker run -d --name %s --restart unless-stopped "+
			"-e RUNNER_NAME=%s "+
			"-e RUNNER_TOKEN=%s "+
			"-e RUNNER_URL=%s "+
			"-e RUNNER_LABELS=%s "+
			"-e RUNNER_WORKDIR=_work "+
			"-v /var/run/docker.sock:/var/run/docker.sock "+
			"%s",
		cname, instanceName, regToken, repoURL, labels, dockerImage,
	)

	out, err := h.Run(cmd)
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

	running, _ := h.Run(fmt.Sprintf("docker inspect -f '{{.State.Running}}' %s 2>/dev/null", cname))
	if strings.TrimSpace(running) != "true" {
		fmt.Printf("  %s: not running\n", instanceName)
		return nil
	}

	if _, err := h.Run(fmt.Sprintf("docker stop -t 30 %s", cname)); err != nil {
		return fmt.Errorf("stopping container: %w", err)
	}

	fmt.Printf("  %s: stopped\n", instanceName)
	return nil
}

func (m *Manager) removeDocker(h *host.Host, instanceName string) error {
	cname := containerName(instanceName)

	_ = m.stopDocker(h, instanceName)

	if _, err := h.Run(fmt.Sprintf("docker rm -f %s 2>/dev/null || true", cname)); err != nil {
		return fmt.Errorf("removing container: %w", err)
	}

	fmt.Printf("  %s: removed\n", instanceName)
	return nil
}

func (m *Manager) statusDocker(h *host.Host, instanceName string) string {
	cname := containerName(instanceName)

	out, err := h.Run(fmt.Sprintf("docker inspect -f '{{.State.Status}}' %s 2>/dev/null", cname))
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
	return h.Run(fmt.Sprintf("docker logs --tail 50 %s 2>&1", cname))
}

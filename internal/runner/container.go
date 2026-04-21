package runner

import (
	"fmt"
	"strings"

	"github.com/an-lee/gh-sr/internal/config"
	"github.com/an-lee/gh-sr/internal/host"
)

// AgenticRunnerImageTag is the local Docker image tag built by gh sr setup.
const AgenticRunnerImageTag = "gh-sr/agentic-runner"

// ContainerDockerName returns the deterministic Docker container name for a runner instance.
func ContainerDockerName(instanceName string) string {
	return "gh-sr-" + instanceName
}

func containerName(instanceName string) string {
	return ContainerDockerName(instanceName)
}

// containerStateDir returns the host-side bind-mount path for runner instance state.
// All runner-state (Docker layer cache, work dirs, logs) is persisted here so the
// container can be destroyed and re-created without losing layer caches.
func containerStateDir(h *host.Host, instanceName string) string {
	return h.RunnerDir(instanceName)
}

// resolveAbsoluteRunnerDir returns the absolute (non-variable) path for the runner
// state directory on the host by expanding $HOME to its real value.
// h.RunnerDir returns a shell-variable string ("$HOME/...") which must not appear
// inside single quotes in Docker arguments — Docker does not perform shell expansion.
func resolveAbsoluteRunnerDir(h *host.Host, instanceName string) (string, error) {
	dir := h.RunnerDir(instanceName)
	if !strings.HasPrefix(dir, "$HOME") {
		return dir, nil
	}
	out, err := h.Run("echo $HOME")
	if err != nil {
		return "", fmt.Errorf("resolving home dir: %w", err)
	}
	home := strings.TrimSpace(out)
	return home + dir[len("$HOME"):], nil
}

// containerRunnerPresent returns true when the Docker container for the instance exists
// (regardless of whether it is running or stopped).
func containerRunnerPresent(h *host.Host, instanceName string) bool {
	name := containerName(instanceName)
	out, err := h.Run(fmt.Sprintf(
		"docker inspect --format='{{.Name}}' %s 2>/dev/null | grep -q '^/%s$' && echo yes || echo no",
		name, name,
	))
	if err != nil {
		return false
	}
	return strings.TrimSpace(out) == "yes"
}

// setupContainer builds the gh-sr container runner image (if not already up to date)
// and creates (but does not start) each runner container.
func (m *Manager) setupContainer(h *host.Host, rc config.RunnerConfig) error {
	if h.OS != "linux" {
		return fmt.Errorf("runner_mode: container is only supported on Linux hosts")
	}

	// Resolve runner version for image build-arg.
	version, err := m.GitHub.GetLatestRunnerVersion()
	if err != nil {
		return fmt.Errorf("resolving runner version: %w", err)
	}

	arch := archForGitHub(h.Arch)

	// Write the Dockerfile and entrypoint to a temp dir on the host, then build.
	imageTag := fmt.Sprintf("%s:%s", AgenticRunnerImageTag, version)

	fmt.Fprintf(m.out(), "  %s: checking container runner image %s...\n", rc.Name, imageTag)

	imageExists, err := containerImageExists(h, imageTag)
	if err != nil {
		return fmt.Errorf("checking image: %w", err)
	}

	if !imageExists {
		fmt.Fprintf(m.out(), "  %s: building container runner image (this may take several minutes)...\n", rc.Name)
		if err := buildAgenticRunnerImage(h, imageTag, version, arch); err != nil {
			return fmt.Errorf("building container runner image: %w", err)
		}
		fmt.Fprintf(m.out(), "  %s: image built: %s\n", rc.Name, imageTag)
	} else {
		fmt.Fprintf(m.out(), "  %s: image already up to date\n", rc.Name)
	}

	for i, name := range rc.InstanceNames() {
		if containerRunnerPresent(h, name) {
			fmt.Fprintf(m.out(), "  %s: container already exists, skipping\n", name)
			continue
		}

		fmt.Fprintf(m.out(), "  %s: creating runner container...\n", name)

		regToken, err := m.GitHub.GetRegistrationTokenScoped(rc.Scope(), rc.ScopeTarget())
		if err != nil {
			return fmt.Errorf("getting registration token for %s: %w", name, err)
		}

		stateDir, err := resolveAbsoluteRunnerDir(h, name)
		if err != nil {
			return fmt.Errorf("resolving state dir for %s: %w", name, err)
		}
		labels := rc.EffectiveLabelsForInstance(h.OS, h.Arch, i)

		runURL := ""
		if rc.Repo != "" {
			runURL = "https://github.com/" + rc.Repo
		} else if rc.Org != "" {
			runURL = "https://github.com/" + rc.Org
		}

		group := rc.Group
		if group == "" {
			group = "Default"
		}

		ephemeral := ""
		if rc.Ephemeral {
			ephemeral = "true"
		}

		// Build the `docker create` command. We use `--restart unless-stopped`
		// so the runner auto-starts on host reboot and auto-restarts after a job.
		// `--privileged` is required for DinD (inner dockerd needs full capabilities).
		cmd := fmt.Sprintf(`
mkdir -p %s
docker create \
  --name %s \
  --privileged \
  --restart unless-stopped \
  -v %s:/runner-state \
  -e GH_SR_RUNNER_NAME=%s \
  -e GH_SR_RUNNER_TOKEN=%s \
  -e GH_SR_RUNNER_URL=%s \
  -e GH_SR_RUNNER_LABELS=%s \
  -e GH_SR_RUNNER_GROUP=%s \
  -e GH_SR_RUNNER_EPHEMERAL=%s \
  %s`,
			posixSingleQuote(stateDir),
			posixSingleQuote(containerName(name)),
			posixSingleQuote(stateDir),
			posixSingleQuote(name),
			posixSingleQuote(regToken),
			posixSingleQuote(runURL),
			posixSingleQuote(strings.Join(labels, ",")),
			posixSingleQuote(group),
			posixSingleQuote(ephemeral),
			posixSingleQuote(imageTag),
		)

		if _, err := h.Run(cmd); err != nil {
			return fmt.Errorf("creating container %s: %w", containerName(name), err)
		}

		fmt.Fprintf(m.out(), "  %s: container created (run `gh sr up` to start)\n", name)
	}

	return nil
}

// startContainer starts an existing runner container (docker start).
func (m *Manager) startContainer(h *host.Host, instanceName string) error {
	name := containerName(instanceName)
	if _, err := h.Run(fmt.Sprintf("docker start %s", name)); err != nil {
		return fmt.Errorf("starting container %s: %w", name, err)
	}
	return nil
}

// stopContainer stops a running runner container (docker stop).
func (m *Manager) stopContainer(h *host.Host, instanceName string) error {
	name := containerName(instanceName)
	if _, err := h.Run(fmt.Sprintf("docker stop %s 2>/dev/null || true", name)); err != nil {
		return fmt.Errorf("stopping container %s: %w", name, err)
	}
	return nil
}

// removeContainer stops and removes a runner container plus its state directory.
func (m *Manager) removeContainer(h *host.Host, rc config.RunnerConfig, instanceName string) error {
	cName := containerName(instanceName)

	// Best-effort deregister from GitHub first.
	removeTok, err := m.GitHub.GetRemovalTokenScoped(rc.Scope(), rc.ScopeTarget())
	if err == nil {
		// Run inside the container if it's still alive; ignore errors.
		_, _ = h.Run(fmt.Sprintf(
			"docker exec %s su - runner -c \"cd /home/runner/actions-runner && ./config.sh remove --token %s\" 2>/dev/null || true",
			cName, posixSingleQuote(removeTok),
		))
	}

	// Stop then remove the container.
	_, _ = h.Run(fmt.Sprintf("docker stop %s 2>/dev/null || true", cName))
	if _, err := h.Run(fmt.Sprintf("docker rm -f %s 2>/dev/null || true", cName)); err != nil {
		return fmt.Errorf("removing container %s: %w", cName, err)
	}

	// Remove state directory.
	stateDir, resolveErr := resolveAbsoluteRunnerDir(h, instanceName)
	if resolveErr != nil {
		// Fall back to unresolved path; rm -rf in the shell will still expand $HOME.
		stateDir = containerStateDir(h, instanceName)
	}
	if _, err := h.Run(fmt.Sprintf("rm -rf %s", posixSingleQuote(stateDir))); err != nil {
		fmt.Fprintf(m.out(), "  %s: warning: failed to remove state dir %s: %v\n", instanceName, stateDir, err)
	}

	return nil
}

// statusContainer returns the local status of a runner container.
func (m *Manager) statusContainer(h *host.Host, instanceName string) string {
	name := containerName(instanceName)
	out, err := h.Run(fmt.Sprintf(
		"docker inspect --format='{{.State.Status}}' %s 2>/dev/null || echo 'not installed'",
		name,
	))
	if err != nil {
		return "not installed"
	}
	status := strings.TrimSpace(out)
	switch status {
	case "running":
		return "running"
	case "not installed":
		return "not installed"
	default:
		// exited, created, paused, restarting, etc.
		return "stopped"
	}
}

// logsContainer returns recent log lines from a runner container.
func (m *Manager) logsContainer(h *host.Host, instanceName string) (string, error) {
	name := containerName(instanceName)
	out, err := h.Run(fmt.Sprintf("docker logs --tail 100 %s 2>&1 || echo 'no logs found'", name))
	if err != nil {
		return "", fmt.Errorf("fetching logs for container %s: %w", name, err)
	}
	return out, nil
}

// rebuildContainerImage tears down all containers for a runner group, removes
// the old agentic-runner image, rebuilds it from the embedded sources, recreates
// the containers, and starts them. The runner state directories (including the
// .runner registration file) are intentionally preserved so the runners do not
// re-register with GitHub on next start.
func (m *Manager) rebuildContainerImage(h *host.Host, rc config.RunnerConfig) error {
	if h.OS != "linux" {
		return fmt.Errorf("runner_mode: container is only supported on Linux hosts")
	}

	// Stop and remove containers (keep state dirs so .runner persists).
	for _, name := range rc.InstanceNames() {
		cName := containerName(name)
		fmt.Fprintf(m.out(), "  %s: stopping container...\n", name)
		_, _ = h.Run(fmt.Sprintf("docker stop %s 2>/dev/null || true", cName))
		fmt.Fprintf(m.out(), "  %s: removing container...\n", name)
		_, _ = h.Run(fmt.Sprintf("docker rm -f %s 2>/dev/null || true", cName))
	}

	// Remove all local gh-sr/agentic-runner images so the build is forced.
	fmt.Fprintf(m.out(), "  %s: removing old container runner image(s)...\n", rc.Name)
	_, _ = h.Run(fmt.Sprintf(
		"docker images %s -q | xargs -r docker rmi -f 2>/dev/null || true",
		posixSingleQuote(AgenticRunnerImageTag),
	))

	// Resolve runner version and architecture for the build.
	version, err := m.GitHub.GetLatestRunnerVersion()
	if err != nil {
		return fmt.Errorf("resolving runner version: %w", err)
	}
	arch := archForGitHub(h.Arch)
	imageTag := fmt.Sprintf("%s:%s", AgenticRunnerImageTag, version)

	fmt.Fprintf(m.out(), "  %s: building container runner image %s (this may take several minutes)...\n", rc.Name, imageTag)
	if err := buildAgenticRunnerImage(h, imageTag, version, arch); err != nil {
		return fmt.Errorf("building container runner image: %w", err)
	}
	fmt.Fprintf(m.out(), "  %s: image built: %s\n", rc.Name, imageTag)

	// Recreate and start each container. Because state dirs still exist on the
	// host (bind-mounted at /runner-state), the entrypoint will find .runner and
	// skip config.sh, so no new registration token is consumed on start.
	if err := m.setupContainer(h, rc); err != nil {
		return err
	}
	for _, name := range rc.InstanceNames() {
		fmt.Fprintf(m.out(), "  %s: starting container...\n", name)
		if err := m.startContainer(h, name); err != nil {
			return err
		}
	}
	return nil
}

// needsSetupContainer reports whether any instance container is missing.
func (m *Manager) needsSetupContainer(h *host.Host, rc config.RunnerConfig) bool {
	for _, name := range rc.InstanceNames() {
		if !containerRunnerPresent(h, name) {
			return true
		}
	}
	return false
}

// containerImageExists checks whether a Docker image with the given tag exists on the host.
func containerImageExists(h *host.Host, imageTag string) (bool, error) {
	out, err := h.Run(fmt.Sprintf(
		"docker image inspect %s --format='{{.Id}}' 2>/dev/null | grep -q . && echo yes || echo no",
		posixSingleQuote(imageTag),
	))
	if err != nil {
		return false, nil
	}
	return strings.TrimSpace(out) == "yes", nil
}

// buildAgenticRunnerImage uploads the embedded Dockerfile+entrypoint to the host
// and builds the image via `docker build`.
func buildAgenticRunnerImage(h *host.Host, imageTag, runnerVersion, runnerArch string) error {
	buildDir := "/tmp/gh-sr-agentic-runner-build"

	// Write Dockerfile.
	dfPath := buildDir + "/Dockerfile"
	writeDockerfile := fmt.Sprintf(`
mkdir -p %s
cat > %s << 'GHSR_EOF'
%s
GHSR_EOF`,
		buildDir,
		dfPath,
		// Escape any occurrence of GHSR_EOF in the content to prevent heredoc injection.
		strings.ReplaceAll(agenticRunnerDockerfile, "GHSR_EOF", "GHSR_E0F"),
	)
	if _, err := h.Run(writeDockerfile); err != nil {
		return fmt.Errorf("writing Dockerfile: %w", err)
	}

	// Write entrypoint.sh.
	epPath := buildDir + "/entrypoint.sh"
	writeEntrypoint := fmt.Sprintf(`cat > %s << 'GHSR_EOF'
%s
GHSR_EOF
chmod +x %s`,
		epPath,
		strings.ReplaceAll(agenticRunnerEntrypoint, "GHSR_EOF", "GHSR_E0F"),
		epPath,
	)
	if _, err := h.Run(writeEntrypoint); err != nil {
		return fmt.Errorf("writing entrypoint.sh: %w", err)
	}

	// Write docker-wrapper.sh.
	wrapperPath := buildDir + "/docker-wrapper.sh"
	writeWrapper := fmt.Sprintf(`cat > %s << 'GHSR_EOF'
%s
GHSR_EOF
chmod +x %s`,
		wrapperPath,
		strings.ReplaceAll(agenticRunnerDockerWrapper, "GHSR_EOF", "GHSR_E0F"),
		wrapperPath,
	)
	if _, err := h.Run(writeWrapper); err != nil {
		return fmt.Errorf("writing docker-wrapper.sh: %w", err)
	}

	// Build.
	buildCmd := fmt.Sprintf(
		"docker build --build-arg RUNNER_VERSION=%s --build-arg RUNNER_ARCH=%s -t %s %s",
		posixSingleQuote(runnerVersion),
		posixSingleQuote(runnerArch),
		posixSingleQuote(imageTag),
		posixSingleQuote(buildDir),
	)
	if _, err := h.Run(buildCmd); err != nil {
		return fmt.Errorf("docker build: %w", err)
	}

	// Clean up build context.
	_, _ = h.Run(fmt.Sprintf("rm -rf %s", buildDir))

	return nil
}

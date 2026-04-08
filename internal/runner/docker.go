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

// ContainerName returns the Docker container name for a runner instance.
func ContainerName(instanceName string) string {
	return "gh-runner-" + instanceName
}

func containerName(instanceName string) string {
	return ContainerName(instanceName)
}

// shellSingleQuote wraps s in single quotes for a POSIX shell word (safe for docker -e on Linux/macOS SSH).
func shellSingleQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
}

// dockerEnvFlag renders one -e flag as a single shell word: -e 'NAME=value'
func dockerEnvFlag(name, value string) string {
	return "-e " + shellSingleQuote(name+"="+value)
}

// dockerCapAddFlags renders zero or more --cap-add flags for docker run (Linux capability names, e.g. NET_ADMIN).
func dockerCapAddFlags(capAdds []string) string {
	if len(capAdds) == 0 {
		return ""
	}
	var b strings.Builder
	for _, c := range capAdds {
		c = strings.TrimSpace(c)
		if c == "" {
			continue
		}
		fmt.Fprintf(&b, "--cap-add %s ", c)
	}
	return b.String()
}

type dockerStartOpts struct {
	ContainerName string
	InstanceName  string
	RegToken      string
	RepoURL       string
	Labels        string
	SockMount     string
	Image         string
	NetworkMode   string
	CapAdds       []string
	RunnerGroup   string
	Ephemeral     bool
}

func dockerStartCommand(opts dockerStartOpts) string {
	var b strings.Builder
	fmt.Fprintf(&b, "docker run -d --name %s ", opts.ContainerName)
	if strings.TrimSpace(opts.NetworkMode) == "host" {
		b.WriteString("--network host ")
	}
	restartPolicy := "unless-stopped"
	if opts.Ephemeral {
		restartPolicy = "no"
	}
	fmt.Fprintf(&b, "--restart %s ", restartPolicy)
	b.WriteString(dockerCapAddFlags(opts.CapAdds))
	b.WriteString(dockerEnvFlag("ACTIONS_RUNNER_INPUT_URL", opts.RepoURL))
	b.WriteByte(' ')
	b.WriteString(dockerEnvFlag("ACTIONS_RUNNER_INPUT_TOKEN", opts.RegToken))
	b.WriteByte(' ')
	b.WriteString(dockerEnvFlag("ACTIONS_RUNNER_INPUT_NAME", opts.InstanceName))
	b.WriteByte(' ')
	b.WriteString(dockerEnvFlag("ACTIONS_RUNNER_INPUT_LABELS", opts.Labels))
	b.WriteByte(' ')
	b.WriteString(dockerEnvFlag("ACTIONS_RUNNER_INPUT_WORK", "_work"))
	b.WriteByte(' ')
	if opts.RunnerGroup != "" {
		b.WriteString(dockerEnvFlag("ACTIONS_RUNNER_INPUT_RUNNERGROUP", opts.RunnerGroup))
		b.WriteByte(' ')
	}
	if opts.Ephemeral {
		b.WriteString(dockerEnvFlag("ACTIONS_RUNNER_INPUT_EPHEMERAL", "true"))
		b.WriteByte(' ')
	}
	if s := strings.TrimSpace(opts.SockMount); s != "" {
		b.WriteString(s)
		b.WriteByte(' ')
	}
	b.WriteString("--entrypoint /bin/bash ")
	b.WriteString(opts.Image)
	b.WriteString(" -c ")
	b.WriteString(shellSingleQuote(dockerRunnerEntryScript))
	return b.String()
}

// DefaultDockerSocket is the conventional Docker daemon socket path on Linux and macOS hosts.
const DefaultDockerSocket = "/var/run/docker.sock"

// defaultDockerSocket is an unexported alias kept for internal use within this package.
const defaultDockerSocket = DefaultDockerSocket

// socketPathFromDockerContextHost returns the filesystem path when endpoint is a unix:// Docker API URL.
func socketPathFromDockerContextHost(endpoint string) (path string, ok bool) {
	endpoint = strings.TrimSpace(endpoint)
	if endpoint == "" {
		return "", false
	}
	if !strings.HasPrefix(endpoint, "unix://") {
		return "", false
	}
	path = strings.TrimPrefix(endpoint, "unix://")
	if path == "" || path[0] != '/' {
		return "", false
	}
	return path, true
}

func remoteDockerSocketOK(h *host.Host, path string) (bool, error) {
	q := shellSingleQuote(path)
	out, err := h.Run(fmt.Sprintf("test -S %s && echo ok || echo missing", q))
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(out) == "ok", nil
}

// EffectiveDockerSocket returns the host-side Docker socket path for bind-mounting on Linux and macOS.
// When HostConfig.DockerSocket is set, that path is used if it exists. Otherwise ghr probes, in order:
// /var/run/docker.sock, the active docker context unix endpoint (Colima, rootless Docker, etc.),
// and on macOS ~/.colima/default/docker.sock.
func EffectiveDockerSocket(h *host.Host) (string, error) {
	if h.OS != "linux" && h.OS != "darwin" {
		return "", fmt.Errorf("EffectiveDockerSocket: unsupported OS %q", h.OS)
	}
	if p := strings.TrimSpace(h.DockerSocket); p != "" {
		ok, err := remoteDockerSocketOK(h, p)
		if err != nil {
			return "", fmt.Errorf("checking docker_socket on host %s: %w", h.Name, err)
		}
		if !ok {
			return "", fmt.Errorf("docker_socket not found or not a socket at %s on host %s", p, h.Name)
		}
		return p, nil
	}
	ok, err := remoteDockerSocketOK(h, defaultDockerSocket)
	if err != nil {
		return "", fmt.Errorf("checking Docker socket on host %s: %w", h.Name, err)
	}
	if ok {
		return defaultDockerSocket, nil
	}
	ctxOut, derr := dockerRun(h, `docker context inspect -f '{{.Endpoints.docker.Host}}'`)
	if derr == nil {
		if p, parseOK := socketPathFromDockerContextHost(ctxOut); parseOK {
			ok2, err2 := remoteDockerSocketOK(h, p)
			if err2 != nil {
				return "", fmt.Errorf("checking Docker socket from context on host %s: %w", h.Name, err2)
			}
			if ok2 {
				return p, nil
			}
		}
	}
	if h.OS == "darwin" {
		out, rerr := h.Run(prependDarwinDockerPATH(h, `p="$HOME/.colima/default/docker.sock"; if test -S "$p"; then printf '%s' "$p"; fi`))
		if rerr == nil {
			if p := strings.TrimSpace(out); p != "" {
				return p, nil
			}
		}
	}
	hint := "ensure Docker is running (rootless Docker: ghr uses your default docker context; set docker_socket to override)"
	if h.OS == "darwin" {
		hint = "ensure Docker Desktop/OrbStack/Colima is running, or set docker_socket for a non-default socket path"
	}
	return "", fmt.Errorf("no usable Docker socket found on host %s; %s", h.Name, hint)
}

// dockerEngineSockFlags returns the docker run flags needed to expose the host Docker socket inside
// a Linux actions-runner container and grant the container's runner user access to it.
//
// It returns both a -v bind-mount and a --group-add <GID> flag. The GID is read from the host
// socket's owning group via `stat -c '%g'`; this avoids "permission denied" when the container's
// runner user (uid 1001) is not in the host docker group. If the GID cannot be determined, only
// the mount is returned (best-effort; caller may still get EACCES at runtime).
//
// socketPath is the host-side socket (from HostConfig.DockerSocket, defaulting to /var/run/docker.sock).
// The container-side path is always /var/run/docker.sock so job scripts use the default DOCKER_HOST.
func dockerEngineSockFlags(h *host.Host, socketPath string) string {
	if socketPath == "" {
		socketPath = defaultDockerSocket
	}
	mount := fmt.Sprintf("-v %s:/var/run/docker.sock ", socketPath)

	// Query the GID that owns the socket on the host so we can pass --group-add.
	out, err := h.Run(fmt.Sprintf("stat -c '%%g' %s 2>/dev/null", socketPath))
	gid := strings.TrimSpace(out)
	if err != nil || gid == "" || gid == "0" {
		// GID 0 means root owns it and no special group; skip --group-add (no benefit or unknown).
		return mount
	}
	return mount + fmt.Sprintf("--group-add %s ", gid)
}

// darwinDockerBindSourcePath returns the filesystem path to use as the host side of
// docker run -v …:/var/run/docker.sock on macOS. Colima exposes the API at
// ~/.colima/.../docker.sock on the macOS host, but bind-mounting that path fails with
// virtiofs (dockerd mkdir on the socket path: operation not supported; see
// https://github.com/abiosoft/colima/issues/997). The same daemon accepts
// /var/run/docker.sock (VM-local). Caller must only use this when h.OS == "darwin".
func darwinDockerBindSourcePath(resolvedSocket string) string {
	if resolvedSocket == "" {
		return resolvedSocket
	}
	if strings.Contains(resolvedSocket, "/.colima/") && strings.HasSuffix(resolvedSocket, "docker.sock") {
		return defaultDockerSocket
	}
	return resolvedSocket
}

// darwinDockerSockFlags returns the docker run flags to bind-mount the Docker socket on a macOS
// host. On macOS (Docker Desktop, OrbStack, Colima) the socket is accessible to all processes
// inside the VM — there is no docker group GID mismatch — so only the -v mount is needed.
//
// socketPath is the host-side socket (from HostConfig.DockerSocket, defaulting to /var/run/docker.sock).
func darwinDockerSockFlags(socketPath string) string {
	if socketPath == "" {
		socketPath = defaultDockerSocket
	}
	return fmt.Sprintf("-v %s:/var/run/docker.sock ", socketPath)
}

// dockerWindowsEngineSockMount is the bind-mount for the Docker engine socket on Windows hosts
// (Docker Desktop Linux engine; path is resolved inside the Hyper-V/WSL2 VM).
const dockerWindowsEngineSockMount = "-v /var/run/docker.sock:/var/run/docker.sock "

// DockerWindowsSockGIDProbeCommand returns the docker CLI line to read the owning GID of
// /var/run/docker.sock inside the Docker Desktop Linux engine. Uses sh+stat because the runner
// image has no one-shot default CMD suitable for stat.
func DockerWindowsSockGIDProbeCommand(image string) string {
	return fmt.Sprintf(
		`docker run --rm -v /var/run/docker.sock:/var/run/docker.sock --entrypoint sh %s -c "stat -c '%%g' /var/run/docker.sock"`,
		image,
	)
}

// appendGroupAddForDockerSockGID appends --group-add when probe output is a non-empty numeric GID.
// Even GID 0 (root) is added because the docker socket is often owned by root:root on Docker Desktop,
// and adding GID 0 as a supplemental group grants the container's runner user access to it.
func appendGroupAddForDockerSockGID(mount, gidProbeOutput string) string {
	gid := strings.TrimSpace(gidProbeOutput)
	if gid == "" {
		return mount
	}
	for _, c := range gid {
		if c < '0' || c > '9' {
			return mount
		}
	}
	return mount + fmt.Sprintf("--group-add %s ", gid)
}

// dockerEngineSockFlagsWindows returns docker run flags for the Docker socket on a Windows host
// running Docker Desktop (Linux containers mode). The socket lives in the Linux engine VM; we
// probe its group GID via a disposable container because Windows has no Unix stat on that path.
func dockerEngineSockFlagsWindows(h *host.Host) string {
	out, err := dockerRun(h, DockerWindowsSockGIDProbeCommand(RunnerDockerImage)+` 2>$null`)
	if err != nil {
		return dockerWindowsEngineSockMount
	}
	return appendGroupAddForDockerSockGID(dockerWindowsEngineSockMount, out)
}

// dockerEngineSockPreflightCheck verifies the socket path is present and is a socket on the host
// before docker run. Returns an error with an actionable message if the socket is missing.
func dockerEngineSockPreflightCheck(h *host.Host, socketPath string) error {
	if socketPath == "" {
		socketPath = defaultDockerSocket
	}
	out, err := h.Run(fmt.Sprintf("test -S %s && echo ok || echo missing", socketPath))
	if err != nil {
		return fmt.Errorf("checking Docker socket %s on host: %w", socketPath, err)
	}
	if strings.TrimSpace(out) != "ok" {
		msg := fmt.Sprintf(
			"Docker socket not found at %s on host %s; ensure Docker is installed and running (or set docker_socket in config for rootless Docker)",
			socketPath, h.Name,
		)
		return fmt.Errorf("%s", msg)
	}
	return nil
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

	// Auto-pull image if not present locally.
	imgCheck := fmt.Sprintf("docker images -q %s", RunnerDockerImage)
	if h.OS == "windows" {
		imgCheck += " 2>$null"
	} else {
		imgCheck += " 2>/dev/null"
	}
	imgOut, _ := dockerRun(h, imgCheck)
	if strings.TrimSpace(imgOut) == "" {
		fmt.Fprintf(m.out(), "  %s: pulling runner image...\n", instanceName)
		if _, pullErr := dockerRun(h, fmt.Sprintf("docker pull %s", RunnerDockerImage)); pullErr != nil {
			return fmt.Errorf("pulling Docker image for %s: %w", instanceName, pullErr)
		}
	}

	regToken, err := m.GitHub.GetRegistrationTokenScoped(rc.Scope(), rc.ScopeTarget())
	if err != nil {
		return err
	}

	labels := strings.Join(rc.EffectiveLabels(h.OS, h.Arch), ",")
	var repoURL string
	if rc.Org != "" {
		repoURL = fmt.Sprintf("https://github.com/%s", rc.Org)
	} else {
		repoURL = fmt.Sprintf("https://github.com/%s", rc.Repo)
	}

	var sockFlags string
	switch h.OS {
	case "linux", "darwin":
		sockPath, err := EffectiveDockerSocket(h)
		if err != nil {
			return err
		}
		if h.OS == "linux" {
			sockFlags = dockerEngineSockFlags(h, sockPath)
		} else {
			sockFlags = darwinDockerSockFlags(darwinDockerBindSourcePath(sockPath))
		}
	case "windows":
		// Docker Desktop (Linux containers mode): bind-mount the engine socket and match Linux
		// behavior by adding --group-add for the socket's GID when we can probe it from a disposable container.
		sockFlags = dockerEngineSockFlagsWindows(h)
	}

	cmd := dockerStartCommand(dockerStartOpts{
		ContainerName: cname,
		InstanceName:  instanceName,
		RegToken:      regToken,
		RepoURL:       repoURL,
		Labels:        labels,
		SockMount:     sockFlags,
		Image:         RunnerDockerImage,
		NetworkMode:   rc.EffectiveDockerNetworkMode(h.OS),
		CapAdds:       rc.DockerCapAdd,
		RunnerGroup:   rc.Group,
		Ephemeral:     rc.Ephemeral,
	})

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

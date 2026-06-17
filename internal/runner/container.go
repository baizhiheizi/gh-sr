package runner

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/an-lee/gh-sr/internal/config"
	"github.com/an-lee/gh-sr/internal/host"
	"github.com/an-lee/gh-sr/internal/hostshell"
)

// AgenticRunnerImageTag is the local Docker image tag built by gh sr setup.
const AgenticRunnerImageTag = "gh-sr/agentic-runner"

// Docker image labels stamped at build time (see buildAgenticRunnerImage).
const (
	dockerLabelImageRevision = "gh-sr.image-revision"
	dockerLabelCLIVersion    = "gh-sr.cli-version"
)

// Host-side marker files under the runner state bind-mount (/runner-state in container).
const (
	bootstrapFailedMarker    = "bootstrap-failed"
	dockerdStartFailuresFile = "dockerd-start-failures"
)

// ContainerImageLayoutRevision returns a short hex fingerprint of the embedded
// container image layout (Dockerfile, manifests, entrypoint, wrapper), gh-sr
// CLI version, and extra apt package list. It changes when any of those inputs change.
func ContainerImageLayoutRevision(ghSrVersion string, extraApt []string) string {
	if ghSrVersion == "" {
		ghSrVersion = "unknown"
	}
	var b strings.Builder
	b.WriteString("gh-sr-container-image/v1\x00")
	b.WriteString(ghSrVersion)
	b.WriteByte(0)
	for _, p := range containerRunnerImageExtraSorted(extraApt) {
		b.WriteString(p)
		b.WriteByte('\n')
	}
	b.WriteString(agenticRunnerDockerfile)
	b.WriteString(agenticRunnerAptPackagesCore)
	b.WriteString(agenticRunnerEntrypoint)
	b.WriteString(agenticRunnerDockerWrapper)
	b.WriteString(agenticRunnerDaemonJSON)
	b.WriteString(agenticRunnerDnsmasqConf)
	b.WriteString(agenticRunnerJobStartedHook)
	b.WriteString(agenticRunnerJobCompletedHook)
	sum := sha256.Sum256([]byte(b.String()))
	return hex.EncodeToString(sum[:])[:12]
}

// containerRunnerImageExtraSorted returns a sorted copy of unique non-empty package names.
func containerRunnerImageExtraSorted(extra []string) []string {
	seen := make(map[string]struct{})
	var out []string
	for _, p := range extra {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		if _, ok := seen[p]; ok {
			continue
		}
		seen[p] = struct{}{}
		out = append(out, p)
	}
	slices.Sort(out)
	return out
}

// ContainerRunnerImageTag returns the Docker image reference for the container runner
// (e.g. gh-sr/agentic-runner:2.320.0 or gh-sr/agentic-runner:2.320.0-xa1b2c3d when extras are set).
func ContainerRunnerImageTag(actionsRunnerVersion string, extraApt []string) string {
	base := fmt.Sprintf("%s:%s", AgenticRunnerImageTag, actionsRunnerVersion)
	sorted := containerRunnerImageExtraSorted(extraApt)
	if len(sorted) == 0 {
		return base
	}
	sum := sha256.Sum256([]byte(strings.Join(sorted, "\n")))
	suffix := hex.EncodeToString(sum[:])[:8]
	return base + "-x" + suffix
}

// ContainerDockerName returns the deterministic Docker container name for a runner instance.
func ContainerDockerName(instanceName string) string {
	return "gh-sr-" + instanceName
}

func containerName(instanceName string) string {
	return ContainerDockerName(instanceName)
}

// containerStateDir returns the host-side bind-mount path for runner instance state
// (mounted at /runner-state inside the container).
//
// Cache vs. per-job scratch separation:
//   - PERSISTENT cache: /runner-state/docker-data holds the inner Docker image-layer
//     cache. It survives container restarts and per-job resets so jobs never re-pull
//     gh-aw's (large) images. The per-job reset hooks never prune images/volumes.
//   - PER-JOB scratch: the gh-aw runtime tree (/tmp/gh-aw, inside the container rootfs),
//     leftover inner containers/networks, and AWF iptables rules are wiped before and
//     after every job by /opt/gh-sr/hooks/job-{started,completed}.sh, so each job starts
//     from a pristine inner environment.
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

	imageTag := ContainerRunnerImageTag(version, m.containerImageExtraApt())

	fmt.Fprintf(m.out(), "  %s: checking container runner image %s...\n", rc.Name, imageTag)

	imageExists, err := containerImageExists(h, imageTag)
	if err != nil {
		return fmt.Errorf("checking image: %w", err)
	}

	if !imageExists {
		fmt.Fprintf(m.out(), "  %s: building container runner image (this may take several minutes)...\n", rc.Name)
		if err := buildAgenticRunnerImage(h, imageTag, version, arch, m.GhSrVersion, m.containerImageExtraApt()); err != nil {
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
		if err := m.createContainerInstance(h, rc, i, name, imageTag); err != nil {
			return err
		}
		fmt.Fprintf(m.out(), "  %s: container created (run `gh sr up` to start)\n", name)
	}

	return nil
}

// DetectHostEgressMTU returns the MTU of the host's primary egress interface (the one
// routing to the public internet), or 0 if it cannot be determined. It is used to pin
// the runner container's inner/outer Docker MTU to the host's real MTU so large-packet
// TLS handshakes survive a reduced host path MTU (cloud overlay networks such as GCP's
// 1460 default, VPN/WireGuard, nested virtualisation). Without this, the outer container
// and inner dockerd bridge keep Docker's 1500 default; when the host path MTU is smaller
// and PMTUD is black-holed, large packets are dropped and downloads like actions/setup-go
// fail with "Client network socket disconnected before secure TLS connection was
// established" while the host downloads fine. Linux-only (container mode is Linux-only).
func DetectHostEgressMTU(h *host.Host) int {
	if h == nil || h.OS != "linux" {
		return 0
	}
	// Resolve the egress interface (route to a public IP, then default route), then read
	// its MTU from sysfs. Failures yield empty output → 0 (caller keeps Docker's default).
	out, err := h.Run(`iface=$(ip -o route get 1.1.1.1 2>/dev/null | awk '{for (i=1;i<=NF;i++) if ($i=="dev") {print $(i+1); exit}}')
[ -n "$iface" ] || iface=$(ip -o route show default 2>/dev/null | awk '{for (i=1;i<=NF;i++) if ($i=="dev") {print $(i+1); exit}}')
[ -n "$iface" ] || exit 0
cat "/sys/class/net/$iface/mtu" 2>/dev/null`)
	if err != nil {
		return 0
	}
	n, convErr := strconv.Atoi(strings.TrimSpace(out))
	if convErr != nil || n < 576 || n > 9000 {
		return 0
	}
	return n
}

// mtuDockerCreateArg returns the `-e GH_SR_HOST_MTU=<n>` line (indented, with a trailing
// line-continuation backslash) for the `docker create` command when mtu is a sub-1500
// value worth pinning, or "" otherwise. The MTU is only ever lowered: 1500 is Docker's
// default (no-op) and values outside [576, 1500) are ignored.
func mtuDockerCreateArg(mtu int) string {
	if mtu < 576 || mtu >= 1500 {
		return ""
	}
	return "  -e GH_SR_HOST_MTU=" + hostshell.PosixSingleQuote(strconv.Itoa(mtu)) + " \\\n"
}

func dockerdStartTimeoutDockerCreateArg(seconds int) string {
	if seconds <= 0 {
		return ""
	}
	return "  -e GH_SR_DOCKERD_START_TIMEOUT=" + hostshell.PosixSingleQuote(strconv.Itoa(seconds)) + " \\\n"
}

func bootstrapMaxRetriesDockerCreateArg(maxRetries int) string {
	if maxRetries <= 0 {
		return ""
	}
	return "  -e GH_SR_BOOTSTRAP_MAX_RETRIES=" + hostshell.PosixSingleQuote(strconv.Itoa(maxRetries)) + " \\\n"
}

func containerRestartPolicy(maxRetries int) string {
	if maxRetries <= 0 {
		maxRetries = 5
	}
	return "on-failure:" + strconv.Itoa(maxRetries)
}

// resolveContainerMTU returns the MTU to pin for a new runner container: the explicit
// config override when set, otherwise the auto-detected host egress MTU.
func (m *Manager) resolveContainerMTU(h *host.Host) int {
	if mtu := m.containerMTU(); mtu > 0 {
		return mtu
	}
	return DetectHostEgressMTU(h)
}

func (m *Manager) containerDockerdStartTimeout() int {
	if m != nil && m.ContainerDockerdStartTimeout > 0 {
		return m.ContainerDockerdStartTimeout
	}
	return 90
}

func (m *Manager) containerBootstrapMaxRetries() int {
	if m != nil && m.ContainerBootstrapMaxRetries > 0 {
		return m.ContainerBootstrapMaxRetries
	}
	return 5
}

func (m *Manager) containerStartStaggerSeconds() int {
	if m != nil && m.ContainerStartStaggerSeconds >= 0 && m.ContainerStartStaggerSeconds != 0 {
		return m.ContainerStartStaggerSeconds
	}
	return 3
}

// createContainerInstance creates (but does not start) a single runner container
// instance. The image must already exist. It is the per-instance unit used by both
// setupContainer and ContainerEnvironment.Provision.
func (m *Manager) createContainerInstance(h *host.Host, rc config.RunnerConfig, instanceIndex int, instanceName, imageTag string) error {
	fmt.Fprintf(m.out(), "  %s: creating runner container...\n", instanceName)

	regToken, err := m.GitHub.GetRegistrationTokenScoped(rc.Scope(), rc.ScopeTarget())
	if err != nil {
		return fmt.Errorf("getting registration token for %s: %w", instanceName, err)
	}

	stateDir, err := resolveAbsoluteRunnerDir(h, instanceName)
	if err != nil {
		return fmt.Errorf("resolving state dir for %s: %w", instanceName, err)
	}
	labels := rc.EffectiveLabelsForInstance(h.OS, h.Arch, instanceIndex)

	runURL := rc.GitHubRegistrationURL()

	group := rc.Group
	if group == "" {
		group = "Default"
	}

	ephemeral := ""
	if rc.Ephemeral {
		ephemeral = "true"
	}

	// Pin the inner/outer Docker MTU to the host egress MTU when it is below 1500 so
	// large-packet TLS handshakes survive a reduced host path MTU (see DetectHostEgressMTU
	// and entrypoint.sh §2a). Empty (no env) on standard 1500 networks — a no-op.
	mtuEnv := mtuDockerCreateArg(m.resolveContainerMTU(h))
	dockerdTimeoutEnv := dockerdStartTimeoutDockerCreateArg(m.containerDockerdStartTimeout())
	bootstrapRetriesEnv := bootstrapMaxRetriesDockerCreateArg(m.containerBootstrapMaxRetries())
	restartPolicy := containerRestartPolicy(m.containerBootstrapMaxRetries())

	// Build the `docker create` command. We use `--restart on-failure:N` so bootstrap
	// failures exit non-zero with bounded Docker retries; the entrypoint also caps
	// consecutive dockerd start failures via persisted state and holds the container
	// instead of looping forever. `--privileged` is required for DinD (inner dockerd
	// needs full capabilities). Large `/dev/shm` avoids Chromium/Selenium flakiness.
	cmd := fmt.Sprintf(`
mkdir -p %s
docker create \
  --name %s \
  --privileged \
  --shm-size=2g \
  --restart %s \
  -v %s:/runner-state \
  -e GH_SR_RUNNER_NAME=%s \
  -e GH_SR_RUNNER_TOKEN=%s \
  -e GH_SR_RUNNER_URL=%s \
  -e GH_SR_RUNNER_LABELS=%s \
  -e GH_SR_RUNNER_GROUP=%s \
  -e GH_SR_RUNNER_EPHEMERAL=%s \
%s%s%s  %s`,
		hostshell.PosixSingleQuote(stateDir),
		hostshell.PosixSingleQuote(containerName(instanceName)),
		hostshell.PosixSingleQuote(restartPolicy),
		hostshell.PosixSingleQuote(stateDir),
		hostshell.PosixSingleQuote(instanceName),
		hostshell.PosixSingleQuote(regToken),
		hostshell.PosixSingleQuote(runURL),
		hostshell.PosixSingleQuote(strings.Join(labels, ",")),
		hostshell.PosixSingleQuote(group),
		hostshell.PosixSingleQuote(ephemeral),
		mtuEnv,
		dockerdTimeoutEnv,
		bootstrapRetriesEnv,
		hostshell.PosixSingleQuote(imageTag),
	)

	if _, err := h.Run(cmd); err != nil {
		return fmt.Errorf("creating container %s: %w", containerName(instanceName), err)
	}
	return nil
}

// startContainer starts an existing runner container (docker start).
func (m *Manager) startContainer(h *host.Host, instanceName string) error {
	name := containerName(instanceName)
	clearContainerBootstrapMarkers(h, instanceName)
	policy := containerRestartPolicy(m.containerBootstrapMaxRetries())
	_, _ = h.Run(fmt.Sprintf(
		"docker update --restart=%s %s 2>/dev/null || true",
		hostshell.PosixSingleQuote(policy),
		hostshell.PosixSingleQuote(name),
	))
	if _, err := h.Run(fmt.Sprintf("docker start %s", name)); err != nil {
		return fmt.Errorf("starting container %s: %w", name, err)
	}
	return nil
}

func clearContainerBootstrapMarkers(h *host.Host, instanceName string) {
	stateDir, err := resolveAbsoluteRunnerDir(h, instanceName)
	if err != nil {
		stateDir = containerStateDir(h, instanceName)
	}
	_, _ = h.Run(fmt.Sprintf(
		"rm -f %s %s",
		hostshell.PosixSingleQuote(stateDir+"/"+bootstrapFailedMarker),
		hostshell.PosixSingleQuote(stateDir+"/"+dockerdStartFailuresFile),
	))
}

// ContainerBootstrapFailed reports whether the runner instance gave up after repeated
// inner-dockerd bootstrap failures (bootstrap-failed marker in the state bind-mount).
func ContainerBootstrapFailed(h *host.Host, instanceName string) bool {
	stateDir, err := resolveAbsoluteRunnerDir(h, instanceName)
	if err != nil {
		stateDir = containerStateDir(h, instanceName)
	}
	out, err := h.Run(fmt.Sprintf(
		"test -f %s && echo yes || echo no",
		hostshell.PosixSingleQuote(stateDir+"/"+bootstrapFailedMarker),
	))
	if err != nil {
		return false
	}
	return strings.TrimSpace(out) == "yes"
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
			cName, hostshell.PosixSingleQuote(removeTok),
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
	if _, err := h.Run(fmt.Sprintf("rm -rf %s", hostshell.PosixSingleQuote(stateDir))); err != nil {
		fmt.Fprintf(m.out(), "  %s: warning: failed to remove state dir %s: %v\n", instanceName, stateDir, err)
	}

	return nil
}

// parseContainerStatusInspectOutput parses one line of the form
// status|configImage|digest|imageRevision (from containerLocalStatusImageAndRevision).
// Missing fields default to "".
func parseContainerStatusInspectOutput(out string) (local, image, imageRev string) {
	line := strings.TrimSpace(out)
	// status|configImage|digest|imageRev — strings.Cut chain is 0-alloc vs
	// strings.Split's 4-element slice + padding loop. The third Cut discards
	// the digest field (we only carry status, image, imageRev through).
	status, rest, _ := strings.Cut(line, "|")
	image, rest, _ = strings.Cut(rest, "|")
	_, imageRev, _ = strings.Cut(rest, "|")
	status = strings.TrimSpace(status)
	image = strings.TrimSpace(image)
	imageRev = strings.TrimSpace(imageRev)
	switch status {
	case "running":
		local = "running"
	case "restarting":
		local = "restarting"
	case "not installed":
		local = "not installed"
	default:
		// exited, created, paused, etc.
		local = "stopped"
	}
	if local == "not installed" {
		return local, "", ""
	}
	return local, image, imageRev
}

// containerLocalStatusImageAndRevision returns local status, Config.Image, and the
// gh-sr.image-revision label on the container's image (one SSH round-trip).
func (m *Manager) containerLocalStatusImageAndRevision(h *host.Host, instanceName string) (string, string, string) {
	if ContainerBootstrapFailed(h, instanceName) {
		local, image, rev := m.containerLocalStatusFromDocker(h, instanceName)
		if local == "not installed" {
			return "failed", "", ""
		}
		return "failed", image, rev
	}
	local, image, rev := m.containerLocalStatusFromDocker(h, instanceName)
	return local, image, rev
}

func (m *Manager) containerLocalStatusFromDocker(h *host.Host, instanceName string) (string, string, string) {
	cid := hostshell.PosixSingleQuote(containerName(instanceName))
	script := fmt.Sprintf(
		"cid=%s\n"+
			"line=$(docker inspect --format '{{.State.Status}}|{{.Config.Image}}|{{.Image}}' \"$cid\" 2>/dev/null) || line=\"\"\n"+
			"if [ -z \"$line\" ]; then\n"+
			"  echo 'not installed|||'\n"+
			"else\n"+
			"  digest=${line##*|}\n"+
			"  rev=\"\"\n"+
			"  if [ -n \"$digest\" ]; then\n"+
			"    rev=$(docker image inspect \"$digest\" --format '{{index .Config.Labels \"gh-sr.image-revision\"}}' 2>/dev/null || true)\n"+
			"  fi\n"+
			"  printf '%%s|%%s\\n' \"$line\" \"$rev\"\n"+
			"fi\n",
		cid,
	)
	out, err := h.Run(script)
	if err != nil {
		return "not installed", "", ""
	}
	return parseContainerStatusInspectOutput(out)
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

	// Resolve runner version and image tag before rmi/build.
	version, err := m.GitHub.GetLatestRunnerVersion()
	if err != nil {
		return fmt.Errorf("resolving runner version: %w", err)
	}
	arch := archForGitHub(h.Arch)
	imageTag := ContainerRunnerImageTag(version, m.containerImageExtraApt())

	// Remove only this tag so we force a fresh build. Do not `docker rmi` every
	// gh-sr/agentic-runner image on the host: other runners' containers may still
	// reference those digests; removing them breaks `docker image inspect` and
	// BUILD shows "?" until those runners are rebuilt too.
	fmt.Fprintf(m.out(), "  %s: removing image %s (if present)...\n", rc.Name, imageTag)
	_, _ = h.Run(fmt.Sprintf("docker rmi -f %s 2>/dev/null || true", hostshell.PosixSingleQuote(imageTag)))

	fmt.Fprintf(m.out(), "  %s: building container runner image %s (this may take several minutes)...\n", rc.Name, imageTag)
	if err := buildAgenticRunnerImage(h, imageTag, version, arch, m.GhSrVersion, m.containerImageExtraApt()); err != nil {
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
		hostshell.PosixSingleQuote(imageTag),
	))
	if err != nil {
		return false, nil
	}
	return strings.TrimSpace(out) == "yes", nil
}

// embedTextForRemoteWrite normalizes CRLF to LF and escapes heredoc delimiters before
// writing embedded files to a remote build context. CRLF in apt manifests breaks the
// Dockerfile grep|xargs|apt-get pipeline (package names gain a trailing \r).
func embedTextForRemoteWrite(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	return strings.ReplaceAll(s, "GHSR_EOF", "GHSR_E0F")
}

// buildAgenticRunnerImage uploads the embedded Dockerfile+entrypoint to the host
// and builds the image via `docker build`.
func buildAgenticRunnerImage(h *host.Host, imageTag, runnerVersion, runnerArch, ghSrVersion string, extraApt []string) error {
	buildDir := "/tmp/gh-sr-agentic-runner-build"

	// Write the 8 build-context files via the shared helpers. writeRemoteHeredocFile
	// creates the parent directory on the host, and writeRemoteHeredocExecutable
	// additionally chmods the file +x — so each site collapses to a single call.
	if err := writeRemoteHeredocFile(h, buildDir+"/Dockerfile", agenticRunnerDockerfile); err != nil {
		return err
	}
	if err := writeRemoteHeredocFile(h, buildDir+"/apt-packages-core.txt", agenticRunnerAptPackagesCore); err != nil {
		return err
	}

	// apt-packages-extra.txt is empty when no extras are configured: truncate instead
	// of writing an empty heredoc (avoids a stray newline and a confusing file).
	extraSorted := containerRunnerImageExtraSorted(extraApt)
	extraPath := buildDir + "/apt-packages-extra.txt"
	if len(extraSorted) == 0 {
		if _, err := h.Run(formatEmptyRemoteFile(extraPath)); err != nil {
			return fmt.Errorf("writing %s: %w", extraPath, err)
		}
	} else {
		if err := writeRemoteHeredocFile(h, extraPath, joinExtraPackages(extraSorted)); err != nil {
			return err
		}
	}

	if err := writeRemoteHeredocExecutable(h, buildDir+"/entrypoint.sh", agenticRunnerEntrypoint); err != nil {
		return err
	}
	if err := writeRemoteHeredocExecutable(h, buildDir+"/docker-wrapper.sh", agenticRunnerDockerWrapper); err != nil {
		return err
	}

	// Write baked inner-Docker network configs (Pillar 2: deterministic DNS, single dockerd start).
	for _, f := range []struct{ name, content string }{
		{"daemon.json", agenticRunnerDaemonJSON},
		{"dnsmasq-gh-sr.conf", agenticRunnerDnsmasqConf},
	} {
		if err := writeRemoteHeredocFile(h, buildDir+"/"+f.name, f.content); err != nil {
			return err
		}
	}

	// Write per-job reset hooks into the build context (Pillar 1). The helper mkdirs
	// the parent of every path, so the explicit `mkdir -p buildDir/hooks` is gone.
	for _, hk := range []struct{ name, content string }{
		{"job-started.sh", agenticRunnerJobStartedHook},
		{"job-completed.sh", agenticRunnerJobCompletedHook},
	} {
		if err := writeRemoteHeredocExecutable(h, buildDir+"/hooks/"+hk.name, hk.content); err != nil {
			return err
		}
	}

	// Build (stamp labels so gh sr status can compare layout to this binary).
	rev := ContainerImageLayoutRevision(ghSrVersion, extraApt)
	labelRev := hostshell.PosixSingleQuote(dockerLabelImageRevision + "=" + rev)
	labelCLI := hostshell.PosixSingleQuote(dockerLabelCLIVersion + "=" + ghSrVersion)
	buildCmd := fmt.Sprintf(
		"docker build --build-arg RUNNER_VERSION=%s --build-arg RUNNER_ARCH=%s --label %s --label %s -t %s %s",
		hostshell.PosixSingleQuote(runnerVersion),
		hostshell.PosixSingleQuote(runnerArch),
		labelRev,
		labelCLI,
		hostshell.PosixSingleQuote(imageTag),
		hostshell.PosixSingleQuote(buildDir),
	)
	if _, err := h.Run(buildCmd); err != nil {
		return fmt.Errorf("docker build: %w", err)
	}

	// Clean up build context.
	_, _ = h.Run(fmt.Sprintf("rm -rf %s", buildDir))

	return nil
}

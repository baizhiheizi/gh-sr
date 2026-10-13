// Package registry deploys and manages the per-host Docker registry
// pull-through mirror (registry:2 with REGISTRY_PROXY_REMOTEURL) so container
// runners' inner dockerd pulls Docker Hub images through the host instead of
// every runner independently downloading the same layers over the WAN.
// One gh-sr-registry container serves every runner container on the same host.
package registry

import (
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/an-lee/gh-sr/internal/cache"
	"github.com/an-lee/gh-sr/internal/host"
	"github.com/an-lee/gh-sr/internal/hostshell"
)

const (
	// ContainerName is the Docker container hosting the per-host mirror.
	ContainerName = "gh-sr-registry"
	// DefaultImage is the mirror image. Pin a digest via registry_mirror.image
	// in runners.yml for reproducible deploys.
	DefaultImage = "registry:2"
	// ContainerPort is the port the registry listens on inside the container.
	ContainerPort = 5000
	// DefaultPort is the host-side published port when registry_mirror.port is
	// unset. Same selection criteria as the cache server's DefaultPort: below
	// the Linux ephemeral range (32768-60999) and away from frequent
	// dev-service ports.
	DefaultPort = 5577
	// DefaultStoragePath is the host-side storage dir (relative to $HOME) when
	// registry_mirror.storage_path is unset.
	DefaultStoragePath = "$HOME/.gh-sr/registry"
	// DefaultUpstream is the registry the mirror pulls through
	// (registry-1.docker.io = Docker Hub, the only upstream dockerd's
	// registry-mirrors mechanism redirects transparently).
	DefaultUpstream = "https://registry-1.docker.io"

	// LayoutLabel is the Docker label stamped on the mirror container with the
	// current deploy layout revision. Ensure recreates containers stamped with
	// an older value (or none), so deploy-argument changes converge on the
	// next Ensure instead of silently running a stale deployment forever.
	LayoutLabel        = "gh-sr.registry-layout"
	registryLayoutRev  = "v1"
	storageRegistrySub = "/var/lib/registry"
)

// Settings is the effective mirror configuration for one operation, built by
// ops from the registry_mirror: section of runners.yml. The zero value is
// disabled.
type Settings struct {
	Enabled     bool
	Port        int    // 0 = DefaultPort
	BindAddr    string // "" = docker0 gateway; 0.0.0.0 = all interfaces
	StoragePath string // "" = DefaultStoragePath
	Image       string // "" = DefaultImage
	Upstream    string // "" = DefaultUpstream
	URLOverride string // escape hatch: runners get this URL verbatim
}

func (s Settings) image() string {
	if s.Image == "" {
		return DefaultImage
	}
	return s.Image
}

func (s Settings) port() int {
	if s.Port == 0 {
		return DefaultPort
	}
	return s.Port
}

func (s Settings) upstream() string {
	if s.Upstream == "" {
		return DefaultUpstream
	}
	return s.Upstream
}

// EffectivePort is the exported form of the resolved host-side published port
// (Port, or DefaultPort when unset) for packages reporting remediation hints.
func (s Settings) EffectivePort() int {
	return s.port()
}

// RunnerURL returns the mirror URL to inject into runners' inner dockerd on
// this host, or "" when injection must be skipped.
//
// Precedence: URLOverride → explicit non-0.0.0.0 BindAddr → docker0 gateway.
// A gateway lookup failure yields "" (deploy then binds 0.0.0.0 and the
// caller warns), because without a bridge gateway there is no
// container-reachable address to inject.
//
// Note this is a host-published port: reaching it from a container traverses
// the host's INPUT chain, so hosts with a default-deny firewall (ufw et al)
// need one allow rule — `gh sr doctor` probes the exact URL from inside a
// runner and prints the command when it is blocked.
func (s Settings) RunnerURL(h *host.Host) string {
	if s.URLOverride != "" {
		return s.URLOverride
	}
	gw, err := cache.ResolveGatewayIP(h)
	if err != nil || gw == "" {
		return ""
	}
	return s.runnerURL(gw)
}

// runnerURL is the URL form of RunnerURL using a pre-resolved docker0 gateway
// IP — lets callers that already need the gateway (Inspect, deploy) avoid a
// second SSH round-trip to resolve it again.
func (s Settings) runnerURL(gatewayIP string) string {
	if s.URLOverride != "" {
		return s.URLOverride
	}
	hostPart := gatewayIP
	if s.BindAddr != "" && s.BindAddr != "0.0.0.0" {
		hostPart = s.BindAddr
	}
	if hostPart == "" {
		return ""
	}
	return fmt.Sprintf("http://%s:%d", hostPart, s.port())
}

// localURL is the mirror API base as seen from the host itself: an explicit
// bind address wins; 0.0.0.0 / auto bind answers on the docker0 gateway (a
// host-local address), falling back to loopback when no gateway was found.
func (s Settings) localURL(gatewayIP string) string {
	if s.BindAddr != "" && s.BindAddr != "0.0.0.0" {
		return fmt.Sprintf("http://%s:%d", s.BindAddr, s.port())
	}
	if gatewayIP != "" {
		return fmt.Sprintf("http://%s:%d", gatewayIP, s.port())
	}
	return fmt.Sprintf("http://127.0.0.1:%d", s.port())
}

// resolvedStoragePath expands the $HOME prefix (the default storage path is
// expressed relative to the remote shell's home) into an absolute path —
// Docker bind mounts and mkdir do not undergo shell expansion.
func (s Settings) resolvedStoragePath(h *host.Host) (string, error) {
	p := s.StoragePath
	if p == "" {
		p = DefaultStoragePath
	}
	if !strings.HasPrefix(p, "$HOME") {
		return p, nil
	}
	out, err := h.Run("echo $HOME")
	if err != nil {
		return "", fmt.Errorf("resolving home dir: %w", err)
	}
	home := strings.TrimSpace(out)
	if home == "" {
		return "", fmt.Errorf("resolving home dir: empty $HOME")
	}
	return home + p[len("$HOME"):], nil
}

// containerState reports "running", "exists" (present but not running) or ""
// (absent) for the mirror container.
func containerState(h *host.Host) (string, error) {
	out, err := h.Run(fmt.Sprintf(
		"docker inspect --format='{{.State.Status}}' %s 2>/dev/null || true", ContainerName))
	if err != nil {
		return "", fmt.Errorf("inspecting registry mirror container: %w", err)
	}
	state := strings.TrimSpace(out)
	switch state {
	case "running":
		return "running", nil
	case "":
		return "", nil
	default:
		return "exists", nil
	}
}

// registryLayoutCurrent reports whether an existing mirror container carries
// the current deploy layout label. Used by Ensure to converge deployments made
// by older gh-sr versions (or after a layout change): a stale container cannot
// be updated in place (its env and ports are baked at create), so it is
// recreated.
func registryLayoutCurrent(h *host.Host) (bool, error) {
	out, err := h.Run(fmt.Sprintf(
		"docker inspect --format='{{index .Config.Labels \"%s\"}}' %s 2>/dev/null || true",
		LayoutLabel, ContainerName))
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(out) == registryLayoutRev, nil
}

// Ensure deploys the mirror idempotently: a running container with the current
// layout label is a no-op, a stopped one is started, a missing one is pulled
// and created, and one stamped with an older layout is recreated.
func Ensure(w io.Writer, h *host.Host, s Settings) error {
	if !s.Enabled {
		return nil
	}
	state, err := containerState(h)
	if err != nil {
		return err
	}
	if state != "" {
		current, err := registryLayoutCurrent(h)
		if err != nil {
			return err
		}
		if !current {
			fmt.Fprintf(w, "  registry: %s was deployed by an older gh-sr layout; recreating...\n", ContainerName)
			if _, err := h.Run("docker rm -f " + ContainerName); err != nil {
				return fmt.Errorf("removing outdated registry mirror container: %w", err)
			}
			state = ""
		}
	}
	switch state {
	case "running":
		return nil
	case "exists":
		fmt.Fprintf(w, "  registry: starting %s...\n", ContainerName)
		if _, err := h.Run("docker start " + ContainerName); err != nil {
			return fmt.Errorf("starting registry mirror container: %w", err)
		}
		return nil
	default:
		return deploy(w, h, s)
	}
}

func deploy(w io.Writer, h *host.Host, s Settings) error {
	storage, err := s.resolvedStoragePath(h)
	if err != nil {
		return err
	}
	if _, err := h.Run("mkdir -p " + hostshell.PosixSingleQuote(storage)); err != nil {
		return fmt.Errorf("creating registry mirror storage dir: %w", err)
	}

	fmt.Fprintf(w, "  registry: pulling %s...\n", s.image())
	// Pull is best-effort: a pre-pulled image must still deploy on hosts
	// without registry access. A genuinely missing image fails at docker run.
	_, _ = h.Run("docker pull " + hostshell.PosixSingleQuote(s.image()))

	bind := s.BindAddr
	var gw string
	if bind == "" {
		var err error
		gw, err = cache.ResolveGatewayIP(h)
		if err != nil || gw == "" {
			bind = "0.0.0.0"
			fmt.Fprintf(w, "  registry: warning: docker0 gateway not found; binding %s (the mirror is exposed on every host interface)\n", bind)
		} else {
			bind = gw
		}
	}

	args := []string{
		"docker", "run", "-d",
		"--name", ContainerName,
		"--restart", "unless-stopped",
		"--label", LayoutLabel + "=" + registryLayoutRev,
		"-p", fmt.Sprintf("%s:%d:%d", bind, s.port(), ContainerPort),
		"-v", fmt.Sprintf("%s:%s", hostshell.PosixSingleQuote(storage), storageRegistrySub),
		// Pull-through cache: pulls that miss the local storage are fetched
		// from the upstream registry and stored for every later runner.
		"-e", hostshell.PosixSingleQuote("REGISTRY_PROXY_REMOTEURL=" + s.upstream()),
		hostshell.PosixSingleQuote(s.image()),
	}

	fmt.Fprintf(w, "  registry: starting %s on %s:%d (upstream %s, storage %s)...\n",
		ContainerName, bind, s.port(), s.upstream(), storage)
	if _, err := h.Run(strings.Join(args, " ")); err != nil {
		return fmt.Errorf("starting registry mirror container: %w", err)
	}
	return nil
}

// Inspect gathers the mirror state for `gh sr registry status`. The health
// probe is best-effort: failures set Healthy=false instead of erroring.
func Inspect(h *host.Host, s Settings) (StatusInfo, error) {
	info := StatusInfo{}
	out, err := h.Run(fmt.Sprintf(
		"docker inspect --format='{{.State.Status}}|{{.Config.Image}}' %s 2>/dev/null || true", ContainerName))
	if err != nil {
		return info, fmt.Errorf("inspecting registry mirror container: %w", err)
	}
	if parts := strings.SplitN(strings.TrimSpace(out), "|", 2); parts[0] != "" {
		info.State = parts[0]
		if len(parts) == 2 {
			info.Image = parts[1]
		}
	}

	storage, err := s.resolvedStoragePath(h)
	if err != nil {
		return info, err
	}
	info.StoragePath = storage

	var gw string
	if s.URLOverride == "" && (s.BindAddr == "" || s.BindAddr == "0.0.0.0") {
		gw, _ = cache.ResolveGatewayIP(h)
	}
	info.URL = s.runnerURL(gw)

	if info.State != "" {
		// GET /v2/ answers 200 on an anonymous pull-through mirror.
		_, healthErr := h.Run(fmt.Sprintf(
			"curl -fsS -m 3 %s/v2/ 2>/dev/null || true", hostshell.PosixSingleQuote(s.localURL(gw))))
		info.Healthy = healthErr == nil

		if duOut, duErr := h.Run(fmt.Sprintf(
			"du -sb %s 2>/dev/null | cut -f1", hostshell.PosixSingleQuote(storage))); duErr == nil {
			if n, perr := strconv.ParseInt(strings.TrimSpace(duOut), 10, 64); perr == nil {
				info.StorageBytes = n
			}
		}
	}
	return info, nil
}

// MeasureStorage reports the resolved on-host storage path and its size in
// bytes (0 when the dir does not exist yet or is empty).
func MeasureStorage(h *host.Host, s Settings) (string, int64, error) {
	storage, err := s.resolvedStoragePath(h)
	if err != nil {
		return "", 0, err
	}
	out, err := h.Run(fmt.Sprintf("du -sb %s 2>/dev/null | cut -f1", hostshell.PosixSingleQuote(storage)))
	if err != nil {
		return storage, 0, err
	}
	n, perr := strconv.ParseInt(strings.TrimSpace(out), 10, 64)
	if perr != nil {
		return storage, 0, nil
	}
	return storage, n, nil
}

// StatusInfo is the reported state of the per-host registry mirror.
type StatusInfo struct {
	State        string // Docker state ("running", "exited", ...) or "" when absent
	Image        string
	URL          string // runner-facing mirror URL ("" when not resolvable)
	StoragePath  string
	StorageBytes int64
	Healthy      bool // GET /v2/ answered 200
}

// Remove stops and deletes the mirror container. purgeData also deletes the
// storage dir (every mirrored layer). This is the only uninstall path —
// `gh sr remove` never touches the mirror.
func Remove(w io.Writer, h *host.Host, s Settings, purgeData bool) error {
	if _, err := h.Run("docker rm -f " + ContainerName); err != nil {
		return fmt.Errorf("removing registry mirror container: %w", err)
	}
	fmt.Fprintf(w, "  registry: removed %s\n", ContainerName)
	if !purgeData {
		return nil
	}
	storage, err := s.resolvedStoragePath(h)
	if err != nil {
		return err
	}
	if _, err := h.Run("rm -rf " + hostshell.PosixSingleQuote(storage)); err != nil {
		return fmt.Errorf("removing registry mirror storage: %w", err)
	}
	fmt.Fprintf(w, "  registry: purged %s\n", storage)
	return nil
}

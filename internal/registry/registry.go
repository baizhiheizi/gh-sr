// Package registry deploys and manages the per-host Docker registry
// pull-through mirrors (registry:2 with REGISTRY_PROXY_REMOTEURL) so container
// runners' inner dockerd pulls Docker Hub images through the host instead of
// every runner independently downloading the same layers over the WAN, and so
// host-side prefetch (registry_mirror.prefetch + `gh sr images sync`) can pull
// from other registries (ghcr.io, …) once per host and load the images into
// every runner's inner dockerd.
//
// The first mirror (container gh-sr-registry, upstream Docker Hub) is wired
// into runner inner dockerds via daemon.json registry-mirrors — the only
// per-registry redirection dockerd supports. Additional mirrors
// (gh-sr-registry-<name>) exist for prefetch: dockerd cannot be pointed at a
// ghcr.io mirror transparently (moby/moby#41456), so those images are pulled
// once on the host through the mirror and `docker save | docker load`-ed into
// each runner.
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
	// ContainerName is the Docker container hosting the first (Docker Hub
	// facing) mirror. Kept stable for existing deployments, doctor output,
	// and the disk-usage row.
	ContainerName = "gh-sr-registry"
	// ContainerNamePrefix names the additional per-upstream mirrors:
	// gh-sr-registry-<name>.
	ContainerNamePrefix = "gh-sr-registry-"
	// DefaultImage is the mirror image. Pin a digest via registry_mirror.image
	// in runners.yml for reproducible deploys.
	DefaultImage = "registry:2"
	// ContainerPort is the port the registry listens on inside the container.
	ContainerPort = 5000
	// DefaultPort is the host-side published port of the first mirror when
	// registry_mirror.port is unset. Same selection criteria as the cache
	// server's DefaultPort: below the Linux ephemeral range (32768-60999) and
	// away from frequent dev-service ports.
	DefaultPort = 5577
	// DefaultStoragePath is the host-side storage dir (relative to $HOME) when
	// registry_mirror.storage_path is unset.
	DefaultStoragePath = "$HOME/.gh-sr/registry"
	// DefaultUpstream is the registry the first mirror pulls through
	// (registry-1.docker.io = Docker Hub, the only upstream dockerd's
	// registry-mirrors mechanism redirects transparently).
	DefaultUpstream = "https://registry-1.docker.io"

	// LayoutLabel is the Docker label stamped on the mirror containers with
	// the current deploy layout revision. Ensure recreates containers stamped
	// with an older value (or none), so deploy-argument changes converge on
	// the next Ensure instead of silently running a stale deployment forever.
	//
	// v2: multi-mirror layout — per-upstream containers, optional
	// REGISTRY_PROXY_USERNAME/PASSWORD, and a loopback publish so the host's
	// own dockerd can pull through the mirror without insecure-registry
	// config (dockerd treats 127.0.0.0/8 as insecure by default).
	LayoutLabel        = "gh-sr.registry-layout"
	registryLayoutRev  = "v2"
	storageRegistrySub = "/var/lib/registry"
)

// MirrorSettings is one resolved pull-through mirror. The first entry (index
// 0) is the Docker Hub mirror wired into runner inner dockerds; the rest serve
// host-side prefetch pulls only.
type MirrorSettings struct {
	Name     string // container-name fragment and storage-dir suffix
	Upstream string // "" on the first mirror = DefaultUpstream
	Port     int    // resolved host-side published port (never 0 post-Load)
	Username string // optional upstream auth (PAT for GHCR)
	Password string // resolved from PasswordEnv at SettingsFromConfig time
}

// Settings is the effective mirror configuration for one operation, built by
// ops from the registry_mirror: section of runners.yml. The zero value is
// disabled.
type Settings struct {
	Enabled     bool
	BindAddr    string // "" = docker0 gateway + loopback; 0.0.0.0 = all interfaces
	StoragePath string // "" = DefaultStoragePath (first mirror; others add -<name>)
	Image       string // "" = DefaultImage
	URLOverride string // escape hatch: runners get this URL verbatim (first mirror)
	Mirrors     []MirrorSettings
}

// first returns the Docker Hub facing mirror (the legacy single mirror).
func (s Settings) first() MirrorSettings {
	if len(s.Mirrors) == 0 {
		return MirrorSettings{Name: "dockerhub", Upstream: DefaultUpstream, Port: DefaultPort}
	}
	return s.Mirrors[0]
}

// mirrorList returns the resolved mirror list: a zero-value Settings (no
// SettingsFromConfig, e.g. tests) behaves as the legacy single Docker Hub
// mirror.
func (s Settings) mirrorList() []MirrorSettings {
	if len(s.Mirrors) == 0 {
		return []MirrorSettings{s.first()}
	}
	return s.Mirrors
}

func (s Settings) image() string {
	if s.Image == "" {
		return DefaultImage
	}
	return s.Image
}

func (s Settings) port() int {
	return s.first().Port
}

func (s Settings) upstream() string {
	if s.first().Upstream == "" {
		return DefaultUpstream
	}
	return s.first().Upstream
}

// EffectivePort is the exported form of the resolved host-side published port
// of the first mirror (Port, or DefaultPort when unset) for packages
// reporting remediation hints.
func (s Settings) EffectivePort() int {
	return s.port()
}

// ContainerName returns the Docker container name of mirror i: the first
// mirror keeps the legacy gh-sr-registry name; others are
// gh-sr-registry-<name>.
func (s Settings) containerName(i int) string {
	if i <= 0 {
		return ContainerName
	}
	return ContainerNamePrefix + s.Mirrors[i].Name
}

// ContainerNames returns the container names of every configured mirror, in
// deploy order. Callers reconcile on-host gh-sr-registry* containers against
// this list to spot mirrors that were removed from runners.yml: nothing in the
// Ensure/Remove path iterates anything but the configured list, so a dropped
// entry would otherwise keep running (and keep its storage dir) forever.
func (s Settings) ContainerNames() []string {
	list := s.mirrorList()
	out := make([]string, len(list))
	for i := range list {
		out[i] = s.containerName(i)
	}
	return out
}

// RunnerURL returns the first mirror's URL to inject into runners' inner
// dockerd on this host, or "" when injection must be skipped.
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
	return s.mirrorRunnerURL(0, gw)
}

// mirrorRunnerURL is the runner-facing URL of mirror i, using a pre-resolved
// docker0 gateway IP (URLOverride applies to the first mirror only).
func (s Settings) mirrorRunnerURL(i int, gatewayIP string) string {
	if i == 0 && s.URLOverride != "" {
		return s.URLOverride
	}
	hostPart := gatewayIP
	if s.BindAddr != "" && s.BindAddr != "0.0.0.0" {
		hostPart = s.BindAddr
	}
	if hostPart == "" {
		return ""
	}
	return fmt.Sprintf("http://%s:%d", hostPart, s.mirrorPort(i))
}

// mirrorPort is the resolved port of mirror i (first mirror defaults to
// DefaultPort when unset — zero-value Settings without a Load).
func (s Settings) mirrorPort(i int) int {
	if p := s.first().Port; i == 0 {
		if p == 0 {
			return DefaultPort
		}
		return p
	}
	if i < len(s.Mirrors) {
		if p := s.Mirrors[i].Port; p != 0 {
			return p
		}
	}
	return DefaultPort + i
}

// hostPullRefBase returns the "host:port" base the host's own dockerd can
// pull through mirror i from, or "" when the bind makes that impossible.
//
// The host daemon is not configured with insecure-registries by gh-sr, so a
// plain-HTTP mirror is only pullable on an address dockerd already treats as
// insecure: loopback (127.0.0.0/8, ::1/128). With the default bind the mirror
// publishes on 127.0.0.1 exactly for this; an explicit non-loopback BindAddr
// yields "" and prefetch falls back to pulling the upstream ref directly.
func (s Settings) hostPullRefBase(i int) string {
	switch {
	case s.BindAddr == "" || s.BindAddr == "0.0.0.0":
		return "127.0.0.1:" + strconv.Itoa(s.mirrorPort(i))
	case strings.HasPrefix(s.BindAddr, "127.") || s.BindAddr == "localhost" || s.BindAddr == "::1":
		return s.BindAddr + ":" + strconv.Itoa(s.mirrorPort(i))
	default:
		return ""
	}
}

// localURL is the mirror API base of mirror i as seen from the host itself:
// an explicit bind address wins; 0.0.0.0 / auto bind answers on the docker0
// gateway (a host-local address), falling back to loopback when no gateway
// was found.
func (s Settings) localURL(i int, gatewayIP string) string {
	if s.BindAddr != "" && s.BindAddr != "0.0.0.0" {
		return fmt.Sprintf("http://%s:%d", s.BindAddr, s.mirrorPort(i))
	}
	if gatewayIP != "" {
		return fmt.Sprintf("http://%s:%d", gatewayIP, s.mirrorPort(i))
	}
	return fmt.Sprintf("http://127.0.0.1:%d", s.mirrorPort(i))
}

// mirrorStoragePath resolves the on-host storage dir of mirror i: the base
// path for the first mirror, <base>-<name> for the rest.
func (s Settings) mirrorStoragePath(i int, h *host.Host) (string, error) {
	paths, err := s.mirrorStoragePaths(h)
	if err != nil {
		return "", err
	}
	if i < 0 || i >= len(paths) {
		return "", fmt.Errorf("mirror index %d out of range (%d mirror(s) configured)", i, len(paths))
	}
	return paths[i], nil
}

// mirrorStoragePaths resolves the on-host storage dir of every configured
// mirror in one pass, expanding the $HOME prefix (the default storage path is
// expressed relative to the remote shell's home) into an absolute path —
// Docker bind mounts and mkdir do not undergo shell expansion. $HOME is
// resolved once for the whole list rather than once per mirror.
func (s Settings) mirrorStoragePaths(h *host.Host) ([]string, error) {
	base := s.StoragePath
	if base == "" {
		base = DefaultStoragePath
	}
	if strings.HasPrefix(base, "$HOME") {
		out, err := h.Run("echo $HOME")
		if err != nil {
			return nil, fmt.Errorf("resolving home dir: %w", err)
		}
		home := strings.TrimSpace(out)
		if home == "" {
			return nil, fmt.Errorf("resolving home dir: empty $HOME")
		}
		base = home + base[len("$HOME"):]
	}
	list := s.mirrorList()
	paths := make([]string, len(list))
	for i := range list {
		paths[i] = base
		if i > 0 {
			paths[i] = base + "-" + list[i].Name
		}
	}
	return paths, nil
}

// containerState reports "running", "exists" (present but not running) or ""
// (absent) for one mirror container.
func containerState(h *host.Host, cname string) (string, error) {
	out, err := h.Run(fmt.Sprintf(
		"docker inspect --format='{{.State.Status}}' %s 2>/dev/null || true", cname))
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
func registryLayoutCurrent(h *host.Host, cname string) (bool, error) {
	out, err := h.Run(fmt.Sprintf(
		"docker inspect --format='{{index .Config.Labels \"%s\"}}' %s 2>/dev/null || true",
		LayoutLabel, cname))
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(out) == registryLayoutRev, nil
}

// Ensure deploys every configured mirror idempotently: a running container
// with the current layout label is a no-op, a stopped one is started, a
// missing one is pulled and created, and one stamped with an older layout is
// recreated.
func Ensure(w io.Writer, h *host.Host, s Settings) error {
	if !s.Enabled {
		return nil
	}
	for i := range s.mirrorList() {
		if err := ensureMirror(w, h, s, i); err != nil {
			return err
		}
	}
	return nil
}

// ensureMirror is the per-mirror unit of Ensure.
func ensureMirror(w io.Writer, h *host.Host, s Settings, i int) error {
	cname := s.containerName(i)
	state, err := containerState(h, cname)
	if err != nil {
		return err
	}
	if state != "" {
		current, err := registryLayoutCurrent(h, cname)
		if err != nil {
			return err
		}
		if !current {
			fmt.Fprintf(w, "  registry: %s was deployed by an older gh-sr layout; recreating...\n", cname)
			if _, err := h.Run("docker rm -f " + cname); err != nil {
				return fmt.Errorf("removing outdated registry mirror container: %w", err)
			}
			state = ""
		}
	}
	switch state {
	case "running":
		return nil
	case "exists":
		fmt.Fprintf(w, "  registry: starting %s...\n", cname)
		if _, err := h.Run("docker start " + cname); err != nil {
			return fmt.Errorf("starting registry mirror container: %w", err)
		}
		return nil
	default:
		return deployMirror(w, h, s, i)
	}
}

func deployMirror(w io.Writer, h *host.Host, s Settings, i int) error {
	cname := s.containerName(i)
	m := s.mirrorList()[i]
	upstream := m.Upstream
	if upstream == "" {
		// Zero-value first mirror (legacy config without an explicit
		// upstream): pull through Docker Hub.
		upstream = DefaultUpstream
	}
	storage, err := s.mirrorStoragePath(i, h)
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
		"--name", cname,
		"--restart", "unless-stopped",
		"--label", LayoutLabel + "=" + registryLayoutRev,
	}
	// Publish the port so BOTH runner containers (docker0 gateway) and the
	// host's own dockerd (loopback, treated as insecure so plain HTTP pulls
	// work without daemon config) can reach the mirror. An explicit
	// non-loopback bind keeps the historical single publish.
	port := s.mirrorPort(i)
	switch {
	case s.BindAddr == "":
		args = append(args,
			"-p", fmt.Sprintf("%s:%d:%d", bind, port, ContainerPort),
			"-p", fmt.Sprintf("127.0.0.1:%d:%d", port, ContainerPort))
	case s.BindAddr == "0.0.0.0":
		args = append(args, "-p", fmt.Sprintf("0.0.0.0:%d:%d", port, ContainerPort))
	default:
		args = append(args, "-p", fmt.Sprintf("%s:%d:%d", bind, port, ContainerPort))
	}
	args = append(args,
		"-v", fmt.Sprintf("%s:%s", hostshell.PosixSingleQuote(storage), storageRegistrySub),
		// Pull-through cache: pulls that miss the local storage are fetched
		// from the upstream registry and stored for every later runner.
		"-e", hostshell.PosixSingleQuote("REGISTRY_PROXY_REMOTEURL="+upstream),
	)
	if m.Username != "" {
		args = append(args,
			"-e", hostshell.PosixSingleQuote("REGISTRY_PROXY_USERNAME="+m.Username),
			"-e", hostshell.PosixSingleQuote("REGISTRY_PROXY_PASSWORD="+m.Password),
		)
	}
	args = append(args, hostshell.PosixSingleQuote(s.image()))

	fmt.Fprintf(w, "  registry: starting %s on port %d (upstream %s, storage %s)...\n",
		cname, port, upstream, storage)
	if _, err := h.Run(strings.Join(args, " ")); err != nil {
		return fmt.Errorf("starting registry mirror container: %w", err)
	}
	return nil
}

// Inspect gathers the state of every configured mirror for `gh sr registry
// status` and doctor. The health probe is best-effort: failures set
// Healthy=false instead of erroring.
func Inspect(h *host.Host, s Settings) ([]MirrorStatus, error) {
	var out []MirrorStatus
	var gw string
	if s.URLOverride == "" && (s.BindAddr == "" || s.BindAddr == "0.0.0.0") {
		gw, _ = cache.ResolveGatewayIP(h)
	}
	// Resolve every mirror's storage dir up front: one $HOME round-trip for
	// the whole list instead of one per mirror.
	storages, err := s.mirrorStoragePaths(h)
	if err != nil {
		return nil, err
	}
	for i := range s.mirrorList() {
		cname := s.containerName(i)
		info := StatusInfo{}
		outRaw, err := h.Run(fmt.Sprintf(
			"docker inspect --format='{{.State.Status}}|{{.Config.Image}}' %s 2>/dev/null || true", cname))
		if err != nil {
			return out, fmt.Errorf("inspecting registry mirror container: %w", err)
		}
		if parts := strings.SplitN(strings.TrimSpace(outRaw), "|", 2); parts[0] != "" {
			info.State = parts[0]
			if len(parts) == 2 {
				info.Image = parts[1]
			}
		}

		storage := storages[i]
		info.StoragePath = storage
		info.URL = s.mirrorRunnerURL(i, gw)

		if info.State != "" {
			// GET /v2/ answers 200 on an anonymous pull-through mirror.
			_, healthErr := h.Run(fmt.Sprintf(
				"curl -fsS -m 3 %s/v2/ 2>/dev/null || true", hostshell.PosixSingleQuote(s.localURL(i, gw))))
			info.Healthy = healthErr == nil

			if duOut, duErr := h.Run(fmt.Sprintf(
				"du -sb %s 2>/dev/null | cut -f1", hostshell.PosixSingleQuote(storage))); duErr == nil {
				if n, perr := strconv.ParseInt(strings.TrimSpace(duOut), 10, 64); perr == nil {
					info.StorageBytes = n
				}
			}
		}
		out = append(out, MirrorStatus{Name: s.mirrorList()[i].Name, Container: cname, Info: info})
	}
	return out, nil
}

// MirrorStatus is the state of one mirror: its config name, Docker container
// name, and the probed StatusInfo.
type MirrorStatus struct {
	Name      string
	Container string
	Info      StatusInfo
}

// StatusInfo is the reported state of one registry mirror.
type StatusInfo struct {
	State        string // Docker state ("running", "exited", ...) or "" when absent
	Image        string
	URL          string // runner-facing mirror URL ("" when not resolvable)
	StoragePath  string
	StorageBytes int64
	Healthy      bool // GET /v2/ answered 200
}

// MirrorStorage is one mirror's storage directory and its apparent size on
// disk (0 when the dir does not exist yet or is empty).
type MirrorStorage struct {
	Container string // gh-sr-registry / gh-sr-registry-<name>
	Path      string
	Bytes     int64
}

// MirrorStorages reports every configured mirror's storage directory and size.
// Each directory is reported against its own container so callers (gh sr disk
// usage) never attribute another mirror's bytes to the first mirror's path.
//
// One `du` covers the whole list — every path is single-quoted individually,
// because quoting the joined string would hand du a single argument
// containing spaces and silently measure nothing — and each output line is
// matched back to its directory, so a mirror whose dir is absent simply
// reports 0 instead of shifting the other mirrors' sizes.
func MirrorStorages(h *host.Host, s Settings) ([]MirrorStorage, error) {
	paths, err := s.mirrorStoragePaths(h)
	if err != nil {
		return nil, err
	}
	list := s.mirrorList()
	out := make([]MirrorStorage, len(list))
	quoted := make([]string, len(paths))
	for i := range paths {
		out[i] = MirrorStorage{Container: s.containerName(i), Path: paths[i]}
		quoted[i] = hostshell.PosixSingleQuote(paths[i])
	}
	duOut, err := h.Run("du -sb " + strings.Join(quoted, " ") + " 2>/dev/null")
	if err != nil {
		return out, err
	}
	byPath := make(map[string]int64, len(paths))
	for _, line := range strings.Split(duOut, "\n") {
		// du prints "<bytes>\t<path>" per argument.
		tab := strings.Index(line, "\t")
		if tab <= 0 {
			continue
		}
		if n, perr := strconv.ParseInt(strings.TrimSpace(line[:tab]), 10, 64); perr == nil {
			byPath[line[tab+1:]] = n
		}
	}
	for i := range out {
		out[i].Bytes = byPath[out[i].Path]
	}
	return out, nil
}

// Remove stops and deletes every mirror container. purgeData also deletes the
// storage dirs (every mirrored layer). This is the only uninstall path —
// `gh sr remove` never touches the mirrors.
func Remove(w io.Writer, h *host.Host, s Settings, purgeData bool) error {
	for i := 0; i < len(s.mirrorList()); i++ {
		cname := s.containerName(i)
		if _, err := h.Run("docker rm -f " + cname); err != nil {
			return fmt.Errorf("removing registry mirror container: %w", err)
		}
		fmt.Fprintf(w, "  registry: removed %s\n", cname)
		if !purgeData {
			continue
		}
		storage, err := s.mirrorStoragePath(i, h)
		if err != nil {
			return err
		}
		if _, err := h.Run("rm -rf " + hostshell.PosixSingleQuote(storage)); err != nil {
			return fmt.Errorf("removing registry mirror storage: %w", err)
		}
		fmt.Fprintf(w, "  registry: purged %s\n", storage)
	}
	return nil
}

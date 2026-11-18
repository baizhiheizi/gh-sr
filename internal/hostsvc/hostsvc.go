// Package hostsvc owns the rules shared by gh-sr's host-published Docker
// services — the per-host Actions cache server and the per-host registry
// pull-through mirrors.
//
// The two services are layer-siblings: each publishes a container port on the
// host, keeps its state in a host-side storage dir that may be expressed
// relative to the remote shell's $HOME, injects a runner-facing URL into the
// runner containers, and reports the same status fields. Each rule used to be
// implemented once per service — and, for the $HOME expansion, once per
// representation (Go-side resolution, the shell snippet that splices $HOME
// into a generated script, and the parser that splices it back) — so a fix
// had to be made in each copy and the copies could drift apart.
//
// This package is the single owner of those rules. It deliberately depends on
// nothing but internal/host and internal/hostshell, so both services can
// import it without a cycle.
package hostsvc

import (
	"fmt"
	"io"
	"strings"

	"github.com/an-lee/gh-sr/internal/host"
	"github.com/an-lee/gh-sr/internal/hostshell"
)

// HomePrefix is the remote-shell home prefix a configured storage path may
// carry. Docker bind mounts and mkdir do not undergo shell expansion, so the
// prefix has to be resolved to an absolute path before use.
const HomePrefix = "$HOME"

// StatusInfo is the reported state of one host-published service container.
type StatusInfo struct {
	State        string // Docker state ("running", "exited", ...) or "" when absent
	Image        string
	URL          string // runner-facing base URL ("" when not resolvable)
	StoragePath  string
	StorageBytes int64
	Healthy      bool // the service's health endpoint answered
}

// StoragePathOrDefault returns the configured storage path, falling back to
// defaultPath when the configuration leaves it empty.
func StoragePathOrDefault(configured, defaultPath string) string {
	if configured == "" {
		return defaultPath
	}
	return configured
}

// RemoteHome resolves the remote shell's home directory.
func RemoteHome(h *host.Host) (string, error) {
	out, err := h.Run("echo $HOME")
	if err != nil {
		return "", fmt.Errorf("resolving home dir: %w", err)
	}
	home := strings.TrimSpace(out)
	if home == "" {
		return "", fmt.Errorf("resolving home dir: empty $HOME")
	}
	return home, nil
}

// ExpandHomePath splices a resolved remote home dir onto a $HOME-prefixed
// path. home is the value the remote shell reported (e.g. the probe's H: tag);
// an empty home is an error, because a path that silently lost its prefix
// would point at the wrong directory.
func ExpandHomePath(home, configured, defaultPath string) (string, error) {
	p := StoragePathOrDefault(configured, defaultPath)
	if !strings.HasPrefix(p, HomePrefix) {
		return p, nil
	}
	if home == "" {
		return "", fmt.Errorf("resolving home dir: empty $HOME")
	}
	return home + p[len(HomePrefix):], nil
}

// ResolveStoragePath resolves the host-side storage dir of a service:
// an empty configuration falls back to defaultPath, and a $HOME-prefixed path
// is expanded against the remote shell's home directory.
func ResolveStoragePath(h *host.Host, configured, defaultPath string) (string, error) {
	p := StoragePathOrDefault(configured, defaultPath)
	if !strings.HasPrefix(p, HomePrefix) {
		return p, nil
	}
	home, err := RemoteHome(h)
	if err != nil {
		return "", err
	}
	return ExpandHomePath(home, p, defaultPath)
}

// StoragePathShell renders the POSIX shell assignment that resolves the
// storage path on the host:
//
//	storage=$HOME'/rest'   for a $HOME-prefixed path (the shell expands $HOME)
//	storage='/abs/path'    for an absolute path
//
// It is the shell-side twin of ExpandHomePath: the generated script and the
// parser that reads its output must agree on the convention.
func StoragePathShell(configured, defaultPath string) string {
	p := StoragePathOrDefault(configured, defaultPath)
	if strings.HasPrefix(p, HomePrefix) {
		return "storage=$HOME" + hostshell.PosixSingleQuote(p[len(HomePrefix):])
	}
	return "storage=" + hostshell.PosixSingleQuote(p)
}

// RunnerURL computes the runner-facing base URL of a host-published service:
//
//	urlOverride  wins verbatim (an escape hatch for exotic networks),
//	bindAddr     wins next when it is an explicit address (not 0.0.0.0),
//	gatewayIP    is the docker0 gateway — the address a container can reach.
//
// An empty gateway yields "" so the caller knows there is nothing to inject.
// The returned URL carries no trailing slash; the actions runner's
// results-URL convention is the caller's business.
func RunnerURL(urlOverride, bindAddr, gatewayIP string, port int) string {
	if urlOverride != "" {
		return urlOverride
	}
	hostPart := bindAddr
	if hostPart == "" || hostPart == "0.0.0.0" {
		hostPart = gatewayIP
	}
	if hostPart == "" {
		return ""
	}
	return fmt.Sprintf("http://%s:%d", hostPart, port)
}

// LocalURL computes the service's base URL as seen from the host itself: an
// explicit bind address wins; the 0.0.0.0 / auto bind answers on the docker0
// gateway (a host-local address), falling back to loopback when no gateway
// was found.
func LocalURL(bindAddr, gatewayIP string, port int) string {
	hostPart := bindAddr
	if hostPart == "" || hostPart == "0.0.0.0" {
		if gatewayIP != "" {
			hostPart = gatewayIP
		} else {
			hostPart = "127.0.0.1"
		}
	}
	return fmt.Sprintf("http://%s:%d", hostPart, port)
}

// RemoveOpts describes the container (and the optional data dir) a
// host-published service removal tears down.
type RemoveOpts struct {
	// Container is the Docker container name.
	Container string
	// Noun names the service in error messages ("cache", "registry mirror").
	Noun string
	// Label prefixes the progress notes ("cache", "registry").
	Label string
	// PurgeData makes the removal delete the host-side storage dir too.
	PurgeData bool
	// StoragePath resolves the host-side data dir to purge. It is called
	// only when PurgeData is set, and only after the container is gone, so a
	// resolution failure can never leave the container running.
	StoragePath func() (string, error)
}

// RemoveContainer stops and deletes a host-published service container. When
// PurgeData is set it also deletes the host-side storage dir.
func RemoveContainer(w io.Writer, h *host.Host, opts RemoveOpts) error {
	if _, err := h.Run("docker rm -f " + opts.Container); err != nil {
		return fmt.Errorf("removing %s container: %w", opts.Noun, err)
	}
	fmt.Fprintf(w, "  %s: removed %s\n", opts.Label, opts.Container)
	if !opts.PurgeData {
		return nil
	}
	storage, err := opts.StoragePath()
	if err != nil {
		return err
	}
	if _, err := h.Run("rm -rf " + hostshell.PosixSingleQuote(storage)); err != nil {
		return fmt.Errorf("removing %s storage: %w", opts.Noun, err)
	}
	fmt.Fprintf(w, "  %s: purged %s\n", opts.Label, storage)
	return nil
}

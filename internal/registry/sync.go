// Prefetch sync: pull each registry_mirror.prefetch image once per host —
// through the matching pull-through mirror when one exists — and load it into
// every container runner's inner dockerd. This is the WAN-egress fix for
// registries dockerd cannot mirror transparently (ghcr.io et al,
// moby/moby#41456): each image costs one upstream fetch per host instead of
// one per runner.

package registry

import (
	"fmt"
	"io"
	"strings"

	"github.com/an-lee/gh-sr/internal/host"
	"github.com/an-lee/gh-sr/internal/hostshell"
)

// SyncResult counts what one host's sync did.
//
// There is deliberately no host-side "skipped" counter: `docker pull` must
// contact the registry to learn whether a :latest moved, so a host-side pull
// always costs a manifest request and there is no meaningful already-present
// case to count. The skip that does save real work is the per-runner one
// (Cached), matched by image ID.
type SyncResult struct {
	Pulled int // images fetched on the host (via mirror or direct)
	Loaded int // docker save | docker load transfers into runner containers
	Cached int // runner containers that already had the image and were skipped
	Failed int // pull or load failures (reported inline; sync continues)
}

// SyncPrefetch pulls refs once on the host and loads them into every named
// runner container's inner dockerd, skipping containers that already have the
// exact image. runnerContainers are Docker container names (e.g. gh-sr-x-1);
// containers that are not running are reported as failures (docker exec needs
// a live container).
func SyncPrefetch(w io.Writer, h *host.Host, s *Settings, refs []string, runnerContainers []string) (SyncResult, error) {
	var res SyncResult
	var pullErrs []string
	for _, ref := range refs {
		ref = strings.TrimSpace(ref)
		if ref == "" {
			continue
		}
		id, err := syncPullOnHost(w, h, s, ref)
		if err != nil {
			res.Failed++
			pullErrs = append(pullErrs, fmt.Sprintf("%s: %v", ref, err))
			continue
		}
		res.Pulled++
		for _, cname := range runnerContainers {
			ok, err := syncIntoRunner(h, cname, ref, id)
			if err != nil {
				res.Failed++
				fmt.Fprintf(w, "  images: %s: load %s failed: %v\n", cname, ref, err)
				continue
			}
			if ok {
				res.Loaded++
			} else {
				res.Cached++
			}
		}
	}
	var err error
	if len(pullErrs) > 0 {
		err = fmt.Errorf("failed to pull %d image(s) on the host: %s", len(pullErrs), strings.Join(pullErrs, "; "))
	}
	return res, err
}

// syncPullOnHost ensures ref exists in the HOST docker image store and returns
// its image ID. It pulls through the matching mirror (127.0.0.1:<port>, plain
// HTTP — dockerd treats loopback as insecure) when one is configured for the
// ref's registry and the bind allows it, so the upstream is fetched through
// the pull-through cache; otherwise it pulls the ref directly.
func syncPullOnHost(w io.Writer, h *host.Host, s *Settings, ref string) (string, error) {
	pullRef := ref
	via := ""
	if s != nil {
		if i := s.mirrorForRef(ref); i >= 0 {
			if base := s.hostPullRefBase(i); base != "" {
				pullRef = mirrorPullRef(base, ref)
				via = s.Mirrors[i].Name
			}
		}
	}
	if via != "" {
		fmt.Fprintf(w, "  images: pulling %s via %s mirror...\n", ref, via)
	} else {
		fmt.Fprintf(w, "  images: pulling %s...\n", ref)
	}
	if _, err := h.Run("docker pull " + hostshell.PosixSingleQuote(pullRef)); err != nil {
		// Already-present images make `docker pull` a no-op, not an error —
		// reaching here means the pull genuinely failed.
		return "", fmt.Errorf("docker pull %s: %w", pullRef, err)
	}
	// Fetch the image ID from the canonical ref; when the pull went through a
	// mirror the fetched image carries the mirror-prefixed name, so retag it
	// to the canonical reference first (pure metadata, no data movement).
	inspectRef := pullRef
	if via != "" {
		if _, err := h.Run("docker tag " + hostshell.PosixSingleQuote(pullRef) + " " + hostshell.PosixSingleQuote(ref)); err != nil {
			return "", fmt.Errorf("docker tag %s -> %s: %w", pullRef, ref, err)
		}
		inspectRef = ref
	}
	idOut, err := h.Run("docker image inspect --format='{{.Id}}' " + hostshell.PosixSingleQuote(inspectRef))
	if err != nil {
		return "", fmt.Errorf("inspecting %s on host: %w", inspectRef, err)
	}
	id := strings.TrimSpace(idOut)
	if id == "" {
		return "", fmt.Errorf("inspecting %s on host: empty image ID", inspectRef)
	}
	return id, nil
}

// syncIntoRunner loads the image (by ID — docker save/load does not preserve
// repository digests, and IDs are stable across both) into one runner
// container's inner dockerd when it is not already present. Returns loaded =
// false when the container already has the image.
func syncIntoRunner(h *host.Host, cname, ref, imageID string) (loaded bool, err error) {
	// Presence check by image ID: works for tag and digest refs alike, and
	// matches images loaded by an earlier sync (which tagged them lazily).
	chkOut, chkErr := h.Run(fmt.Sprintf(
		"docker exec %s docker image inspect %s >/dev/null 2>&1 && echo yes || echo no",
		hostshell.PosixSingleQuote(cname), hostshell.PosixSingleQuote(imageID)))
	if chkErr == nil && strings.TrimSpace(chkOut) == "yes" {
		return false, nil
	}
	// Stream the image into the inner dockerd. docker load restores the
	// layers keyed by content; a tag is reapplied below when the ref carries
	// one (a bare digest reference cannot be tagged).
	if _, err := h.Run(fmt.Sprintf("docker save %s | docker exec -i %s docker load",
		hostshell.PosixSingleQuote(imageID), hostshell.PosixSingleQuote(cname))); err != nil {
		return false, fmt.Errorf("docker save | docker load: %w", err)
	}
	if tagOf(ref) != "" {
		if _, err := h.Run(fmt.Sprintf("docker exec %s docker tag %s %s",
			hostshell.PosixSingleQuote(cname), hostshell.PosixSingleQuote(imageID), hostshell.PosixSingleQuote(ref))); err != nil {
			return false, fmt.Errorf("docker tag %s in %s: %w", ref, cname, err)
		}
	}
	return true, nil
}

// mirrorForRef returns the index of the mirror whose upstream serves ref's
// registry, or -1. Docker Hub references (bare names, docker.io) match the
// first mirror via its registry-1.docker.io default.
func (s *Settings) mirrorForRef(ref string) int {
	if s == nil {
		return -1
	}
	regHost := normalizeRegistryHost(RegistryHostOf(ref))
	for i, m := range s.Mirrors {
		up := m.Upstream
		if i == 0 && up == "" {
			up = DefaultUpstream
		}
		if u := strings.Index(up, "://"); u >= 0 {
			up = up[u+3:]
		}
		if idx := strings.Index(up, "/"); idx >= 0 {
			up = up[:idx]
		}
		if normalizeRegistryHost(up) == regHost {
			return i
		}
	}
	return -1
}

// RegistryHostOf extracts the registry host from an image reference: the
// first path segment when it looks like a host (contains '.' or ':' or is
// "localhost"), else "docker.io" (the implicit default registry). A
// slash-less reference ("postgres:16") has no registry part at all — the ':'
// there is the tag separator.
func RegistryHostOf(ref string) string {
	i := strings.Index(ref, "/")
	if i <= 0 {
		return "docker.io"
	}
	first := ref[:i]
	if strings.ContainsAny(first, ".:") || first == "localhost" {
		return strings.ToLower(first)
	}
	return "docker.io"
}

// normalizeRegistryHost maps Docker Hub's registry endpoints onto one
// canonical name for mirror matching.
func normalizeRegistryHost(host string) string {
	host = strings.ToLower(strings.TrimSuffix(host, "/"))
	switch host {
	case "docker.io", "index.docker.io", "registry-1.docker.io":
		return "docker.io"
	}
	return host
}

// mirrorPullRef rewrites an upstream reference into one served by a mirror on
// base ("127.0.0.1:5578"): the registry host is replaced by the mirror and
// bare Docker Hub official-image names gain their library/ prefix (the mirror
// serves the upstream repository path verbatim).
func mirrorPullRef(base, ref string) string {
	rest := ref
	if i := strings.Index(ref, "/"); i > 0 && (strings.ContainsAny(ref[:i], ".:") || ref[:i] == "localhost") {
		rest = ref[i+1:]
	}
	if !strings.Contains(rest, "/") {
		rest = "library/" + rest
	}
	return base + "/" + rest
}

// tagOf returns the tag of an image reference, or "" for digest-only or
// tag-less references ("ghcr.io/a/b@sha256:…", "ghcr.io/a/b").
func tagOf(ref string) string {
	if at := strings.Index(ref, "@"); at >= 0 {
		return ""
	}
	last := ref
	if i := strings.LastIndex(ref, "/"); i >= 0 {
		last = ref[i+1:]
	}
	if c := strings.Index(last, ":"); c >= 0 {
		return last[c+1:]
	}
	return ""
}

package ops

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/an-lee/gh-sr/internal/host"
	"github.com/an-lee/gh-sr/internal/hostshell"
	"github.com/an-lee/gh-sr/internal/runner"
)

// measureSharedVolumes reports one gh sr disk usage row per gh-sr-* Docker
// volume on the host (the shared_caches volumes, plus any mirror/registry
// volumes gh-sr ever created). This is the observability slice of issue #559
// proposal 4: it makes the shared-cache artifacts' growth visible next to the
// cache server and registry mirror rows, which is the local proxy for "how
// many WAN bytes did the caches save".
//
// Volume data lives under /var/lib/docker/volumes, which the (non-root)
// operator user cannot read directly — the portable measurement is a
// throwaway container mounting every volume. busybox is small and, with the
// per-host mirror deployed, is itself served from the Docker Hub pull-through
// cache. Everything is best-effort: a docker failure yields a single row
// carrying the error instead of failing the whole disk-usage report, and a
// host without gh-sr volumes (or without Docker) yields no rows.
func measureSharedVolumes(h *host.Host, hostName string) []runner.DiskUsageEntry {
	const base = "/var/lib/docker/volumes"
	out, err := h.Run("docker volume ls -q 2>/dev/null | grep '^gh-sr-' | sort")
	if err != nil {
		return nil
	}
	var names []string
	for _, l := range strings.Split(out, "\n") {
		if n := strings.TrimSpace(l); n != "" {
			names = append(names, n)
		}
	}
	if len(names) == 0 {
		return nil
	}

	// One container measuring every volume: -v <vol>:/v/<i> then
	// `du -sb /v/<i>` per mount, paths stripped back to indices.
	var mounts strings.Builder
	for i, n := range names {
		mounts.WriteString(" -v " + hostshell.PosixSingleQuote(n+":/v/"+strconv.Itoa(i)))
	}
	duOut, duErr := h.Run(fmt.Sprintf(
		"docker run --rm%s busybox du -sb /v/* 2>/dev/null | sed 's|/v/||'", mounts.String()))
	sizes := make(map[int]int64)
	if duErr == nil {
		for _, l := range strings.Split(duOut, "\n") {
			raw, path, ok := strings.Cut(strings.TrimSpace(l), "\t")
			if !ok {
				continue
			}
			idx, ierr := strconv.Atoi(path)
			n, perr := strconv.ParseInt(raw, 10, 64)
			if ierr != nil || perr != nil || idx < 0 || idx >= len(names) {
				continue
			}
			sizes[idx] = n
		}
	}

	// The throwaway container's output is piped through `sed`, so the pipeline's
	// exit status is sed's — a failed `docker run` (most likely: busybox not
	// pullable on an offline or metered host without the mirror configured)
	// still exits 0. Detect "du produced nothing at all" separately, otherwise
	// every volume would be reported as "empty or missing", which both
	// misdescribes the state and silently drops real cache data from the host
	// total on the very command meant to warn about disk pressure.
	measureFailed := duErr == nil && len(sizes) == 0

	entries := make([]runner.DiskUsageEntry, 0, len(names))
	for i, n := range names {
		e := runner.DiskUsageEntry{
			Host:     hostName,
			Instance: n,
			Path:     base + "/" + n + "/_data",
			Mode:     "volume",
		}
		switch {
		case measureFailed:
			e.Err = fmt.Errorf("measuring volumes failed (the busybox helper produced no output — is busybox pullable on this host?)")
		default:
			if size, ok := sizes[i]; ok {
				e.TotalBytes = size
			} else if duErr != nil {
				e.Err = fmt.Errorf("measuring volume: %v", duErr)
			} else {
				e.Err = fmt.Errorf("no size reported (volume empty or missing)")
			}
		}
		entries = append(entries, e)
	}
	return entries
}

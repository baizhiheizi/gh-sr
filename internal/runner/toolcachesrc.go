// Host-side content-addressed cache for container_runner_image.toolcache
// sources (issue #559 proposal 3). The bake is a single Docker RUN layer, so
// ANY change to the toolcache list — or a base-image bump, or a builder-cache
// prune — re-runs the whole loop and re-downloads every tarball over the WAN
// (Docker cannot partially reuse a RUN layer). Downloading each URL once into
// ~/.gh-sr/toolcache-src/<sha256(url)[:16]> and feeding the blobs into the
// build context makes every later rebuild read local disk instead: the key is
// derived from the URL alone, so a tarball whose URL is unchanged is never
// fetched again.

package runner

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/an-lee/gh-sr/internal/config"
	"github.com/an-lee/gh-sr/internal/host"
	"github.com/an-lee/gh-sr/internal/hostshell"
)

// toolcacheSrcDir is the host-side cache directory ("$HOME/..." — resolved by
// the remote shell itself, so no extra SSH round-trip for $HOME).
const toolcacheSrcDir = "$HOME/.gh-sr/toolcache-src"

// toolcacheSourceKey derives the cache key of a toolcache URL: the first 16
// hex chars of sha256(url). URLs are validated (https, no whitespace) at
// config load, so the key is filename-safe by construction.
func toolcacheSourceKey(url string) string {
	sum := sha256.Sum256([]byte(url))
	return hex.EncodeToString(sum[:])[:16]
}

// syncToolcacheSources ensures every toolcache tarball exists in the
// host-side cache and links the cached blobs into the build context's
// toolcache-blobs/ directory (created with a .keep file even when the list is
// empty — the Dockerfile COPYs the directory unconditionally).
//
// It returns, per entry index, the blob file name that landed in the context;
// entries whose download failed (or that had no URL) are absent, and the
// Dockerfile bake loop then falls back to curl-ing the URL itself — identical
// failure semantics to the pre-cache build, minus the WAN cost on success.
//
// The whole thing is one SSH round-trip: a generated shell script that skips
// entries already cached (`[ -s ]`), downloads missing ones with the same
// retry/timeout policy the Dockerfile bake uses, atomically renames into
// place (`.tmp` + mv so a partial download can never be mistaken for a
// complete blob), and hard-links (falling back to cp across filesystems)
// each blob into the build context.
func syncToolcacheSources(h *host.Host, entries []config.ContainerToolcacheEntry) map[int]string {
	blobs := make(map[int]string)
	if len(entries) == 0 {
		// Still prepare the context dir: the Dockerfile COPYs it
		// unconditionally and COPY fails on a missing source. The path is a
		// fixed gh-sr constant — no quoting needed.
		_, _ = h.Run("mkdir -p /tmp/gh-sr-agentic-runner-build/toolcache-blobs && : > /tmp/gh-sr-agentic-runner-build/toolcache-blobs/.keep")
		return blobs
	}

	var script strings.Builder
	script.WriteString("set -u\n")
	script.WriteString("mkdir -p '" + toolcacheSrcDir + "' /tmp/gh-sr-agentic-runner-build/toolcache-blobs\n")
	script.WriteString(": > /tmp/gh-sr-agentic-runner-build/toolcache-blobs/.keep\n")
	script.WriteString("echo READY\n")
	for _, e := range entries {
		if e.URL == "" {
			continue
		}
		key := toolcacheSourceKey(e.URL)
		dest := toolcacheSrcDir + "/" + key
		// URLs are https-only and whitespace-free (config validation), so
		// single-quoting is belt-and-braces.
		script.WriteString(fmt.Sprintf(`if [ -s %s ]; then echo 'CACHED %s'; else
  if curl -fsSL --retry 8 --retry-all-errors --retry-delay 20 --connect-timeout 20 --speed-limit 10240 --speed-time 60 --max-time 1800 %s -o %s.tmp 2>/dev/null && mv -f %s.tmp %s; then echo 'FETCHED %s'; else rm -f %s.tmp; echo 'MISS %s'; fi
fi
`,
			hostshell.PosixSingleQuote(dest),
			key,
			hostshell.PosixSingleQuote(e.URL),
			hostshell.PosixSingleQuote(dest),
			hostshell.PosixSingleQuote(dest),
			hostshell.PosixSingleQuote(dest),
			key,
			hostshell.PosixSingleQuote(dest),
			key,
		))
		// Link every cached-or-fetched blob into the build context.
		script.WriteString(fmt.Sprintf(
			"[ -s %[1]s ] && { cp -l %[1]s /tmp/gh-sr-agentic-runner-build/toolcache-blobs/%[2]s 2>/dev/null || cp %[1]s /tmp/gh-sr-agentic-runner-build/toolcache-blobs/%[2]s; } || true\n",
			hostshell.PosixSingleQuote(dest), key))
	}
	out, err := h.Run(script.String())
	if err != nil {
		// Best-effort by design: a failed pre-sync degrades to the
		// Dockerfile's own curl fallback (the pre-cache behaviour).
		return blobs
	}
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		var verb, key string
		if _, err := fmt.Sscanf(line, "%s %s", &verb, &key); err != nil {
			continue
		}
		if verb != "CACHED" && verb != "FETCHED" {
			continue
		}
		for i, e := range entries {
			if e.URL != "" && toolcacheSourceKey(e.URL) == key {
				blobs[i] = key
			}
		}
	}
	return blobs
}

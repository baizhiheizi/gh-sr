package ops

import (
	"fmt"
	"io"

	"github.com/an-lee/gh-sr/internal/cache"
	"github.com/an-lee/gh-sr/internal/config"
	"github.com/an-lee/gh-sr/internal/host"
)

// cacheDescriptor describes the per-host Actions cache server to the shared
// per-host service orchestrators. The cache server is a single container, so
// its inspect returns exactly one status row.
var cacheDescriptor = serviceDescriptor[cache.Settings, cache.StatusInfo]{
	label:     "cache",
	title:     "Cache",
	configKey: "cache",
	workload:  "the cache server",
	settings:  cacheSettings,
	ensure:    cache.Ensure,
	inspect: func(h *host.Host, s cache.Settings) ([]cache.StatusInfo, error) {
		info, err := cache.Inspect(h, s)
		if err != nil {
			return nil, err
		}
		return []cache.StatusInfo{info}, nil
	},
	print:  printCacheStatus,
	remove: cache.Remove,
}

// cacheSettings builds the effective cache settings from runners.yml. nil when
// the cache is disabled (zero-value Configs without a Load are disabled — the
// enabled default is applied by config.applyDefaults).
func cacheSettings(cfg *config.Config) *cache.Settings {
	return cache.SettingsFromConfig(cfg)
}

// cacheTargets returns the sorted host names a cache command should run on,
// narrowed by serviceTargets' shared Linux-only/--host filtering. The workload
// wording comes from cacheDescriptor, so the skip note names the service
// exactly as the descriptor's status/deploy/remove paths do.
func cacheTargets(w io.Writer, cfg *config.Config, filterHost string) ([]string, error) {
	return serviceTargets(w, cfg, filterHost, cacheDescriptor.workload)
}

// CacheDeploy ensures the per-host cache server container exists and runs.
func CacheDeploy(w io.Writer, cfg *config.Config, filterHost string) error {
	return deployService(w, cfg, filterHost, cacheDescriptor)
}

// CacheStatus reports the per-host cache server state, health, and storage.
func CacheStatus(w io.Writer, cfg *config.Config, filterHost string) error {
	return statusService(w, cfg, filterHost, cacheDescriptor)
}

func printCacheStatus(w io.Writer, info cache.StatusInfo) {
	if info.State == "" {
		fmt.Fprintf(w, "  cache: not deployed\n")
		return
	}
	fmt.Fprintf(w, "  state:   %s\n", serviceState(info))
	fmt.Fprintf(w, "  image:   %s\n", info.Image)
	if info.URL != "" {
		fmt.Fprintf(w, "  url:     %s\n", info.URL)
	}
	fmt.Fprintf(w, "  storage: %s\n", serviceStorage(info.StoragePath, info.StorageBytes))
}

// CachePrune deletes all cache entries on every target host (best-effort per
// host). The cache server is the only managed service with a management API,
// so prune stays a cache-specific operation.
func CachePrune(w io.Writer, cfg *config.Config, filterHost string) error {
	hosts, err := cacheTargets(w, cfg, filterHost)
	if err != nil {
		return err
	}
	return forEachServiceHost(w, cfg, hosts, "Pruning cache", cacheSettings, func(w io.Writer, h *host.Host, s *cache.Settings) error {
		if s == nil {
			fmt.Fprintf(w, "  cache: disabled in runners.yml\n")
			return nil
		}
		return cache.Prune(w, h, *s)
	})
}

// CacheRemove uninstalls the cache server; purgeData also deletes stored cache
// data. Runner removal (gh sr remove) never touches the cache — this command
// is the only uninstall path.
func CacheRemove(w io.Writer, cfg *config.Config, filterHost string, purgeData bool) error {
	return removeService(w, cfg, filterHost, purgeData, cacheDescriptor)
}

package ops

import (
	"fmt"
	"io"

	"github.com/an-lee/gh-sr/internal/cache"
	"github.com/an-lee/gh-sr/internal/config"
	"github.com/an-lee/gh-sr/internal/host"
	"github.com/an-lee/gh-sr/internal/runner"
)

// cacheSettings builds the effective cache settings from runners.yml. nil when
// the cache is disabled (zero-value Configs without a Load are disabled — the
// enabled default is applied by config.applyDefaults).
func cacheSettings(cfg *config.Config) *cache.Settings {
	return cache.SettingsFromConfig(cfg)
}

// cacheTargets returns the sorted host names a cache command should run on,
// narrowed by serviceTargets' shared Linux-only/--host filtering.
func cacheTargets(w io.Writer, cfg *config.Config, filterHost string) ([]string, error) {
	return serviceTargets(w, cfg, filterHost, "the cache server")
}

// CacheDeploy ensures the per-host cache server container exists and runs.
func CacheDeploy(w io.Writer, cfg *config.Config, filterHost string) error {
	if cacheSettings(cfg) == nil {
		return fmt.Errorf("cache is disabled in runners.yml (cache.enabled: false); enable it or remove the setting")
	}
	hosts, err := cacheTargets(w, cfg, filterHost)
	if err != nil {
		return err
	}
	return forEachServiceHost(w, cfg, hosts, "Deploying cache", cacheSettings, func(w io.Writer, h *host.Host, s *cache.Settings) error {
		return cache.Ensure(w, h, *s)
	})
}

// CacheStatus reports the per-host cache server state, health, and storage.
func CacheStatus(w io.Writer, cfg *config.Config, filterHost string) error {
	hosts, err := cacheTargets(w, cfg, filterHost)
	if err != nil {
		return err
	}
	return forEachServiceHost(w, cfg, hosts, "Cache status", cacheSettings, func(w io.Writer, h *host.Host, s *cache.Settings) error {
		if s == nil {
			fmt.Fprintf(w, "  cache: disabled in runners.yml\n")
			return nil
		}
		info, err := cache.Inspect(h, *s)
		if err != nil {
			return err
		}
		printCacheStatus(w, info)
		return nil
	})
}

func printCacheStatus(w io.Writer, info cache.StatusInfo) {
	if info.State == "" {
		fmt.Fprintf(w, "  cache: not deployed\n")
		return
	}
	state := info.State
	if info.Healthy {
		state += " (healthy)"
	}
	fmt.Fprintf(w, "  state:   %s\n", state)
	fmt.Fprintf(w, "  image:   %s\n", info.Image)
	if info.URL != "" {
		fmt.Fprintf(w, "  url:     %s\n", info.URL)
	}
	fmt.Fprintf(w, "  storage: %s (%s)\n", info.StoragePath, runner.FormatBytesHuman(info.StorageBytes))
}

// CachePrune deletes all cache entries on every target host (best-effort per host).
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
	hosts, err := cacheTargets(w, cfg, filterHost)
	if err != nil {
		return err
	}
	return forEachServiceHost(w, cfg, hosts, "Removing cache", cacheSettings, func(w io.Writer, h *host.Host, s *cache.Settings) error {
		var settings cache.Settings
		if s != nil {
			settings = *s
		}
		return cache.Remove(w, h, settings, purgeData)
	})
}

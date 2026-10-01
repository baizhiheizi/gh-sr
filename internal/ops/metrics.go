package ops

import (
	"io"
	"sort"

	"github.com/an-lee/gh-sr/internal/config"
	"github.com/an-lee/gh-sr/internal/host"
)

// CollectHostMetrics connects to each unique host in the config and gathers
// resource usage concurrently. Hosts are returned sorted by name.
func CollectHostMetrics(w io.Writer, cfg *config.Config, filterHost string) []host.HostMetrics {
	names := sortedHostNames(cfg, filterHost)

	// hostGroup carries just the host name here (no runners); fanOutHosts
	// only needs the name to connect and to order results.
	groups := make([]hostGroup, len(names))
	for i, name := range names {
		groups[i] = hostGroup{name: name}
	}

	results := fanOutHosts(w, cfg, groups, func(_ io.Writer, h *host.Host, g hostGroup) (host.HostMetrics, error) {
		return h.CollectMetrics(), nil
	})

	metrics := make([]host.HostMetrics, len(results))
	for i, r := range results {
		if r.ConnectErr != nil {
			metrics[i] = host.HostMetrics{Name: r.Name, Err: r.ConnectErr}
			continue
		}
		metrics[i] = r.Val
	}
	return metrics
}

func sortedHostNames(cfg *config.Config, filterHost string) []string {
	if filterHost != "" {
		if _, ok := cfg.Hosts[filterHost]; ok {
			return []string{filterHost}
		}
		return nil
	}
	names := make([]string, 0, len(cfg.Hosts))
	for k := range cfg.Hosts {
		names = append(names, k)
	}
	sort.Strings(names)
	return names
}

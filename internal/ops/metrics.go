package ops

import (
	"fmt"
	"io"
	"sort"

	"github.com/an-lee/ghr/internal/config"
	"github.com/an-lee/ghr/internal/host"
)

// CollectHostMetrics connects to each unique host in the config and gathers
// resource usage. Hosts are returned sorted by name.
func CollectHostMetrics(w io.Writer, cfg *config.Config, filterHost string) []host.HostMetrics {
	names := sortedHostNames(cfg, filterHost)
	metrics := make([]host.HostMetrics, 0, len(names))

	for _, name := range names {
		hcfg := cfg.Hosts[name]
		h, err := ConnectHost(name, hcfg)
		if err != nil {
			if w != nil {
				fmt.Fprintf(w, "Warning: cannot connect to %s: %v\n", name, err)
			}
			metrics = append(metrics, host.HostMetrics{Name: name, Err: err})
			continue
		}
		m := h.CollectMetrics()
		h.Close()
		metrics = append(metrics, m)
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

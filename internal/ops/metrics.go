package ops

import (
	"fmt"
	"io"
	"sort"
	"sync"

	"github.com/an-lee/ghr/internal/config"
	"github.com/an-lee/ghr/internal/host"
)

// CollectHostMetrics connects to each unique host in the config and gathers
// resource usage concurrently. Hosts are returned sorted by name.
func CollectHostMetrics(w io.Writer, cfg *config.Config, filterHost string) []host.HostMetrics {
	names := sortedHostNames(cfg, filterHost)
	metrics := make([]host.HostMetrics, len(names))

	var wg sync.WaitGroup
	var wMu sync.Mutex // guards writes to w

	for i, name := range names {
		wg.Add(1)
		go func(i int, name string) {
			defer wg.Done()
			hcfg := cfg.Hosts[name]
			h, err := ConnectHost(name, hcfg)
			if err != nil {
				if w != nil {
					wMu.Lock()
					fmt.Fprintf(w, "Warning: cannot connect to %s: %v\n", name, err)
					wMu.Unlock()
				}
				metrics[i] = host.HostMetrics{Name: name, Err: err}
				return
			}
			m := h.CollectMetrics()
			h.Close()
			metrics[i] = m
		}(i, name)
	}
	wg.Wait()
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

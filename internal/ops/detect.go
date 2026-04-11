package ops

import (
	"fmt"
	"io"
	"sync"

	"github.com/an-lee/gh-sr/internal/config"
	"github.com/an-lee/gh-sr/internal/host"
)

// ResolveHostInfo connects to each host that is missing OS or arch and auto-detects them.
// It mutates cfg.Hosts in place so all downstream code sees resolved values.
//
// Hosts are probed concurrently so that N-host configurations incur O(SSH_latency)
// instead of O(N × SSH_latency) startup overhead.
func ResolveHostInfo(w io.Writer, cfg *config.Config) error {
	if !cfg.NeedsDetection() {
		return nil
	}

	type hostEntry struct {
		name string
		hcfg config.HostConfig
	}
	type detectionResult struct {
		name string
		hcfg config.HostConfig
		err  error
	}

	// Collect only hosts that need detection.
	var toDetect []hostEntry
	for name, hcfg := range cfg.Hosts {
		if config.IsLocalAddr(hcfg.Addr) {
			continue
		}
		if hcfg.OS != "" && hcfg.Arch != "" {
			continue
		}
		toDetect = append(toDetect, hostEntry{name, hcfg})
	}

	results := make([]detectionResult, len(toDetect))
	var wg sync.WaitGroup
	var wMu sync.Mutex // guards writes to w

	for i, e := range toDetect {
		wg.Add(1)
		go func(i int, name string, hcfg config.HostConfig) {
			defer wg.Done()
			if w != nil {
				wMu.Lock()
				fmt.Fprintf(w, "Detecting OS/arch for host %s (%s)...\n", name, hcfg.Addr)
				wMu.Unlock()
			}
			conn, err := ConnectHost(name, hcfg)
			if err != nil {
				results[i] = detectionResult{name: name, hcfg: hcfg, err: fmt.Errorf("auto-detect %s: %w", name, err)}
				return
			}
			if hcfg.OS == "" {
				detectedOS, err := host.DetectOS(conn)
				if err != nil {
					conn.Close()
					results[i] = detectionResult{name: name, hcfg: hcfg, err: fmt.Errorf("auto-detect OS for %s: %w", name, err)}
					return
				}
				hcfg.OS = detectedOS
			}
			if hcfg.Arch == "" {
				detectedArch, err := host.DetectArch(conn)
				if err != nil {
					conn.Close()
					results[i] = detectionResult{name: name, hcfg: hcfg, err: fmt.Errorf("auto-detect arch for %s: %w", name, err)}
					return
				}
				hcfg.Arch = detectedArch
			}
			conn.Close()
			if w != nil {
				wMu.Lock()
				fmt.Fprintf(w, "  %s: detected os=%s arch=%s\n", name, hcfg.OS, hcfg.Arch)
				wMu.Unlock()
			}
			results[i] = detectionResult{name: name, hcfg: hcfg}
		}(i, e.name, e.hcfg)
	}
	wg.Wait()

	for _, r := range results {
		if r.err != nil {
			return r.err
		}
		cfg.Hosts[r.name] = r.hcfg
	}
	return nil
}

package ops

import (
	"fmt"
	"io"

	"github.com/an-lee/gh-sr/internal/config"
	"github.com/an-lee/gh-sr/internal/fanout"
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

	// The progress lines are interleaved across hosts, so they go through a
	// serialised writer; the per-host results land in their own slice index.
	var out io.Writer = io.Discard
	if w != nil {
		out = fanout.NewLockedWriter(w)
	}

	results := fanout.Indexed(toDetect, func(_ int, e hostEntry) detectionResult {
		res := detectionResult{name: e.name, hcfg: e.hcfg}
		fmt.Fprintf(out, "Detecting OS/arch for host %s (%s)...\n", e.name, e.hcfg.Addr)
		conn, err := connectHostFn(e.name, e.hcfg)
		if err != nil {
			res.err = fmt.Errorf("auto-detect %s: %w", e.name, err)
			return res
		}
		if e.hcfg.OS == "" {
			detectedOS, err := host.DetectOS(conn)
			if err != nil {
				conn.Close()
				res.err = fmt.Errorf("auto-detect OS for %s: %w", e.name, err)
				return res
			}
			e.hcfg.OS = detectedOS
		}
		if e.hcfg.Arch == "" {
			detectedArch, err := host.DetectArch(conn)
			if err != nil {
				conn.Close()
				res.err = fmt.Errorf("auto-detect arch for %s: %w", e.name, err)
				return res
			}
			e.hcfg.Arch = detectedArch
		}
		conn.Close()
		fmt.Fprintf(out, "  %s: detected os=%s arch=%s\n", e.name, e.hcfg.OS, e.hcfg.Arch)
		res.hcfg = e.hcfg
		return res
	})

	for _, r := range results {
		if r.err != nil {
			return r.err
		}
		cfg.Hosts[r.name] = r.hcfg
	}
	return nil
}

package ops

import (
	"fmt"
	"io"

	"github.com/an-lee/gh-wm/internal/config"
	"github.com/an-lee/gh-wm/internal/host"
	"github.com/an-lee/gh-wm/internal/runner"
)

// ResolveHostInfo connects to each host that is missing OS or arch and auto-detects them.
// It mutates cfg.Hosts in place so all downstream code sees resolved values.
func ResolveHostInfo(w io.Writer, cfg *config.Config) error {
	if !cfg.NeedsDetection() {
		return nil
	}
	for name, hcfg := range cfg.Hosts {
		if config.IsLocalAddr(hcfg.Addr) {
			continue
		}
		if hcfg.OS != "" && hcfg.Arch != "" {
			continue
		}
		if w != nil {
			fmt.Fprintf(w, "Detecting OS/arch for host %s (%s)...\n", name, hcfg.Addr)
		}
		h, err := ConnectHost(name, hcfg)
		if err != nil {
			return fmt.Errorf("auto-detect %s: %w", name, err)
		}
		if hcfg.OS == "" {
			detectedOS, err := host.DetectOS(h)
			if err != nil {
				h.Close()
				return fmt.Errorf("auto-detect OS for %s: %w", name, err)
			}
			hcfg.OS = detectedOS
		}
		if hcfg.Arch == "" {
			detectedArch, err := host.DetectArch(h)
			if err != nil {
				h.Close()
				return fmt.Errorf("auto-detect arch for %s: %w", name, err)
			}
			hcfg.Arch = detectedArch
		}
		h.Close()
		cfg.Hosts[name] = hcfg
		if w != nil {
			fmt.Fprintf(w, "  %s: detected os=%s arch=%s\n", name, hcfg.OS, hcfg.Arch)
		}
	}
	return nil
}

// ResolveModes probes Docker availability on each host and sets Mode on runners
// that don't have it explicitly configured, so all downstream code sees a resolved value.
func ResolveModes(w io.Writer, cfg *config.Config) error {
	needsProbe := false
	for _, rc := range cfg.Runners {
		if rc.Mode == "" {
			needsProbe = true
			break
		}
	}
	if !needsProbe {
		return nil
	}

	hostModes := make(map[string]string)
	for i := range cfg.Runners {
		rc := &cfg.Runners[i]
		if rc.Mode != "" {
			continue
		}
		hostName := rc.Host
		if _, done := hostModes[hostName]; !done {
			hcfg := cfg.Hosts[hostName]
			h, err := ConnectHost(hostName, hcfg)
			if err != nil {
				return fmt.Errorf("resolving mode for %s: %w", hostName, err)
			}
			resolved := runner.ResolveModeOnHost(h, *rc)
			h.Close()
			hostModes[hostName] = resolved
			if w != nil {
				fmt.Fprintf(w, "  %s: auto-detected mode=%s\n", hostName, resolved)
			}
		}
		rc.Mode = hostModes[hostName]
	}
	return nil
}

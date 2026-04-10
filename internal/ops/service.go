package ops

import (
	"fmt"
	"io"

	"github.com/an-lee/gh-sr/internal/autostart"
	"github.com/an-lee/gh-sr/internal/config"
	"github.com/an-lee/gh-sr/internal/runner"
)

// ServiceInstall installs OS-level autostart for native runners (systemd, LaunchAgent, or scheduled task).
func ServiceInstall(w io.Writer, cfg *config.Config, filterHost, filterRepo string, nameArgs []string, system bool) error {
	if err := ResolveHostInfo(w, cfg); err != nil {
		return err
	}
	runners := config.FilterRunners(cfg, filterHost, filterRepo, nameArgs)
	for _, rc := range runners {
		hcfg := cfg.Hosts[rc.Host]
		if system && hcfg.OS != "linux" {
			return fmt.Errorf("--system applies only to Linux hosts (host %q is %s)", rc.Host, hcfg.OS)
		}
		if config.IsLocalAddr(hcfg.Addr) {
			fmt.Fprintf(w, "Autostart for %s on %s (local)...\n", rc.Name, rc.Host)
		} else {
			fmt.Fprintf(w, "Autostart for %s on %s (%s)...\n", rc.Name, rc.Host, hcfg.Addr)
		}
		h, err := ConnectHost(rc.Host, hcfg)
		if err != nil {
			return err
		}
		for _, inst := range rc.InstanceNames() {
			ok, err := runner.NativeRunnerConfigPresent(h, inst)
			if err != nil {
				h.Close()
				return fmt.Errorf("%s: %w", inst, err)
			}
			if !ok {
				h.Close()
				return fmt.Errorf("%s: runner not configured on host; run: gh sr setup %s", inst, rc.Name)
			}
			if err := autostart.Install(h, inst, autostart.InstallOpts{System: system}); err != nil {
				h.Close()
				return fmt.Errorf("%s: %w", inst, err)
			}
			fmt.Fprintf(w, "  %s: autostart installed\n", inst)
		}
		h.Close()
	}
	return nil
}

// ServiceUninstall removes autostart definitions created by gh sr service install.
func ServiceUninstall(w io.Writer, cfg *config.Config, filterHost, filterRepo string, nameArgs []string) error {
	if err := ResolveHostInfo(w, cfg); err != nil {
		return err
	}
	runners := config.FilterRunners(cfg, filterHost, filterRepo, nameArgs)
	for _, rc := range runners {
		hcfg := cfg.Hosts[rc.Host]
		if config.IsLocalAddr(hcfg.Addr) {
			fmt.Fprintf(w, "Removing autostart for %s on %s (local)...\n", rc.Name, rc.Host)
		} else {
			fmt.Fprintf(w, "Removing autostart for %s on %s (%s)...\n", rc.Name, rc.Host, hcfg.Addr)
		}
		h, err := ConnectHost(rc.Host, hcfg)
		if err != nil {
			return err
		}
		for _, inst := range rc.InstanceNames() {
			kind, err := autostart.Detect(h, inst)
			if err != nil {
				h.Close()
				return fmt.Errorf("%s: %w", inst, err)
			}
			if kind == autostart.KindNone {
				fmt.Fprintf(w, "  %s: no autostart to remove\n", inst)
				continue
			}
			if err := autostart.Uninstall(h, inst); err != nil {
				h.Close()
				return fmt.Errorf("%s: %w", inst, err)
			}
			fmt.Fprintf(w, "  %s: autostart removed\n", inst)
		}
		h.Close()
	}
	return nil
}

// ServiceStatus prints autostart installation state per runner instance.
func ServiceStatus(w io.Writer, cfg *config.Config, filterHost, filterRepo string, nameArgs []string) error {
	if err := ResolveHostInfo(w, cfg); err != nil {
		return err
	}
	runners := config.FilterRunners(cfg, filterHost, filterRepo, nameArgs)
	for _, rc := range runners {
		hcfg := cfg.Hosts[rc.Host]
		h, err := ConnectHost(rc.Host, hcfg)
		if err != nil {
			return err
		}
		for _, inst := range rc.InstanceNames() {
			row, err := autostart.Status(h, rc.Host, inst, "native")
			if err != nil {
				h.Close()
				return fmt.Errorf("%s: %w", inst, err)
			}
			fmt.Fprintf(w, "%s @ %s [%s]: %s\n", inst, row.Host, row.Mode, row.Detail)
		}
		h.Close()
	}
	return nil
}

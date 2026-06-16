package ops

import (
	"fmt"
	"io"

	"github.com/an-lee/gh-sr/internal/autostart"
	"github.com/an-lee/gh-sr/internal/config"
	"github.com/an-lee/gh-sr/internal/host"
	"github.com/an-lee/gh-sr/internal/runner"
)

// ServiceInstall installs OS-level autostart for native runners (systemd, LaunchAgent, or scheduled task).
func ServiceInstall(w io.Writer, cfg *config.Config, filterHost, filterRepo string, nameArgs []string, system bool) error {
	runners, err := resolveAndFilter(w, cfg, filterHost, filterRepo, nameArgs)
	if err != nil {
		return err
	}
	return runPerHostParallel(w, cfg, runners, func(w io.Writer, h *host.Host, rc config.RunnerConfig) error {
		hcfg := cfg.Hosts[rc.Host]
		if rc.IsContainerMode() {
			fmt.Fprintf(w, "Skipping autostart for %s on %s (runner_mode: container; containers use --restart unless-stopped)\n", rc.Name, rc.Host)
			return nil
		}
		if system && hcfg.OS != "linux" {
			return fmt.Errorf("--system applies only to Linux hosts (host %q is %s)", rc.Host, hcfg.OS)
		}
		if config.IsLocalAddr(hcfg.Addr) {
			fmt.Fprintf(w, "Autostart for %s on %s (local)...\n", rc.Name, rc.Host)
		} else {
			fmt.Fprintf(w, "Autostart for %s on %s (%s)...\n", rc.Name, rc.Host, hcfg.Addr)
		}
		for _, inst := range rc.InstanceNames() {
			ok, err := runner.NativeRunnerConfigPresent(h, inst)
			if err != nil {
				return fmt.Errorf("%s: %w", inst, err)
			}
			if !ok {
				return fmt.Errorf("%s: runner not configured on host; run: gh sr setup %s", inst, rc.Name)
			}
			if err := autostart.Install(h, inst, autostart.InstallOpts{System: system}); err != nil {
				return fmt.Errorf("%s: %w", inst, err)
			}
			fmt.Fprintf(w, "  %s: autostart installed\n", inst)
		}
		return nil
	})
}

// ServiceUninstall removes autostart definitions created by gh sr service install.
func ServiceUninstall(w io.Writer, cfg *config.Config, filterHost, filterRepo string, nameArgs []string) error {
	runners, err := resolveAndFilter(w, cfg, filterHost, filterRepo, nameArgs)
	if err != nil {
		return err
	}
	return runPerHostParallel(w, cfg, runners, func(w io.Writer, h *host.Host, rc config.RunnerConfig) error {
		hcfg := cfg.Hosts[rc.Host]
		if rc.IsContainerMode() {
			fmt.Fprintf(w, "Skipping autostart removal for %s on %s (runner_mode: container)\n", rc.Name, rc.Host)
			return nil
		}
		if config.IsLocalAddr(hcfg.Addr) {
			fmt.Fprintf(w, "Removing autostart for %s on %s (local)...\n", rc.Name, rc.Host)
		} else {
			fmt.Fprintf(w, "Removing autostart for %s on %s (%s)...\n", rc.Name, rc.Host, hcfg.Addr)
		}
		for _, inst := range rc.InstanceNames() {
			kind, err := autostart.Detect(h, inst)
			if err != nil {
				return fmt.Errorf("%s: %w", inst, err)
			}
			if kind == autostart.KindNone {
				fmt.Fprintf(w, "  %s: no autostart to remove\n", inst)
				continue
			}
			if err := autostart.Uninstall(h, inst); err != nil {
				return fmt.Errorf("%s: %w", inst, err)
			}
			fmt.Fprintf(w, "  %s: autostart removed\n", inst)
		}
		return nil
	})
}

// ServiceStatus prints autostart installation state per runner instance.
func ServiceStatus(w io.Writer, cfg *config.Config, filterHost, filterRepo string, nameArgs []string) error {
	runners, err := resolveAndFilter(w, cfg, filterHost, filterRepo, nameArgs)
	if err != nil {
		return err
	}
	return runPerHostParallel(w, cfg, runners, func(w io.Writer, h *host.Host, rc config.RunnerConfig) error {
		for _, inst := range rc.InstanceNames() {
			row, err := autostart.Status(h, rc.Host, inst, "native")
			if err != nil {
				return fmt.Errorf("%s: %w", inst, err)
			}
			fmt.Fprintf(w, "%s @ %s [%s]: %s\n", inst, row.Host, row.Mode, row.Detail)
		}
		return nil
	})
}

// ServiceCleanup removes orphan runner services and directories on each host that are
// not listed in runners.yml (typical after rename or manual config edits).
func ServiceCleanup(w io.Writer, cfg *config.Config, filterHost string, dryRun bool) error {
	if err := ResolveHostInfo(w, cfg); err != nil {
		return err
	}
	names := sortedHostNames(cfg, filterHost)
	if len(names) == 0 {
		if filterHost != "" {
			return fmt.Errorf("unknown host %q", filterHost)
		}
		return fmt.Errorf("no hosts configured")
	}

	mgr := runner.NewManager("")
	mgr.Out = w

	var orphanCount, autostartCount, dirCount int
	for _, name := range names {
		hcfg := cfg.Hosts[name]
		if config.IsLocalAddr(hcfg.Addr) {
			fmt.Fprintf(w, "Checking orphan runners on %s (local)...\n", name)
		} else {
			fmt.Fprintf(w, "Checking orphan runners on %s (%s)...\n", name, hcfg.Addr)
		}
		h, err := connectHostFn(name, hcfg)
		if err != nil {
			return fmt.Errorf("%s: connect: %w", name, err)
		}

		configured := configuredInstanceSet(cfg, name)
		orphans, err := runner.OrphanInstances(h, configured)
		if err != nil {
			h.Close()
			return fmt.Errorf("%s: list orphans: %w", name, err)
		}

		for _, inst := range orphans {
			plan, err := mgr.CleanupOrphanInstance(h, inst, dryRun)
			if err != nil {
				h.Close()
				return fmt.Errorf("%s: %s: %w", name, inst, err)
			}
			if !plan.Autostart && !plan.Directory {
				continue
			}
			orphanCount++
			if plan.Autostart {
				autostartCount++
			}
			if plan.Directory {
				dirCount++
			}
		}
		h.Close()
	}

	if dryRun {
		fmt.Fprintf(w, "\nFound %d orphan instance(s) (%d autostart, %d directories); re-run without --dry-run to remove.\n",
			orphanCount, autostartCount, dirCount)
	} else if orphanCount > 0 {
		fmt.Fprintf(w, "\nCleaned up %d orphan instance(s) (%d autostart, %d directories).\n",
			orphanCount, autostartCount, dirCount)
	} else {
		fmt.Fprintln(w, "\nNo orphan runner services or directories found.")
	}
	return nil
}

func configuredInstanceSet(cfg *config.Config, hostName string) map[string]struct{} {
	return runner.ConfiguredInstanceSet(cfg.Runners, hostName)
}
